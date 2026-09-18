package httpapi

import (
	"net/http"
	"time"

	"github.com/jdavidrt/theflock-twitter-clone/server/internal/config"
	"github.com/jdavidrt/theflock-twitter-clone/server/internal/service/auth"
)

// cookieName is the session cookie (D-09).
const cookieName = "token"

// setSessionCookie sets the httpOnly session cookie after a successful register/login (D-09).
func setSessionCookie(w http.ResponseWriter, cfg config.Config, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    token,
		Path:     "/",
		Expires:  time.Now().Add(auth.TokenTTL),
		HttpOnly: true,
		Secure:   cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

// clearSessionCookie removes the session cookie on logout (D-10).
func clearSessionCookie(w http.ResponseWriter, cfg config.Config) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}
