// Package httpapi is the HTTP layer of the API: routing, middlewares and JSON responses (D-53).
// Handlers decode and validate requests, call services and write responses; they never touch
// storage directly.
package httpapi

import (
	"io"
	"log/slog"
	"net/http"

	"github.com/jdavidrt/theflock-twitter-clone/server/internal/config"
)

// MaxBodyBytes caps request bodies (D-55).
const MaxBodyBytes = 16 << 10

// Deps are the dependencies handlers need. More arrive with each feature step (store, services).
type Deps struct {
	Config config.Config
	// Logger receives request and panic logs. nil means silent, which is what tests want.
	Logger *slog.Logger
}

// NewHandler builds the API's root handler: routes on the Go 1.22 ServeMux, wrapped by the
// middleware chain (outermost first): request log → recover → security headers → body limit.
func NewHandler(deps Deps) http.Handler {
	logger := deps.Logger
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", handleHealth)
	// Anything else under /api answers with the JSON envelope instead of net/http's text 404.
	mux.HandleFunc("/api/", handleNotFound)

	var h http.Handler = mux
	h = limitBody(MaxBodyBytes)(h)
	h = securityHeaders(h)
	h = recoverPanic(logger)(h)
	h = requestLog(logger)(h)
	return h
}

// handleHealth is the unauthenticated liveness probe (D-11).
func handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleNotFound answers unknown /api routes with the D-52 envelope.
func handleNotFound(w http.ResponseWriter, _ *http.Request) {
	writeError(w, http.StatusNotFound, CodeNotFound, "Route not found", nil)
}
