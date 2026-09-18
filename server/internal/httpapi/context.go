package httpapi

import (
	"context"
	"net/http"

	"github.com/jdavidrt/theflock-twitter-clone/server/internal/service/auth"
)

// ctxKey namespaces context values this package sets, so it never collides with another
// package's context key of the same underlying type.
type ctxKey int

const userIDCtxKey ctxKey = iota

// requireAuth reads and verifies the session cookie and puts the user id in the request
// context for downstream handlers; it rejects the request with 401 otherwise (D-09/D-11).
func requireAuth(svc *auth.Service) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(cookieName)
			if err != nil {
				writeError(w, http.StatusUnauthorized, CodeUnauthorized, "Authentication required", nil)
				return
			}
			userID, err := svc.VerifyToken(cookie.Value)
			if err != nil {
				writeError(w, http.StatusUnauthorized, CodeUnauthorized, "Authentication required", nil)
				return
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userIDCtxKey, userID)))
		})
	}
}

// userIDFromContext returns the authenticated user id set by requireAuth.
func userIDFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(userIDCtxKey).(string)
	return id, ok
}
