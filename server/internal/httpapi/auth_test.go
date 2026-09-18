package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jdavidrt/theflock-twitter-clone/server/internal/domain"
	"github.com/jdavidrt/theflock-twitter-clone/server/internal/service/auth"
	"github.com/jdavidrt/theflock-twitter-clone/server/internal/store"
)

// explodingEmailStore wraps a real store but fails GetUserByEmail with an error that is
// neither ErrNotFound nor a sentinel Login/Register know about, so callers can exercise the
// "unexpected store error" branch (a 500, not a 401/409) without needing a second store backend.
type explodingEmailStore struct {
	store.Store
}

func (s *explodingEmailStore) GetUserByEmail(string) (domain.User, error) {
	return domain.User{}, errors.New("boom: database unavailable")
}

func newAuthTestHandler(t *testing.T) http.Handler {
	t.Helper()
	return NewHandler(Deps{Config: testConfig(), Store: newStore(t)})
}

// jsonRequest builds a request with a JSON body and the Content-Type header the jsonOnly
// middleware (D-56) requires on state-changing routes.
func jsonRequest(t *testing.T, method, path string, body any) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encoding request body: %v", err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	return req
}

func registerBody(email, username, password string) map[string]string {
	return map[string]string{"email": email, "username": username, "password": password}
}

func TestRegisterSuccessSetsCookieAndReturnsUser(t *testing.T) {
	t.Parallel()
	h := newAuthTestHandler(t)
	res := httptest.NewRecorder()
	h.ServeHTTP(res, jsonRequest(t, http.MethodPost, "/api/auth/register", registerBody("alice@example.com", "alice", "Password123!")))

	if res.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201, body=%s", res.Code, res.Body.String())
	}
	var user domain.User
	decodeBody(t, res, &user)
	if user.Email != "alice@example.com" || user.Username != "alice" {
		t.Errorf("user = %+v, want normalized alice@example.com/alice", user)
	}
	if user.DisplayName != "alice" {
		t.Errorf("DisplayName = %q, want default to username (D-01)", user.DisplayName)
	}
	if strings.Contains(res.Body.String(), "Password123!") || strings.Contains(res.Body.String(), "passwordHash") {
		t.Errorf("response body must never carry the password or its hash, got %s", res.Body.String())
	}

	assertSessionCookieSet(t, res)
}

func TestRegisterDuplicateEmailReturns409(t *testing.T) {
	t.Parallel()
	h := newAuthTestHandler(t)
	res := httptest.NewRecorder()
	h.ServeHTTP(res, jsonRequest(t, http.MethodPost, "/api/auth/register", registerBody("dup@example.com", "first", "Password123!")))
	if res.Code != http.StatusCreated {
		t.Fatalf("first register status = %d, want 201", res.Code)
	}

	res = httptest.NewRecorder()
	h.ServeHTTP(res, jsonRequest(t, http.MethodPost, "/api/auth/register", registerBody("dup@example.com", "second", "Password123!")))
	if res.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409, body=%s", res.Code, res.Body.String())
	}
	var body ErrorBody
	decodeBody(t, res, &body)
	if body.Error.Code != CodeConflict {
		t.Errorf("error.code = %q, want %q", body.Error.Code, CodeConflict)
	}
	if len(body.Error.Details["email"]) == 0 {
		t.Errorf("details should name the email field, got %v", body.Error.Details)
	}
}

func TestRegisterDuplicateUsernameReturns409(t *testing.T) {
	t.Parallel()
	h := newAuthTestHandler(t)
	res := httptest.NewRecorder()
	h.ServeHTTP(res, jsonRequest(t, http.MethodPost, "/api/auth/register", registerBody("one@example.com", "dupname", "Password123!")))
	if res.Code != http.StatusCreated {
		t.Fatalf("first register status = %d, want 201", res.Code)
	}

	res = httptest.NewRecorder()
	h.ServeHTTP(res, jsonRequest(t, http.MethodPost, "/api/auth/register", registerBody("two@example.com", "dupname", "Password123!")))
	if res.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409, body=%s", res.Code, res.Body.String())
	}
	var body ErrorBody
	decodeBody(t, res, &body)
	if len(body.Error.Details["username"]) == 0 {
		t.Errorf("details should name the username field, got %v", body.Error.Details)
	}
}

func TestRegisterReservedUsernameReturns400(t *testing.T) {
	t.Parallel()
	h := newAuthTestHandler(t)
	res := httptest.NewRecorder()
	h.ServeHTTP(res, jsonRequest(t, http.MethodPost, "/api/auth/register", registerBody("admin@example.com", "admin", "Password123!")))

	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400, body=%s", res.Code, res.Body.String())
	}
	var body ErrorBody
	decodeBody(t, res, &body)
	if body.Error.Code != CodeValidation {
		t.Errorf("error.code = %q, want %q", body.Error.Code, CodeValidation)
	}
	if len(body.Error.Details["username"]) == 0 {
		t.Errorf("details should name the username field, got %v", body.Error.Details)
	}
}

func TestLoginSuccessSetsCookie(t *testing.T) {
	t.Parallel()
	h := newAuthTestHandler(t)
	registerRes := httptest.NewRecorder()
	h.ServeHTTP(registerRes, jsonRequest(t, http.MethodPost, "/api/auth/register", registerBody("carol@example.com", "carol", "Password123!")))
	if registerRes.Code != http.StatusCreated {
		t.Fatalf("register status = %d, want 201", registerRes.Code)
	}

	res := httptest.NewRecorder()
	h.ServeHTTP(res, jsonRequest(t, http.MethodPost, "/api/auth/login", map[string]string{"email": "carol@example.com", "password": "Password123!"}))
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", res.Code, res.Body.String())
	}
	var user domain.User
	decodeBody(t, res, &user)
	if user.Username != "carol" {
		t.Errorf("user.Username = %q, want carol", user.Username)
	}
	assertSessionCookieSet(t, res)
}

func TestLoginWrongPasswordReturns401Generic(t *testing.T) {
	t.Parallel()
	h := newAuthTestHandler(t)
	h.ServeHTTP(httptest.NewRecorder(), jsonRequest(t, http.MethodPost, "/api/auth/register", registerBody("dave@example.com", "dave", "Password123!")))

	res := httptest.NewRecorder()
	h.ServeHTTP(res, jsonRequest(t, http.MethodPost, "/api/auth/login", map[string]string{"email": "dave@example.com", "password": "WrongPassword1"}))
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401, body=%s", res.Code, res.Body.String())
	}
	var body ErrorBody
	decodeBody(t, res, &body)
	if body.Error.Code != CodeUnauthorized {
		t.Errorf("error.code = %q, want %q", body.Error.Code, CodeUnauthorized)
	}
	if body.Error.Message != "Invalid email or password" {
		t.Errorf("message = %q, want the generic D-08 message", body.Error.Message)
	}
}

func TestLoginUnknownEmailReturnsSameGeneric401(t *testing.T) {
	t.Parallel()
	h := newAuthTestHandler(t)
	res := httptest.NewRecorder()
	h.ServeHTTP(res, jsonRequest(t, http.MethodPost, "/api/auth/login", map[string]string{"email": "nobody@example.com", "password": "Password123!"}))
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401, body=%s", res.Code, res.Body.String())
	}
	var body ErrorBody
	decodeBody(t, res, &body)
	if body.Error.Message != "Invalid email or password" {
		t.Errorf("message = %q, want the generic D-08 message (no user enumeration)", body.Error.Message)
	}
}

// registerAndExtractCookie registers a user and returns the session cookie the client should
// send on subsequent requests.
func registerAndExtractCookie(t *testing.T, h http.Handler, email, username string) *http.Cookie {
	t.Helper()
	res := httptest.NewRecorder()
	h.ServeHTTP(res, jsonRequest(t, http.MethodPost, "/api/auth/register", registerBody(email, username, "Password123!")))
	if res.Code != http.StatusCreated {
		t.Fatalf("register status = %d, want 201, body=%s", res.Code, res.Body.String())
	}
	for _, c := range res.Result().Cookies() {
		if c.Name == cookieName {
			return c
		}
	}
	t.Fatal("register response did not set a session cookie")
	return nil
}

func TestMeReturnsAuthenticatedUser(t *testing.T) {
	t.Parallel()
	h := newAuthTestHandler(t)
	cookie := registerAndExtractCookie(t, h, "erin@example.com", "erin")

	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	req.AddCookie(cookie)
	res := httptest.NewRecorder()
	h.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", res.Code, res.Body.String())
	}
	var user domain.User
	decodeBody(t, res, &user)
	if user.Username != "erin" {
		t.Errorf("user.Username = %q, want erin", user.Username)
	}
}

func TestMeWithTokenForUnknownUserReturns401(t *testing.T) {
	t.Parallel()
	cfg := testConfig()
	st := newStore(t)
	h := NewHandler(Deps{Config: cfg, Store: st})

	// A token that verifies fine (correct signature, not expired) but whose subject does not
	// resolve to any user — e.g. the account was removed after the token was issued.
	svc := auth.New(st, cfg.BcryptCost(), cfg.JWTSecret)
	token, err := svc.IssueToken("does-not-exist")
	if err != nil {
		t.Fatalf("IssueToken() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	req.AddCookie(&http.Cookie{Name: cookieName, Value: token})
	res := httptest.NewRecorder()
	h.ServeHTTP(res, req)

	if res.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401, body=%s", res.Code, res.Body.String())
	}
}

func TestLoginUnexpectedStoreErrorReturns500(t *testing.T) {
	t.Parallel()
	h := NewHandler(Deps{Config: testConfig(), Store: &explodingEmailStore{Store: newStore(t)}})
	res := httptest.NewRecorder()
	h.ServeHTTP(res, jsonRequest(t, http.MethodPost, "/api/auth/login", map[string]string{"email": "x@example.com", "password": "whatever"}))

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

func TestMeWithoutCookieReturns401(t *testing.T) {
	t.Parallel()
	h := newAuthTestHandler(t)
	res := httptest.NewRecorder()
	h.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/api/auth/me", nil))

	if res.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401, body=%s", res.Code, res.Body.String())
	}
	var body ErrorBody
	decodeBody(t, res, &body)
	if body.Error.Code != CodeUnauthorized {
		t.Errorf("error.code = %q, want %q", body.Error.Code, CodeUnauthorized)
	}
}

func TestLogoutClearsSessionAndRequiresAuth(t *testing.T) {
	t.Parallel()
	h := newAuthTestHandler(t)

	// Without a cookie, logout is a protected route like any other (D-11).
	noCookieReq := jsonRequest(t, http.MethodPost, "/api/auth/logout", nil)
	res := httptest.NewRecorder()
	h.ServeHTTP(res, noCookieReq)
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("logout without cookie status = %d, want 401", res.Code)
	}

	cookie := registerAndExtractCookie(t, h, "frank@example.com", "frank")
	logoutReq := jsonRequest(t, http.MethodPost, "/api/auth/logout", nil)
	logoutReq.AddCookie(cookie)
	res = httptest.NewRecorder()
	h.ServeHTTP(res, logoutReq)
	if res.Code != http.StatusNoContent {
		t.Fatalf("logout status = %d, want 204, body=%s", res.Code, res.Body.String())
	}

	var cleared *http.Cookie
	for _, c := range res.Result().Cookies() {
		if c.Name == cookieName {
			cleared = c
		}
	}
	if cleared == nil {
		t.Fatal("logout response did not set a cookie to clear the session")
	}
	if cleared.MaxAge >= 0 {
		t.Errorf("cleared cookie MaxAge = %d, want negative (delete now)", cleared.MaxAge)
	}

	// The client can no longer use the same (now stale, but still cryptographically valid
	// until expiry, D-10) cookie against /me in a fresh unauthenticated request — this test
	// only proves the *response* tells the browser to drop it, matching D-10's documented
	// limitation that the token itself is not server-side revoked.
}

func TestMalformedJSONBodyReturns400(t *testing.T) {
	t.Parallel()
	h := newAuthTestHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", strings.NewReader("{not valid json"))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()
	h.ServeHTTP(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400, body=%s", res.Code, res.Body.String())
	}
	var body ErrorBody
	decodeBody(t, res, &body)
	if body.Error.Code != CodeValidation {
		t.Errorf("error.code = %q, want %q", body.Error.Code, CodeValidation)
	}
}

func TestWrongContentTypeReturns400(t *testing.T) {
	t.Parallel()
	h := newAuthTestHandler(t)
	body, _ := json.Marshal(registerBody("greg@example.com", "greg", "Password123!"))
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res := httptest.NewRecorder()
	h.ServeHTTP(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400, body=%s", res.Code, res.Body.String())
	}
	var errBody ErrorBody
	decodeBody(t, res, &errBody)
	if errBody.Error.Code != CodeValidation {
		t.Errorf("error.code = %q, want %q", errBody.Error.Code, CodeValidation)
	}
}

func TestMissingContentTypeReturns400(t *testing.T) {
	t.Parallel()
	h := newAuthTestHandler(t)
	body, _ := json.Marshal(registerBody("harriet@example.com", "harriet", "Password123!"))
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader(body))
	res := httptest.NewRecorder()
	h.ServeHTTP(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400, body=%s", res.Code, res.Body.String())
	}
}

func assertSessionCookieSet(t *testing.T, res *httptest.ResponseRecorder) {
	t.Helper()
	for _, c := range res.Result().Cookies() {
		if c.Name != cookieName {
			continue
		}
		if !c.HttpOnly {
			t.Error("session cookie must be HttpOnly")
		}
		if c.SameSite != http.SameSiteLaxMode {
			t.Errorf("session cookie SameSite = %v, want Lax", c.SameSite)
		}
		if c.Path != "/" {
			t.Errorf("session cookie Path = %q, want /", c.Path)
		}
		if c.Secure {
			t.Error("session cookie must not be Secure when COOKIE_SECURE=false")
		}
		if c.Value == "" {
			t.Error("session cookie must carry a token")
		}
		return
	}
	t.Fatal("response did not set a session cookie")
}
