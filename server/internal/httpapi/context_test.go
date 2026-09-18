package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jdavidrt/theflock-twitter-clone/server/internal/store/memory"
)

func TestRequireAuthRejectsGarbageCookie(t *testing.T) {
	t.Parallel()
	h := newAuthTestHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	req.AddCookie(&http.Cookie{Name: cookieName, Value: "not-a-jwt"})

	res := httptest.NewRecorder()
	h.ServeHTTP(res, req)
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401, body=%s", res.Code, res.Body.String())
	}
}

func TestRequireAuthRejectsTokenSignedWithAnotherSecret(t *testing.T) {
	t.Parallel()
	st := memory.New()
	cfgA := testConfig()
	cfgA.JWTSecret = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	cfgB := testConfig()
	cfgB.JWTSecret = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

	hA := NewHandler(Deps{Config: cfgA, Store: st})
	hB := NewHandler(Deps{Config: cfgB, Store: st})

	cookie := registerAndExtractCookie(t, hA, "ivy@example.com", "ivy")

	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	req.AddCookie(cookie)
	res := httptest.NewRecorder()
	hB.ServeHTTP(res, req)
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 for a token signed with a different JWT_SECRET", res.Code)
	}
}

func TestUserIDFromContextAbsentByDefault(t *testing.T) {
	t.Parallel()
	if _, ok := userIDFromContext(httptest.NewRequest(http.MethodGet, "/", nil).Context()); ok {
		t.Error("a fresh context must not carry a user id")
	}
}
