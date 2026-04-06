package auth

import (
	"net/http"
	"strings"
)

const sessionCookieName = "foundry_admin_session"

func (m *Middleware) setSessionCookie(w http.ResponseWriter, r *http.Request, token string) {
	if w == nil || token == "" {
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     m.adminPath(),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   true,
		MaxAge:   int(m.SessionTTL().Seconds()),
	})
}

func (m *Middleware) clearSessionCookie(w http.ResponseWriter, r *http.Request) {
	if w == nil {
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     m.adminPath(),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   true,
		MaxAge:   -1,
	})
}

func requestIsHTTPS(r *http.Request) bool {
	if r == nil {
		return false
	}
	if r.TLS != nil {
		return true
	}
	if strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")), "https") {
		return true
	}
	return strings.Contains(strings.ToLower(r.Header.Get("Forwarded")), "proto=https")
}

func (m *Middleware) adminPath() string {
	if m == nil || m.cfg == nil {
		return "/__admin"
	}
	return m.cfg.AdminPath()
}
