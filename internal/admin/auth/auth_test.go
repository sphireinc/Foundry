package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base32"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
	cfg := testAuthConfig(t)
	cfg.Admin.LocalOnly = true
	m := New(cfg)
	req := httptest.NewRequest("GET", "/__admin/api/status", nil)
	req.RemoteAddr = "8.8.8.8:12345"
	req.Header.Set("X-Foundry-Admin-Token", "secret-token")

	if err := m.Authorize(req); err == nil {
		t.Fatal("expected non-local request to be rejected")
	}
}

func TestAuthorizeRejectsForwardedLoopbackRequest(t *testing.T) {
	cfg := testAuthConfig(t)
	cfg.Admin.LocalOnly = true
	m := New(cfg)
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

	identity, err := m.Login(rr, loginReq, "admin", "secret-password", "")
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

	if _, err := m.Login(rr, req, "admin", "secret-password", ""); err != nil {
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
		if _, err := m.Login(nil, req, "admin", "wrong-password", ""); err == nil {
			t.Fatalf("expected login failure %d", i+1)
		}
	}

	req := httptest.NewRequest(http.MethodPost, "/__admin/api/login", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	if _, err := m.Login(nil, req, "admin", "secret-password", ""); err == nil || err.Error() != "too many login attempts; try again later" {
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
	if _, err := m.Login(loginRR, loginReq, "editor", "editor-password", ""); err != nil {
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

func TestLoginUpgradesLegacyPasswordHash(t *testing.T) {
	cfg := testAuthConfig(t)
	cfg.Admin.AccessToken = ""
	legacySalt := base64.RawStdEncoding.EncodeToString([]byte("0123456789abcdef"))
	legacyKey := base64.RawStdEncoding.EncodeToString(usersTestPBKDF2([]byte("secret-password"), []byte("0123456789abcdef"), 120000, 32))
	legacyHash := "pbkdf2-sha256$120000$" + legacySalt + "$" + legacyKey
	body := []byte("users:\n  - username: admin\n    name: Admin User\n    email: admin@example.com\n    role: admin\n    password_hash: " + legacyHash + "\n")
	if err := os.WriteFile(cfg.Admin.UsersFile, body, 0o644); err != nil {
		t.Fatalf("write legacy users file: %v", err)
	}
	m := New(cfg)

	loginReq := httptest.NewRequest(http.MethodPost, "/__admin/api/login", nil)
	loginReq.RemoteAddr = "127.0.0.1:12345"
	rr := httptest.NewRecorder()
	if _, err := m.Login(rr, loginReq, "admin", "secret-password", ""); err != nil {
		t.Fatalf("login failed: %v", err)
	}
	updatedBody, err := os.ReadFile(cfg.Admin.UsersFile)
	if err != nil {
		t.Fatalf("read updated users file: %v", err)
	}
	if !strings.Contains(string(updatedBody), "argon2id$") {
		t.Fatal("expected login to upgrade legacy password hash to argon2id")
	}
}

func TestEnableTOTPRevokesExistingSessions(t *testing.T) {
	cfg := testAuthConfig(t)
	cfg.Admin.AccessToken = ""
	m := New(cfg)

	loginReq := httptest.NewRequest(http.MethodPost, "/__admin/api/login", nil)
	loginReq.RemoteAddr = "127.0.0.1:12345"
	loginRR := httptest.NewRecorder()
	identity, err := m.Login(loginRR, loginReq, "admin", "secret-password", "")
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}
	cookies := loginRR.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected session cookie")
	}

	setupResp, err := m.SetupTOTP(identity, "admin")
	if err != nil {
		t.Fatalf("setup totp: %v", err)
	}
	code := totpCodeForCounter(mustDecodeBase32(t, setupResp.Secret), counterForTime(time.Now()))
	if err := m.EnableTOTP(identity, "admin", code); err != nil {
		t.Fatalf("enable totp: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/__admin/api/status", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	req.AddCookie(cookies[0])
	if err := m.Authorize(req); err == nil {
		t.Fatal("expected TOTP enablement to revoke existing session")
	}
}

func TestSetupTOTPStoresEncryptedSecret(t *testing.T) {
	cfg := testAuthConfig(t)
	cfg.Admin.AccessToken = ""
	m := New(cfg)

	resp, err := m.SetupTOTP(&Identity{Username: "admin", Capabilities: []string{"*"}}, "admin")
	if err != nil {
		t.Fatalf("setup totp: %v", err)
	}
	body, err := os.ReadFile(cfg.Admin.UsersFile)
	if err != nil {
		t.Fatalf("read users file: %v", err)
	}
	text := string(body)
	if strings.Contains(text, resp.Secret) {
		t.Fatal("expected users file to avoid storing raw TOTP secret")
	}
	if !strings.Contains(text, "totp_secret: enc:v1:") {
		t.Fatal("expected encrypted TOTP secret to be persisted")
	}
}

func TestLoginMigratesLegacyPlaintextTOTPSecret(t *testing.T) {
	cfg := testAuthConfig(t)
	cfg.Admin.AccessToken = ""
	m := New(cfg)

	secret, err := GenerateTOTPSecret()
	if err != nil {
		t.Fatalf("generate TOTP secret: %v", err)
	}
	body, err := os.ReadFile(cfg.Admin.UsersFile)
	if err != nil {
		t.Fatalf("read users file: %v", err)
	}
	text := strings.Replace(string(body), "password_hash:", "totp_enabled: true\n    totp_secret: "+secret+"\n    password_hash:", 1)
	if err := os.WriteFile(cfg.Admin.UsersFile, []byte(text), 0o644); err != nil {
		t.Fatalf("write users file: %v", err)
	}

	loginReq := httptest.NewRequest(http.MethodPost, "/__admin/api/login", nil)
	loginReq.RemoteAddr = "127.0.0.1:12345"
	code := totpCodeForCounter(mustDecodeBase32(t, secret), counterForTime(time.Now()))
	if _, err := m.Login(httptest.NewRecorder(), loginReq, "admin", "secret-password", code); err != nil {
		t.Fatalf("login failed: %v", err)
	}
	updatedBody, err := os.ReadFile(cfg.Admin.UsersFile)
	if err != nil {
		t.Fatalf("read updated users file: %v", err)
	}
	if strings.Contains(string(updatedBody), "totp_secret: "+secret) {
		t.Fatal("expected plaintext TOTP secret to be migrated")
	}
	if !strings.Contains(string(updatedBody), "totp_secret: enc:v1:") {
		t.Fatal("expected migrated encrypted TOTP secret")
	}
}

func TestLoginIgnoresUnavailableTOTPSecretWhenMFAIsDisabled(t *testing.T) {
	cfg := testAuthConfig(t)
	cfg.Admin.TOTPSecretKey = ""
	m := New(cfg)

	body, err := os.ReadFile(cfg.Admin.UsersFile)
	if err != nil {
		t.Fatalf("read users file: %v", err)
	}
	text := strings.Replace(string(body), "password_hash:", "totp_secret: enc:v1:invalidciphertext\n    password_hash:", 1)
	if err := os.WriteFile(cfg.Admin.UsersFile, []byte(text), 0o644); err != nil {
		t.Fatalf("write users file: %v", err)
	}

	loginReq := httptest.NewRequest(http.MethodPost, "/__admin/api/login", nil)
	loginReq.RemoteAddr = "127.0.0.1:12345"
	if _, err := m.Login(httptest.NewRecorder(), loginReq, "admin", "secret-password", ""); err != nil {
		t.Fatalf("expected login to ignore unused TOTP secret, got %v", err)
	}
}

func TestSetupTOTPRequiresEncryptionKey(t *testing.T) {
	cfg := testAuthConfig(t)
	cfg.Admin.TOTPSecretKey = ""
	m := New(cfg)

	if _, err := m.SetupTOTP(&Identity{Username: "admin", Capabilities: []string{"*"}}, "admin"); err == nil {
		t.Fatal("expected TOTP setup to require an encryption key")
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
		DataDir: filepath.Join(root, "data"),
		Admin: config.AdminConfig{
			Enabled:           true,
			LocalOnly:         true,
			AccessToken:       "secret-token",
			TOTPSecretKey:     testTOTPKey(),
			UsersFile:         usersPath,
			SessionTTLMinutes: 30,
		},
	}
	cfg.ApplyDefaults()
	return cfg
}

func usersTestPBKDF2(password, salt []byte, iterations, keyLen int) []byte {
	hLen := 32
	blocks := (keyLen + hLen - 1) / hLen
	out := make([]byte, 0, blocks*hLen)
	for block := 1; block <= blocks; block++ {
		u := usersTestHMACSHA256(password, append(salt, byte(block>>24), byte(block>>16), byte(block>>8), byte(block)))
		t := append([]byte(nil), u...)
		for i := 1; i < iterations; i++ {
			u = usersTestHMACSHA256(password, u)
			for j := range t {
				t[j] ^= u[j]
			}
		}
		out = append(out, t...)
	}
	return out[:keyLen]
}

func usersTestHMACSHA256(key, data []byte) []byte {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write(data)
	return mac.Sum(nil)
}

func mustDecodeBase32(t *testing.T, secret string) []byte {
	t.Helper()
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.ToUpper(secret))
	if err != nil {
		t.Fatalf("decode base32: %v", err)
	}
	return key
}

func testTOTPKey() string {
	return base64.RawStdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))
}
