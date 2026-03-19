package auth

import (
	"net/http"
)

const sessionCookieName = "foundry_admin_session"

func (m *Middleware) setSessionCookie(w http.ResponseWriter, token string) {
	if w == nil || token == "" {
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/__admin",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(m.SessionTTL().Seconds()),
	})
}

func (m *Middleware) clearSessionCookie(w http.ResponseWriter) {
	if w == nil {
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/__admin",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}
