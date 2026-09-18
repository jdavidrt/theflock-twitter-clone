// Package social implements profile reads/updates and the follow graph (D-53): user profiles
// with counts computed on read (D-23), idempotent follow/unfollow (D-21), and cursor-paginated
// followers/following lists (D-19/D-24). httpapi handlers call into it and never touch the
// store directly.
package social

import (
	"errors"
	"fmt"
	"time"

	"github.com/jdavidrt/theflock-twitter-clone/server/internal/domain"
	"github.com/jdavidrt/theflock-twitter-clone/server/internal/store"
	"github.com/jdavidrt/theflock-twitter-clone/server/internal/validation"
)

// ErrSelfFollow is returned by Follow when the viewer targets their own username (D-21). The
// store also rejects a self-follow edge, but the service checks first so the caller gets a
// clean 400 without depending on the store's guarantee.
var ErrSelfFollow = errors.New("cannot follow yourself")

// FieldErrors maps a request field name to the problems found with it (D-52 `details`).
type FieldErrors map[string][]string

// ValidationError is returned by UpdateProfile when the input fails field validation.
type ValidationError struct {
	Fields FieldErrors
}

func (e *ValidationError) Error() string { return "validation failed" }

// SearchResultLimit caps GET /api/search/users results (D-25). Search is not paginated.
const SearchResultLimit = 20

// Service implements profile and follow-graph operations against a Store.
type Service struct {
	store store.Store
}

// New builds a Service.
func New(st store.Store) *Service {
	return &Service{store: st}
}

// Profile is the computed, read-time view of a user's public profile (D-23): raw fields plus
// counts and, from the viewer's perspective, whether they already follow this user.
type Profile struct {
	User           domain.User
	TweetCount     int
	FollowerCount  int
	FollowingCount int
	IsFollowedByMe bool
}

// GetProfile resolves username and computes its profile view for viewerID. It returns
// store.ErrNotFound when username does not resolve to a user.
func (s *Service) GetProfile(viewerID, username string) (Profile, error) {
	u, err := s.store.GetUserByUsername(validation.NormalizeUsername(username))
	if err != nil {
		return Profile{}, err
	}
	return s.profileOf(viewerID, u)
}

func (s *Service) profileOf(viewerID string, u domain.User) (Profile, error) {
	tweetCount, err := s.store.TweetCount(u.ID)
	if err != nil {
		return Profile{}, fmt.Errorf("counting tweets for %s: %w", u.Username, err)
	}
	followerCount, err := s.store.FollowerCount(u.ID)
	if err != nil {
		return Profile{}, fmt.Errorf("counting followers for %s: %w", u.Username, err)
	}
	followingCount, err := s.store.FollowingCount(u.ID)
	if err != nil {
		return Profile{}, fmt.Errorf("counting following for %s: %w", u.Username, err)
	}
	isFollowedByMe, err := s.store.IsFollowing(viewerID, u.ID)
	if err != nil {
		return Profile{}, fmt.Errorf("checking follow state for %s: %w", u.Username, err)
	}
	return Profile{
		User:           u,
		TweetCount:     tweetCount,
		FollowerCount:  followerCount,
		FollowingCount: followingCount,
		IsFollowedByMe: isFollowedByMe,
	}, nil
}

// UpdateProfile validates and applies displayName/bio changes to userID's own profile (D-12).
func (s *Service) UpdateProfile(userID, displayName, bio string) (domain.User, error) {
	fields := FieldErrors{}

	name, errs := validation.DisplayName(displayName)
	if len(errs) > 0 {
		fields["displayName"] = errs
	}
	normalizedBio, errs := validation.Bio(bio)
	if len(errs) > 0 {
		fields["bio"] = errs
	}
	if len(fields) > 0 {
		return domain.User{}, &ValidationError{Fields: fields}
	}

	return s.store.UpdateUserProfile(userID, name, normalizedBio)
}

// Follow makes viewerID follow the user named username (D-21). It is idempotent: following
// twice succeeds without creating a second edge. It returns store.ErrNotFound if username does
// not resolve, and ErrSelfFollow if it resolves to viewerID.
func (s *Service) Follow(viewerID, username string) (Profile, error) {
	target, err := s.store.GetUserByUsername(validation.NormalizeUsername(username))
	if err != nil {
		return Profile{}, err
	}
	if target.ID == viewerID {
		return Profile{}, ErrSelfFollow
	}
	if err := s.store.Follow(domain.Follow{FollowerID: viewerID, FolloweeID: target.ID, CreatedAt: time.Now().UTC()}); err != nil {
		return Profile{}, err
	}
	return s.profileOf(viewerID, target)
}

// Unfollow removes the edge from viewerID to username, if any (D-21). Unfollowing a user
// viewerID does not follow is a no-op, not an error.
func (s *Service) Unfollow(viewerID, username string) (Profile, error) {
	target, err := s.store.GetUserByUsername(validation.NormalizeUsername(username))
	if err != nil {
		return Profile{}, err
	}
	if err := s.store.Unfollow(viewerID, target.ID); err != nil {
		return Profile{}, err
	}
	return s.profileOf(viewerID, target)
}

// FollowListItem is one row of a followers/following list (D-24): the listed user plus whether
// the viewer already follows them, so the client can render the right follow-button state.
type FollowListItem struct {
	User           domain.User
	IsFollowedByMe bool
}

// ListFollowers returns username's followers, newest-first (D-24), from viewerID's perspective.
func (s *Service) ListFollowers(viewerID, username string, cursor *store.Cursor, limit int) (store.Page[FollowListItem], error) {
	target, err := s.store.GetUserByUsername(validation.NormalizeUsername(username))
	if err != nil {
		return store.Page[FollowListItem]{}, err
	}
	page, err := s.store.ListFollowers(target.ID, cursor, limit)
	if err != nil {
		return store.Page[FollowListItem]{}, err
	}
	return s.toFollowListPage(viewerID, page)
}

// ListFollowing returns who username follows, newest-first (D-24), from viewerID's perspective.
func (s *Service) ListFollowing(viewerID, username string, cursor *store.Cursor, limit int) (store.Page[FollowListItem], error) {
	target, err := s.store.GetUserByUsername(validation.NormalizeUsername(username))
	if err != nil {
		return store.Page[FollowListItem]{}, err
	}
	page, err := s.store.ListFollowing(target.ID, cursor, limit)
	if err != nil {
		return store.Page[FollowListItem]{}, err
	}
	return s.toFollowListPage(viewerID, page)
}

// SearchUsers matches query as a case-insensitive substring of username or displayName (D-25):
// trimmed, 1-50 chars, results capped at SearchResultLimit and ordered by username ascending.
// The current user is not excluded from results.
func (s *Service) SearchUsers(query string) ([]domain.User, error) {
	normalized, errs := validation.SearchQuery(query)
	if len(errs) > 0 {
		return nil, &ValidationError{Fields: FieldErrors{"q": errs}}
	}
	return s.store.SearchUsers(normalized, SearchResultLimit)
}

func (s *Service) toFollowListPage(viewerID string, page store.Page[domain.User]) (store.Page[FollowListItem], error) {
	items := make([]FollowListItem, 0, len(page.Items))
	for _, u := range page.Items {
		isFollowedByMe, err := s.store.IsFollowing(viewerID, u.ID)
		if err != nil {
			return store.Page[FollowListItem]{}, fmt.Errorf("checking follow state for %s: %w", u.Username, err)
		}
		items = append(items, FollowListItem{User: u, IsFollowedByMe: isFollowedByMe})
	}
	return store.Page[FollowListItem]{Items: items, NextCursor: page.NextCursor}, nil
}
