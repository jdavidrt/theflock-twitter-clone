package storetest

import (
	"errors"
	"testing"
	"time"

	"github.com/jdavidrt/theflock-twitter-clone/server/internal/store"
)

func testTweets(t *testing.T, newStore NewStoreFunc) {
	t.Run("CreateAndGet", func(t *testing.T) {
		s := newStore(t)
		a := mustCreateUser(t, s, fixtureUser("alice", "alice@example.com"))
		tw := mustCreateTweet(t, s, a.ID, "hello world", nil, baseTime)

		got, err := s.GetTweet(tw.ID)
		requireNoError(t, err)
		if got.Content != "hello world" || got.AuthorID != a.ID {
			t.Fatalf("GetTweet: got %+v", got)
		}
	})

	t.Run("GetMissingReturnsNotFound", func(t *testing.T) {
		s := newStore(t)
		if _, err := s.GetTweet("nope"); !errors.Is(err, store.ErrNotFound) {
			t.Fatalf("GetTweet(missing): got %v, want ErrNotFound", err)
		}
	})

	t.Run("SoftDeleteExcludesFromGetAndLists", func(t *testing.T) {
		s := newStore(t)
		a := mustCreateUser(t, s, fixtureUser("alice", "alice@example.com"))
		tw := mustCreateTweet(t, s, a.ID, "hello world", nil, baseTime)

		requireNoError(t, s.SoftDeleteTweet(tw.ID))

		if _, err := s.GetTweet(tw.ID); !errors.Is(err, store.ErrNotFound) {
			t.Fatalf("GetTweet(deleted): got %v, want ErrNotFound (D-16)", err)
		}
		authorPage, err := s.ListTweetsByAuthor(a.ID, nil, 20)
		requireNoError(t, err)
		if len(authorPage.Items) != 0 {
			t.Fatalf("ListTweetsByAuthor still includes a deleted tweet: %+v", authorPage.Items)
		}
		timelinePage, err := s.Timeline(a.ID, nil, 20)
		requireNoError(t, err)
		if len(timelinePage.Items) != 0 {
			t.Fatalf("Timeline still includes a deleted tweet: %+v", timelinePage.Items)
		}
	})

	t.Run("SoftDeleteIsNotIdempotent", func(t *testing.T) {
		s := newStore(t)
		a := mustCreateUser(t, s, fixtureUser("alice", "alice@example.com"))
		tw := mustCreateTweet(t, s, a.ID, "hello world", nil, baseTime)
		requireNoError(t, s.SoftDeleteTweet(tw.ID))
		if err := s.SoftDeleteTweet(tw.ID); !errors.Is(err, store.ErrNotFound) {
			t.Fatalf("SoftDeleteTweet(already deleted): got %v, want ErrNotFound (D-16)", err)
		}
	})

	t.Run("SoftDeleteMissingReturnsNotFound", func(t *testing.T) {
		s := newStore(t)
		if err := s.SoftDeleteTweet("nope"); !errors.Is(err, store.ErrNotFound) {
			t.Fatalf("SoftDeleteTweet(missing): got %v, want ErrNotFound", err)
		}
	})

	t.Run("ListTweetsByAuthorExcludesReplies", func(t *testing.T) {
		s := newStore(t)
		a := mustCreateUser(t, s, fixtureUser("alice", "alice@example.com"))
		top := mustCreateTweet(t, s, a.ID, "top", nil, baseTime)
		mustCreateTweet(t, s, a.ID, "reply", &top.ID, baseTime.Add(time.Minute))

		page, err := s.ListTweetsByAuthor(a.ID, nil, 20)
		requireNoError(t, err)
		if len(page.Items) != 1 || page.Items[0].ID != top.ID {
			t.Fatalf("ListTweetsByAuthor: got %+v, want only the top-level tweet", page.Items)
		}
	})

	t.Run("TimelineIncludesSelfAndFolloweesExcludesOthers", func(t *testing.T) {
		s := newStore(t)
		alice := mustCreateUser(t, s, fixtureUser("alice", "alice@example.com"))
		bob := mustCreateUser(t, s, fixtureUser("bob", "bob@example.com"))
		carol := mustCreateUser(t, s, fixtureUser("carol", "carol@example.com"))
		mustFollow(t, s, alice.ID, bob.ID, baseTime)

		mine := mustCreateTweet(t, s, alice.ID, "mine", nil, baseTime.Add(time.Minute))
		bobs := mustCreateTweet(t, s, bob.ID, "bobs", nil, baseTime.Add(2*time.Minute))
		mustCreateTweet(t, s, carol.ID, "carols", nil, baseTime.Add(3*time.Minute)) // not followed

		page, err := s.Timeline(alice.ID, nil, 20)
		requireNoError(t, err)
		got := idsOf(page.Items, tweetID)
		want := []string{bobs.ID, mine.ID} // newest first
		if !equalStrings(got, want) {
			t.Fatalf("Timeline: got %v, want %v", got, want)
		}
	})

	t.Run("TimelineExcludesReplies", func(t *testing.T) {
		s := newStore(t)
		a := mustCreateUser(t, s, fixtureUser("alice", "alice@example.com"))
		top := mustCreateTweet(t, s, a.ID, "top", nil, baseTime)
		mustCreateTweet(t, s, a.ID, "reply", &top.ID, baseTime.Add(time.Minute))

		page, err := s.Timeline(a.ID, nil, 20)
		requireNoError(t, err)
		if len(page.Items) != 1 || page.Items[0].ID != top.ID {
			t.Fatalf("Timeline: got %+v, want only the top-level tweet", page.Items)
		}
	})

	t.Run("ListRepliesOrderedOldestFirst", func(t *testing.T) {
		s := newStore(t)
		a := mustCreateUser(t, s, fixtureUser("alice", "alice@example.com"))
		top := mustCreateTweet(t, s, a.ID, "top", nil, baseTime)
		r1 := mustCreateTweet(t, s, a.ID, "r1", &top.ID, baseTime.Add(time.Minute))
		r2 := mustCreateTweet(t, s, a.ID, "r2", &top.ID, baseTime.Add(2*time.Minute))

		page, err := s.ListReplies(top.ID, nil, 20)
		requireNoError(t, err)
		got := idsOf(page.Items, tweetID)
		want := []string{r1.ID, r2.ID} // oldest first (D-48 conversation order)
		if !equalStrings(got, want) {
			t.Fatalf("ListReplies order: got %v, want %v", got, want)
		}
	})

	t.Run("ListRepliesExcludesDeleted", func(t *testing.T) {
		s := newStore(t)
		a := mustCreateUser(t, s, fixtureUser("alice", "alice@example.com"))
		top := mustCreateTweet(t, s, a.ID, "top", nil, baseTime)
		reply := mustCreateTweet(t, s, a.ID, "reply", &top.ID, baseTime.Add(time.Minute))
		requireNoError(t, s.SoftDeleteTweet(reply.ID))

		page, err := s.ListReplies(top.ID, nil, 20)
		requireNoError(t, err)
		if len(page.Items) != 0 {
			t.Fatalf("ListReplies still includes a deleted reply: %+v", page.Items)
		}
		count, err := s.ReplyCount(top.ID)
		requireNoError(t, err)
		if count != 0 {
			t.Fatalf("ReplyCount: got %d, want 0 (D-49 excludes deleted replies)", count)
		}
	})

	t.Run("AncestorsWalkToRootRootFirstIncludingDeleted", func(t *testing.T) {
		s := newStore(t)
		a := mustCreateUser(t, s, fixtureUser("alice", "alice@example.com"))
		root := mustCreateTweet(t, s, a.ID, "root", nil, baseTime)
		mid := mustCreateTweet(t, s, a.ID, "mid", &root.ID, baseTime.Add(time.Minute))
		leaf := mustCreateTweet(t, s, a.ID, "leaf", &mid.ID, baseTime.Add(2*time.Minute))
		requireNoError(t, s.SoftDeleteTweet(mid.ID))

		ancestors, err := s.Ancestors(leaf.ID, 50)
		requireNoError(t, err)
		got := idsOf(ancestors, tweetID)
		want := []string{root.ID, mid.ID}
		if !equalStrings(got, want) {
			t.Fatalf("Ancestors: got %v, want %v (root-first)", got, want)
		}
		if !ancestors[1].IsDeleted() {
			t.Fatalf("Ancestors: deleted ancestor %q lost its DeletedAt (needed for the D-49 placeholder)", mid.ID)
		}
	})

	t.Run("AncestorsCappedAtMaxHops", func(t *testing.T) {
		s := newStore(t)
		a := mustCreateUser(t, s, fixtureUser("alice", "alice@example.com"))
		root := mustCreateTweet(t, s, a.ID, "0", nil, baseTime)
		prev := root
		for i := 1; i <= 4; i++ {
			prev = mustCreateTweet(t, s, a.ID, "n", &prev.ID, baseTime.Add(time.Duration(i)*time.Minute))
		}

		ancestors, err := s.Ancestors(prev.ID, 2)
		requireNoError(t, err)
		if len(ancestors) != 2 {
			t.Fatalf("Ancestors with maxHops=2: got %d entries, want 2", len(ancestors))
		}
	})

	t.Run("TweetCountExcludesRepliesAndDeleted", func(t *testing.T) {
		s := newStore(t)
		a := mustCreateUser(t, s, fixtureUser("alice", "alice@example.com"))
		top1 := mustCreateTweet(t, s, a.ID, "top1", nil, baseTime)
		mustCreateTweet(t, s, a.ID, "top2", nil, baseTime.Add(time.Minute))
		mustCreateTweet(t, s, a.ID, "reply", &top1.ID, baseTime.Add(2*time.Minute))
		requireNoError(t, s.SoftDeleteTweet(top1.ID))

		count, err := s.TweetCount(a.ID)
		requireNoError(t, err)
		if count != 1 {
			t.Fatalf("TweetCount: got %d, want 1 (excludes the reply and the deleted tweet)", count)
		}
	})
}
