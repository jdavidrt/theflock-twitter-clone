package httpapi

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jdavidrt/theflock-twitter-clone/server/internal/store"
	"github.com/jdavidrt/theflock-twitter-clone/server/internal/store/sample"
)

// realSampleDataPath finds server/data/sample.json regardless of the working directory go test
// uses (it runs from this package's own directory, internal/httpapi).
const realSampleDataPath = "../../data/sample.json"

// newSampleDataTestHandler builds a handler backed by a store loaded from the real sample.json,
// for tests that need more than a couple of hand-registered users (D-67).
func newSampleDataTestHandler(t *testing.T) http.Handler {
	t.Helper()
	cfg := testConfig()
	st := newStore(t)
	if _, err := sample.Load(realSampleDataPath, cfg.BcryptCost(), st); err != nil {
		t.Fatalf("loading sample data: %v", err)
	}
	return NewHandler(Deps{Config: cfg, Store: st})
}

func loginAndExtractCookie(t *testing.T, h http.Handler, email, password string) *http.Cookie {
	t.Helper()
	res := httptest.NewRecorder()
	h.ServeHTTP(res, jsonRequest(t, http.MethodPost, "/api/auth/login", map[string]string{"email": email, "password": password}))
	if res.Code != http.StatusOK {
		t.Fatalf("login status = %d, want 200, body=%s", res.Code, res.Body.String())
	}
	for _, c := range res.Result().Cookies() {
		if c.Name == cookieName {
			return c
		}
	}
	t.Fatal("login response did not set a session cookie")
	return nil
}

func doGet(t *testing.T, h http.Handler, path string, cookie *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	res := httptest.NewRecorder()
	h.ServeHTTP(res, req)
	return res
}

func doFollowAction(t *testing.T, h http.Handler, method, username string, cookie *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	req := jsonRequest(t, method, "/api/users/"+username+"/follow", nil)
	req.AddCookie(cookie)
	res := httptest.NewRecorder()
	h.ServeHTTP(res, req)
	return res
}

func TestGetProfileReturnsCountsAndFollowState(t *testing.T) {
	t.Parallel()
	h := newAuthTestHandler(t)
	aliceCookie := registerAndExtractCookie(t, h, "alice@example.com", "alice")
	bobCookie := registerAndExtractCookie(t, h, "bob@example.com", "bob")

	if res := doFollowAction(t, h, http.MethodPost, "alice", bobCookie); res.Code != http.StatusOK {
		t.Fatalf("follow status = %d, want 200, body=%s", res.Code, res.Body.String())
	}

	res := doGet(t, h, "/api/users/alice", aliceCookie)
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", res.Code, res.Body.String())
	}
	var profile profileResponse
	decodeBody(t, res, &profile)
	if profile.FollowerCount != 1 {
		t.Errorf("FollowerCount = %d, want 1", profile.FollowerCount)
	}
	if profile.IsFollowedByMe {
		t.Error("IsFollowedByMe (alice viewing herself) = true, want false")
	}

	res = doGet(t, h, "/api/users/alice", bobCookie)
	decodeBody(t, res, &profile)
	if !profile.IsFollowedByMe {
		t.Error("IsFollowedByMe (bob viewing alice) = false, want true")
	}
}

func TestGetProfileUnknownUsernameReturns404(t *testing.T) {
	t.Parallel()
	h := newAuthTestHandler(t)
	cookie := registerAndExtractCookie(t, h, "alice@example.com", "alice")

	res := doGet(t, h, "/api/users/ghost", cookie)
	if res.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404, body=%s", res.Code, res.Body.String())
	}
	var body ErrorBody
	decodeBody(t, res, &body)
	if body.Error.Code != CodeNotFound {
		t.Errorf("error.code = %q, want %q", body.Error.Code, CodeNotFound)
	}
}

func TestGetProfileWithoutCookieReturns401(t *testing.T) {
	t.Parallel()
	h := newAuthTestHandler(t)
	res := doGet(t, h, "/api/users/anyone", nil)
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401, body=%s", res.Code, res.Body.String())
	}
}

func TestUpdateMePersistsDisplayNameAndBio(t *testing.T) {
	t.Parallel()
	h := newAuthTestHandler(t)
	cookie := registerAndExtractCookie(t, h, "alice@example.com", "alice")

	req := jsonRequest(t, http.MethodPatch, "/api/users/me", map[string]string{"displayName": "Alice Doe", "bio": "Hello there"})
	req.AddCookie(cookie)
	res := httptest.NewRecorder()
	h.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", res.Code, res.Body.String())
	}

	res = doGet(t, h, "/api/users/alice", cookie)
	var profile profileResponse
	decodeBody(t, res, &profile)
	if profile.DisplayName != "Alice Doe" {
		t.Errorf("DisplayName = %q, want Alice Doe", profile.DisplayName)
	}
	if profile.Bio != "Hello there" {
		t.Errorf("Bio = %q, want Hello there", profile.Bio)
	}
}

func TestUpdateMeRejectsInvalidFields(t *testing.T) {
	t.Parallel()
	h := newAuthTestHandler(t)
	cookie := registerAndExtractCookie(t, h, "alice@example.com", "alice")

	req := jsonRequest(t, http.MethodPatch, "/api/users/me", map[string]string{"displayName": "", "bio": "x"})
	req.AddCookie(cookie)
	res := httptest.NewRecorder()
	h.ServeHTTP(res, req)
	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400, body=%s", res.Code, res.Body.String())
	}
	var body ErrorBody
	decodeBody(t, res, &body)
	if len(body.Error.Details["displayName"]) == 0 {
		t.Errorf("details should name the displayName field, got %v", body.Error.Details)
	}
}

func TestUpdateMeWithoutCookieReturns401(t *testing.T) {
	t.Parallel()
	h := newAuthTestHandler(t)
	res := httptest.NewRecorder()
	h.ServeHTTP(res, jsonRequest(t, http.MethodPatch, "/api/users/me", map[string]string{"displayName": "X", "bio": ""}))
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401, body=%s", res.Code, res.Body.String())
	}
}

func TestFollowIsIdempotent(t *testing.T) {
	t.Parallel()
	h := newAuthTestHandler(t)
	registerAndExtractCookie(t, h, "alice@example.com", "alice")
	bobCookie := registerAndExtractCookie(t, h, "bob@example.com", "bob")

	res := doFollowAction(t, h, http.MethodPost, "alice", bobCookie)
	if res.Code != http.StatusOK {
		t.Fatalf("first follow status = %d, want 200, body=%s", res.Code, res.Body.String())
	}
	res = doFollowAction(t, h, http.MethodPost, "alice", bobCookie)
	if res.Code != http.StatusOK {
		t.Fatalf("repeat follow status = %d, want 200, body=%s", res.Code, res.Body.String())
	}
	var profile profileResponse
	decodeBody(t, res, &profile)
	if profile.FollowerCount != 1 {
		t.Errorf("FollowerCount after repeat follow = %d, want 1 (idempotent, D-21)", profile.FollowerCount)
	}
}

func TestUnfollowWhenNotFollowingIsNoOp(t *testing.T) {
	t.Parallel()
	h := newAuthTestHandler(t)
	registerAndExtractCookie(t, h, "alice@example.com", "alice")
	bobCookie := registerAndExtractCookie(t, h, "bob@example.com", "bob")

	res := doFollowAction(t, h, http.MethodDelete, "alice", bobCookie)
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (no-op unfollow, D-21), body=%s", res.Code, res.Body.String())
	}
	var profile profileResponse
	decodeBody(t, res, &profile)
	if profile.IsFollowedByMe {
		t.Error("IsFollowedByMe = true after a no-op unfollow, want false")
	}
}

func TestUnfollowRemovesAnExistingFollow(t *testing.T) {
	t.Parallel()
	h := newAuthTestHandler(t)
	registerAndExtractCookie(t, h, "alice@example.com", "alice")
	bobCookie := registerAndExtractCookie(t, h, "bob@example.com", "bob")

	doFollowAction(t, h, http.MethodPost, "alice", bobCookie)
	res := doFollowAction(t, h, http.MethodDelete, "alice", bobCookie)
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", res.Code, res.Body.String())
	}
	var profile profileResponse
	decodeBody(t, res, &profile)
	if profile.IsFollowedByMe || profile.FollowerCount != 0 {
		t.Errorf("profile after unfollow = %+v, want IsFollowedByMe=false FollowerCount=0", profile)
	}
}

func TestSelfFollowReturns400(t *testing.T) {
	t.Parallel()
	h := newAuthTestHandler(t)
	cookie := registerAndExtractCookie(t, h, "alice@example.com", "alice")

	res := doFollowAction(t, h, http.MethodPost, "alice", cookie)
	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400, body=%s", res.Code, res.Body.String())
	}
	var body ErrorBody
	decodeBody(t, res, &body)
	if body.Error.Code != CodeValidation {
		t.Errorf("error.code = %q, want %q", body.Error.Code, CodeValidation)
	}
}

func TestFollowUnknownUsernameReturns404(t *testing.T) {
	t.Parallel()
	h := newAuthTestHandler(t)
	cookie := registerAndExtractCookie(t, h, "alice@example.com", "alice")

	res := doFollowAction(t, h, http.MethodPost, "ghost", cookie)
	if res.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404, body=%s", res.Code, res.Body.String())
	}
}

func TestUnfollowUnknownUsernameReturns404(t *testing.T) {
	t.Parallel()
	h := newAuthTestHandler(t)
	cookie := registerAndExtractCookie(t, h, "alice@example.com", "alice")

	res := doFollowAction(t, h, http.MethodDelete, "ghost", cookie)
	if res.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404, body=%s", res.Code, res.Body.String())
	}
}

// explodingTweetCountStore wraps a real store but fails TweetCount with an error that is
// neither ErrNotFound nor a sentinel the social service knows about, so callers can exercise the
// "unexpected store error" branch (a 500, not a leaked 500 body) without a second store backend.
type explodingTweetCountStore struct {
	store.Store
}

func (s *explodingTweetCountStore) TweetCount(string) (int, error) {
	return 0, errors.New("boom: database unavailable")
}

func TestGetProfileUnexpectedStoreErrorReturns500(t *testing.T) {
	t.Parallel()
	st := &explodingTweetCountStore{Store: newStore(t)}
	h := NewHandler(Deps{Config: testConfig(), Store: st})
	cookie := registerAndExtractCookie(t, h, "alice@example.com", "alice")

	res := doGet(t, h, "/api/users/alice", cookie)
	if res.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500, body=%s", res.Code, res.Body.String())
	}
	var body ErrorBody
	decodeBody(t, res, &body)
	if body.Error.Code != CodeInternal {
		t.Errorf("error.code = %q, want %q", body.Error.Code, CodeInternal)
	}
	if strings.Contains(res.Body.String(), "database unavailable") {
		t.Error("the underlying error must not leak to the client (D-55)")
	}
}

func TestListFollowersAndFollowingOrderedNewestFirst(t *testing.T) {
	t.Parallel()
	h := newAuthTestHandler(t)
	registerAndExtractCookie(t, h, "alice@example.com", "alice")
	bobCookie := registerAndExtractCookie(t, h, "bob@example.com", "bob")
	carolCookie := registerAndExtractCookie(t, h, "carol@example.com", "carol")
	daveCookie := registerAndExtractCookie(t, h, "dave@example.com", "dave")

	// bob, then carol, then dave follow alice, in that order. The service stamps CreatedAt with
	// real time, so a small sleep between calls guarantees a distinct, orderable timestamp for
	// each edge instead of leaving the assertion at the mercy of clock resolution.
	doFollowAction(t, h, http.MethodPost, "alice", bobCookie)
	time.Sleep(5 * time.Millisecond)
	doFollowAction(t, h, http.MethodPost, "alice", carolCookie)
	time.Sleep(5 * time.Millisecond)
	doFollowAction(t, h, http.MethodPost, "alice", daveCookie)

	res := doGet(t, h, "/api/users/alice/followers", bobCookie)
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", res.Code, res.Body.String())
	}
	var list followListResponse
	decodeBody(t, res, &list)
	if len(list.Items) != 3 {
		t.Fatalf("followers count = %d, want 3", len(list.Items))
	}
	gotOrder := []string{list.Items[0].Username, list.Items[1].Username, list.Items[2].Username}
	wantOrder := []string{"dave", "carol", "bob"}
	for i := range wantOrder {
		if gotOrder[i] != wantOrder[i] {
			t.Fatalf("followers order = %v, want %v (newest follow first, D-24)", gotOrder, wantOrder)
		}
	}

	// alice's following list, from bob's perspective, should be empty (alice follows nobody).
	res = doGet(t, h, "/api/users/alice/following", bobCookie)
	decodeBody(t, res, &list)
	if len(list.Items) != 0 {
		t.Errorf("alice's following list = %d items, want 0", len(list.Items))
	}
}

func TestListFollowersUnknownUsernameReturns404(t *testing.T) {
	t.Parallel()
	h := newAuthTestHandler(t)
	cookie := registerAndExtractCookie(t, h, "alice@example.com", "alice")

	res := doGet(t, h, "/api/users/ghost/followers", cookie)
	if res.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404, body=%s", res.Code, res.Body.String())
	}
}

func TestListFollowersInvalidCursorReturns400(t *testing.T) {
	t.Parallel()
	h := newAuthTestHandler(t)
	cookie := registerAndExtractCookie(t, h, "alice@example.com", "alice")

	res := doGet(t, h, "/api/users/alice/followers?cursor=not-a-valid-cursor", cookie)
	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400, body=%s", res.Code, res.Body.String())
	}
}

func TestListFollowersInvalidLimitReturns400(t *testing.T) {
	t.Parallel()
	h := newAuthTestHandler(t)
	cookie := registerAndExtractCookie(t, h, "alice@example.com", "alice")

	res := doGet(t, h, "/api/users/alice/followers?limit=0", cookie)
	if res.Code != http.StatusBadRequest {
		t.Fatalf("limit=0 status = %d, want 400, body=%s", res.Code, res.Body.String())
	}
	res = doGet(t, h, "/api/users/alice/followers?limit=51", cookie)
	if res.Code != http.StatusBadRequest {
		t.Fatalf("limit=51 status = %d, want 400, body=%s", res.Code, res.Body.String())
	}
}

// TestFollowersPaginationHasNoDuplicatesOrGapsAcrossSampleData exercises D-19's pagination
// contract through the real HTTP layer against the real sample.json (D-67), per the Step 4
// prompt's "pagination cursor returns no duplicates/gaps across the sample data" requirement.
func TestFollowersPaginationHasNoDuplicatesOrGapsAcrossSampleData(t *testing.T) {
	t.Parallel()
	h := newSampleDataTestHandler(t)
	cookie := loginAndExtractCookie(t, h, "alice@example.com", "Password123!")

	res := doGet(t, h, "/api/users/alice/followers?limit=50", cookie)
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", res.Code, res.Body.String())
	}
	var full followListResponse
	decodeBody(t, res, &full)
	if len(full.Items) < 5 {
		t.Fatalf("expected alice to have several followers in the sample data (D-67), got %d", len(full.Items))
	}
	wantIDs := make([]string, len(full.Items))
	for i, item := range full.Items {
		wantIDs[i] = item.ID
	}

	var got []string
	var cursor string
	for pages := 0; ; pages++ {
		if pages > len(wantIDs)+1 {
			t.Fatalf("pagination did not terminate after %d pages", pages)
		}
		url := "/api/users/alice/followers?limit=3"
		if cursor != "" {
			url += "&cursor=" + cursor
		}
		res := doGet(t, h, url, cookie)
		if res.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200, body=%s", res.Code, res.Body.String())
		}
		var page followListResponse
		decodeBody(t, res, &page)
		for _, item := range page.Items {
			got = append(got, item.ID)
		}
		if page.NextCursor == nil {
			break
		}
		cursor = *page.NextCursor
	}

	if len(got) != len(wantIDs) {
		t.Fatalf("paginated total = %d items, want %d (no duplicates/gaps)", len(got), len(wantIDs))
	}
	seen := make(map[string]bool, len(got))
	for i, id := range got {
		if id != wantIDs[i] {
			t.Fatalf("item %d = %s, want %s (paginated order must match the unpaginated list)", i, id, wantIDs[i])
		}
		if seen[id] {
			t.Fatalf("duplicate id %s across pages", id)
		}
		seen[id] = true
	}
}

// TestSearchUsersMixedCaseAgainstSampleData is the Step 6 prompt's explicit "mixed-case query
// against the sample data" requirement (D-25): "aR" must match carol and oscar's usernames by
// case-insensitive substring, ordered by username ascending, without matching every sample user.
func TestSearchUsersMixedCaseAgainstSampleData(t *testing.T) {
	t.Parallel()
	h := newSampleDataTestHandler(t)
	cookie := loginAndExtractCookie(t, h, "alice@example.com", "Password123!")

	res := doGet(t, h, "/api/search/users?q=aR", cookie)
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", res.Code, res.Body.String())
	}
	var body searchUsersResponse
	decodeBody(t, res, &body)
	if len(body.Items) != 2 || body.Items[0].Username != "carol" || body.Items[1].Username != "oscar" {
		t.Fatalf("search(aR) = %+v, want [carol, oscar] ordered by username ascending", body.Items)
	}
}

func TestSearchUsersOrderedByUsernameAscending(t *testing.T) {
	t.Parallel()
	h := newAuthTestHandler(t)
	cookie := registerAndExtractCookie(t, h, "alice@example.com", "alice")
	registerUser(t, h, "carla@example.com", "carla")
	registerUser(t, h, "carl@example.com", "carl")

	res := doGet(t, h, "/api/search/users?q=car", cookie)
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", res.Code, res.Body.String())
	}
	var body searchUsersResponse
	decodeBody(t, res, &body)
	if len(body.Items) != 2 || body.Items[0].Username != "carl" || body.Items[1].Username != "carla" {
		t.Fatalf("search(car) = %+v, want [carl, carla] ordered by username ascending", body.Items)
	}
}

func TestSearchUsersEmptyQueryReturns400(t *testing.T) {
	t.Parallel()
	h := newAuthTestHandler(t)
	cookie := registerAndExtractCookie(t, h, "alice@example.com", "alice")

	res := doGet(t, h, "/api/search/users?q=", cookie)
	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400, body=%s", res.Code, res.Body.String())
	}
	var errBody ErrorBody
	decodeBody(t, res, &errBody)
	if len(errBody.Error.Details["q"]) == 0 {
		t.Errorf("details = %v, want a q field error", errBody.Error.Details)
	}
}

func TestSearchUsersWithoutCookieReturns401(t *testing.T) {
	t.Parallel()
	h := newAuthTestHandler(t)
	res := doGet(t, h, "/api/search/users?q=a", nil)
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401, body=%s", res.Code, res.Body.String())
	}
}

// registerUser creates a user without needing its cookie, for tests that only care that the
// user exists (e.g. as a search fixture).
func registerUser(t *testing.T, h http.Handler, email, username string) {
	t.Helper()
	res := httptest.NewRecorder()
	h.ServeHTTP(res, jsonRequest(t, http.MethodPost, "/api/auth/register", registerBody(email, username, "Password123!")))
	if res.Code != http.StatusCreated {
		t.Fatalf("register(%s) status = %d, want 201, body=%s", username, res.Code, res.Body.String())
	}
}
