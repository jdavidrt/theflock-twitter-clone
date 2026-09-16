package httpapi

import (
	"bytes"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// bufferLogger returns a logger writing text lines into buf, so tests can assert on what was logged.
func bufferLogger(buf *bytes.Buffer) *slog.Logger {
	return slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

func TestRecoverPanicRespondsWithInternalEnvelopeAndLogs(t *testing.T) {
	t.Parallel()
	var logs bytes.Buffer
	boom := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("kaboom") })
	h := requestLog(bufferLogger(&logs))(recoverPanic(bufferLogger(&logs))(boom))

	res := httptest.NewRecorder()
	h.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/api/explode", nil))

	if res.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", res.Code)
	}
	body := res.Body.String()
	if !strings.Contains(body, CodeInternal) {
		t.Errorf("body should carry the INTERNAL code, got %s", body)
	}
	if strings.Contains(body, "kaboom") {
		t.Errorf("panic value must not leak to the client, got %s", body)
	}
	logged := logs.String()
	if !strings.Contains(logged, "panic recovered") || !strings.Contains(logged, "kaboom") {
		t.Errorf("panic should be logged server-side with its value, got %q", logged)
	}
	if !strings.Contains(logged, "status=500") {
		t.Errorf("request log should record the 500 written by the recover middleware, got %q", logged)
	}
}

func TestPanicInsideTheRealHandlerChainIsRecovered(t *testing.T) {
	t.Parallel()
	// NewHandler has no panicking route, so exercise the same chain order with one injected.
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/panic", func(http.ResponseWriter, *http.Request) { panic(errors.New("nope")) })
	var h http.Handler = mux
	h = securityHeaders(h)
	h = recoverPanic(slog.New(slog.NewTextHandler(io.Discard, nil)))(h)

	res := httptest.NewRecorder()
	h.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/api/panic", nil))
	if res.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", res.Code)
	}
	if res.Header().Get("X-Frame-Options") != "DENY" {
		t.Error("security headers should still be set on a recovered response")
	}
}

func TestRequestLogRecordsMethodPathAndStatus(t *testing.T) {
	t.Parallel()
	var logs bytes.Buffer
	created := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, "made")
	})
	h := requestLog(bufferLogger(&logs))(created)

	res := httptest.NewRecorder()
	h.ServeHTTP(res, httptest.NewRequest(http.MethodPost, "/api/things", nil))

	if res.Code != http.StatusCreated || res.Body.String() != "made" {
		t.Fatalf("middleware altered the response: %d %q", res.Code, res.Body.String())
	}
	for _, want := range []string{"method=POST", "path=/api/things", "status=201", "duration="} {
		if !strings.Contains(logs.String(), want) {
			t.Errorf("log line %q should contain %q", logs.String(), want)
		}
	}
}

func TestRequestLogDefaultsStatusTo200(t *testing.T) {
	t.Parallel()
	var logs bytes.Buffer
	implicit := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "no explicit WriteHeader")
	})
	requestLog(bufferLogger(&logs))(implicit).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	if !strings.Contains(logs.String(), "status=200") {
		t.Errorf("implicit 200 should be logged, got %q", logs.String())
	}

	logs.Reset()
	silent := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	requestLog(bufferLogger(&logs))(silent).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	if !strings.Contains(logs.String(), "status=200") {
		t.Errorf("a handler that writes nothing is a 200, got %q", logs.String())
	}
}

func TestStatusRecorderKeepsFirstStatus(t *testing.T) {
	t.Parallel()
	rec := &statusRecorder{ResponseWriter: httptest.NewRecorder()}
	rec.WriteHeader(http.StatusTeapot)
	rec.WriteHeader(http.StatusOK) // net/http ignores the second call; so must the recorder
	if rec.Status() != http.StatusTeapot {
		t.Errorf("Status() = %d, want 418", rec.Status())
	}
}

func TestLimitBodyRejectsOversizedBodies(t *testing.T) {
	t.Parallel()
	const limit = 8
	var readErr error
	reader := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, readErr = io.ReadAll(r.Body)
		if readErr != nil {
			w.WriteHeader(http.StatusRequestEntityTooLarge)
		}
	})
	h := limitBody(limit)(reader)

	res := httptest.NewRecorder()
	h.ServeHTTP(res, httptest.NewRequest(http.MethodPost, "/", strings.NewReader("this body is longer than eight bytes")))
	var maxErr *http.MaxBytesError
	if !errors.As(readErr, &maxErr) {
		t.Fatalf("expected *http.MaxBytesError, got %v", readErr)
	}
	if maxErr.Limit != limit {
		t.Errorf("limit = %d, want %d", maxErr.Limit, limit)
	}
	if res.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("status = %d, want 413", res.Code)
	}

	readErr = nil
	res = httptest.NewRecorder()
	h.ServeHTTP(res, httptest.NewRequest(http.MethodPost, "/", strings.NewReader("tiny")))
	if readErr != nil || res.Code != http.StatusOK {
		t.Errorf("a body under the limit must read fine, got err=%v status=%d", readErr, res.Code)
	}
}

func TestLimitBodyLeavesNilBodyAlone(t *testing.T) {
	t.Parallel()
	called := false
	h := limitBody(1)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		called = true
		if r.Body != nil && r.Body != http.NoBody {
			t.Errorf("expected no body, got %T", r.Body)
		}
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Body = nil
	h.ServeHTTP(httptest.NewRecorder(), req)
	if !called {
		t.Error("next handler was not called")
	}
}

func TestNewHandlerDefaultsToSilentLogger(t *testing.T) {
	t.Parallel()
	// Must not panic with a nil Logger, and must still serve.
	res := httptest.NewRecorder()
	NewHandler(Deps{Config: testConfig(), Logger: nil}).ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/api/health", nil))
	if res.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", res.Code)
	}
	if MaxBodyBytes != 16*1024 {
		t.Errorf("MaxBodyBytes = %d, want 16 kB (D-55)", MaxBodyBytes)
	}
}
