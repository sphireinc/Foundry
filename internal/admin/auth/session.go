package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

type Session struct {
	ID           string    `yaml:"id,omitempty"`
	Token        string    `yaml:"-"`
	TokenHash    string    `yaml:"token_hash,omitempty"`
	LegacyToken  string    `yaml:"token,omitempty"`
	CSRFToken    string    `yaml:"csrf_token"`
	Username     string    `yaml:"username"`
	Name         string    `yaml:"name"`
	Email        string    `yaml:"email"`
	Role         string    `yaml:"role"`
	Capabilities []string  `yaml:"capabilities,omitempty"`
	MFAComplete  bool      `yaml:"mfa_complete,omitempty"`
	RemoteAddr   string    `yaml:"remote_addr,omitempty"`
	UserAgent    string    `yaml:"user_agent,omitempty"`
	IssuedAt     time.Time `yaml:"issued_at"`
	LastSeen     time.Time `yaml:"last_seen"`
	ExpiresAt    time.Time `yaml:"expires_at"`
}

type SessionIssueMeta struct {
	RemoteAddr string
	UserAgent  string
}

type SessionSummary struct {
	ID          string
	Username    string
	Name        string
	Email       string
	Role        string
	MFAComplete bool
	RemoteAddr  string
	UserAgent   string
	IssuedAt    time.Time
	LastSeen    time.Time
	ExpiresAt   time.Time
	Current     bool
}

type sessionFile struct {
	Sessions []Session `yaml:"sessions"`
}

type SessionManager struct {
	path              string
	secret            string
	ttl               time.Duration
	idleTimeout       time.Duration
	maxAge            time.Duration
	singleSessionUser bool
	mu                sync.Mutex
	sessions          map[string]Session
}

func NewSessionManager(path string, ttl, idleTimeout, maxAge time.Duration, secret string, singleSessionUser bool) *SessionManager {
	if ttl <= 0 {
		ttl = 30 * time.Minute
	}
	m := &SessionManager{
		path:              path,
		secret:            stringsTrimSpace(secret),
		ttl:               ttl,
		idleTimeout:       idleTimeout,
		maxAge:            maxAge,
		singleSessionUser: singleSessionUser,
		sessions:          make(map[string]Session),
	}
	_ = m.load()
	return m
}

func (m *SessionManager) Issue(identity Identity, meta SessionIssueMeta, now time.Time) (Session, error) {
	token, err := randomToken()
	if err != nil {
		return Session{}, err
	}
	csrfToken, err := randomToken()
	if err != nil {
		return Session{}, err
	}

	session := Session{
		ID:           sessionID(now),
		Token:        token,
		TokenHash:    m.hashToken(token),
		CSRFToken:    csrfToken,
		Username:     identity.Username,
		Name:         identity.Name,
		Email:        identity.Email,
		Role:         identity.Role,
		Capabilities: append([]string(nil), identity.Capabilities...),
		MFAComplete:  identity.MFAComplete,
		RemoteAddr:   normalizeRemoteAddr(meta.RemoteAddr),
		UserAgent:    normalizeUserAgent(meta.UserAgent),
		IssuedAt:     now,
		LastSeen:     now,
		ExpiresAt:    now.Add(m.ttl),
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.singleSessionUser {
		for tokenHash, existing := range m.sessions {
			if normalizeUsername(existing.Username) == normalizeUsername(session.Username) {
				delete(m.sessions, tokenHash)
			}
		}
	}
	session.ExpiresAt = m.nextExpiryLocked(session, now)
	m.sessions[session.TokenHash] = session
	if err := m.saveLocked(); err != nil {
		delete(m.sessions, session.TokenHash)
		return Session{}, err
	}
	return session, nil
}

func (m *SessionManager) Authenticate(token string, now time.Time) (Session, bool) {
	session, ok, _ := m.authenticateDetailed(token, now)
	return session, ok
}

func (m *SessionManager) AuthenticateDetailed(token string, now time.Time) (Session, bool, string) {
	return m.authenticateDetailed(token, now)
}

func (m *SessionManager) authenticateDetailed(token string, now time.Time) (Session, bool, string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	sessionHash := m.hashToken(token)
	session, ok := m.sessions[sessionHash]
	if !ok {
		return Session{}, false, ""
	}
	if m.maxAge > 0 && !session.IssuedAt.IsZero() && now.Sub(session.IssuedAt) > m.maxAge {
		delete(m.sessions, sessionHash)
		_ = m.saveLocked()
		return Session{}, false, "maximum lifetime"
	}
	lastSeen := session.LastSeen
	if lastSeen.IsZero() {
		lastSeen = session.IssuedAt
	}
	if m.idleTimeout > 0 && !lastSeen.IsZero() && now.Sub(lastSeen) > m.idleTimeout {
		delete(m.sessions, sessionHash)
		_ = m.saveLocked()
		return Session{}, false, "inactivity"
	}
	if now.After(session.ExpiresAt) {
		delete(m.sessions, sessionHash)
		_ = m.saveLocked()
		return Session{}, false, "expiration"
	}

	session.LastSeen = now
	session.ExpiresAt = m.nextExpiryLocked(session, now)
	session.Token = token
	session.TokenHash = sessionHash
	m.sessions[sessionHash] = session
	_ = m.saveLocked()
	return session, true, ""
}

func (m *SessionManager) Revoke(token string) {
	if token == "" {
		return
	}
	m.mu.Lock()
	delete(m.sessions, m.hashToken(token))
	_ = m.saveLocked()
	m.mu.Unlock()
}

func (m *SessionManager) RevokeUser(username string) int {
	username = normalizeUsername(username)
	if username == "" {
		return 0
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	count := 0
	for token, session := range m.sessions {
		if normalizeUsername(session.Username) == username {
			delete(m.sessions, token)
			count++
		}
	}
	if count > 0 {
		_ = m.saveLocked()
	}
	return count
}

func (m *SessionManager) RevokeAll() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	count := len(m.sessions)
	m.sessions = make(map[string]Session)
	_ = m.saveLocked()
	return count
}

func (m *SessionManager) RevokeSessionID(id string) bool {
	id = stringsTrimSpace(id)
	if id == "" {
		return false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for tokenHash, session := range m.sessions {
		if stringsTrimSpace(session.ID) == id {
			delete(m.sessions, tokenHash)
			_ = m.saveLocked()
			return true
		}
	}
	return false
}

func (m *SessionManager) List(username, currentToken string, now time.Time) []SessionSummary {
	currentHash := m.hashToken(currentToken)
	username = normalizeUsername(username)
	m.mu.Lock()
	defer m.mu.Unlock()

	dirty := false
	out := make([]SessionSummary, 0, len(m.sessions))
	for tokenHash, session := range m.sessions {
		if now.After(session.ExpiresAt) {
			delete(m.sessions, tokenHash)
			dirty = true
			continue
		}
		if username != "" && normalizeUsername(session.Username) != username {
			continue
		}
		if stringsTrimSpace(session.ID) == "" {
			session.ID = legacySessionID(tokenHash)
			m.sessions[tokenHash] = session
			dirty = true
		}
		out = append(out, SessionSummary{
			ID:          session.ID,
			Username:    session.Username,
			Name:        session.Name,
			Email:       session.Email,
			Role:        session.Role,
			MFAComplete: session.MFAComplete,
			RemoteAddr:  session.RemoteAddr,
			UserAgent:   session.UserAgent,
			IssuedAt:    session.IssuedAt,
			LastSeen:    session.LastSeen,
			ExpiresAt:   session.ExpiresAt,
			Current:     tokenHash == currentHash && currentHash != "",
		})
	}
	if dirty {
		_ = m.saveLocked()
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Current != out[j].Current {
			return out[i].Current
		}
		return out[i].LastSeen.After(out[j].LastSeen)
	})
	return out
}

func (m *SessionManager) TTL() time.Duration {
	return m.ttl
}

func (m *SessionManager) IdleTimeout() time.Duration {
	return m.idleTimeout
}

func (m *SessionManager) MaxAge() time.Duration {
	return m.maxAge
}

func (m *SessionManager) load() error {
	if stringsTrimSpace(m.path) == "" {
		return nil
	}
	body, err := os.ReadFile(m.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var file sessionFile
	if err := yaml.Unmarshal(body, &file); err != nil {
		return err
	}
	now := time.Now()
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, session := range file.Sessions {
		tokenHash := stringsTrimSpace(session.TokenHash)
		if tokenHash == "" && stringsTrimSpace(session.LegacyToken) != "" {
			tokenHash = m.hashToken(session.LegacyToken)
		}
		if tokenHash == "" || now.After(session.ExpiresAt) {
			continue
		}
		if m.maxAge > 0 && !session.IssuedAt.IsZero() && now.Sub(session.IssuedAt) > m.maxAge {
			continue
		}
		lastSeen := session.LastSeen
		if lastSeen.IsZero() {
			lastSeen = session.IssuedAt
		}
		if m.idleTimeout > 0 && !lastSeen.IsZero() && now.Sub(lastSeen) > m.idleTimeout {
			continue
		}
		if stringsTrimSpace(session.ID) == "" {
			session.ID = legacySessionID(tokenHash)
		}
		session.RemoteAddr = normalizeRemoteAddr(session.RemoteAddr)
		session.UserAgent = normalizeUserAgent(session.UserAgent)
		session.Token = ""
		session.LegacyToken = ""
		session.TokenHash = tokenHash
		m.sessions[tokenHash] = session
	}
	return nil
}

func (m *SessionManager) saveLocked() error {
	if stringsTrimSpace(m.path) == "" {
		return nil
	}
	sessions := make([]Session, 0, len(m.sessions))
	for _, session := range m.sessions {
		session.Token = ""
		session.LegacyToken = ""
		sessions = append(sessions, session)
	}
	body, err := yaml.Marshal(sessionFile{Sessions: sessions})
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(m.path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(m.path, body, 0o600)
}

func randomToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func sessionID(now time.Time) string {
	buf := make([]byte, 12)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("sess-%d", now.UnixNano())
	}
	return "sess-" + strconv.FormatInt(now.Unix(), 36) + "-" + base64.RawURLEncoding.EncodeToString(buf)
}

func legacySessionID(tokenHash string) string {
	tokenHash = stringsTrimSpace(tokenHash)
	if tokenHash == "" {
		return ""
	}
	if len(tokenHash) > 16 {
		tokenHash = tokenHash[:16]
	}
	return "sess-legacy-" + tokenHash
}

func (m *SessionManager) hashToken(token string) string {
	token = stringsTrimSpace(token)
	if token == "" {
		return ""
	}
	if m == nil || stringsTrimSpace(m.secret) == "" {
		sum := sha256.Sum256([]byte(token))
		return base64.RawURLEncoding.EncodeToString(sum[:])
	}
	mac := hmac.New(sha256.New, []byte(m.secret))
	_, _ = mac.Write([]byte(token))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (m *SessionManager) nextExpiryLocked(session Session, now time.Time) time.Time {
	candidates := make([]time.Time, 0, 3)
	if m.ttl > 0 {
		candidates = append(candidates, now.Add(m.ttl))
	}
	if m.idleTimeout > 0 {
		candidates = append(candidates, now.Add(m.idleTimeout))
	}
	if m.maxAge > 0 && !session.IssuedAt.IsZero() {
		candidates = append(candidates, session.IssuedAt.Add(m.maxAge))
	}
	if len(candidates) == 0 {
		return now
	}
	earliest := candidates[0]
	for _, candidate := range candidates[1:] {
		if candidate.Before(earliest) {
			earliest = candidate
		}
	}
	return earliest
}

func normalizeUsername(value string) string {
	return stringsTrimSpaceLower(value)
}

func stringsTrimSpace(value string) string {
	return strings.TrimSpace(value)
}

func stringsTrimSpaceLower(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func normalizeRemoteAddr(value string) string {
	value = stringsTrimSpace(value)
	if value == "" {
		return ""
	}
	if host, _, err := net.SplitHostPort(value); err == nil && stringsTrimSpace(host) != "" {
		return stringsTrimSpace(host)
	}
	return value
}

func normalizeUserAgent(value string) string {
	value = strings.Join(strings.Fields(stringsTrimSpace(value)), " ")
	if len(value) > 200 {
		return value[:200]
	}
	return value
}
