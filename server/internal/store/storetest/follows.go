package storetest

import (
	"errors"
	"testing"
	"time"

	"github.com/jdavidrt/theflock-twitter-clone/server/internal/domain"
	"github.com/jdavidrt/theflock-twitter-clone/server/internal/store"
)

func testFollows(t *testing.T, newStore NewStoreFunc) {
	t.Run("FollowIsIdempotent", func(t *testing.T) {
		s := newStore(t)
		a := mustCreateUser(t, s, fixtureUser("alice", "alice@example.com"))
		b := mustCreateUser(t, s, fixtureUser("bob", "bob@example.com"))

		mustFollow(t, s, a.ID, b.ID, baseTime)
		mustFollow(t, s, a.ID, b.ID, baseTime.Add(time.Hour)) // repeat: no error, no new edge

		following, err := s.IsFollowing(a.ID, b.ID)
		requireNoError(t, err)
		if !following {
			t.Fatalf("IsFollowing: got false, want true")
		}
		count, err := s.FollowingCount(a.ID)
		requireNoError(t, err)
		if count != 1 {
			t.Fatalf("FollowingCount after double-follow: got %d, want 1", count)
		}
	})

	t.Run("SelfFollowRejected", func(t *testing.T) {
		s := newStore(t)
		a := mustCreateUser(t, s, fixtureUser("alice", "alice@example.com"))
		err := s.Follow(domain.Follow{FollowerID: a.ID, FolloweeID: a.ID, CreatedAt: baseTime})
		if !errors.Is(err, store.ErrSelfFollow) {
			t.Fatalf("Follow(self): got %v, want ErrSelfFollow", err)
		}
	})

	t.Run("UnfollowIsIdempotent", func(t *testing.T) {
		s := newStore(t)
		a := mustCreateUser(t, s, fixtureUser("alice", "alice@example.com"))
		b := mustCreateUser(t, s, fixtureUser("bob", "bob@example.com"))

		requireNoError(t, s.Unfollow(a.ID, b.ID)) // never followed: still no error
		mustFollow(t, s, a.ID, b.ID, baseTime)
		requireNoError(t, s.Unfollow(a.ID, b.ID))
		requireNoError(t, s.Unfollow(a.ID, b.ID)) // repeat: still no error

		following, err := s.IsFollowing(a.ID, b.ID)
		requireNoError(t, err)
		if following {
			t.Fatalf("IsFollowing after unfollow: got true, want false")
		}
	})

	t.Run("FollowerAndFollowingCounts", func(t *testing.T) {
		s := newStore(t)
		a := mustCreateUser(t, s, fixtureUser("alice", "alice@example.com"))
		b := mustCreateUser(t, s, fixtureUser("bob", "bob@example.com"))
		c := mustCreateUser(t, s, fixtureUser("carol", "carol@example.com"))

		mustFollow(t, s, b.ID, a.ID, baseTime)
		mustFollow(t, s, c.ID, a.ID, baseTime.Add(time.Hour))

		followers, err := s.FollowerCount(a.ID)
		requireNoError(t, err)
		if followers != 2 {
			t.Fatalf("FollowerCount(alice): got %d, want 2", followers)
		}
		following, err := s.FollowingCount(b.ID)
		requireNoError(t, err)
		if following != 1 {
			t.Fatalf("FollowingCount(bob): got %d, want 1", following)
		}
	})

	t.Run("ListFollowersOrderedNewestFirst", func(t *testing.T) {
		s := newStore(t)
		a := mustCreateUser(t, s, fixtureUser("alice", "alice@example.com"))
		b := mustCreateUser(t, s, fixtureUser("bob", "bob@example.com"))
		c := mustCreateUser(t, s, fixtureUser("carol", "carol@example.com"))
		d := mustCreateUser(t, s, fixtureUser("dave", "dave@example.com"))

		mustFollow(t, s, b.ID, a.ID, baseTime)
		mustFollow(t, s, c.ID, a.ID, baseTime.Add(time.Hour))
		mustFollow(t, s, d.ID, a.ID, baseTime.Add(2*time.Hour))

		page, err := s.ListFollowers(a.ID, nil, 20)
		requireNoError(t, err)
		got := idsOf(page.Items, userID)
		want := []string{d.ID, c.ID, b.ID}
		if !equalStrings(got, want) {
			t.Fatalf("ListFollowers order: got %v, want %v (newest follow first)", got, want)
		}
	})

	t.Run("ListFollowingOrderedNewestFirst", func(t *testing.T) {
		s := newStore(t)
		a := mustCreateUser(t, s, fixtureUser("alice", "alice@example.com"))
		b := mustCreateUser(t, s, fixtureUser("bob", "bob@example.com"))
		c := mustCreateUser(t, s, fixtureUser("carol", "carol@example.com"))

		mustFollow(t, s, a.ID, b.ID, baseTime)
		mustFollow(t, s, a.ID, c.ID, baseTime.Add(time.Hour))

		page, err := s.ListFollowing(a.ID, nil, 20)
		requireNoError(t, err)
		got := idsOf(page.Items, userID)
		want := []string{c.ID, b.ID}
		if !equalStrings(got, want) {
			t.Fatalf("ListFollowing order: got %v, want %v (newest follow first)", got, want)
		}
	})
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
