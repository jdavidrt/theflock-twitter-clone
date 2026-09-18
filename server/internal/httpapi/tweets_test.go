package httpapi

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jdavidrt/theflock-twitter-clone/server/internal/store/memory"
)

// explodingLikeCountStore wraps a real memory store but fails LikeCount with an error that is
// neither ErrNotFound nor a sentinel tweet.Service knows about, so tests can exercise the
// "unexpected store error" branch (a 500, not a 404) in tweet.Service.viewOf and
// writeTweetError, mirroring auth_test.go's explodingEmailStore.
type explodingLikeCountStore struct {
	*memory.Store
}

func (s *explodingLikeCountStore) LikeCount(string) (int, error) {
	return 0, errors.New("boom: database unavailable")
}

// doAction issues a state-changing request with an optional JSON body and session cookie,
// reused across create/delete/like/unlike tests.
func doAction(t *testing.T, h http.Handler, method, path string, body any, cookie *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	req := jsonRequest(t, method, path, body)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	res := httptest.NewRecorder()
	h.ServeHTTP(res, req)
	return res
}

func createTweet(t *testing.T, h http.Handler, content string, cookie *http.Cookie) tweetResponse {
	t.Helper()
	res := doAction(t, h, http.MethodPost, "/api/tweets", map[string]string{"content": content}, cookie)
	if res.Code != http.StatusCreated {
		t.Fatalf("create tweet status = %d, want 201, body=%s", res.Code, res.Body.String())
	}
	var tw tweetResponse
	decodeBody(t, res, &tw)
	return tw
}

func TestCreateTweetSuccess(t *testing.T) {
	t.Parallel()
	h := newAuthTestHandler(t)
	cookie := registerAndExtractCookie(t, h, "alice@example.com", "alice")

	tw := createTweet(t, h, "  hello world  ", cookie)
	if tw.Content != "hello world" {
		t.Errorf("Content = %q, want trimmed", tw.Content)
	}
	if tw.Author.Username != "alice" {
		t.Errorf("Author.Username = %q, want alice", tw.Author.Username)
	}
	if tw.LikeCount != 0 || tw.ReplyCount != 0 || tw.LikedByMe {
		t.Errorf("fresh tweet = %+v, want zero counts and LikedByMe false", tw)
	}
}

func TestCreateTweetRejectsEmptyAndOverlong(t *testing.T) {
	t.Parallel()
	h := newAuthTestHandler(t)
	cookie := registerAndExtractCookie(t, h, "alice@example.com", "alice")

	for _, content := range []string{"", "   ", make281CharString()} {
		res := doAction(t, h, http.MethodPost, "/api/tweets", map[string]string{"content": content}, cookie)
		if res.Code != http.StatusBadRequest {
			t.Errorf("content=%q status = %d, want 400, body=%s", content, res.Code, res.Body.String())
		}
		var body ErrorBody
		decodeBody(t, res, &body)
		if len(body.Error.Details["content"]) == 0 {
			t.Errorf("content=%q details = %v, want a content field error", content, body.Error.Details)
		}
	}
}

// make281CharString exercises the multi-byte-emoji code-point count (D-13): 281 emoji, one
// code point each but four UTF-8 bytes, must still be rejected as over the limit.
func make281CharString() string {
	s := ""
	for i := 0; i < 281; i++ {
		s += "\U0001F600"
	}
	return s
}

func TestCreateTweetWithoutCookieReturns401(t *testing.T) {
	t.Parallel()
	h := newAuthTestHandler(t)
	res := doAction(t, h, http.MethodPost, "/api/tweets", map[string]string{"content": "hello"}, nil)
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401, body=%s", res.Code, res.Body.String())
	}
}

func TestGetTweetByID(t *testing.T) {
	t.Parallel()
	h := newAuthTestHandler(t)
	cookie := registerAndExtractCookie(t, h, "alice@example.com", "alice")
	tw := createTweet(t, h, "hello", cookie)

	res := doGet(t, h, "/api/tweets/"+tw.ID, cookie)
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", res.Code, res.Body.String())
	}
	var got tweetResponse
	decodeBody(t, res, &got)
	if got.ID != tw.ID || got.Content != "hello" {
		t.Errorf("got = %+v, want id=%s content=hello", got, tw.ID)
	}
}

func TestGetTweetUnknownReturns404(t *testing.T) {
	t.Parallel()
	h := newAuthTestHandler(t)
	cookie := registerAndExtractCookie(t, h, "alice@example.com", "alice")
	res := doGet(t, h, "/api/tweets/nope", cookie)
	if res.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404, body=%s", res.Code, res.Body.String())
	}
}

func TestDeleteTweetByAuthorSoftDeletes(t *testing.T) {
	t.Parallel()
	h := newAuthTestHandler(t)
	cookie := registerAndExtractCookie(t, h, "alice@example.com", "alice")
	tw := createTweet(t, h, "hello", cookie)

	res := doAction(t, h, http.MethodDelete, "/api/tweets/"+tw.ID, nil, cookie)
	if res.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, want 204, body=%s", res.Code, res.Body.String())
	}

	res = doGet(t, h, "/api/tweets/"+tw.ID, cookie)
	if res.Code != http.StatusNotFound {
		t.Fatalf("get after delete status = %d, want 404 (D-16)", res.Code)
	}
}

func TestDeleteTweetByNonAuthorReturns403(t *testing.T) {
	t.Parallel()
	h := newAuthTestHandler(t)
	aliceCookie := registerAndExtractCookie(t, h, "alice@example.com", "alice")
	bobCookie := registerAndExtractCookie(t, h, "bob@example.com", "bob")
	tw := createTweet(t, h, "hello", aliceCookie)

	res := doAction(t, h, http.MethodDelete, "/api/tweets/"+tw.ID, nil, bobCookie)
	if res.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403, body=%s", res.Code, res.Body.String())
	}
}

func TestDeleteTweetUnknownReturns404(t *testing.T) {
	t.Parallel()
	h := newAuthTestHandler(t)
	cookie := registerAndExtractCookie(t, h, "alice@example.com", "alice")
	res := doAction(t, h, http.MethodDelete, "/api/tweets/nope", nil, cookie)
	if res.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404, body=%s", res.Code, res.Body.String())
	}
}

func TestListUserTweetsExcludesDeleted(t *testing.T) {
	t.Parallel()
	h := newAuthTestHandler(t)
	cookie := registerAndExtractCookie(t, h, "alice@example.com", "alice")
	kept := createTweet(t, h, "kept", cookie)
	gone := createTweet(t, h, "gone", cookie)

	if res := doAction(t, h, http.MethodDelete, "/api/tweets/"+gone.ID, nil, cookie); res.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, want 204", res.Code)
	}

	res := doGet(t, h, "/api/users/alice/tweets", cookie)
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", res.Code, res.Body.String())
	}
	var list tweetListResponse
	decodeBody(t, res, &list)
	if len(list.Items) != 1 || list.Items[0].ID != kept.ID {
		t.Errorf("items = %+v, want only %q", list.Items, kept.ID)
	}
}

func TestListUserTweetsUnknownUsernameReturns404(t *testing.T) {
	t.Parallel()
	h := newAuthTestHandler(t)
	cookie := registerAndExtractCookie(t, h, "alice@example.com", "alice")
	res := doGet(t, h, "/api/users/ghost/tweets", cookie)
	if res.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404, body=%s", res.Code, res.Body.String())
	}
}

func TestTimelineIncludesOwnAndFollowedExcludesOthers(t *testing.T) {
	t.Parallel()
	h := newAuthTestHandler(t)
	aliceCookie := registerAndExtractCookie(t, h, "alice@example.com", "alice")
	bobCookie := registerAndExtractCookie(t, h, "bob@example.com", "bob")
	carolCookie := registerAndExtractCookie(t, h, "carol@example.com", "carol")

	own := createTweet(t, h, "alice tweet", aliceCookie)
	followed := createTweet(t, h, "bob tweet", bobCookie)
	notFollowed := createTweet(t, h, "carol tweet", carolCookie)

	if res := doAction(t, h, http.MethodPost, "/api/users/bob/follow", nil, aliceCookie); res.Code != http.StatusOK {
		t.Fatalf("follow status = %d, want 200, body=%s", res.Code, res.Body.String())
	}

	res := doGet(t, h, "/api/timeline", aliceCookie)
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", res.Code, res.Body.String())
	}
	var list tweetListResponse
	decodeBody(t, res, &list)

	ids := map[string]bool{}
	for _, tw := range list.Items {
		ids[tw.ID] = true
	}
	if !ids[own.ID] {
		t.Error("timeline must include the viewer's own tweet (D-17)")
	}
	if !ids[followed.ID] {
		t.Error("timeline must include a followed user's tweet")
	}
	if ids[notFollowed.ID] {
		t.Error("timeline must not include a non-followed user's tweet")
	}
}

func TestTimelineOrderingNewestFirst(t *testing.T) {
	t.Parallel()
	h := newAuthTestHandler(t)
	cookie := registerAndExtractCookie(t, h, "alice@example.com", "alice")

	first := createTweet(t, h, "first", cookie)
	time.Sleep(2 * time.Millisecond)
	second := createTweet(t, h, "second", cookie)

	res := doGet(t, h, "/api/timeline", cookie)
	var list tweetListResponse
	decodeBody(t, res, &list)

	if len(list.Items) < 2 || list.Items[0].ID != second.ID || list.Items[1].ID != first.ID {
		t.Fatalf("items = %+v, want newest first (%s, %s)", list.Items, second.ID, first.ID)
	}
}

func TestLikeTwiceCountsOnceAndUnlikeRemoves(t *testing.T) {
	t.Parallel()
	h := newAuthTestHandler(t)
	aliceCookie := registerAndExtractCookie(t, h, "alice@example.com", "alice")
	bobCookie := registerAndExtractCookie(t, h, "bob@example.com", "bob")
	tw := createTweet(t, h, "hello", aliceCookie)

	res := doAction(t, h, http.MethodPost, "/api/tweets/"+tw.ID+"/like", nil, bobCookie)
	if res.Code != http.StatusOK {
		t.Fatalf("like status = %d, want 200, body=%s", res.Code, res.Body.String())
	}
	res = doAction(t, h, http.MethodPost, "/api/tweets/"+tw.ID+"/like", nil, bobCookie)
	var liked tweetResponse
	decodeBody(t, res, &liked)
	if liked.LikeCount != 1 {
		t.Errorf("LikeCount after liking twice = %d, want 1", liked.LikeCount)
	}
	if !liked.LikedByMe {
		t.Error("LikedByMe = false, want true")
	}

	res = doAction(t, h, http.MethodDelete, "/api/tweets/"+tw.ID+"/like", nil, bobCookie)
	if res.Code != http.StatusOK {
		t.Fatalf("unlike status = %d, want 200, body=%s", res.Code, res.Body.String())
	}
	var unliked tweetResponse
	decodeBody(t, res, &unliked)
	if unliked.LikeCount != 0 || unliked.LikedByMe {
		t.Errorf("after unlike = %+v, want count 0 and LikedByMe false", unliked)
	}
}

func TestLikeDeletedOrMissingTweetReturns404(t *testing.T) {
	t.Parallel()
	h := newAuthTestHandler(t)
	cookie := registerAndExtractCookie(t, h, "alice@example.com", "alice")
	tw := createTweet(t, h, "hello", cookie)
	if res := doAction(t, h, http.MethodDelete, "/api/tweets/"+tw.ID, nil, cookie); res.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, want 204", res.Code)
	}

	res := doAction(t, h, http.MethodPost, "/api/tweets/"+tw.ID+"/like", nil, cookie)
	if res.Code != http.StatusNotFound {
		t.Fatalf("like deleted tweet status = %d, want 404 (D-22)", res.Code)
	}

	res = doAction(t, h, http.MethodPost, "/api/tweets/nope/like", nil, cookie)
	if res.Code != http.StatusNotFound {
		t.Fatalf("like missing tweet status = %d, want 404 (D-22)", res.Code)
	}
}

func TestTimelineInvalidCursorReturns400(t *testing.T) {
	t.Parallel()
	h := newAuthTestHandler(t)
	cookie := registerAndExtractCookie(t, h, "alice@example.com", "alice")

	res := doGet(t, h, "/api/timeline?cursor=not-valid-base64url!!", cookie)
	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400, body=%s", res.Code, res.Body.String())
	}

	res = doGet(t, h, "/api/timeline?limit=0", cookie)
	if res.Code != http.StatusBadRequest {
		t.Fatalf("limit=0 status = %d, want 400, body=%s", res.Code, res.Body.String())
	}
}

// TestGetTweetUnexpectedStoreErrorReturns500 proves an unexpected store error surfaces as a
// 500 with no leaked detail rather than propagating the underlying error message, covering the
// tweet.Service.viewOf and writeTweetError default branches.
func TestGetTweetUnexpectedStoreErrorReturns500(t *testing.T) {
	t.Parallel()
	st := memory.New()
	plainHandler := NewHandler(Deps{Config: testConfig(), Store: st})
	cookie := registerAndExtractCookie(t, plainHandler, "alice@example.com", "alice")
	tw := createTweet(t, plainHandler, "hello", cookie)

	explodingHandler := NewHandler(Deps{Config: testConfig(), Store: &explodingLikeCountStore{Store: st}})
	res := doGet(t, explodingHandler, "/api/tweets/"+tw.ID, cookie)
	if res.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500, body=%s", res.Code, res.Body.String())
	}
	var body ErrorBody
	decodeBody(t, res, &body)
	if body.Error.Code != CodeInternal {
		t.Errorf("error.code = %q, want %q", body.Error.Code, CodeInternal)
	}
	if strings.Contains(res.Body.String(), "boom") {
		t.Error("response must not leak the underlying store error")
	}
}

// TestTimelinePaginationHasNoDuplicatesOrGapsAcrossSampleData exercises the real cursor
// contract through the real HTTP layer against the real sample.json (D-67), mirroring the
// Step 4 followers pagination test for this step's "pagination has no duplicates/gaps"
// requirement, using alice's real sample timeline as the fixture.
func TestTimelinePaginationHasNoDuplicatesOrGapsAcrossSampleData(t *testing.T) {
	t.Parallel()
	h := newSampleDataTestHandler(t)
	cookie := loginAndExtractCookie(t, h, "alice@example.com", "Password123!")

	walkTimeline := func(limit int) []string {
		var ids []string
		var cursor string
		for pages := 0; ; pages++ {
			if pages > 200 {
				t.Fatalf("pagination did not terminate after %d pages", pages)
			}
			url := fmt.Sprintf("/api/timeline?limit=%d", limit)
			if cursor != "" {
				url += "&cursor=" + cursor
			}
			res := doGet(t, h, url, cookie)
			if res.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200, body=%s", res.Code, res.Body.String())
			}
			var page tweetListResponse
			decodeBody(t, res, &page)
			for _, item := range page.Items {
				ids = append(ids, item.ID)
			}
			if page.NextCursor == nil {
				return ids
			}
			cursor = *page.NextCursor
		}
	}

	wantIDs := walkTimeline(50)
	if len(wantIDs) < 5 {
		t.Fatalf("expected alice to have a nontrivial timeline in the sample data (D-67), got %d", len(wantIDs))
	}
	got := walkTimeline(3)

	if len(got) != len(wantIDs) {
		t.Fatalf("paginated through %d ids, want %d (no duplicates/gaps): got=%v want=%v", len(got), len(wantIDs), got, wantIDs)
	}
	for i := range wantIDs {
		if got[i] != wantIDs[i] {
			t.Fatalf("id at position %d = %q, want %q (order mismatch)", i, got[i], wantIDs[i])
		}
	}
}
