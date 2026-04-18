package managed

import (
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
