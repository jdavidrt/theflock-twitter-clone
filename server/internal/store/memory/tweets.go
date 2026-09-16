package memory

import (
	"time"

	"github.com/jdavidrt/theflock-twitter-clone/server/internal/domain"
	"github.com/jdavidrt/theflock-twitter-clone/server/internal/store"
)

func tweetKey(t domain.Tweet) (time.Time, string) { return t.CreatedAt, t.ID }

func (s *Store) CreateTweet(t domain.Tweet) (domain.Tweet, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tweets[t.ID] = t
	return t, nil
}

// GetTweet excludes soft-deleted tweets (D-16): a deleted tweet reads as not found.
func (s *Store) GetTweet(id string) (domain.Tweet, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.tweets[id]
	if !ok || t.IsDeleted() {
		return domain.Tweet{}, store.ErrNotFound
	}
	return t, nil
}

func (s *Store) SoftDeleteTweet(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.tweets[id]
	if !ok || t.IsDeleted() {
		return store.ErrNotFound
	}
	deletedAt := time.Now().UTC()
	t.DeletedAt = &deletedAt
	s.tweets[id] = t
	return nil
}

func (s *Store) ListTweetsByAuthor(authorID string, cursor *store.Cursor, limit int) (store.Page[domain.Tweet], error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var rows []domain.Tweet
	for _, t := range s.tweets {
		if t.AuthorID == authorID && !t.IsReply() && !t.IsDeleted() {
			rows = append(rows, t)
		}
	}
	sortDescBy(rows, tweetKey)
	page, next := paginateDesc(rows, cursor, limit, tweetKey)
	return store.Page[domain.Tweet]{Items: page, NextCursor: next}, nil
}

// Timeline returns top-level, non-deleted tweets from viewerID and everyone viewerID
// follows, newest first (D-17).
func (s *Store) Timeline(viewerID string, cursor *store.Cursor, limit int) (store.Page[domain.Tweet], error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	visible := map[string]bool{viewerID: true}
	for _, f := range s.follows {
		if f.FollowerID == viewerID {
			visible[f.FolloweeID] = true
		}
	}

	var rows []domain.Tweet
	for _, t := range s.tweets {
		if !t.IsReply() && !t.IsDeleted() && visible[t.AuthorID] {
			rows = append(rows, t)
		}
	}
	sortDescBy(rows, tweetKey)
	page, next := paginateDesc(rows, cursor, limit, tweetKey)
	return store.Page[domain.Tweet]{Items: page, NextCursor: next}, nil
}

// ListReplies returns parentTweetID's direct, non-deleted replies in conversation order (D-48).
func (s *Store) ListReplies(parentTweetID string, cursor *store.Cursor, limit int) (store.Page[domain.Tweet], error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var rows []domain.Tweet
	for _, t := range s.tweets {
		if t.ParentTweetID != nil && *t.ParentTweetID == parentTweetID && !t.IsDeleted() {
			rows = append(rows, t)
		}
	}
	sortAscBy(rows, tweetKey)
	page, next := paginateAsc(rows, cursor, limit, tweetKey)
	return store.Page[domain.Tweet]{Items: page, NextCursor: next}, nil
}

// Ancestors walks up from tweetID's parent to the root, root-first, capped at maxHops.
// Unlike every other read method it does not exclude soft-deleted tweets — see the Store
// doc comment (D-49 needs the deleted-ancestor placeholder data).
func (s *Store) Ancestors(tweetID string, maxHops int) ([]domain.Tweet, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	current, ok := s.tweets[tweetID]
	if !ok {
		return nil, nil
	}
	var chain []domain.Tweet
	for hops := 0; current.ParentTweetID != nil && hops < maxHops; hops++ {
		parent, ok := s.tweets[*current.ParentTweetID]
		if !ok {
			break
		}
		chain = append(chain, parent)
		current = parent
	}
	for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
		chain[i], chain[j] = chain[j], chain[i]
	}
	return chain, nil
}

func (s *Store) TweetCount(authorID string) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	n := 0
	for _, t := range s.tweets {
		if t.AuthorID == authorID && !t.IsReply() && !t.IsDeleted() {
			n++
		}
	}
	return n, nil
}

// ReplyCount excludes deleted replies (D-49).
func (s *Store) ReplyCount(tweetID string) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	n := 0
	for _, t := range s.tweets {
		if t.ParentTweetID != nil && *t.ParentTweetID == tweetID && !t.IsDeleted() {
			n++
		}
	}
	return n, nil
}
