package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jdavidrt/theflock-twitter-clone/server/internal/config"
)

func testConfig() config.Config {
	return config.Config{
		Port:      3000,
		AppEnv:    config.EnvTest,
		JWTSecret: strings.Repeat("t", config.MinJWTSecretLen),
	}
}

func newTestHandler(t *testing.T) http.Handler {
	t.Helper()
	return NewHandler(Deps{Config: testConfig()})
}

func decodeBody(t *testing.T, res *httptest.ResponseRecorder, into any) {
	t.Helper()
	if ct := res.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	if err := json.NewDecoder(res.Body).Decode(into); err != nil {
		t.Fatalf("response body is not valid JSON: %v", err)
	}
}

func TestHealth(t *testing.T) {
	t.Parallel()
	res := httptest.NewRecorder()
	newTestHandler(t).ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/api/health", nil))

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.Code)
	}
	var body map[string]string
	decodeBody(t, res, &body)
	if body["status"] != "ok" {
		t.Errorf(`body = %v, want {"status":"ok"}`, body)
	}
}

func TestSecurityHeadersOnEveryResponse(t *testing.T) {
	t.Parallel()
	want := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"Referrer-Policy":        "no-referrer",
		"Cache-Control":          "no-store",
	}
	for _, path := range []string{"/api/health", "/api/does-not-exist"} {
		res := httptest.NewRecorder()
		newTestHandler(t).ServeHTTP(res, httptest.NewRequest(http.MethodGet, path, nil))
		for header, value := range want {
			if got := res.Header().Get(header); got != value {
				t.Errorf("%s: header %s = %q, want %q", path, header, got, value)
			}
		}
	}
}

func TestUnknownAPIRouteReturnsJSONEnvelope(t *testing.T) {
	t.Parallel()
	res := httptest.NewRecorder()
	newTestHandler(t).ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/api/nope", nil))

	if res.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", res.Code)
	}
	var body ErrorBody
	decodeBody(t, res, &body)
	if body.Error.Code != CodeNotFound {
		t.Errorf("error.code = %q, want %q", body.Error.Code, CodeNotFound)
	}
	if body.Error.Message == "" {
		t.Error("error.message must not be empty")
	}
	if body.Error.Details != nil {
		t.Errorf("error.details should be omitted, got %v", body.Error.Details)
	}
}

func TestErrorEnvelopeOmitsDetailsWhenNil(t *testing.T) {
	t.Parallel()
	res := httptest.NewRecorder()
	writeError(res, http.StatusConflict, CodeConflict, "taken", nil)

	if res.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", res.Code)
	}
	if strings.Contains(res.Body.String(), "details") {
		t.Errorf("nil details must be omitted from the JSON, got %s", res.Body.String())
	}

	res = httptest.NewRecorder()
	writeError(res, http.StatusBadRequest, CodeValidation, "invalid", map[string][]string{"email": {"is required"}})
	var body ErrorBody
	decodeBody(t, res, &body)
	if got := body.Error.Details["email"]; len(got) != 1 || got[0] != "is required" {
		t.Errorf("details = %v, want email: [is required]", body.Error.Details)
	}
}
