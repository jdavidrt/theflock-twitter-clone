// Package httpapi is the HTTP layer of the API: routing, middlewares and JSON responses (D-53).
// Handlers decode and validate requests, call services and write responses; they never touch
// storage directly.
package httpapi

import (
	"io"
	"log/slog"
	"net/http"

	"github.com/jdavidrt/theflock-twitter-clone/server/internal/config"
	"github.com/jdavidrt/theflock-twitter-clone/server/internal/service/auth"
	"github.com/jdavidrt/theflock-twitter-clone/server/internal/service/social"
	"github.com/jdavidrt/theflock-twitter-clone/server/internal/service/tweet"
	"github.com/jdavidrt/theflock-twitter-clone/server/internal/store"
)

// MaxBodyBytes caps request bodies (D-55).
const MaxBodyBytes = 16 << 10

// Deps are the dependencies handlers need. More arrive with each feature step (services).
type Deps struct {
	Config config.Config
	Store  store.Store
	// Logger receives request and panic logs. nil means silent, which is what tests want.
	Logger *slog.Logger
}

// NewHandler builds the API's root handler: routes on the Go 1.22 ServeMux, wrapped by the
// middleware chain (outermost first): request log → recover → security headers → auth rate
// limit → JSON-only → body limit.
func NewHandler(deps Deps) http.Handler {
	logger := deps.Logger
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	authSvc := auth.New(deps.Store, deps.Config.BcryptCost(), deps.Config.JWTSecret)
	socialSvc := social.New(deps.Store)
	tweetSvc := tweet.New(deps.Store)
	requireUser := requireAuth(authSvc)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", handleHealth)
	mux.HandleFunc("POST /api/auth/register", handleRegister(deps, authSvc))
	mux.HandleFunc("POST /api/auth/login", handleLogin(deps, authSvc))
	mux.Handle("POST /api/auth/logout", requireUser(handleLogout(deps)))
	mux.Handle("GET /api/auth/me", requireUser(handleMe(deps)))
	mux.Handle("GET /api/users/{username}", requireUser(handleGetProfile(deps, socialSvc)))
	mux.Handle("PATCH /api/users/me", requireUser(handleUpdateMe(deps, socialSvc)))
	mux.Handle("POST /api/users/{username}/follow", requireUser(handleFollow(deps, socialSvc)))
	mux.Handle("DELETE /api/users/{username}/follow", requireUser(handleUnfollow(deps, socialSvc)))
	mux.Handle("GET /api/users/{username}/followers", requireUser(handleFollowList(deps, socialSvc.ListFollowers)))
	mux.Handle("GET /api/users/{username}/following", requireUser(handleFollowList(deps, socialSvc.ListFollowing)))
	mux.Handle("GET /api/search/users", requireUser(handleSearchUsers(deps, socialSvc)))
	mux.Handle("GET /api/users/{username}/tweets", requireUser(handleListUserTweets(deps, tweetSvc)))
	mux.Handle("POST /api/tweets", requireUser(handleCreateTweet(deps, tweetSvc)))
	mux.Handle("GET /api/tweets/{id}", requireUser(handleGetTweet(deps, tweetSvc)))
	mux.Handle("DELETE /api/tweets/{id}", requireUser(handleDeleteTweet(deps, tweetSvc)))
	mux.Handle("POST /api/tweets/{id}/like", requireUser(handleLikeAction(deps, tweetSvc.Like)))
	mux.Handle("DELETE /api/tweets/{id}/like", requireUser(handleLikeAction(deps, tweetSvc.Unlike)))
	mux.Handle("GET /api/timeline", requireUser(handleTimeline(deps, tweetSvc)))
	// Anything else under /api answers with the JSON envelope instead of net/http's text 404.
	mux.HandleFunc("/api/", handleNotFound)

	limiter := newIPRateLimiter()

	var h http.Handler = mux
	h = limitBody(MaxBodyBytes)(h)
	h = jsonOnly(h)
	h = authRateLimit(deps.Config, limiter)(h)
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
