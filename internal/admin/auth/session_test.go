package auth

import (
	"testing"
	"time"
)

func TestSessionManagerIssueAuthenticateAndRevoke(t *testing.T) {
	now := time.Now()
	manager := NewSessionManager(30 * time.Minute)

	session, err := manager.Issue(Identity{Username: "admin", Name: "Admin"}, now)
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
	manager := NewSessionManager(30 * time.Minute)

	session, err := manager.Issue(Identity{Username: "admin"}, now)
	if err != nil {
		t.Fatalf("issue session: %v", err)
	}

	if _, ok := manager.Authenticate(session.Token, now.Add(31*time.Minute)); ok {
		t.Fatal("expected expired session to fail authentication")
	}
}
