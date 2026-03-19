package auth

import (
	"crypto/rand"
	"encoding/base64"
	"sync"
	"time"
)

type Session struct {
	Token     string
	Username  string
	Name      string
	Email     string
	Role      string
	IssuedAt  time.Time
	LastSeen  time.Time
	ExpiresAt time.Time
}

type SessionManager struct {
	ttl      time.Duration
	mu       sync.Mutex
	sessions map[string]Session
}

func NewSessionManager(ttl time.Duration) *SessionManager {
	if ttl <= 0 {
		ttl = 30 * time.Minute
	}
	return &SessionManager{
		ttl:      ttl,
		sessions: make(map[string]Session),
	}
}

func (m *SessionManager) Issue(identity Identity, now time.Time) (Session, error) {
	token, err := randomToken()
	if err != nil {
		return Session{}, err
	}

	session := Session{
		Token:     token,
		Username:  identity.Username,
		Name:      identity.Name,
		Email:     identity.Email,
		Role:      identity.Role,
		IssuedAt:  now,
		LastSeen:  now,
		ExpiresAt: now.Add(m.ttl),
	}

	m.mu.Lock()
	m.sessions[token] = session
	m.mu.Unlock()
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
		return Session{}, false
	}

	session.LastSeen = now
	session.ExpiresAt = now.Add(m.ttl)
	m.sessions[token] = session
	return session, true
}

func (m *SessionManager) Revoke(token string) {
	if token == "" {
		return
	}
	m.mu.Lock()
	delete(m.sessions, token)
	m.mu.Unlock()
}

func (m *SessionManager) TTL() time.Duration {
	return m.ttl
}

func randomToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
