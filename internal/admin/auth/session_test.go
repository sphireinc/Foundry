package auth

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSessionManagerIssueAuthenticateAndRevoke(t *testing.T) {
	now := time.Now()
	manager := NewSessionManager("", 30*time.Minute, 0, 0, "test-secret", false)

	session, err := manager.Issue(Identity{Username: "admin", Name: "Admin"}, SessionIssueMeta{}, now)
	if err != nil {
		t.Fatalf("issue session: %v", err)
	}
	if session.Token == "" {
		t.Fatal("expected session token")
	}

	authenticated, ok := manager.Authenticate(session.Token, now.Add(5*time.Minute))
	if !ok || authenticated.Username != "admin" {
		t.Fatalf("expected session authenticate to succeed, got %#v %v", authenticated, ok)
	}

	manager.Revoke(session.Token)
	if _, ok := manager.Authenticate(session.Token, now.Add(6*time.Minute)); ok {
		t.Fatal("expected revoked session to fail authentication")
	}
}

func TestSessionManagerExpiresIdleSessions(t *testing.T) {
	now := time.Now()
	manager := NewSessionManager("", 30*time.Minute, 0, 0, "test-secret", false)

	session, err := manager.Issue(Identity{Username: "admin"}, SessionIssueMeta{}, now)
	if err != nil {
		t.Fatalf("issue session: %v", err)
	}

	if _, ok := manager.Authenticate(session.Token, now.Add(31*time.Minute)); ok {
		t.Fatal("expected expired session to fail authentication")
	}
}

func TestSessionManagerStoresTokenHashNotRawToken(t *testing.T) {
	now := time.Now()
	root := t.TempDir()
	path := filepath.Join(root, "sessions.yaml")
	manager := NewSessionManager(path, 30*time.Minute, 0, 0, "test-secret", false)

	session, err := manager.Issue(Identity{Username: "admin"}, SessionIssueMeta{}, now)
	if err != nil {
		t.Fatalf("issue session: %v", err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read session file: %v", err)
	}
	text := string(body)
	if strings.Contains(text, session.Token) {
		t.Fatal("expected session file to avoid storing raw token")
	}
	if !strings.Contains(text, "token_hash:") {
		t.Fatal("expected session file to store token_hash")
	}
}

func TestSessionManagerListsAndRevokesSessionByID(t *testing.T) {
	now := time.Now().UTC()
	manager := NewSessionManager("", 30*time.Minute, 0, 0, "test-secret", false)

	first, err := manager.Issue(Identity{Username: "admin"}, SessionIssueMeta{
		RemoteAddr: "127.0.0.1:12345",
		UserAgent:  "Foundry Test Browser",
	}, now)
	if err != nil {
		t.Fatalf("issue first session: %v", err)
	}
	second, err := manager.Issue(Identity{Username: "editor"}, SessionIssueMeta{
		RemoteAddr: "10.0.0.2:4567",
		UserAgent:  "Editor Browser",
	}, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("issue second session: %v", err)
	}

	sessions := manager.List("admin", first.Token, now.Add(2*time.Minute))
	if len(sessions) != 1 {
		t.Fatalf("expected one admin session, got %#v", sessions)
	}
	if !sessions[0].Current {
		t.Fatal("expected current session to be marked current")
	}
	if sessions[0].RemoteAddr != "127.0.0.1" {
		t.Fatalf("expected normalized remote addr, got %q", sessions[0].RemoteAddr)
	}

	if !manager.RevokeSessionID(sessions[0].ID) {
		t.Fatal("expected targeted session revoke to succeed")
	}
	if _, ok := manager.Authenticate(first.Token, now.Add(3*time.Minute)); ok {
		t.Fatal("expected revoked session to fail authentication")
	}
	if _, ok := manager.Authenticate(second.Token, now.Add(3*time.Minute)); !ok {
		t.Fatal("expected unrelated session to remain valid")
	}
}

func TestSessionManagerLoadsLegacyPlaintextSessionFile(t *testing.T) {
	now := time.Now().UTC()
	root := t.TempDir()
	path := filepath.Join(root, "sessions.yaml")
	body := []byte("sessions:\n  - token: legacy-token\n    csrf_token: csrf\n    username: admin\n    issued_at: " + now.Format(time.RFC3339) + "\n    last_seen: " + now.Format(time.RFC3339) + "\n    expires_at: " + now.Add(30*time.Minute).Format(time.RFC3339) + "\n")
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatalf("write legacy session file: %v", err)
	}
	manager := NewSessionManager(path, 30*time.Minute, 0, 0, "test-secret", false)
	session, ok := manager.Authenticate("legacy-token", now.Add(5*time.Minute))
	if !ok || session.Username != "admin" {
		t.Fatalf("expected legacy session authenticate to succeed, got %#v %v", session, ok)
	}
	updatedBody, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read migrated session file: %v", err)
	}
	if strings.Contains(string(updatedBody), "token: legacy-token") {
		t.Fatal("expected legacy raw token to be removed after rewrite")
	}
	if !strings.Contains(string(updatedBody), "token_hash:") {
		t.Fatal("expected migrated session file to contain token_hash")
	}
}

func TestSessionManagerEnforcesIdleTimeoutAndMaxAge(t *testing.T) {
	now := time.Now().UTC()
	manager := NewSessionManager("", 8*time.Hour, 15*time.Minute, 45*time.Minute, "test-secret", false)

	session, err := manager.Issue(Identity{Username: "admin"}, SessionIssueMeta{}, now)
	if err != nil {
		t.Fatalf("issue session: %v", err)
	}

	if _, ok, reason := manager.AuthenticateDetailed(session.Token, now.Add(20*time.Minute)); ok || reason != "inactivity" {
		t.Fatalf("expected inactivity expiration, got ok=%v reason=%q", ok, reason)
	}

	session, err = manager.Issue(Identity{Username: "admin"}, SessionIssueMeta{}, now)
	if err != nil {
		t.Fatalf("issue replacement session: %v", err)
	}
	if _, ok, reason := manager.AuthenticateDetailed(session.Token, now.Add(50*time.Minute)); ok || reason != "maximum lifetime" {
		t.Fatalf("expected max-age expiration, got ok=%v reason=%q", ok, reason)
	}
}

func TestSessionManagerSingleSessionPerUser(t *testing.T) {
	now := time.Now().UTC()
	manager := NewSessionManager("", 30*time.Minute, 0, 0, "test-secret", true)

	first, err := manager.Issue(Identity{Username: "admin"}, SessionIssueMeta{}, now)
	if err != nil {
		t.Fatalf("issue first session: %v", err)
	}
	second, err := manager.Issue(Identity{Username: "admin"}, SessionIssueMeta{}, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("issue second session: %v", err)
	}
	if _, ok := manager.Authenticate(first.Token, now.Add(2*time.Minute)); ok {
		t.Fatal("expected first session to be revoked by single-session policy")
	}
	if _, ok := manager.Authenticate(second.Token, now.Add(2*time.Minute)); !ok {
		t.Fatal("expected latest session to remain valid")
	}
}
