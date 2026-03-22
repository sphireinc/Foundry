package auth

import (
	"crypto/subtle"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/sphireinc/foundry/internal/admin/users"
	"github.com/sphireinc/foundry/internal/config"
)

type Middleware struct {
	cfg            *config.Config
	sessions       *SessionManager
	loginThrottler *loginThrottler
}

func New(cfg *config.Config) *Middleware {
	ttl := 30 * time.Minute
	sessionStorePath := ""
	if cfg != nil && cfg.Admin.SessionTTLMinutes > 0 {
		ttl = time.Duration(cfg.Admin.SessionTTLMinutes) * time.Minute
	}
	if cfg != nil {
		sessionStorePath = cfg.Admin.SessionStoreFile
	}
	return &Middleware{
		cfg:            cfg,
		sessions:       NewSessionManager(sessionStorePath, ttl),
		loginThrottler: newLoginThrottler(),
	}
}

type Identity struct {
	Username     string   `json:"username"`
	Name         string   `json:"name"`
	Email        string   `json:"email"`
	Role         string   `json:"role,omitempty"`
	Capabilities []string `json:"capabilities,omitempty"`
	MFAComplete  bool     `json:"mfa_complete,omitempty"`
	CSRFToken    string   `json:"csrf_token,omitempty"`
}

func (m *Middleware) Authorize(r *http.Request) error {
	_, _, err := m.authorizeRequest(r)
	return err
}

func (m *Middleware) Authenticate(w http.ResponseWriter, r *http.Request) (*Identity, error) {
	identity, sessionToken, err := m.authorizeRequest(r)
	if err != nil {
		return nil, err
	}
	if w != nil && sessionToken != "" {
		m.setSessionCookie(w, r, sessionToken)
	}
	return identity, nil
}

func (m *Middleware) Login(w http.ResponseWriter, r *http.Request, username, password, totpCode string) (*Identity, error) {
	if err := m.checkAccess(r); err != nil {
		return nil, err
	}
	if err := m.loginThrottler.Allow(r, username, time.Now()); err != nil {
		return nil, err
	}

	user, err := users.Find(m.cfg.Admin.UsersFile, username)
	if err != nil {
		m.loginThrottler.Failure(r, username, time.Now())
		return nil, fmt.Errorf("invalid username or password")
	}
	if user.Disabled || !users.VerifyPassword(user.PasswordHash, password) {
		m.loginThrottler.Failure(r, username, time.Now())
		return nil, fmt.Errorf("invalid username or password")
	}
	if user.TOTPEnabled && !VerifyTOTP(user.TOTPSecret, totpCode, time.Now()) {
		m.loginThrottler.Failure(r, username, time.Now())
		return nil, fmt.Errorf("two-factor authentication code is required")
	}
	m.loginThrottler.Success(r, username)

	identity := Identity{
		Username:     user.Username,
		Name:         user.Name,
		Email:        user.Email,
		Role:         normalizeRole(user.Role),
		Capabilities: capabilitiesFor(user.Role, user.Capabilities),
		MFAComplete:  !user.TOTPEnabled || VerifyTOTP(user.TOTPSecret, totpCode, time.Now()),
	}
	session, err := m.sessions.Issue(identity, time.Now())
	if err != nil {
		return nil, err
	}
	identity.CSRFToken = session.CSRFToken
	m.setSessionCookie(w, r, session.Token)
	return &identity, nil
}

func (m *Middleware) Logout(w http.ResponseWriter, r *http.Request) error {
	if err := m.checkAccess(r); err != nil {
		return err
	}
	if cookie, err := r.Cookie(sessionCookieName); err == nil {
		m.sessions.Revoke(strings.TrimSpace(cookie.Value))
	}
	m.clearSessionCookie(w, r)
	return nil
}

func (m *Middleware) SessionTTL() time.Duration {
	if m == nil || m.sessions == nil {
		return 30 * time.Minute
	}
	return m.sessions.TTL()
}

func (m *Middleware) RevokeUserSessions(username string) int {
	if m == nil || m.sessions == nil {
		return 0
	}
	return m.sessions.RevokeUser(username)
}

func (m *Middleware) RevokeAllSessions() int {
	if m == nil || m.sessions == nil {
		return 0
	}
	return m.sessions.RevokeAll()
}

func (m *Middleware) authorizeRequest(r *http.Request) (*Identity, string, error) {
	if m == nil || m.cfg == nil {
		return nil, "", nil
	}
	if err := m.checkAccess(r); err != nil {
		return nil, "", err
	}

	if sessionToken := extractSessionToken(r); sessionToken != "" {
		session, ok := m.sessions.Authenticate(sessionToken, time.Now())
		if ok {
			return &Identity{
				Username:     session.Username,
				Name:         session.Name,
				Email:        session.Email,
				Role:         normalizeRole(session.Role),
				Capabilities: append([]string(nil), session.Capabilities...),
				MFAComplete:  session.MFAComplete,
				CSRFToken:    session.CSRFToken,
			}, session.Token, nil
		}
		return nil, "", fmt.Errorf("admin session expired")
	}

	token := strings.TrimSpace(m.cfg.Admin.AccessToken)
	if token != "" && tokensEqual(extractAccessToken(r), token) {
		return &Identity{
			Username:     "api-token",
			Name:         "API Token",
			Role:         "admin",
			Capabilities: capabilitiesFor("admin", nil),
			MFAComplete:  true,
		}, "", nil
	}

	if strings.TrimSpace(extractAccessToken(r)) != "" {
		return nil, "", fmt.Errorf("invalid admin access token")
	}
	return nil, "", fmt.Errorf("admin login is required")
}

func (m *Middleware) checkAccess(r *http.Request) error {
	if m == nil || m.cfg == nil {
		return nil
	}
	if !m.cfg.Admin.Enabled {
		return fmt.Errorf("admin is disabled")
	}
	if m.cfg.Admin.LocalOnly && !isLocalRequest(r) {
		return fmt.Errorf("admin is restricted to local requests")
	}
	return nil
}

func isLocalRequest(r *http.Request) bool {
	if r == nil {
		return false
	}
	if hasProxyHeaders(r) {
		return false
	}

	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err != nil {
		host = strings.TrimSpace(r.RemoteAddr)
	}

	ip := net.ParseIP(host)
	if ip == nil {
		return host == "localhost"
	}

	return ip.IsLoopback()
}

func hasProxyHeaders(r *http.Request) bool {
	for _, name := range []string{"Forwarded", "X-Forwarded-For", "X-Forwarded-Host", "X-Forwarded-Proto", "X-Real-IP"} {
		if strings.TrimSpace(r.Header.Get(name)) != "" {
			return true
		}
	}
	return false
}

func extractAccessToken(r *http.Request) string {
	if r == nil {
		return ""
	}

	if token := strings.TrimSpace(r.Header.Get("X-Foundry-Admin-Token")); token != "" {
		return token
	}

	authz := strings.TrimSpace(r.Header.Get("Authorization"))
	const bearerPrefix = "Bearer "
	if strings.HasPrefix(authz, bearerPrefix) {
		return strings.TrimSpace(authz[len(bearerPrefix):])
	}

	return ""
}

func extractSessionToken(r *http.Request) string {
	if r == nil {
		return ""
	}
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(cookie.Value)
}

func tokensEqual(got, want string) bool {
	if len(got) != len(want) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}
