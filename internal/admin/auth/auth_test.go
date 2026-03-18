package auth

import (
	"net/http/httptest"
	"testing"

	"github.com/sphireinc/foundry/internal/config"
)

func TestAuthorizeAllowsLocalRequest(t *testing.T) {
	m := New(&config.Config{Admin: config.AdminConfig{Enabled: true, LocalOnly: true, AccessToken: "secret-token"}})
	req := httptest.NewRequest("GET", "/__admin/api/status", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("X-Foundry-Admin-Token", "secret-token")

	if err := m.Authorize(req); err != nil {
		t.Fatalf("expected local request to be allowed, got %v", err)
	}
}

func TestAuthorizeRejectsNonLocalRequest(t *testing.T) {
	m := New(&config.Config{Admin: config.AdminConfig{Enabled: true, LocalOnly: true, AccessToken: "secret-token"}})
	req := httptest.NewRequest("GET", "/__admin/api/status", nil)
	req.RemoteAddr = "8.8.8.8:12345"
	req.Header.Set("X-Foundry-Admin-Token", "secret-token")

	if err := m.Authorize(req); err == nil {
		t.Fatal("expected non-local request to be rejected")
	}
}

func TestAuthorizeRejectsRemoteAdminWithoutToken(t *testing.T) {
	m := New(&config.Config{Admin: config.AdminConfig{Enabled: true, LocalOnly: false, AccessToken: "secret-token"}})
	req := httptest.NewRequest("GET", "/__admin/api/status", nil)
	req.RemoteAddr = "8.8.8.8:12345"

	if err := m.Authorize(req); err == nil {
		t.Fatal("expected remote admin without token to be rejected")
	}
}

func TestAuthorizeRequiresConfiguredToken(t *testing.T) {
	m := New(&config.Config{Admin: config.AdminConfig{Enabled: true, LocalOnly: false, AccessToken: "secret-token"}})

	req := httptest.NewRequest("GET", "/__admin/api/status", nil)
	req.RemoteAddr = "8.8.8.8:12345"
	if err := m.Authorize(req); err == nil {
		t.Fatal("expected missing token to be rejected")
	}

	req = httptest.NewRequest("GET", "/__admin/api/status", nil)
	req.RemoteAddr = "8.8.8.8:12345"
	req.Header.Set("Authorization", "Bearer secret-token")
	if err := m.Authorize(req); err != nil {
		t.Fatalf("expected bearer token to be accepted, got %v", err)
	}

	req = httptest.NewRequest("GET", "/__admin/api/status", nil)
	req.RemoteAddr = "8.8.8.8:12345"
	req.Header.Set("X-Foundry-Admin-Token", "secret-token")
	if err := m.Authorize(req); err != nil {
		t.Fatalf("expected header token to be accepted, got %v", err)
	}
}
