package httpapi

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/jdavidrt/theflock-twitter-clone/server/internal/domain"
	"github.com/jdavidrt/theflock-twitter-clone/server/internal/service/auth"
)

type registerRequest struct {
	Email       string `json:"email"`
	Username    string `json:"username"`
	Password    string `json:"password"`
	DisplayName string `json:"displayName"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// handleRegister is POST /api/auth/register (D-01…D-06). On success it sets the session
// cookie the same as login, so a freshly registered client is immediately authenticated.
func handleRegister(deps Deps, svc *auth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req registerRequest
		if !decodeJSON(w, r, &req) {
			return
		}
		user, err := svc.Register(auth.RegisterInput{
			Email:       req.Email,
			Username:    req.Username,
			Password:    req.Password,
			DisplayName: req.DisplayName,
		})
		if err != nil {
			writeAuthError(w, deps.Logger, err)
			return
		}
		issueSessionAndRespond(w, deps, svc, user, http.StatusCreated)
	}
}

// handleLogin is POST /api/auth/login (D-07/D-08).
func handleLogin(deps Deps, svc *auth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req loginRequest
		if !decodeJSON(w, r, &req) {
			return
		}
		user, err := svc.Login(req.Email, req.Password)
		if err != nil {
			writeAuthError(w, deps.Logger, err)
			return
		}
		issueSessionAndRespond(w, deps, svc, user, http.StatusOK)
	}
}

// handleLogout is POST /api/auth/logout, behind requireAuth (D-11). It only clears the cookie
// (D-10) — there is no server-side revocation list.
func handleLogout(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		clearSessionCookie(w, deps.Config)
		w.WriteHeader(http.StatusNoContent)
	}
}

// handleMe is GET /api/auth/me, behind requireAuth (D-11).
func handleMe(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _ := userIDFromContext(r.Context())
		user, err := deps.Store.GetUserByID(userID)
		if err != nil {
			// The token verified but its subject no longer resolves to a user — treat it the
			// same as an absent session rather than leaking a 404/500 for a valid-looking cookie.
			writeError(w, http.StatusUnauthorized, CodeUnauthorized, "Authentication required", nil)
			return
		}
		writeJSON(w, http.StatusOK, user)
	}
}

// issueSessionAndRespond signs a session token, sets the cookie and writes the user as the
// response body — the shared tail of register and login.
func issueSessionAndRespond(w http.ResponseWriter, deps Deps, svc *auth.Service, user domain.User, status int) {
	token, err := svc.IssueToken(user.ID)
	if err != nil {
		if deps.Logger != nil {
			deps.Logger.Error("issuing session token", "error", err)
		}
		writeError(w, http.StatusInternalServerError, CodeInternal, "Internal server error", nil)
		return
	}
	setSessionCookie(w, deps.Config, token)
	writeJSON(w, status, user)
}

// writeAuthError maps a Register/Login error onto the D-52 envelope.
func writeAuthError(w http.ResponseWriter, logger *slog.Logger, err error) {
	var vErr *auth.ValidationError
	var cErr *auth.ConflictError
	switch {
	case errors.As(err, &vErr):
		writeError(w, http.StatusBadRequest, CodeValidation, "Validation failed", vErr.Fields)
	case errors.As(err, &cErr):
		writeError(w, http.StatusConflict, CodeConflict, cErr.Field+" is already taken", map[string][]string{cErr.Field: {"is already taken"}})
	case errors.Is(err, auth.ErrInvalidCredentials):
		writeError(w, http.StatusUnauthorized, CodeUnauthorized, "Invalid email or password", nil)
	default:
		if logger != nil {
			logger.Error("auth service error", "error", err)
		}
		writeError(w, http.StatusInternalServerError, CodeInternal, "Internal server error", nil)
	}
}
