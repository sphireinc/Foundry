package managed

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestValidateBootstrapPayload(t *testing.T) {
	secret := []byte(strings.Repeat("s", 32))
	now := time.Date(2026, 4, 18, 12, 0, 0, 0, time.UTC)
	payload := validBootstrapPayload(now)

	signed, err := SignBootstrapPayload(payload, secret)
	if err != nil {
		t.Fatalf("sign payload: %v", err)
	}
	got, err := ValidateBootstrapPayload(signed, secret, BootstrapValidationOptions{Now: now})
	if err != nil {
		t.Fatalf("validate payload: %v", err)
	}
	if got.WorkspaceID != payload.WorkspaceID || got.InstanceID != payload.InstanceID {
		t.Fatalf("unexpected validated payload: %#v", got)
	}
}

func TestValidateBootstrapPayloadRejectsExpired(t *testing.T) {
	secret := []byte(strings.Repeat("s", 32))
	now := time.Date(2026, 4, 18, 12, 0, 0, 0, time.UTC)
	payload := validBootstrapPayload(now)
	payload.IssuedAt = now.Add(-2 * time.Hour)
	payload.ExpiresAt = now.Add(-time.Hour)
	signed, err := SignBootstrapPayload(payload, secret)
	if err != nil {
		t.Fatalf("sign payload: %v", err)
	}
	if _, err := ValidateBootstrapPayload(signed, secret, BootstrapValidationOptions{Now: now}); err == nil {
		t.Fatal("expected expired payload to be rejected")
	}
}

func TestValidateBootstrapPayloadRejectsFutureIssuedAt(t *testing.T) {
	secret := []byte(strings.Repeat("s", 32))
	now := time.Date(2026, 4, 18, 12, 0, 0, 0, time.UTC)
	payload := validBootstrapPayload(now)
	payload.IssuedAt = now.Add(time.Minute)
	payload.ExpiresAt = now.Add(time.Hour)
	signed, err := SignBootstrapPayload(payload, secret)
	if err != nil {
		t.Fatalf("sign payload: %v", err)
	}
	if _, err := ValidateBootstrapPayload(signed, secret, BootstrapValidationOptions{Now: now}); err == nil {
		t.Fatal("expected future-issued payload to be rejected")
	}
}

func TestValidateBootstrapPayloadRejectsTamperedPayload(t *testing.T) {
	secret := []byte(strings.Repeat("s", 32))
	now := time.Date(2026, 4, 18, 12, 0, 0, 0, time.UTC)
	signed, err := SignBootstrapPayload(validBootstrapPayload(now), secret)
	if err != nil {
		t.Fatalf("sign payload: %v", err)
	}
	signed.Payload.OwnerEmail = "attacker@example.com"
	if _, err := ValidateBootstrapPayload(signed, secret, BootstrapValidationOptions{Now: now}); err == nil {
		t.Fatal("expected tampered payload to be rejected")
	}
}

func TestValidateBootstrapPayloadRejectsMissingField(t *testing.T) {
	secret := []byte(strings.Repeat("s", 32))
	now := time.Date(2026, 4, 18, 12, 0, 0, 0, time.UTC)
	payload := validBootstrapPayload(now)
	payload.WorkspaceID = ""
	signed := SignedBootstrapPayload{
		Algorithm: BootstrapSignatureAlgorithm,
		Payload:   payload,
		Signature: strings.Repeat("0", 64),
	}
	if _, err := ValidateBootstrapPayload(signed, secret, BootstrapValidationOptions{Now: now}); err == nil {
		t.Fatal("expected missing workspace_id to be rejected")
	}
}

func TestValidateBootstrapPayloadRejectsBadSignature(t *testing.T) {
	secret := []byte(strings.Repeat("s", 32))
	now := time.Date(2026, 4, 18, 12, 0, 0, 0, time.UTC)
	signed, err := SignBootstrapPayload(validBootstrapPayload(now), secret)
	if err != nil {
		t.Fatalf("sign payload: %v", err)
	}
	signed.Signature = strings.Repeat("f", 64)
	if _, err := ValidateBootstrapPayload(signed, secret, BootstrapValidationOptions{Now: now}); err == nil {
		t.Fatal("expected bad signature to be rejected")
	}
}

func TestValidateBootstrapPayloadRejectsInvalidAdminPathAndCallbackURL(t *testing.T) {
	secret := []byte(strings.Repeat("s", 32))
	now := time.Date(2026, 4, 18, 12, 0, 0, 0, time.UTC)
	for name, mutate := range map[string]func(*BootstrapPayload){
		"root admin path":          func(p *BootstrapPayload) { p.AdminPath = "/" },
		"unnormalized admin path":  func(p *BootstrapPayload) { p.AdminPath = "/admin/../admin" },
		"missing callback host":    func(p *BootstrapPayload) { p.RuntimeCallbackURL = "https://" },
		"unsupported callback URL": func(p *BootstrapPayload) { p.RuntimeCallbackURL = "file:///tmp/callback" },
	} {
		t.Run(name, func(t *testing.T) {
			payload := validBootstrapPayload(now)
			mutate(&payload)
			signed := SignedBootstrapPayload{
				Algorithm: BootstrapSignatureAlgorithm,
				Payload:   payload,
				Signature: strings.Repeat("0", 64),
			}
			if _, err := ValidateBootstrapPayload(signed, secret, BootstrapValidationOptions{Now: now}); err == nil {
				t.Fatal("expected invalid payload to be rejected")
			}
		})
	}
}

func TestApplyBootstrapOnceWritesStateAndRejectsReplay(t *testing.T) {
	now := time.Date(2026, 4, 18, 12, 0, 0, 0, time.UTC)
	dataDir := t.TempDir()
	applied := 0

	state, err := ApplyBootstrapOnce(BootstrapApplyOptions{
		DataDir: dataDir,
		Payload: validBootstrapPayload(now),
		Now:     now,
		Apply: func(payload BootstrapPayload) error {
			applied++
			if payload.WorkspaceID != "workspace-123" {
				t.Fatalf("unexpected workspace ID passed to apply: %q", payload.WorkspaceID)
			}
			return nil
		},
	})
	if err != nil {
		t.Fatalf("apply bootstrap: %v", err)
	}
	if applied != 1 {
		t.Fatalf("expected apply callback once, got %d", applied)
	}
	if state.WorkspaceID != "workspace-123" || state.InstanceID != "instance-456" {
		t.Fatalf("unexpected bootstrap state: %#v", state)
	}
	if state.CompletedAt != now {
		t.Fatalf("expected completed_at %s, got %s", now, state.CompletedAt)
	}
	if state.Nonce == "" || state.PayloadSHA256 == "" {
		t.Fatalf("expected nonce and payload hash in state: %#v", state)
	}

	read, err := ReadBootstrapState(dataDir)
	if err != nil {
		t.Fatalf("read bootstrap state: %v", err)
	}
	if read.PayloadSHA256 != state.PayloadSHA256 {
		t.Fatalf("expected persisted hash %q, got %q", state.PayloadSHA256, read.PayloadSHA256)
	}

	_, err = ApplyBootstrapOnce(BootstrapApplyOptions{
		DataDir: dataDir,
		Payload: validBootstrapPayload(now.Add(time.Minute)),
		Now:     now.Add(time.Minute),
		Apply: func(BootstrapPayload) error {
			applied++
			return nil
		},
	})
	if err == nil || !strings.Contains(err.Error(), "already completed") {
		t.Fatalf("expected replay rejection, got %v", err)
	}
	if applied != 1 {
		t.Fatalf("expected replay not to call apply callback, got %d calls", applied)
	}
}

func TestApplyBootstrapOnceFailedApplyLeavesNoCompletedMarker(t *testing.T) {
	now := time.Date(2026, 4, 18, 12, 0, 0, 0, time.UTC)
	dataDir := t.TempDir()
	expectedErr := errors.New("write site config")

	if _, err := ApplyBootstrapOnce(BootstrapApplyOptions{
		DataDir: dataDir,
		Payload: validBootstrapPayload(now),
		Now:     now,
		Apply: func(BootstrapPayload) error {
			return expectedErr
		},
	}); !errors.Is(err, expectedErr) {
		t.Fatalf("expected apply error %v, got %v", expectedErr, err)
	}
	if _, err := ReadBootstrapState(dataDir); !os.IsNotExist(err) {
		t.Fatalf("expected no completed bootstrap state, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(dataDir, bootstrapStateDir, bootstrapLockFile)); !os.IsNotExist(err) {
		t.Fatalf("expected bootstrap lock cleanup, got %v", err)
	}

	applied := 0
	if _, err := ApplyBootstrapOnce(BootstrapApplyOptions{
		DataDir: dataDir,
		Payload: validBootstrapPayload(now),
		Now:     now,
		Apply: func(BootstrapPayload) error {
			applied++
			return nil
		},
	}); err != nil {
		t.Fatalf("expected retry after failed apply to succeed, got %v", err)
	}
	if applied != 1 {
		t.Fatalf("expected retry apply callback once, got %d", applied)
	}
}

func TestApplyBootstrapOnceRejectsInProgressBootstrap(t *testing.T) {
	now := time.Date(2026, 4, 18, 12, 0, 0, 0, time.UTC)
	dataDir := t.TempDir()
	lockDir := filepath.Join(dataDir, bootstrapStateDir)
	if err := os.MkdirAll(lockDir, 0o700); err != nil {
		t.Fatalf("create lock dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(lockDir, bootstrapLockFile), []byte("locked"), 0o600); err != nil {
		t.Fatalf("write lock: %v", err)
	}

	called := false
	_, err := ApplyBootstrapOnce(BootstrapApplyOptions{
		DataDir: dataDir,
		Payload: validBootstrapPayload(now),
		Now:     now,
		Apply: func(BootstrapPayload) error {
			called = true
			return nil
		},
	})
	if err == nil || !strings.Contains(err.Error(), "already in progress") {
		t.Fatalf("expected in-progress rejection, got %v", err)
	}
	if called {
		t.Fatal("expected in-progress bootstrap not to call apply callback")
	}
}

func validBootstrapPayload(now time.Time) BootstrapPayload {
	return BootstrapPayload{
		WorkspaceID:        "workspace-123",
		InstanceID:         "instance-456",
		OwnerEmail:         "owner@example.com",
		InitialSiteConfig:  map[string]any{"title": "Managed Foundry", "base_url": "https://site.example.com"},
		AdminPath:          "/__admin",
		RuntimeCallbackURL: "https://control.example.com/runtime/callbacks",
		IssuedAt:           now.Add(-time.Minute),
		ExpiresAt:          now.Add(time.Hour),
		Nonce:              "nonce-1234567890abcdef",
	}
}
