// Package store defines the persistence boundary every service depends on (D-53, D-66).
// Two implementations satisfy it: store/memory (Phase 1, Steps 2-10) and store/sqlite
// (Phase 2, Step 11). Handlers never call a Store directly; only internal/service does.
package store

import (
	"errors"

	"github.com/jdavidrt/theflock-twitter-clone/server/internal/domain"
)

// Sentinel errors every implementation returns so services can branch with errors.Is.
var (
	ErrNotFound   = errors.New("not found")
	ErrConflict   = errors.New("conflict")
	ErrSelfFollow = errors.New("cannot follow yourself")
)

// Pagination defaults shared by every cursor-paginated endpoint (D-19).
const (
	DefaultLimit = 20
	MaxLimit     = 50
)

// Page is a cursor-paginated slice of items. NextCursor is nil when the caller has reached
// the end of the list.
type Page[T any] struct {
	Items      []T
	NextCursor *Cursor
}

// Store is the persistence interface every service is built against (D-53). Implementations
// must exclude soft-deleted tweets (Tweet.DeletedAt set) from every list and read method,
// with one deliberate exception: Ancestors includes deleted tweets so the thread page can
// render the "this tweet was deleted" placeholder required by D-49 — every other method
// follows D-16's blanket rule, and the storetest conformance suite asserts both halves of
// that split. Follow/Like assume both ids refer to rows that already exist; callers
// (internal/service) are responsible for existence checks before calling them.
type Store interface {
	// Users
	CreateUser(u domain.User) (domain.User, error)
	GetUserByID(id string) (domain.User, error)
	GetUserByUsername(username string) (domain.User, error)
	GetUserByEmail(email string) (domain.User, error)
	UpdateUserProfile(userID, displayName, bio string) (domain.User, error)
	// SearchUsers matches query as a case-insensitive substring of username or displayName
	// (D-25), ordered by username ascending, capped at limit.
	SearchUsers(query string, limit int) ([]domain.User, error)

	// Follows (D-21). Follow and Unfollow are idempotent: creating an edge that already
	// exists, or removing one that does not, succeeds without error.
	Follow(f domain.Follow) error
	Unfollow(followerID, followeeID string) error
	IsFollowing(followerID, followeeID string) (bool, error)
	FollowerCount(userID string) (int, error)
	FollowingCount(userID string) (int, error)
	// ListFollowers and ListFollowing order by the follow edge's CreatedAt descending (D-24).
	ListFollowers(userID string, cursor *Cursor, limit int) (Page[domain.User], error)
	ListFollowing(userID string, cursor *Cursor, limit int) (Page[domain.User], error)

	// Tweets
	CreateTweet(t domain.Tweet) (domain.Tweet, error)
	// GetTweet returns ErrNotFound for a missing or soft-deleted tweet (D-16).
	GetTweet(id string) (domain.Tweet, error)
	// SoftDeleteTweet sets DeletedAt. It returns ErrNotFound if the tweet does not exist or
	// is already deleted — deletion is not idempotent past the first call (D-16).
	SoftDeleteTweet(id string) error
	// ListTweetsByAuthor returns the author's top-level (non-reply), non-deleted tweets,
	// newest first.
	ListTweetsByAuthor(authorID string, cursor *Cursor, limit int) (Page[domain.Tweet], error)
	// Timeline returns top-level, non-deleted tweets authored by viewerID or by users
	// viewerID follows, ordered createdAt DESC, id DESC (D-17/D-19).
	Timeline(viewerID string, cursor *Cursor, limit int) (Page[domain.Tweet], error)
	// ListReplies returns parentTweetID's direct, non-deleted replies, ordered createdAt ASC
	// (D-48 — conversation order).
	ListReplies(parentTweetID string, cursor *Cursor, limit int) (Page[domain.Tweet], error)
	// Ancestors walks up from tweetID's parent to the root, capped at maxHops, root-first.
	// See the Store doc comment above for why this one method does not exclude deleted rows.
	Ancestors(tweetID string, maxHops int) ([]domain.Tweet, error)
	// TweetCount is the author's live, top-level tweet count (D-23).
	TweetCount(authorID string) (int, error)
	// ReplyCount excludes deleted replies (D-49).
	ReplyCount(tweetID string) (int, error)

	// Likes (D-22). Like and Unlike are idempotent the same way Follow/Unfollow are.
	Like(l domain.Like) error
	Unlike(userID, tweetID string) error
	LikeCount(tweetID string) (int, error)
	LikedByMe(userID, tweetID string) (bool, error)
	LikedBy(tweetID string) ([]domain.User, error)
}
