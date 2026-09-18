package httpapi

import (
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"github.com/jdavidrt/theflock-twitter-clone/server/internal/config"
)

// Login/register rate-limit budget (D-55): 20 requests per 15 minutes, per client IP.
const (
	authRateLimitRequests = 20
	authRateLimitWindow   = 15 * time.Minute
)

// ipRateLimiter hands out one token-bucket limiter per client IP, lazily created. It lives for
// the process lifetime of the handler, the same as the mux it guards.
type ipRateLimiter struct {
	mu       sync.Mutex
	limiters map[string]*rate.Limiter
}

func newIPRateLimiter() *ipRateLimiter {
	return &ipRateLimiter{limiters: make(map[string]*rate.Limiter)}
}

func (l *ipRateLimiter) allow(ip string) bool {
	l.mu.Lock()
	lim, ok := l.limiters[ip]
	if !ok {
		lim = rate.NewLimiter(rate.Every(authRateLimitWindow/authRateLimitRequests), authRateLimitRequests)
		l.limiters[ip] = lim
	}
	l.mu.Unlock()
	return lim.Allow()
}

// authRateLimit throttles POST /api/auth/login and /api/auth/register per client IP; every
// other route passes through untouched. Disabled under APP_ENV=test (D-55) so the rest of the
// suite is not rate-limited by accident.
func authRateLimit(cfg config.Config, limiter *ipRateLimiter) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if cfg.IsTest() || !isAuthRateLimitedPath(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}
			if !limiter.allow(clientIP(r)) {
				writeError(w, http.StatusTooManyRequests, CodeRateLimited, "Too many requests, try again later", nil)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func isAuthRateLimitedPath(path string) bool {
	return path == "/api/auth/login" || path == "/api/auth/register"
}

// clientIP strips the port from r.RemoteAddr, falling back to the raw value if it has none
// (as httptest's synthetic requests sometimes do).
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
