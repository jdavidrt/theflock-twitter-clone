// Package tweet implements tweet creation, deletion, the home timeline and likes (D-53):
// content validation (D-13/D-14), soft delete (D-16), timeline composition (D-17), and
// idempotent likes (D-22). httpapi handlers call into it and never touch the store directly.
package tweet

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/jdavidrt/theflock-twitter-clone/server/internal/domain"
	"github.com/jdavidrt/theflock-twitter-clone/server/internal/store"
	"github.com/jdavidrt/theflock-twitter-clone/server/internal/validation"
)

// ErrForbidden is returned by Delete when the caller is not the tweet's author (D-16).
var ErrForbidden = errors.New("only the author can delete this tweet")

// MaxAncestorHops caps how far GetThread walks up a reply chain (D-48 safety valve).
const MaxAncestorHops = 50

// FieldErrors maps a request field name to the problems found with it (D-52 `details`).
type FieldErrors map[string][]string

// ValidationError is returned by Create when content fails validation.
type ValidationError struct {
	Fields FieldErrors
}

func (e *ValidationError) Error() string { return "validation failed" }

// Service implements tweet, timeline and like operations against a Store.
type Service struct {
	store store.Store
}

// New builds a Service.
func New(st store.Store) *Service {
	return &Service{store: st}
}

// View is the computed, read-time representation of a tweet (D-23): the row plus its author
// and counts, and whether viewerID has liked it.
type View struct {
	Tweet      domain.Tweet
	Author     domain.User
	LikeCount  int
	ReplyCount int
	LikedByMe  bool
}

// Create validates content (D-13/D-14) and creates a new top-level tweet authored by authorID.
func (s *Service) Create(authorID, content string) (View, error) {
	normalized, errs := validation.TweetContent(content)
	if len(errs) > 0 {
		return View{}, &ValidationError{Fields: FieldErrors{"content": errs}}
	}

	created, err := s.store.CreateTweet(domain.Tweet{
		ID:        uuid.NewString(),
		AuthorID:  authorID,
		Content:   normalized,
		CreatedAt: time.Now().UTC(),
	})
	if err != nil {
		return View{}, err
	}
	return s.viewOf(authorID, created)
}

// Get returns tweetID's view from viewerID's perspective. It returns store.ErrNotFound for a
// missing or soft-deleted tweet (D-16).
func (s *Service) Get(viewerID, tweetID string) (View, error) {
	t, err := s.store.GetTweet(tweetID)
	if err != nil {
		return View{}, err
	}
	return s.viewOf(viewerID, t)
}

// CreateReply validates content (D-13/D-14) and creates a reply to parentTweetID authored by
// authorID (D-47). It returns store.ErrNotFound if the parent tweet is missing or deleted.
func (s *Service) CreateReply(authorID, parentTweetID, content string) (View, error) {
	normalized, errs := validation.TweetContent(content)
	if len(errs) > 0 {
		return View{}, &ValidationError{Fields: FieldErrors{"content": errs}}
	}
	if _, err := s.store.GetTweet(parentTweetID); err != nil {
		return View{}, err
	}

	created, err := s.store.CreateTweet(domain.Tweet{
		ID:            uuid.NewString(),
		AuthorID:      authorID,
		Content:       normalized,
		ParentTweetID: &parentTweetID,
		CreatedAt:     time.Now().UTC(),
	})
	if err != nil {
		return View{}, err
	}
	return s.viewOf(authorID, created)
}

// Thread is the D-48 thread-page composition: the ancestor chain (root-first; an ancestor may
// be a soft-deleted placeholder per D-49), the focused tweet, and its direct replies.
type Thread struct {
	Ancestors []View
	Tweet     View
	Replies   store.Page[View]
}

// GetThread returns tweetID's thread page from viewerID's perspective (D-48). It returns
// store.ErrNotFound if tweetID is missing or deleted — a deleted tweet cannot be the focus of
// a thread page (D-49).
func (s *Service) GetThread(viewerID, tweetID string, cursor *store.Cursor, limit int) (Thread, error) {
	t, err := s.store.GetTweet(tweetID)
	if err != nil {
		return Thread{}, err
	}
	focused, err := s.viewOf(viewerID, t)
	if err != nil {
		return Thread{}, err
	}

	ancestorTweets, err := s.store.Ancestors(tweetID, MaxAncestorHops)
	if err != nil {
		return Thread{}, err
	}
	ancestors := make([]View, 0, len(ancestorTweets))
	for _, at := range ancestorTweets {
		v, err := s.viewOf(viewerID, at)
		if err != nil {
			return Thread{}, err
		}
		ancestors = append(ancestors, v)
	}

	repliesPage, err := s.store.ListReplies(tweetID, cursor, limit)
	if err != nil {
		return Thread{}, err
	}
	replies, err := s.viewPage(viewerID, repliesPage)
	if err != nil {
		return Thread{}, err
	}

	return Thread{Ancestors: ancestors, Tweet: focused, Replies: replies}, nil
}

// Delete soft-deletes tweetID on behalf of viewerID (D-16). It returns store.ErrNotFound for a
// missing or already-deleted tweet, and ErrForbidden if viewerID is not the author.
func (s *Service) Delete(viewerID, tweetID string) error {
	t, err := s.store.GetTweet(tweetID)
	if err != nil {
		return err
	}
	if t.AuthorID != viewerID {
		return ErrForbidden
	}
	return s.store.SoftDeleteTweet(tweetID)
}

// ListByUsername returns username's top-level tweets, newest first, from viewerID's
// perspective. It returns store.ErrNotFound if username does not resolve.
func (s *Service) ListByUsername(viewerID, username string, cursor *store.Cursor, limit int) (store.Page[View], error) {
	u, err := s.store.GetUserByUsername(validation.NormalizeUsername(username))
	if err != nil {
		return store.Page[View]{}, err
	}
	page, err := s.store.ListTweetsByAuthor(u.ID, cursor, limit)
	if err != nil {
		return store.Page[View]{}, err
	}
	return s.viewPage(viewerID, page)
}

// Timeline returns viewerID's home timeline (D-17): top-level tweets from viewerID and the
// users viewerID follows, newest first.
func (s *Service) Timeline(viewerID string, cursor *store.Cursor, limit int) (store.Page[View], error) {
	page, err := s.store.Timeline(viewerID, cursor, limit)
	if err != nil {
		return store.Page[View]{}, err
	}
	return s.viewPage(viewerID, page)
}

// Like makes viewerID like tweetID (D-22): idempotent, liking your own tweet is allowed. It
// returns store.ErrNotFound for a missing or deleted tweet.
func (s *Service) Like(viewerID, tweetID string) (View, error) {
	t, err := s.store.GetTweet(tweetID)
	if err != nil {
		return View{}, err
	}
	if err := s.store.Like(domain.Like{UserID: viewerID, TweetID: tweetID, CreatedAt: time.Now().UTC()}); err != nil {
		return View{}, err
	}
	return s.viewOf(viewerID, t)
}

// Unlike removes viewerID's like from tweetID, if any (D-22). It returns store.ErrNotFound for
// a missing or deleted tweet; unliking a tweet you have not liked is a no-op.
func (s *Service) Unlike(viewerID, tweetID string) (View, error) {
	t, err := s.store.GetTweet(tweetID)
	if err != nil {
		return View{}, err
	}
	if err := s.store.Unlike(viewerID, tweetID); err != nil {
		return View{}, err
	}
	return s.viewOf(viewerID, t)
}

func (s *Service) viewOf(viewerID string, t domain.Tweet) (View, error) {
	author, err := s.store.GetUserByID(t.AuthorID)
	if err != nil {
		return View{}, fmt.Errorf("loading author for tweet %s: %w", t.ID, err)
	}
	likeCount, err := s.store.LikeCount(t.ID)
	if err != nil {
		return View{}, fmt.Errorf("counting likes for tweet %s: %w", t.ID, err)
	}
	replyCount, err := s.store.ReplyCount(t.ID)
	if err != nil {
		return View{}, fmt.Errorf("counting replies for tweet %s: %w", t.ID, err)
	}
	likedByMe, err := s.store.LikedByMe(viewerID, t.ID)
	if err != nil {
		return View{}, fmt.Errorf("checking like state for tweet %s: %w", t.ID, err)
	}
	return View{Tweet: t, Author: author, LikeCount: likeCount, ReplyCount: replyCount, LikedByMe: likedByMe}, nil
}

func (s *Service) viewPage(viewerID string, page store.Page[domain.Tweet]) (store.Page[View], error) {
	items := make([]View, 0, len(page.Items))
	for _, t := range page.Items {
		v, err := s.viewOf(viewerID, t)
		if err != nil {
			return store.Page[View]{}, err
		}
		items = append(items, v)
	}
	return store.Page[View]{Items: items, NextCursor: page.NextCursor}, nil
}
