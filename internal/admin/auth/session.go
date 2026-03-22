package auth

import (
	"crypto/rand"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

type Session struct {
	Token        string    `yaml:"token"`
	CSRFToken    string    `yaml:"csrf_token"`
	Username     string    `yaml:"username"`
	Name         string    `yaml:"name"`
	Email        string    `yaml:"email"`
	Role         string    `yaml:"role"`
	Capabilities []string  `yaml:"capabilities,omitempty"`
	MFAComplete  bool      `yaml:"mfa_complete,omitempty"`
	IssuedAt     time.Time `yaml:"issued_at"`
	LastSeen     time.Time `yaml:"last_seen"`
	ExpiresAt    time.Time `yaml:"expires_at"`
}

type sessionFile struct {
	Sessions []Session `yaml:"sessions"`
}

type SessionManager struct {
	path     string
	ttl      time.Duration
	mu       sync.Mutex
	sessions map[string]Session
}

func NewSessionManager(path string, ttl time.Duration) *SessionManager {
	if ttl <= 0 {
		ttl = 30 * time.Minute
	}
	m := &SessionManager{
		path:     path,
		ttl:      ttl,
		sessions: make(map[string]Session),
	}
	_ = m.load()
	return m
}

func (m *SessionManager) Issue(identity Identity, now time.Time) (Session, error) {
	token, err := randomToken()
	if err != nil {
		return Session{}, err
	}
	csrfToken, err := randomToken()
	if err != nil {
		return Session{}, err
	}

	session := Session{
		Token:        token,
		CSRFToken:    csrfToken,
		Username:     identity.Username,
		Name:         identity.Name,
		Email:        identity.Email,
		Role:         identity.Role,
		Capabilities: append([]string(nil), identity.Capabilities...),
		MFAComplete:  identity.MFAComplete,
		IssuedAt:     now,
		LastSeen:     now,
		ExpiresAt:    now.Add(m.ttl),
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[token] = session
	if err := m.saveLocked(); err != nil {
		delete(m.sessions, token)
		return Session{}, err
	}
	return session, nil
}

func (m *SessionManager) Authenticate(token string, now time.Time) (Session, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, ok := m.sessions[token]
	if !ok {
		return Session{}, false
	}
	if now.After(session.ExpiresAt) {
		delete(m.sessions, token)
		_ = m.saveLocked()
		return Session{}, false
	}

	session.LastSeen = now
	session.ExpiresAt = now.Add(m.ttl)
	m.sessions[token] = session
	_ = m.saveLocked()
	return session, true
}

func (m *SessionManager) Revoke(token string) {
	if token == "" {
		return
	}
	m.mu.Lock()
	delete(m.sessions, token)
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

func (m *SessionManager) TTL() time.Duration {
	return m.ttl
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
		if session.Token == "" || now.After(session.ExpiresAt) {
			continue
		}
		m.sessions[session.Token] = session
	}
	return nil
}

func (m *SessionManager) saveLocked() error {
	if stringsTrimSpace(m.path) == "" {
		return nil
	}
	sessions := make([]Session, 0, len(m.sessions))
	for _, session := range m.sessions {
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

func normalizeUsername(value string) string {
	return stringsTrimSpaceLower(value)
}

func stringsTrimSpace(value string) string {
	return strings.TrimSpace(value)
}

func stringsTrimSpaceLower(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
