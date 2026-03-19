package auth

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/sphireinc/foundry/internal/admin/users"
	"github.com/sphireinc/foundry/internal/config"
)

func TestAuthorizeAllowsLocalRequest(t *testing.T) {
	m := New(testAuthConfig(t))
	req := httptest.NewRequest("GET", "/__admin/api/status", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("X-Foundry-Admin-Token", "secret-token")

	if err := m.Authorize(req); err != nil {
		t.Fatalf("expected local request to be allowed, got %v", err)
	}
}

func TestAuthorizeRejectsNonLocalRequest(t *testing.T) {
	m := New(testAuthConfig(t))
	req := httptest.NewRequest("GET", "/__admin/api/status", nil)
	req.RemoteAddr = "8.8.8.8:12345"
	req.Header.Set("X-Foundry-Admin-Token", "secret-token")

	if err := m.Authorize(req); err == nil {
		t.Fatal("expected non-local request to be rejected")
	}
}

func TestAuthorizeRejectsForwardedLoopbackRequest(t *testing.T) {
	m := New(testAuthConfig(t))
	req := httptest.NewRequest("GET", "/__admin/api/status", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("X-Forwarded-For", "8.8.8.8")
	req.Header.Set("X-Foundry-Admin-Token", "secret-token")

	if err := m.Authorize(req); err == nil {
		t.Fatal("expected forwarded request to be rejected for local-only admin")
	}
}

func TestAuthorizeRejectsRemoteAdminWithoutToken(t *testing.T) {
	cfg := testAuthConfig(t)
	cfg.Admin.LocalOnly = false
	m := New(cfg)
	req := httptest.NewRequest("GET", "/__admin/api/status", nil)
	req.RemoteAddr = "8.8.8.8:12345"

	if err := m.Authorize(req); err == nil {
		t.Fatal("expected remote admin without token to be rejected")
	}
}

func TestAuthorizeRequiresConfiguredToken(t *testing.T) {
	cfg := testAuthConfig(t)
	cfg.Admin.LocalOnly = false
	m := New(cfg)

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

func TestWrapRejectsUnauthorizedAndHandlesNilNext(t *testing.T) {
	m := New(testAuthConfig(t))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/__admin/api/status", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	m.Wrap(nil).ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected nil handler to map to 404, got %d", rr.Code)
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/__admin/api/status", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	m.Wrap(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})).ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden without token, got %d", rr.Code)
	}
}

func TestLoginAndSessionCookieAuthentication(t *testing.T) {
	cfg := testAuthConfig(t)
	cfg.Admin.AccessToken = ""
	m := New(cfg)

	loginReq := httptest.NewRequest(http.MethodPost, "/__admin/api/login", nil)
	loginReq.RemoteAddr = "127.0.0.1:12345"
	rr := httptest.NewRecorder()

	identity, err := m.Login(rr, loginReq, "admin", "secret-password")
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}
	if identity.Username != "admin" {
		t.Fatalf("unexpected identity: %#v", identity)
	}

	cookies := rr.Result().Cookies()
	if len(cookies) == 0 || cookies[0].Name != sessionCookieName {
		t.Fatalf("expected session cookie, got %#v", cookies)
	}
	if cookies[0].Secure {
		t.Fatal("expected non-TLS login cookie to be non-secure")
	}

	req := httptest.NewRequest(http.MethodGet, "/__admin/api/status", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	req.AddCookie(cookies[0])
	if err := m.Authorize(req); err != nil {
		t.Fatalf("expected cookie session auth to succeed, got %v", err)
	}

	logoutReq := httptest.NewRequest(http.MethodPost, "/__admin/api/logout", nil)
	logoutReq.RemoteAddr = "127.0.0.1:12345"
	logoutReq.AddCookie(cookies[0])
	logoutRR := httptest.NewRecorder()
	if err := m.Logout(logoutRR, logoutReq); err != nil {
		t.Fatalf("logout failed: %v", err)
	}
	if err := m.Authorize(req); err == nil {
		t.Fatal("expected revoked cookie session to fail")
	}
}

func TestLoginSetsSecureCookieForTLSRequests(t *testing.T) {
	cfg := testAuthConfig(t)
	cfg.Admin.AccessToken = ""
	m := New(cfg)

	req := httptest.NewRequest(http.MethodPost, "/__admin/api/login", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	req.TLS = &tls.ConnectionState{}
	rr := httptest.NewRecorder()

	if _, err := m.Login(rr, req, "admin", "secret-password"); err != nil {
		t.Fatalf("login failed: %v", err)
	}
	cookies := rr.Result().Cookies()
	if len(cookies) == 0 || !cookies[0].Secure {
		t.Fatalf("expected secure cookie for TLS request, got %#v", cookies)
	}
}

func TestLoginThrottlesRepeatedFailures(t *testing.T) {
	cfg := testAuthConfig(t)
	cfg.Admin.AccessToken = ""
	m := New(cfg)

	for i := 0; i < loginMaxFailures; i++ {
		req := httptest.NewRequest(http.MethodPost, "/__admin/api/login", nil)
		req.RemoteAddr = "127.0.0.1:12345"
		if _, err := m.Login(nil, req, "admin", "wrong-password"); err == nil {
			t.Fatalf("expected login failure %d", i+1)
		}
	}

	req := httptest.NewRequest(http.MethodPost, "/__admin/api/login", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	if _, err := m.Login(nil, req, "admin", "secret-password"); err == nil || err.Error() != "too many login attempts; try again later" {
		t.Fatalf("expected throttling error, got %v", err)
	}
}

func TestWrapRoleRejectsInsufficientRole(t *testing.T) {
	cfg := testAuthConfig(t)
	cfg.Admin.AccessToken = ""
	m := New(cfg)

	loginReq := httptest.NewRequest(http.MethodPost, "/__admin/api/login", nil)
	loginReq.RemoteAddr = "127.0.0.1:12345"
	loginRR := httptest.NewRecorder()
	if _, err := m.Login(loginRR, loginReq, "editor", "editor-password"); err != nil {
		t.Fatalf("editor login failed: %v", err)
	}
	cookies := loginRR.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected session cookie")
	}

	req := httptest.NewRequest(http.MethodPost, "/__admin/api/users/save", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	req.AddCookie(cookies[0])
	rr := httptest.NewRecorder()
	m.WrapRole(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}), "admin").ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected insufficient-role request to be forbidden, got %d", rr.Code)
	}
}

func testAuthConfig(t *testing.T) *config.Config {
	t.Helper()
	root := t.TempDir()
	hash, err := users.HashPassword("secret-password")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	usersPath := filepath.Join(root, "content", "config", "admin-users.yaml")
	if err := os.MkdirAll(filepath.Dir(usersPath), 0o755); err != nil {
		t.Fatalf("mkdir users file dir: %v", err)
	}
	editorHash, err := users.HashPassword("editor-password")
	if err != nil {
		t.Fatalf("hash editor password: %v", err)
	}
	body := []byte("users:\n  - username: admin\n    name: Admin User\n    email: admin@example.com\n    role: admin\n    password_hash: " + hash + "\n  - username: editor\n    name: Editor User\n    email: editor@example.com\n    role: editor\n    password_hash: " + editorHash + "\n")
	if err := os.WriteFile(usersPath, body, 0o644); err != nil {
		t.Fatalf("write users file: %v", err)
	}

	cfg := &config.Config{
		Admin: config.AdminConfig{
			Enabled:           true,
			LocalOnly:         true,
			AccessToken:       "secret-token",
			UsersFile:         usersPath,
			SessionTTLMinutes: 30,
		},
	}
	cfg.ApplyDefaults()
	return cfg
}
