package httpapi

import (
	"log/slog"
	"net/http"
	"time"
)

// Middleware wraps a handler with cross-cutting behavior.
type Middleware func(http.Handler) http.Handler

// requestLog logs one line per request — method, path, status, duration (D-57).
func requestLog(log *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w}
			next.ServeHTTP(rec, r)
			log.Info("request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", rec.Status(),
				"duration", time.Since(start),
			)
		})
	}
}

// recoverPanic turns a panic in a handler into a 500 with the D-52 INTERNAL envelope, logging
// the panic value server-side without leaking it to the client (D-55).
func recoverPanic(log *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					log.Error("panic recovered", "method", r.Method, "path", r.URL.Path, "panic", rec)
					writeError(w, http.StatusInternalServerError, CodeInternal, "Internal server error", nil)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// securityHeaders sets the response headers helmet used to set for the Express version (D-55).
// The API only ever serves JSON, so framing and caching are denied outright.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

// limitBody caps the readable request body at n bytes (D-55). A handler that reads past the
// limit gets a *http.MaxBytesError and the connection is closed after the response.
func limitBody(n int64) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body != nil {
				r.Body = http.MaxBytesReader(w, r.Body, n)
			}
			next.ServeHTTP(w, r)
		})
	}
}

// statusRecorder remembers the status code written through it, for the request log.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	if s.status == 0 {
		s.status = code
	}
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Write(b []byte) (int, error) {
	if s.status == 0 {
		s.status = http.StatusOK
	}
	return s.ResponseWriter.Write(b)
}

// Status returns the recorded status, defaulting to 200 when the handler wrote nothing
// (which is what net/http sends in that case).
func (s *statusRecorder) Status() int {
	if s.status == 0 {
		return http.StatusOK
	}
	return s.status
}
