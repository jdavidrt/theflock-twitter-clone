package storetest

import (
	"testing"
	"time"

	"github.com/jdavidrt/theflock-twitter-clone/server/internal/store"
)

// testPagination asserts D-19's core promise: walking every page with the returned cursor
// visits every item exactly once, in order, with no duplicates and no gaps.
func testPagination(t *testing.T, newStore NewStoreFunc) {
	t.Run("TimelineHasNoDuplicatesOrGapsAcrossPages", func(t *testing.T) {
		s := newStore(t)
		a := mustCreateUser(t, s, fixtureUser("alice", "alice@example.com"))

		const n = 45
		want := make([]string, n)
		for i := 0; i < n; i++ {
			tw := mustCreateTweet(t, s, a.ID, "tweet", nil, baseTime.Add(time.Duration(i)*time.Minute))
			want[n-1-i] = tw.ID // Timeline is newest-first; tweets were created oldest-first.
		}

		var got []string
		var cursor *store.Cursor
		for pages := 0; ; pages++ {
			if pages > n {
				t.Fatalf("Timeline pagination did not terminate after %d pages", pages)
			}
			page, err := s.Timeline(a.ID, cursor, 10)
			requireNoError(t, err)
			got = append(got, idsOf(page.Items, tweetID)...)
			if page.NextCursor == nil {
				break
			}
			cursor = page.NextCursor
		}
		if !equalStrings(got, want) {
			t.Fatalf("Timeline pagination mismatch:\n got  %v\n want %v", got, want)
		}
	})

	t.Run("ListRepliesHasNoDuplicatesOrGapsAcrossPages", func(t *testing.T) {
		s := newStore(t)
		a := mustCreateUser(t, s, fixtureUser("alice", "alice@example.com"))
		top := mustCreateTweet(t, s, a.ID, "top", nil, baseTime)

		const n = 37
		want := make([]string, n)
		for i := 0; i < n; i++ {
			r := mustCreateTweet(t, s, a.ID, "reply", &top.ID, baseTime.Add(time.Duration(i+1)*time.Minute))
			want[i] = r.ID // ListReplies is oldest-first, same order as creation.
		}

		var got []string
		var cursor *store.Cursor
		for pages := 0; ; pages++ {
			if pages > n {
				t.Fatalf("ListReplies pagination did not terminate after %d pages", pages)
			}
			page, err := s.ListReplies(top.ID, cursor, 10)
			requireNoError(t, err)
			got = append(got, idsOf(page.Items, tweetID)...)
			if page.NextCursor == nil {
				break
			}
			cursor = page.NextCursor
		}
		if !equalStrings(got, want) {
			t.Fatalf("ListReplies pagination mismatch:\n got  %v\n want %v", got, want)
		}
	})
}
