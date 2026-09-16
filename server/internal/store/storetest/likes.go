package storetest

import (
	"testing"
	"time"
)

func testLikes(t *testing.T, newStore NewStoreFunc) {
	t.Run("LikeIsIdempotent", func(t *testing.T) {
		s := newStore(t)
		a := mustCreateUser(t, s, fixtureUser("alice", "alice@example.com"))
		b := mustCreateUser(t, s, fixtureUser("bob", "bob@example.com"))
		tw := mustCreateTweet(t, s, b.ID, "hello", nil, baseTime)

		mustLike(t, s, a.ID, tw.ID, baseTime)
		mustLike(t, s, a.ID, tw.ID, baseTime.Add(time.Hour)) // repeat: no error, no double-count

		count, err := s.LikeCount(tw.ID)
		requireNoError(t, err)
		if count != 1 {
			t.Fatalf("LikeCount after double-like: got %d, want 1", count)
		}
	})

	t.Run("UnlikeIsIdempotent", func(t *testing.T) {
		s := newStore(t)
		a := mustCreateUser(t, s, fixtureUser("alice", "alice@example.com"))
		b := mustCreateUser(t, s, fixtureUser("bob", "bob@example.com"))
		tw := mustCreateTweet(t, s, b.ID, "hello", nil, baseTime)

		requireNoError(t, s.Unlike(a.ID, tw.ID)) // never liked: still no error
		mustLike(t, s, a.ID, tw.ID, baseTime)
		requireNoError(t, s.Unlike(a.ID, tw.ID))
		requireNoError(t, s.Unlike(a.ID, tw.ID)) // repeat: still no error

		liked, err := s.LikedByMe(a.ID, tw.ID)
		requireNoError(t, err)
		if liked {
			t.Fatalf("LikedByMe after unlike: got true, want false")
		}
	})

	t.Run("OwnTweetCanBeLiked", func(t *testing.T) {
		s := newStore(t)
		a := mustCreateUser(t, s, fixtureUser("alice", "alice@example.com"))
		tw := mustCreateTweet(t, s, a.ID, "hello", nil, baseTime)

		mustLike(t, s, a.ID, tw.ID, baseTime) // D-22: liking your own tweet is allowed
		liked, err := s.LikedByMe(a.ID, tw.ID)
		requireNoError(t, err)
		if !liked {
			t.Fatalf("LikedByMe(own tweet): got false, want true")
		}
	})

	t.Run("LikedByOrderedByUsername", func(t *testing.T) {
		s := newStore(t)
		author := mustCreateUser(t, s, fixtureUser("author", "author@example.com"))
		carol := mustCreateUser(t, s, fixtureUser("carol", "carol@example.com"))
		alice := mustCreateUser(t, s, fixtureUser("alice", "alice@example.com"))
		tw := mustCreateTweet(t, s, author.ID, "hello", nil, baseTime)

		mustLike(t, s, carol.ID, tw.ID, baseTime)
		mustLike(t, s, alice.ID, tw.ID, baseTime.Add(time.Minute))

		likers, err := s.LikedBy(tw.ID)
		requireNoError(t, err)
		got := idsOf(likers, userID)
		want := []string{alice.ID, carol.ID}
		if !equalStrings(got, want) {
			t.Fatalf("LikedBy order: got %v, want %v (username ASC)", got, want)
		}
	})
}
