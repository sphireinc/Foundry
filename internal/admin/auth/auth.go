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
	idleTimeout := time.Duration(0)
	maxAge := time.Duration(0)
	sessionStorePath := ""
	sessionSecret := ""
	singleSessionPerUser := false
	if cfg != nil && cfg.Admin.SessionTTLMinutes > 0 {
		ttl = time.Duration(cfg.Admin.SessionTTLMinutes) * time.Minute
	}
	if cfg != nil {
		if cfg.Admin.SessionIdleTimeoutMinutes > 0 {
			idleTimeout = time.Duration(cfg.Admin.SessionIdleTimeoutMinutes) * time.Minute
		}
		if cfg.Admin.SessionMaxAgeMinutes > 0 {
			maxAge = time.Duration(cfg.Admin.SessionMaxAgeMinutes) * time.Minute
		}
		sessionStorePath = cfg.Admin.SessionStoreFile
		sessionSecret = cfg.Admin.SessionSecret
		singleSessionPerUser = cfg.Admin.SingleSessionPerUser
	}
	return &Middleware{
		cfg:            cfg,
		sessions:       NewSessionManager(sessionStorePath, ttl, idleTimeout, maxAge, sessionSecret, singleSessionPerUser),
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
	plainTOTPSecret := ""
	migratedTOTPSecret := ""
	ok, upgradedHash, err := users.VerifyPasswordWithUpgrade(user.PasswordHash, password)
	if err != nil || user.Disabled || !ok {
		m.loginThrottler.Failure(r, username, time.Now())
		return nil, fmt.Errorf("invalid username or password")
	}
	if upgradedHash != "" {
		if err := users.UpdatePasswordHash(m.cfg.Admin.UsersFile, user.Username, upgradedHash); err == nil {
			user.PasswordHash = upgradedHash
		}
	}
	if user.TOTPEnabled {
		plainTOTPSecret, migratedTOTPSecret, err = m.decodeTOTPSecret(user.TOTPSecret)
		if err != nil {
			m.loginThrottler.Failure(r, username, time.Now())
			return nil, fmt.Errorf("two-factor authentication is not available")
		}
		if !VerifyTOTP(plainTOTPSecret, totpCode, time.Now()) {
			m.loginThrottler.Failure(r, username, time.Now())
			return nil, fmt.Errorf("two-factor authentication code is required")
		}
		if migratedTOTPSecret != "" {
			_ = users.UpdateTOTPSecret(m.cfg.Admin.UsersFile, user.Username, migratedTOTPSecret)
			user.TOTPSecret = migratedTOTPSecret
		}
	}
	m.loginThrottler.Success(r, username)

	identity := Identity{
		Username:     user.Username,
		Name:         user.Name,
		Email:        user.Email,
		Role:         normalizeRole(user.Role),
		Capabilities: capabilitiesFor(user.Role, user.Capabilities),
		MFAComplete:  !user.TOTPEnabled || VerifyTOTP(plainTOTPSecret, totpCode, time.Now()),
	}
	session, err := m.sessions.Issue(identity, SessionIssueMeta{
		RemoteAddr: requestRemoteAddr(r),
		UserAgent:  requestUserAgent(r),
	}, time.Now())
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

func (m *Middleware) RevokeSessionID(id string) bool {
	if m == nil || m.sessions == nil {
		return false
	}
	return m.sessions.RevokeSessionID(id)
}

func (m *Middleware) ListSessions(username string, r *http.Request) []SessionSummary {
	if m == nil || m.sessions == nil {
		return nil
	}
	return m.sessions.List(username, extractSessionToken(r), time.Now())
}

func (m *Middleware) authorizeRequest(r *http.Request) (*Identity, string, error) {
	if m == nil || m.cfg == nil {
		return nil, "", nil
	}
	if err := m.checkAccess(r); err != nil {
		return nil, "", err
	}

	if sessionToken := extractSessionToken(r); sessionToken != "" {
		session, ok, reason := m.sessions.AuthenticateDetailed(sessionToken, time.Now())
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
		switch strings.TrimSpace(reason) {
		case "inactivity":
			return nil, "", fmt.Errorf("admin session expired due to inactivity")
		case "maximum lifetime":
			return nil, "", fmt.Errorf("admin session expired after reaching maximum lifetime")
		default:
			return nil, "", fmt.Errorf("admin session expired")
		}
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

func requestRemoteAddr(r *http.Request) string {
	if r == nil {
		return ""
	}
	return strings.TrimSpace(r.RemoteAddr)
}

func requestUserAgent(r *http.Request) string {
	if r == nil {
		return ""
	}
	return strings.TrimSpace(r.UserAgent())
}

func tokensEqual(got, want string) bool {
	if len(got) != len(want) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}
