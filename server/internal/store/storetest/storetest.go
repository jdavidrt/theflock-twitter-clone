// Package storetest is the conformance suite every store.Store implementation must pass
// (D-66). store/memory runs it today; store/sqlite runs the identical suite from Step 11.
package storetest

import (
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jdavidrt/theflock-twitter-clone/server/internal/domain"
	"github.com/jdavidrt/theflock-twitter-clone/server/internal/store"
)

// NewStoreFunc builds a fresh, empty store for one subtest. Implementations must return an
// independent instance every call so subtests can run in parallel.
type NewStoreFunc func(t *testing.T) store.Store

// Run executes the full conformance suite against newStore.
func Run(t *testing.T, newStore NewStoreFunc) {
	t.Run("Users", func(t *testing.T) { testUsers(t, newStore) })
	t.Run("Follows", func(t *testing.T) { testFollows(t, newStore) })
	t.Run("Tweets", func(t *testing.T) { testTweets(t, newStore) })
	t.Run("Likes", func(t *testing.T) { testLikes(t, newStore) })
	t.Run("Pagination", func(t *testing.T) { testPagination(t, newStore) })
}

var baseTime = time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)

var idSeq int64

func nextID(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, atomic.AddInt64(&idSeq, 1))
}

func fixtureUser(username, email string) domain.User {
	return fixtureUserFull(username, email, username, baseTime)
}

func fixtureUserFull(username, email, displayName string, createdAt time.Time) domain.User {
	return domain.User{
		ID:           nextID("user"),
		Username:     username,
		Email:        email,
		DisplayName:  displayName,
		PasswordHash: "hashed-password",
		CreatedAt:    createdAt,
		UpdatedAt:    createdAt,
	}
}

func mustCreateUser(t *testing.T, s store.Store, u domain.User) domain.User {
	t.Helper()
	created, err := s.CreateUser(u)
	if err != nil {
		t.Fatalf("CreateUser(%s): %v", u.Username, err)
	}
	return created
}

func mustCreateTweet(t *testing.T, s store.Store, authorID, content string, parentID *string, createdAt time.Time) domain.Tweet {
	t.Helper()
	created, err := s.CreateTweet(domain.Tweet{
		ID:            nextID("tweet"),
		AuthorID:      authorID,
		Content:       content,
		ParentTweetID: parentID,
		CreatedAt:     createdAt,
	})
	if err != nil {
		t.Fatalf("CreateTweet: %v", err)
	}
	return created
}

func mustFollow(t *testing.T, s store.Store, followerID, followeeID string, createdAt time.Time) {
	t.Helper()
	if err := s.Follow(domain.Follow{FollowerID: followerID, FolloweeID: followeeID, CreatedAt: createdAt}); err != nil {
		t.Fatalf("Follow(%s -> %s): %v", followerID, followeeID, err)
	}
}

func mustLike(t *testing.T, s store.Store, userID, tweetID string, createdAt time.Time) {
	t.Helper()
	if err := s.Like(domain.Like{UserID: userID, TweetID: tweetID, CreatedAt: createdAt}); err != nil {
		t.Fatalf("Like(%s -> %s): %v", userID, tweetID, err)
	}
}

func requireNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func idsOf[T any](items []T, id func(T) string) []string {
	out := make([]string, len(items))
	for i, item := range items {
		out[i] = id(item)
	}
	return out
}

func tweetID(t domain.Tweet) string { return t.ID }
func userID(u domain.User) string   { return u.ID }
