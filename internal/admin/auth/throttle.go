package auth

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	loginMaxFailures   = 5
	loginFailureWindow = 10 * time.Minute
	loginLockDuration  = 15 * time.Minute
)

type loginAttempt struct {
	failures    int
	firstFailed time.Time
	lockedUntil time.Time
}

type loginThrottler struct {
	mu       sync.Mutex
	attempts map[string]loginAttempt
}

func newLoginThrottler() *loginThrottler {
	return &loginThrottler{attempts: make(map[string]loginAttempt)}
}

func (t *loginThrottler) Allow(r *http.Request, username string, now time.Time) error {
	if t == nil {
		return nil
	}
	key := loginAttemptKey(r, username)
	t.mu.Lock()
	defer t.mu.Unlock()
	attempt := t.attempts[key]
	if attempt.lockedUntil.After(now) {
		return fmt.Errorf("too many login attempts; try again later")
	}
	if !attempt.firstFailed.IsZero() && now.Sub(attempt.firstFailed) > loginFailureWindow {
		delete(t.attempts, key)
	}
	return nil
}

func (t *loginThrottler) Failure(r *http.Request, username string, now time.Time) {
	if t == nil {
		return
	}
	key := loginAttemptKey(r, username)
	t.mu.Lock()
	defer t.mu.Unlock()
	attempt := t.attempts[key]
	if attempt.firstFailed.IsZero() || now.Sub(attempt.firstFailed) > loginFailureWindow {
		attempt = loginAttempt{firstFailed: now}
	}
	attempt.failures++
	if attempt.failures >= loginMaxFailures {
		attempt.lockedUntil = now.Add(loginLockDuration)
	}
	t.attempts[key] = attempt
}

func (t *loginThrottler) Success(r *http.Request, username string) {
	if t == nil {
		return
	}
	key := loginAttemptKey(r, username)
	t.mu.Lock()
	delete(t.attempts, key)
	t.mu.Unlock()
}

func loginAttemptKey(r *http.Request, username string) string {
	return requestIP(r) + "|" + strings.ToLower(strings.TrimSpace(username))
}

func requestIP(r *http.Request) string {
	if r == nil {
		return ""
	}
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err != nil {
		return strings.TrimSpace(r.RemoteAddr)
	}
	return host
}
