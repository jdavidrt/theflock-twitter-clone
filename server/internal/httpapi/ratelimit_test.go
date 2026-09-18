package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jdavidrt/theflock-twitter-clone/server/internal/config"
	"github.com/jdavidrt/theflock-twitter-clone/server/internal/store/memory"
)

func devConfig() config.Config {
	return config.Config{Port: 3000, AppEnv: config.EnvDevelopment, JWTSecret: strings.Repeat("t", config.MinJWTSecretLen)}
}

func loginAttempt(t *testing.T, remoteAddr string) *http.Request {
	t.Helper()
	req := jsonRequest(t, http.MethodPost, "/api/auth/login", map[string]string{"email": "x@example.com", "password": "whatever"})
	req.RemoteAddr = remoteAddr
	return req
}

func TestAuthRateLimitBlocksAfterBudgetExhausted(t *testing.T) {
	t.Parallel()
	h := NewHandler(Deps{Config: devConfig(), Store: memory.New()})

	var last *httptest.ResponseRecorder
	for range authRateLimitRequests + 1 {
		last = httptest.NewRecorder()
		h.ServeHTTP(last, loginAttempt(t, "203.0.113.9:12345"))
	}
	if last.Code != http.StatusTooManyRequests {
		t.Fatalf("status after exceeding the budget = %d, want 429, body=%s", last.Code, last.Body.String())
	}
	var body ErrorBody
	decodeBody(t, last, &body)
	if body.Error.Code != CodeRateLimited {
		t.Errorf("error.code = %q, want %q", body.Error.Code, CodeRateLimited)
	}
}

func TestAuthRateLimitIsPerIP(t *testing.T) {
	t.Parallel()
	h := NewHandler(Deps{Config: devConfig(), Store: memory.New()})

	for range authRateLimitRequests {
		res := httptest.NewRecorder()
		h.ServeHTTP(res, loginAttempt(t, "203.0.113.20:1"))
		if res.Code == http.StatusTooManyRequests {
			t.Fatalf("exhausted the budget for 203.0.113.20 too early")
		}
	}
	// A different IP has its own, untouched budget.
	res := httptest.NewRecorder()
	h.ServeHTTP(res, loginAttempt(t, "203.0.113.21:1"))
	if res.Code == http.StatusTooManyRequests {
		t.Error("a different client IP must not share the exhausted budget")
	}
}

func TestAuthRateLimitDisabledInTestEnv(t *testing.T) {
	t.Parallel()
	h := NewHandler(Deps{Config: testConfig(), Store: memory.New()})

	var last *httptest.ResponseRecorder
	for range authRateLimitRequests + 5 {
		last = httptest.NewRecorder()
		h.ServeHTTP(last, loginAttempt(t, "203.0.113.10:12345"))
	}
	if last.Code == http.StatusTooManyRequests {
		t.Error("rate limiting must be disabled under APP_ENV=test (D-55)")
	}
}

func TestAuthRateLimitDoesNotAffectOtherRoutes(t *testing.T) {
	t.Parallel()
	h := NewHandler(Deps{Config: devConfig(), Store: memory.New()})
	for i := range authRateLimitRequests + 5 {
		req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
		req.RemoteAddr = "203.0.113.11:12345"
		res := httptest.NewRecorder()
		h.ServeHTTP(res, req)
		if res.Code == http.StatusTooManyRequests {
			t.Fatalf("health should never be rate-limited, got 429 at request %d", i)
		}
	}
}
