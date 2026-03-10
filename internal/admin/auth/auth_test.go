package auth

import (
	"net/http/httptest"
	"testing"

	"github.com/sphireinc/foundry/internal/config"
)

func TestAuthorizeAllowsLocalRequest(t *testing.T) {
	m := New(&config.Config{Admin: config.AdminConfig{Enabled: true, LocalOnly: true}})
	req := httptest.NewRequest("GET", "/__admin/api/status", nil)
	req.RemoteAddr = "127.0.0.1:12345"

	if err := m.Authorize(req); err != nil {
		t.Fatalf("expected local request to be allowed, got %v", err)
	}
}

func TestAuthorizeRejectsNonLocalRequest(t *testing.T) {
	m := New(&config.Config{Admin: config.AdminConfig{Enabled: true, LocalOnly: true}})
	req := httptest.NewRequest("GET", "/__admin/api/status", nil)
	req.RemoteAddr = "8.8.8.8:12345"

	if err := m.Authorize(req); err == nil {
		t.Fatal("expected non-local request to be rejected")
	}
}
