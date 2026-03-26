package httpadmin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"html/template"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sphireinc/foundry/internal/admin/service"
	admintypes "github.com/sphireinc/foundry/internal/admin/types"
	"github.com/sphireinc/foundry/internal/admin/users"
	"github.com/sphireinc/foundry/internal/config"
	"github.com/sphireinc/foundry/internal/content"
	"github.com/sphireinc/foundry/internal/plugins"
	"github.com/sphireinc/foundry/internal/server"
)

func TestStatusEndpoint(t *testing.T) {
	cfg := testConfig(t)
	r := newTestRouter(t, cfg)

	req := httptest.NewRequest(http.MethodGet, "/__admin/api/status", nil)
	req.RemoteAddr = "127.0.0.1:10000"
	req.Header.Set("X-Foundry-Admin-Token", cfg.Admin.AccessToken)
	rr := httptest.NewRecorder()

	mux := http.NewServeMux()
	r.RegisterRoutes(mux)
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var status admintypes.SystemStatus
	if err := json.Unmarshal(rr.Body.Bytes(), &status); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if status.Title != cfg.Title {
		t.Fatalf("expected title %q, got %q", cfg.Title, status.Title)
	}
}

func TestCapabilitiesIncludePprofFeatureWhenEnabled(t *testing.T) {
	cfg := testConfig(t)
	cfg.Admin.Debug.Pprof = true
	r := newTestRouter(t, cfg)
	mux := http.NewServeMux()
	r.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/__admin/api/capabilities", nil)
	req.RemoteAddr = "127.0.0.1:10000"
	req.Header.Set("X-Foundry-Admin-Token", cfg.Admin.AccessToken)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp admintypes.CapabilityResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode capabilities: %v", err)
	}
	if !resp.Modules["debug"] || !resp.Features["pprof"] {
		t.Fatalf("expected pprof debug capability flags, got %#v %#v", resp.Modules, resp.Features)
	}
}

func TestPprofRouteRequiresOptInAndAuth(t *testing.T) {
	cfg := testConfig(t)
	r := newTestRouter(t, cfg)
	mux := http.NewServeMux()
	r.RegisterRoutes(mux)

	disabledReq := httptest.NewRequest(http.MethodGet, "/__admin/debug/pprof/", nil)
	disabledReq.RemoteAddr = "127.0.0.1:10000"
	disabledRR := httptest.NewRecorder()
	mux.ServeHTTP(disabledRR, disabledReq)
	if disabledRR.Code != http.StatusNotFound {
		t.Fatalf("expected disabled pprof route to 404, got %d", disabledRR.Code)
	}

	cfg.Admin.Debug.Pprof = true
	r = newTestRouter(t, cfg)
	mux = http.NewServeMux()
	r.RegisterRoutes(mux)

	unauthReq := httptest.NewRequest(http.MethodGet, "/__admin/debug/pprof/", nil)
	unauthReq.RemoteAddr = "127.0.0.1:10000"
	unauthRR := httptest.NewRecorder()
	mux.ServeHTTP(unauthRR, unauthReq)
	if unauthRR.Code != http.StatusForbidden {
		t.Fatalf("expected unauthenticated pprof route to 403, got %d", unauthRR.Code)
	}

	req := httptest.NewRequest(http.MethodGet, "/__admin/debug/pprof/", nil)
	req.RemoteAddr = "127.0.0.1:10000"
	req.Header.Set("X-Foundry-Admin-Token", cfg.Admin.AccessToken)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected authenticated pprof index 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "profile") {
		t.Fatalf("expected pprof index content, got %q", rr.Body.String())
	}

	runtimeReq := httptest.NewRequest(http.MethodGet, "/__admin/api/debug/runtime", nil)
	runtimeReq.RemoteAddr = "127.0.0.1:10000"
	runtimeReq.Header.Set("X-Foundry-Admin-Token", cfg.Admin.AccessToken)
	runtimeRR := httptest.NewRecorder()
	mux.ServeHTTP(runtimeRR, runtimeReq)
	if runtimeRR.Code != http.StatusOK {
		t.Fatalf("expected runtime debug endpoint 200, got %d: %s", runtimeRR.Code, runtimeRR.Body.String())
	}
	var runtimeResp admintypes.RuntimeStatus
	if err := json.Unmarshal(runtimeRR.Body.Bytes(), &runtimeResp); err != nil {
		t.Fatalf("decode runtime debug response: %v", err)
	}
	if runtimeResp.Goroutines <= 0 {
		t.Fatalf("expected positive goroutine count, got %#v", runtimeResp)
	}
	if runtimeResp.Content.ByType == nil || runtimeResp.Storage.MediaBytes == nil || runtimeResp.Activity.RecentAuditByAction == nil {
		t.Fatalf("expected expanded runtime debug payload, got %#v", runtimeResp)
	}
}

func TestDocumentsListAndDetailEndpoints(t *testing.T) {
	cfg := testConfig(t)
	r := newTestRouter(t, cfg)
	mux := http.NewServeMux()
	r.RegisterRoutes(mux)

	listReq := httptest.NewRequest(http.MethodGet, "/__admin/api/documents?include_drafts=1", nil)
	listReq.RemoteAddr = "127.0.0.1:10000"
	listReq.Header.Set("X-Foundry-Admin-Token", cfg.Admin.AccessToken)
	listRR := httptest.NewRecorder()
	mux.ServeHTTP(listRR, listReq)

	if listRR.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", listRR.Code, listRR.Body.String())
	}

	var docs []admintypes.DocumentSummary
	if err := json.Unmarshal(listRR.Body.Bytes(), &docs); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(docs) != 2 {
		t.Fatalf("expected 2 documents, got %d", len(docs))
	}

	detailReq := httptest.NewRequest(http.MethodGet, "/__admin/api/document?id=doc-1&include_drafts=1", nil)
	detailReq.RemoteAddr = "127.0.0.1:10000"
	detailReq.Header.Set("X-Foundry-Admin-Token", cfg.Admin.AccessToken)
	detailRR := httptest.NewRecorder()
	mux.ServeHTTP(detailRR, detailReq)

	if detailRR.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", detailRR.Code, detailRR.Body.String())
	}

	var detail admintypes.DocumentDetail
	if err := json.Unmarshal(detailRR.Body.Bytes(), &detail); err != nil {
		t.Fatalf("decode detail response: %v", err)
	}
	if detail.ID != "doc-1" {
		t.Fatalf("expected doc-1, got %s", detail.ID)
	}
	if !strings.Contains(detail.RawBody, "title: About") || !strings.Contains(detail.RawBody, "# Hello from admin") {
		t.Fatalf("expected raw body to contain full document, got %q", detail.RawBody)
	}
}

func TestSaveAndPreviewEndpoints(t *testing.T) {
	cfg := testConfig(t)
	r := newTestRouter(t, cfg)
	mux := http.NewServeMux()
	r.RegisterRoutes(mux)

	saveBody := `{"source_path":"pages/test-admin.md","raw":"---\ntitle: Test Admin\nslug: test-admin\nlayout: page\ndraft: true\n---\n\n# Hello Admin"}`
	saveReq := httptest.NewRequest(http.MethodPost, "/__admin/api/documents/save", bytes.NewBufferString(saveBody))
	saveReq.RemoteAddr = "127.0.0.1:10000"
	saveReq.Header.Set("Content-Type", "application/json")
	saveReq.Header.Set("X-Foundry-Admin-Token", cfg.Admin.AccessToken)
	saveRR := httptest.NewRecorder()
	mux.ServeHTTP(saveRR, saveReq)

	if saveRR.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", saveRR.Code, saveRR.Body.String())
	}

	savedPath := filepath.Join(cfg.ContentDir, "pages", "test-admin.md")
	b, err := os.ReadFile(savedPath)
	if err != nil {
		t.Fatalf("expected saved file to exist: %v", err)
	}
	if !strings.Contains(string(b), "Hello Admin") {
		t.Fatalf("expected saved content, got %q", string(b))
	}

	previewBody := `{"raw":"---\ntitle: Preview Me\nslug: preview-me\nlayout: page\ndraft: true\n---\n\n# Preview Hello"}`
	previewReq := httptest.NewRequest(http.MethodPost, "/__admin/api/documents/preview", bytes.NewBufferString(previewBody))
	previewReq.RemoteAddr = "127.0.0.1:10000"
	previewReq.Header.Set("Content-Type", "application/json")
	previewReq.Header.Set("X-Foundry-Admin-Token", cfg.Admin.AccessToken)
	previewRR := httptest.NewRecorder()
	mux.ServeHTTP(previewRR, previewReq)

	if previewRR.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", previewRR.Code, previewRR.Body.String())
	}

	var resp admintypes.DocumentPreviewResponse
	if err := json.Unmarshal(previewRR.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode preview response: %v", err)
	}
	if resp.Title != "Preview Me" {
		t.Fatalf("expected title Preview Me, got %q", resp.Title)
	}
	if !strings.Contains(resp.HTML, "Preview Hello") {
		t.Fatalf("expected preview HTML to contain heading text, got %q", resp.HTML)
	}

	var mediaBody bytes.Buffer
	writer := multipart.NewWriter(&mediaBody)
	if err := writer.WriteField("dir", "posts/about"); err != nil {
		t.Fatalf("write field: %v", err)
	}
	part, err := writer.CreateFormFile("file", "diagram.png")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write(testPNGBytes()); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	uploadReq := httptest.NewRequest(http.MethodPost, "/__admin/api/media/upload", &mediaBody)
	uploadReq.RemoteAddr = "127.0.0.1:10000"
	uploadReq.Header.Set("Content-Type", writer.FormDataContentType())
	uploadReq.Header.Set("X-Foundry-Admin-Token", cfg.Admin.AccessToken)
	uploadRR := httptest.NewRecorder()
	mux.ServeHTTP(uploadRR, uploadReq)
	if uploadRR.Code != http.StatusOK {
		t.Fatalf("expected media upload 200, got %d: %s", uploadRR.Code, uploadRR.Body.String())
	}

	var uploadResp admintypes.MediaUploadResponse
	if err := json.Unmarshal(uploadRR.Body.Bytes(), &uploadResp); err != nil {
		t.Fatalf("decode upload response: %v", err)
	}
	if !strings.HasPrefix(uploadResp.Reference, "media:images/posts/about/diagram-") || !strings.HasSuffix(uploadResp.Reference, ".png") {
		t.Fatalf("unexpected upload response: %#v", uploadResp)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/__admin/api/media", nil)
	listReq.RemoteAddr = "127.0.0.1:10000"
	listReq.Header.Set("X-Foundry-Admin-Token", cfg.Admin.AccessToken)
	listRR := httptest.NewRecorder()
	mux.ServeHTTP(listRR, listReq)
	if listRR.Code != http.StatusOK {
		t.Fatalf("expected media list 200, got %d: %s", listRR.Code, listRR.Body.String())
	}
	var mediaItems []admintypes.MediaItem
	if err := json.Unmarshal(listRR.Body.Bytes(), &mediaItems); err != nil {
		t.Fatalf("decode media list: %v", err)
	}
	if len(mediaItems) != 1 || mediaItems[0].Reference != uploadResp.Reference {
		t.Fatalf("unexpected media list: %#v", mediaItems)
	}

	createReq := httptest.NewRequest(http.MethodPost, "/__admin/api/documents/create", strings.NewReader(`{"kind":"page","slug":"new-page","lang":"en","archetype":"page"}`))
	createReq.RemoteAddr = "127.0.0.1:10000"
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("X-Foundry-Admin-Token", cfg.Admin.AccessToken)
	createRR := httptest.NewRecorder()
	mux.ServeHTTP(createRR, createReq)
	if createRR.Code != http.StatusOK {
		t.Fatalf("expected document create 200, got %d: %s", createRR.Code, createRR.Body.String())
	}

	statusReq := httptest.NewRequest(http.MethodPost, "/__admin/api/documents/status", strings.NewReader(`{"source_path":"pages/test-admin.md","status":"archived"}`))
	statusReq.RemoteAddr = "127.0.0.1:10000"
	statusReq.Header.Set("Content-Type", "application/json")
	statusReq.Header.Set("X-Foundry-Admin-Token", cfg.Admin.AccessToken)
	statusRR := httptest.NewRecorder()
	mux.ServeHTTP(statusRR, statusReq)
	if statusRR.Code != http.StatusOK {
		t.Fatalf("expected document status 200, got %d: %s", statusRR.Code, statusRR.Body.String())
	}

	mediaDetailReq := httptest.NewRequest(http.MethodGet, "/__admin/api/media/detail?reference="+uploadResp.Reference, nil)
	mediaDetailReq.RemoteAddr = "127.0.0.1:10000"
	mediaDetailReq.Header.Set("X-Foundry-Admin-Token", cfg.Admin.AccessToken)
	mediaDetailRR := httptest.NewRecorder()
	mux.ServeHTTP(mediaDetailRR, mediaDetailReq)
	if mediaDetailRR.Code != http.StatusOK {
		t.Fatalf("expected media detail 200, got %d: %s", mediaDetailRR.Code, mediaDetailRR.Body.String())
	}

	mediaMetaReq := httptest.NewRequest(http.MethodPost, "/__admin/api/media/metadata", strings.NewReader(`{"reference":"`+uploadResp.Reference+`","metadata":{"title":"Diagram","alt":"Alt text","tags":["docs"]}}`))
	mediaMetaReq.RemoteAddr = "127.0.0.1:10000"
	mediaMetaReq.Header.Set("Content-Type", "application/json")
	mediaMetaReq.Header.Set("X-Foundry-Admin-Token", cfg.Admin.AccessToken)
	mediaMetaRR := httptest.NewRecorder()
	mux.ServeHTTP(mediaMetaRR, mediaMetaReq)
	if mediaMetaRR.Code != http.StatusOK {
		t.Fatalf("expected media metadata 200, got %d: %s", mediaMetaRR.Code, mediaMetaRR.Body.String())
	}

	deleteDocReq := httptest.NewRequest(http.MethodPost, "/__admin/api/documents/delete", strings.NewReader(`{"source_path":"pages/test-admin.md"}`))
	deleteDocReq.RemoteAddr = "127.0.0.1:10000"
	deleteDocReq.Header.Set("Content-Type", "application/json")
	deleteDocReq.Header.Set("X-Foundry-Admin-Token", cfg.Admin.AccessToken)
	deleteDocRR := httptest.NewRecorder()
	mux.ServeHTTP(deleteDocRR, deleteDocReq)
	if deleteDocRR.Code != http.StatusOK {
		t.Fatalf("expected document delete 200, got %d: %s", deleteDocRR.Code, deleteDocRR.Body.String())
	}
}

func TestAdminRoutesRequireTokenWhenConfigured(t *testing.T) {
	cfg := testConfig(t)
	cfg.Admin.LocalOnly = false
	cfg.Admin.AccessToken = "secret-token"

	r := newTestRouter(t, cfg)
	mux := http.NewServeMux()
	r.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/__admin/api/status", nil)
	req.RemoteAddr = "8.8.8.8:10000"
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403 without token, got %d", rr.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/__admin/api/status", nil)
	req.RemoteAddr = "8.8.8.8:10000"
	req.Header.Set("X-Foundry-Admin-Token", "secret-token")
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 with token, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestAdminRejectsOversizedJSONBodies(t *testing.T) {
	cfg := testConfig(t)
	r := newTestRouter(t, cfg)
	mux := http.NewServeMux()
	r.RegisterRoutes(mux)

	raw := strings.Repeat("a", int(largeJSONBodyLimit)+1024)
	req := httptest.NewRequest(http.MethodPost, "/__admin/api/documents/save", bytes.NewBufferString(`{"source_path":"pages/huge.md","raw":"`+raw+`"}`))
	req.RemoteAddr = "127.0.0.1:10000"
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Foundry-Admin-Token", cfg.Admin.AccessToken)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestMediaUploadRejectsDangerousTypesAndSymlinkEscape(t *testing.T) {
	cfg := testConfig(t)
	r := newTestRouter(t, cfg)
	mux := http.NewServeMux()
	r.RegisterRoutes(mux)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "danger.html")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write([]byte("<html><script>alert(1)</script></html>")); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/__admin/api/media/upload", &body)
	req.RemoteAddr = "127.0.0.1:10000"
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-Foundry-Admin-Token", cfg.Admin.AccessToken)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected dangerous upload rejection, got %d: %s", rr.Code, rr.Body.String())
	}

	externalDir := filepath.Join(t.TempDir(), "escaped")
	if err := os.MkdirAll(externalDir, 0o755); err != nil {
		t.Fatalf("mkdir external dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(cfg.ContentDir, cfg.Content.ImagesDir), 0o755); err != nil {
		t.Fatalf("mkdir images dir: %v", err)
	}
	if err := os.Symlink(externalDir, filepath.Join(cfg.ContentDir, cfg.Content.ImagesDir, "linked")); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	body.Reset()
	writer = multipart.NewWriter(&body)
	if err := writer.WriteField("dir", "linked"); err != nil {
		t.Fatalf("write field: %v", err)
	}
	part, err = writer.CreateFormFile("file", "safe.png")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write(testPNGBytes()); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	req = httptest.NewRequest(http.MethodPost, "/__admin/api/media/upload", &body)
	req.RemoteAddr = "127.0.0.1:10000"
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-Foundry-Admin-Token", cfg.Admin.AccessToken)
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected symlink escape rejection, got %d: %s", rr.Code, rr.Body.String())
	}
	if _, err := os.Stat(filepath.Join(externalDir, "safe.png")); !os.IsNotExist(err) {
		t.Fatalf("expected no file to be written outside media root, got %v", err)
	}
}

func TestAdminLoginLogoutAndSessionEndpoints(t *testing.T) {
	cfg := testConfig(t)
	cfg.Admin.AccessToken = ""

	r := newTestRouter(t, cfg)
	mux := http.NewServeMux()
	r.RegisterRoutes(mux)

	loginReq := httptest.NewRequest(http.MethodPost, "/__admin/api/login", strings.NewReader(`{"username":"admin","password":"secret-password"}`))
	loginReq.RemoteAddr = "127.0.0.1:10000"
	loginReq.Header.Set("Content-Type", "application/json")
	loginRR := httptest.NewRecorder()
	mux.ServeHTTP(loginRR, loginReq)
	if loginRR.Code != http.StatusOK {
		t.Fatalf("expected login 200, got %d: %s", loginRR.Code, loginRR.Body.String())
	}
	cookies := loginRR.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected login session cookie")
	}
	var loginResp admintypes.SessionResponse
	if err := json.Unmarshal(loginRR.Body.Bytes(), &loginResp); err != nil {
		t.Fatalf("decode login response: %v", err)
	}

	sessionReq := httptest.NewRequest(http.MethodGet, "/__admin/api/session", nil)
	sessionReq.RemoteAddr = "127.0.0.1:10000"
	sessionReq.AddCookie(cookies[0])
	sessionRR := httptest.NewRecorder()
	mux.ServeHTTP(sessionRR, sessionReq)
	if sessionRR.Code != http.StatusOK {
		t.Fatalf("expected session 200, got %d: %s", sessionRR.Code, sessionRR.Body.String())
	}

	var sessionResp admintypes.SessionResponse
	if err := json.Unmarshal(sessionRR.Body.Bytes(), &sessionResp); err != nil {
		t.Fatalf("decode session response: %v", err)
	}
	if !sessionResp.Authenticated || sessionResp.Username != "admin" {
		t.Fatalf("unexpected session response: %#v", sessionResp)
	}

	logoutReq := httptest.NewRequest(http.MethodPost, "/__admin/api/logout", nil)
	logoutReq.RemoteAddr = "127.0.0.1:10000"
	logoutReq.AddCookie(cookies[0])
	logoutReq.Header.Set("X-Foundry-CSRF-Token", loginResp.CSRFToken)
	logoutRR := httptest.NewRecorder()
	mux.ServeHTTP(logoutRR, logoutReq)
	if logoutRR.Code != http.StatusOK {
		t.Fatalf("expected logout 200, got %d: %s", logoutRR.Code, logoutRR.Body.String())
	}
}

func TestManagementEndpoints(t *testing.T) {
	cfg := testConfig(t)
	r := newTestRouter(t, cfg)
	mux := http.NewServeMux()
	r.RegisterRoutes(mux)

	loginReq := httptest.NewRequest(http.MethodPost, "/__admin/api/login", strings.NewReader(`{"username":"admin","password":"secret-password"}`))
	loginReq.RemoteAddr = "127.0.0.1:10000"
	loginReq.Header.Set("Content-Type", "application/json")
	loginRR := httptest.NewRecorder()
	mux.ServeHTTP(loginRR, loginReq)
	cookie := loginRR.Result().Cookies()[0]
	var loginSession admintypes.SessionResponse
	if err := json.Unmarshal(loginRR.Body.Bytes(), &loginSession); err != nil {
		t.Fatalf("decode login session: %v", err)
	}

	doReq := func(method, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.RemoteAddr = "127.0.0.1:10000"
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		req.AddCookie(cookie)
		if method != http.MethodGet && method != http.MethodHead && method != http.MethodOptions {
			req.Header.Set("X-Foundry-CSRF-Token", loginSession.CSRFToken)
		}
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)
		return rr
	}

	if rr := doReq(http.MethodGet, "/__admin/documents", ""); rr.Code != http.StatusOK {
		t.Fatalf("expected documents shell route 200, got %d", rr.Code)
	}
	if rr := doReq(http.MethodGet, "/__admin/api/users", ""); rr.Code != http.StatusOK {
		t.Fatalf("expected users 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if rr := doReq(http.MethodPost, "/__admin/api/users/save", `{"username":"editor","name":"Editor User","email":"editor@example.com","password":"Secret-password1!"}`); rr.Code != http.StatusOK {
		t.Fatalf("expected save user 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if rr := doReq(http.MethodPost, "/__admin/api/users/save", `{"username":"editor","name":"Updated Editor","email":"updated@example.com"}`); rr.Code != http.StatusOK {
		t.Fatalf("expected update user 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if rr := doReq(http.MethodGet, "/__admin/api/config", ""); rr.Code != http.StatusOK {
		t.Fatalf("expected config 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if rr := doReq(http.MethodGet, "/__admin/api/settings/form", ""); rr.Code != http.StatusOK {
		t.Fatalf("expected settings form 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if rr := doReq(http.MethodPost, "/__admin/api/settings/form/save", `{"value":{"Title":"Updated Title","Theme":"default","DefaultLang":"en","ContentDir":"content","PublicDir":"public","ThemesDir":"themes","DataDir":"data","PluginsDir":"plugins","Server":{"Addr":":8080"},"Feed":{"RSSPath":"/rss.xml","SitemapPath":"/sitemap.xml"},"Admin":{"LocalOnly":false}}}`); rr.Code != http.StatusOK {
		t.Fatalf("expected settings form save 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if rr := doReq(http.MethodGet, "/__admin/api/settings/custom-css", ""); rr.Code != http.StatusOK {
		t.Fatalf("expected custom css 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if rr := doReq(http.MethodPost, "/__admin/api/settings/custom-css/save", `{"raw":"body { color: #123456; }"}`); rr.Code != http.StatusOK {
		t.Fatalf("expected save custom css 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if rr := doReq(http.MethodGet, "/__admin/api/plugins", ""); rr.Code != http.StatusOK {
		t.Fatalf("expected plugins 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if rr := doReq(http.MethodPost, "/__admin/api/plugins/validate", `{"name":"alpha"}`); rr.Code != http.StatusOK {
		t.Fatalf("expected plugin validate 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if rr := doReq(http.MethodGet, "/__admin/api/themes", ""); rr.Code != http.StatusOK {
		t.Fatalf("expected themes 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if rr := doReq(http.MethodPost, "/__admin/api/themes/validate", `{"name":"default","kind":"frontend"}`); rr.Code != http.StatusOK {
		t.Fatalf("expected theme validate 200, got %d: %s", rr.Code, rr.Body.String())
	}
	auditRR := doReq(http.MethodGet, "/__admin/api/audit", "")
	if auditRR.Code != http.StatusOK {
		t.Fatalf("expected audit 200, got %d: %s", auditRR.Code, auditRR.Body.String())
	}
	var auditEntries []admintypes.AuditEntry
	if err := json.Unmarshal(auditRR.Body.Bytes(), &auditEntries); err != nil {
		t.Fatalf("decode audit entries: %v", err)
	}
	if len(auditEntries) == 0 {
		t.Fatal("expected audit log entries")
	}
}

func TestDocumentAndMediaHistoryEndpoints(t *testing.T) {
	cfg := testConfig(t)
	r := newTestRouter(t, cfg)
	mux := http.NewServeMux()
	r.RegisterRoutes(mux)

	doReq := func(method, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.RemoteAddr = "127.0.0.1:10000"
		req.Header.Set("X-Foundry-Admin-Token", cfg.Admin.AccessToken)
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)
		return rr
	}

	if rr := doReq(http.MethodPost, "/__admin/api/documents/save", `{"source_path":"pages/about.md","raw":"---\ntitle: About\nslug: about\nlayout: page\ndraft: false\n---\n\n# About v2","version_comment":"Polish the about page"}`); rr.Code != http.StatusOK {
		t.Fatalf("expected document save 200, got %d: %s", rr.Code, rr.Body.String())
	}
	historyRR := doReq(http.MethodGet, "/__admin/api/documents/history?source_path=pages/about.md", "")
	if historyRR.Code != http.StatusOK {
		t.Fatalf("expected document history 200, got %d: %s", historyRR.Code, historyRR.Body.String())
	}
	var docHistory admintypes.DocumentHistoryResponse
	if err := json.Unmarshal(historyRR.Body.Bytes(), &docHistory); err != nil {
		t.Fatalf("decode document history: %v", err)
	}
	if len(docHistory.Entries) != 2 {
		t.Fatalf("expected 2 document history entries, got %#v", docHistory.Entries)
	}
	if docHistory.Entries[1].VersionComment != "Polish the about page" {
		t.Fatalf("expected version comment, got %#v", docHistory.Entries[1])
	}

	diffReq := `{"left_path":"` + docHistory.Entries[1].Path + `","right_path":"` + docHistory.Entries[0].Path + `"}`
	diffRR := doReq(http.MethodPost, "/__admin/api/documents/diff", diffReq)
	if diffRR.Code != http.StatusOK {
		t.Fatalf("expected document diff 200, got %d: %s", diffRR.Code, diffRR.Body.String())
	}

	if rr := doReq(http.MethodPost, "/__admin/api/documents/delete", `{"source_path":"pages/about.md"}`); rr.Code != http.StatusOK {
		t.Fatalf("expected document delete 200, got %d: %s", rr.Code, rr.Body.String())
	}
	trashRR := doReq(http.MethodGet, "/__admin/api/documents/trash", "")
	if trashRR.Code != http.StatusOK {
		t.Fatalf("expected document trash 200, got %d: %s", trashRR.Code, trashRR.Body.String())
	}
	var docTrash []admintypes.DocumentHistoryEntry
	if err := json.Unmarshal(trashRR.Body.Bytes(), &docTrash); err != nil {
		t.Fatalf("decode document trash: %v", err)
	}
	if len(docTrash) == 0 {
		t.Fatal("expected trashed document entry")
	}
	restoreReq := `{"path":"` + docTrash[0].Path + `"}`
	if rr := doReq(http.MethodPost, "/__admin/api/documents/restore", restoreReq); rr.Code != http.StatusOK {
		t.Fatalf("expected document restore 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var mediaBody bytes.Buffer
	writer := multipart.NewWriter(&mediaBody)
	if err := writer.WriteField("collection", "images"); err != nil {
		t.Fatalf("write media collection field: %v", err)
	}
	part, err := writer.CreateFormFile("file", "history.png")
	if err != nil {
		t.Fatalf("create media file: %v", err)
	}
	if _, err := part.Write(testPNGBytes()); err != nil {
		t.Fatalf("write media body: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close media writer: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/__admin/api/media/upload", &mediaBody)
	req.RemoteAddr = "127.0.0.1:10000"
	req.Header.Set("X-Foundry-Admin-Token", cfg.Admin.AccessToken)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected media upload 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var upload admintypes.MediaUploadResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &upload); err != nil {
		t.Fatalf("decode media upload: %v", err)
	}

	var replaceBody bytes.Buffer
	replaceWriter := multipart.NewWriter(&replaceBody)
	if err := replaceWriter.WriteField("collection", "images"); err != nil {
		t.Fatalf("write media collection field: %v", err)
	}
	if err := replaceWriter.WriteField("reference", upload.Reference); err != nil {
		t.Fatalf("write media reference field: %v", err)
	}
	part, err = replaceWriter.CreateFormFile("file", "history.png")
	if err != nil {
		t.Fatalf("create replace media file: %v", err)
	}
	if _, err := part.Write(testPNGBytes()); err != nil {
		t.Fatalf("write replace media body: %v", err)
	}
	if err := replaceWriter.Close(); err != nil {
		t.Fatalf("close replace writer: %v", err)
	}
	req = httptest.NewRequest(http.MethodPost, "/__admin/api/media/replace", &replaceBody)
	req.RemoteAddr = "127.0.0.1:10000"
	req.Header.Set("X-Foundry-Admin-Token", cfg.Admin.AccessToken)
	req.Header.Set("Content-Type", replaceWriter.FormDataContentType())
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected media replace 200, got %d: %s", rr.Code, rr.Body.String())
	}

	mediaListRR := doReq(http.MethodGet, "/__admin/api/media", "")
	if mediaListRR.Code != http.StatusOK {
		t.Fatalf("expected media list 200, got %d: %s", mediaListRR.Code, mediaListRR.Body.String())
	}
	var mediaList []admintypes.MediaItem
	if err := json.Unmarshal(mediaListRR.Body.Bytes(), &mediaList); err != nil {
		t.Fatalf("decode media list: %v", err)
	}
	if len(mediaList) != 1 {
		t.Fatalf("expected one current media item, got %#v", mediaList)
	}

	mediaHistoryRR := doReq(http.MethodGet, "/__admin/api/media/history?path="+filepath.ToSlash(filepath.Join("content", mediaList[0].Collection, mediaList[0].Path)), "")
	if mediaHistoryRR.Code != http.StatusOK {
		t.Fatalf("expected media history 200, got %d: %s", mediaHistoryRR.Code, mediaHistoryRR.Body.String())
	}
	var mediaHistory admintypes.MediaHistoryResponse
	if err := json.Unmarshal(mediaHistoryRR.Body.Bytes(), &mediaHistory); err != nil {
		t.Fatalf("decode media history: %v", err)
	}
	if len(mediaHistory.Entries) < 2 {
		t.Fatalf("expected media history entries, got %#v", mediaHistory.Entries)
	}

	if rr := doReq(http.MethodPost, "/__admin/api/media/delete", `{"reference":"`+mediaList[0].Reference+`"}`); rr.Code != http.StatusOK {
		t.Fatalf("expected media delete 200, got %d: %s", rr.Code, rr.Body.String())
	}
	mediaTrashRR := doReq(http.MethodGet, "/__admin/api/media/trash", "")
	if mediaTrashRR.Code != http.StatusOK {
		t.Fatalf("expected media trash 200, got %d: %s", mediaTrashRR.Code, mediaTrashRR.Body.String())
	}
	var mediaTrash []admintypes.MediaHistoryEntry
	if err := json.Unmarshal(mediaTrashRR.Body.Bytes(), &mediaTrash); err != nil {
		t.Fatalf("decode media trash: %v", err)
	}
	if len(mediaTrash) == 0 {
		t.Fatal("expected trashed media entry")
	}
	if rr := doReq(http.MethodPost, "/__admin/api/media/restore", `{"path":"`+mediaTrash[0].Path+`"}`); rr.Code != http.StatusOK {
		t.Fatalf("expected media restore 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestEditorRoleCannotAccessAdminOnlyEndpoints(t *testing.T) {
	cfg := testConfig(t)
	cfg.Admin.AccessToken = ""
	r := newTestRouter(t, cfg)
	mux := http.NewServeMux()
	r.RegisterRoutes(mux)

	loginReq := httptest.NewRequest(http.MethodPost, "/__admin/api/login", strings.NewReader(`{"username":"editor","password":"editor-password"}`))
	loginReq.RemoteAddr = "127.0.0.1:10000"
	loginReq.Header.Set("Content-Type", "application/json")
	loginRR := httptest.NewRecorder()
	mux.ServeHTTP(loginRR, loginReq)
	if loginRR.Code != http.StatusOK {
		t.Fatalf("expected editor login 200, got %d: %s", loginRR.Code, loginRR.Body.String())
	}
	cookie := loginRR.Result().Cookies()[0]

	req := httptest.NewRequest(http.MethodGet, "/__admin/api/documents?include_drafts=1", nil)
	req.RemoteAddr = "127.0.0.1:10000"
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected editor document access 200, got %d: %s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/__admin/api/users", nil)
	req.RemoteAddr = "127.0.0.1:10000"
	req.AddCookie(cookie)
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected editor user-management access to be forbidden, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestAdminIndexMethodAndErrorPaths(t *testing.T) {
	cfg := testConfig(t)
	r := newTestRouter(t, cfg)
	mux := http.NewServeMux()
	r.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/__admin", nil)
	req.RemoteAddr = "127.0.0.1:10000"
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `data-admin-base="/__admin"`) || !strings.Contains(rr.Body.String(), "/__admin/theme/admin.js") {
		t.Fatalf("unexpected admin index response: %d %s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/__admin/theme/admin.css", nil)
	req.RemoteAddr = "127.0.0.1:10000"
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), "--bg:") {
		t.Fatalf("unexpected admin theme asset response: %d %s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/__admin/api/document", nil)
	req.RemoteAddr = "127.0.0.1:10000"
	req.Header.Set("X-Foundry-Admin-Token", cfg.Admin.AccessToken)
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected missing id bad request, got %d", rr.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/__admin/api/status", nil)
	req.RemoteAddr = "127.0.0.1:10000"
	req.Header.Set("X-Foundry-Admin-Token", cfg.Admin.AccessToken)
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected method not allowed, got %d", rr.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/__admin/api/documents", nil)
	req.RemoteAddr = "127.0.0.1:10000"
	req.Header.Set("X-Foundry-Admin-Token", cfg.Admin.AccessToken)
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected documents method not allowed, got %d", rr.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/__admin/api/document", nil)
	req.RemoteAddr = "127.0.0.1:10000"
	req.Header.Set("X-Foundry-Admin-Token", cfg.Admin.AccessToken)
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected document method not allowed, got %d", rr.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/__admin/api/documents/save", strings.NewReader("{"))
	req.RemoteAddr = "127.0.0.1:10000"
	req.Header.Set("X-Foundry-Admin-Token", cfg.Admin.AccessToken)
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected save bad request, got %d", rr.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/__admin/api/documents/create", nil)
	req.RemoteAddr = "127.0.0.1:10000"
	req.Header.Set("X-Foundry-Admin-Token", cfg.Admin.AccessToken)
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected create method not allowed, got %d", rr.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/__admin/api/documents/preview", strings.NewReader("{"))
	req.RemoteAddr = "127.0.0.1:10000"
	req.Header.Set("X-Foundry-Admin-Token", cfg.Admin.AccessToken)
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected preview bad request, got %d", rr.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/__admin/api/document?id=missing&include_drafts=1", nil)
	req.RemoteAddr = "127.0.0.1:10000"
	req.Header.Set("X-Foundry-Admin-Token", cfg.Admin.AccessToken)
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected document not found, got %d", rr.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/__admin/api/media", nil)
	req.RemoteAddr = "127.0.0.1:10000"
	req.Header.Set("X-Foundry-Admin-Token", cfg.Admin.AccessToken)
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected media method not allowed, got %d", rr.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/__admin/api/media/upload", strings.NewReader("nope"))
	req.RemoteAddr = "127.0.0.1:10000"
	req.Header.Set("Content-Type", "text/plain")
	req.Header.Set("X-Foundry-Admin-Token", cfg.Admin.AccessToken)
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected media upload bad request, got %d", rr.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/__admin/api/media/detail", nil)
	req.RemoteAddr = "127.0.0.1:10000"
	req.Header.Set("X-Foundry-Admin-Token", cfg.Admin.AccessToken)
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected media detail bad request, got %d", rr.Code)
	}
}

func TestAdminCustomPathRoutesAndAssets(t *testing.T) {
	cfg := testConfig(t)
	cfg.Admin.Path = "/cms-admin"
	cfg.ApplyDefaults()
	if err := os.MkdirAll(filepath.Join(cfg.PluginsDir, "search", "admin"), 0o755); err != nil {
		t.Fatalf("mkdir plugin admin assets: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cfg.PluginsDir, "search", "admin", "console.js"), []byte(`export default function () {}`), 0o644); err != nil {
		t.Fatalf("write plugin admin asset: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cfg.PluginsDir, "search", "admin", "secret.txt"), []byte(`top-secret`), 0o644); err != nil {
		t.Fatalf("write undeclared plugin admin asset: %v", err)
	}
	svc := service.New(cfg, service.WithPluginMetadata(func() map[string]plugins.Metadata {
		return map[string]plugins.Metadata{
			"search": {
				Name: "search",
				AdminExtensions: plugins.AdminExtensions{
					Pages: []plugins.AdminPage{{Key: "search-console", Title: "Search Console", Route: "/plugins/search", Module: "admin/console.js"}},
				},
			},
		}
	}))
	r := New(cfg, svc)
	mux := http.NewServeMux()
	r.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/cms-admin", nil)
	req.RemoteAddr = "127.0.0.1:10000"
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `data-admin-base="/cms-admin"`) || !strings.Contains(rr.Body.String(), "/cms-admin/theme/admin.js") {
		t.Fatalf("unexpected custom admin index response: %d %s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/cms-admin/theme/admin.css", nil)
	req.RemoteAddr = "127.0.0.1:10000"
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("unexpected custom theme asset response: %d %s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/cms-admin/api/status", nil)
	req.RemoteAddr = "127.0.0.1:10000"
	req.Header.Set("X-Foundry-Admin-Token", cfg.Admin.AccessToken)
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected custom admin status route to work, got %d: %s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/cms-admin/extensions/search/admin/console.js", nil)
	req.RemoteAddr = "127.0.0.1:10000"
	req.Header.Set("X-Foundry-Admin-Token", cfg.Admin.AccessToken)
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), "export default") {
		t.Fatalf("unexpected custom plugin extension asset response: %d %s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/cms-admin/a-extensions", nil)
	req.RemoteAddr = "127.0.0.1:10000"
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `data-admin-base="/cms-admin"`) {
		t.Fatalf("expected admin extensions section route to serve the admin shell, got %d: %s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/cms-admin/extensions/search/admin/secret.txt", nil)
	req.RemoteAddr = "127.0.0.1:10000"
	req.Header.Set("X-Foundry-Admin-Token", cfg.Admin.AccessToken)
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected undeclared plugin asset to be hidden, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHelpersAndHookWrapping(t *testing.T) {
	if !truthy("yes") || truthy("no") {
		t.Fatal("unexpected truthy helper behavior")
	}

	rr := httptest.NewRecorder()
	writeJSONError(rr, http.StatusBadRequest, errors.New("boom"))
	if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), "boom") {
		t.Fatalf("unexpected writeJSONError response: %d %s", rr.Code, rr.Body.String())
	}

	base := stubHooks{}
	admin := &Router{}
	hooks := WrapHooks(base, admin)
	if hooks.OnServerStarted("addr") != nil || hooks.OnRoutesAssigned(nil) != nil || hooks.OnAssetsBuilding(nil) != nil {
		t.Fatal("expected wrapped hooks to delegate cleanly")
	}
	if _, ok := NewHooks(&config.Config{}, nil).(hookBase); !ok {
		t.Fatal("expected disabled admin hooks to return hookBase when no base hooks")
	}
	if _, ok := NewHooks(&config.Config{Admin: config.AdminConfig{Enabled: true}}, base).(hookSet); !ok {
		t.Fatal("expected enabled admin hooks to wrap base hooks")
	}

	r := New(&config.Config{Admin: config.AdminConfig{Enabled: true}}, service.New(testConfig(t)))
	before := len(r.registrars)
	r.RegisterRegistrar(nil)
	if len(r.registrars) != before {
		t.Fatal("expected nil registrar to be ignored")
	}

	mux := http.NewServeMux()
	WrapHooks(nil, nil).RegisterRoutes(mux)

	empty := httptest.NewRecorder()
	New(nil, nil).RegisterRoutes(nil)
	writeJSON(empty, http.StatusCreated, map[string]string{"ok": "1"})
	if empty.Code != http.StatusCreated || !strings.Contains(empty.Body.String(), "\"ok\":\"1\"") {
		t.Fatalf("unexpected writeJSON response: %d %s", empty.Code, empty.Body.String())
	}
}

type stubHooks struct{}

func (stubHooks) RegisterRoutes(*http.ServeMux)             {}
func (stubHooks) OnServerStarted(string) error              { return nil }
func (stubHooks) OnRoutesAssigned(*content.SiteGraph) error { return nil }
func (stubHooks) OnAssetsBuilding(*config.Config) error     { return nil }

var _ server.Hooks = stubHooks{}

func newTestRouter(t *testing.T, cfg *config.Config) *Router {
	t.Helper()
	if err := os.WriteFile(filepath.Join(cfg.ContentDir, "pages", "about.md"), []byte("---\ntitle: About\nslug: about\nlayout: page\n---\n\n# Hello from admin"), 0o644); err != nil {
		t.Fatalf("write about document: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cfg.ContentDir, "posts", "draft-post.md"), []byte("---\ntitle: Draft Post\nslug: draft-post\nlayout: post\ndraft: true\n---\n\n# Draft body"), 0o644); err != nil {
		t.Fatalf("write draft document: %v", err)
	}
	svc := service.New(cfg, service.WithGraphLoader(func(context.Context, *config.Config, bool) (*content.SiteGraph, error) {
		g := content.NewSiteGraph(cfg)
		g.Add(&content.Document{
			ID:         "doc-1",
			Type:       "page",
			Lang:       cfg.DefaultLang,
			Title:      "About",
			Slug:       "about",
			URL:        "/about/",
			Layout:     "page",
			SourcePath: filepath.ToSlash(filepath.Join(cfg.ContentDir, "pages", "about.md")),
			RawBody:    "# Hello from admin",
			HTMLBody:   template.HTML("<h1>Hello from admin</h1>"),
			Summary:    "About page",
			Taxonomies: map[string][]string{"tags": {"intro"}},
		})
		g.Add(&content.Document{
			ID:         "doc-2",
			Type:       "post",
			Lang:       cfg.DefaultLang,
			Title:      "Draft Post",
			Slug:       "draft-post",
			URL:        "/posts/draft-post/",
			Layout:     "post",
			SourcePath: filepath.ToSlash(filepath.Join(cfg.ContentDir, "posts", "draft-post.md")),
			RawBody:    "# Draft body",
			HTMLBody:   template.HTML("<h1>Draft body</h1>"),
			Summary:    "Draft summary",
			Draft:      true,
		})
		return g, nil
	}))
	return New(cfg, svc)
}

func testConfig(t *testing.T) *config.Config {
	t.Helper()
	root := t.TempDir()
	prevWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(prevWD)
	})
	cfg := &config.Config{
		Name:        "foundry",
		Title:       "Foundry CMS",
		DefaultLang: "en",
		Theme:       "default",
		ContentDir:  filepath.Join(root, "content"),
		PublicDir:   filepath.Join(root, "public"),
		ThemesDir:   filepath.Join(root, "themes"),
		PluginsDir:  filepath.Join(root, "plugins"),
		DataDir:     filepath.Join(root, "data"),
		Admin: config.AdminConfig{
			Enabled:           true,
			LocalOnly:         true,
			AccessToken:       "test-token",
			SessionTTLMinutes: 30,
		},
		Content: config.ContentConfig{
			PagesDir:          "pages",
			PostsDir:          "posts",
			AssetsDir:         "assets",
			ImagesDir:         "images",
			UploadsDir:        "uploads",
			DefaultLayoutPage: "page",
			DefaultLayoutPost: "post",
		},
	}
	cfg.ApplyDefaults()
	_ = os.MkdirAll(filepath.Join(cfg.ContentDir, cfg.Content.PagesDir), 0o755)
	_ = os.MkdirAll(filepath.Join(cfg.ContentDir, cfg.Content.PostsDir), 0o755)
	_ = os.MkdirAll(cfg.ThemesDir, 0o755)
	_ = os.MkdirAll(cfg.PluginsDir, 0o755)
	_ = os.MkdirAll(cfg.DataDir, 0o755)
	_ = os.MkdirAll(filepath.Join(cfg.ContentDir, "config"), 0o755)
	if err := os.WriteFile(filepath.Join(cfg.ContentDir, "config", "site.yaml"), []byte("theme: default\ncontent_dir: content\npublic_dir: public\nthemes_dir: themes\ndata_dir: data\nplugins_dir: plugins\nserver:\n  addr: :8080\nfeed:\n  rss_path: /rss.xml\n  sitemap_path: /sitemap.xml\n"), 0o644); err != nil {
		t.Fatalf("write site config: %v", err)
	}
	writeRouterTheme(t, cfg, "default")
	writeRouterTheme(t, cfg, "alt")
	writeRouterPluginMetadata(t, cfg, "alpha")
	cfg.Plugins.Enabled = []string{"alpha"}

	hash, err := users.HashPassword("secret-password")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	editorHash, err := users.HashPassword("editor-password")
	if err != nil {
		t.Fatalf("hash editor password: %v", err)
	}
	usersPath := filepath.Join(cfg.ContentDir, "config", "admin-users.yaml")
	_ = os.MkdirAll(filepath.Dir(usersPath), 0o755)
	if err := os.WriteFile(usersPath, []byte("users:\n  - username: admin\n    name: Admin User\n    email: admin@example.com\n    role: admin\n    password_hash: "+hash+"\n  - username: editor\n    name: Editor User\n    email: editor@example.com\n    role: editor\n    password_hash: "+editorHash+"\n"), 0o644); err != nil {
		t.Fatalf("write admin users file: %v", err)
	}
	cfg.Admin.UsersFile = usersPath
	return cfg
}

func writeRouterTheme(t *testing.T, cfg *config.Config, name string) {
	t.Helper()
	root := filepath.Join(cfg.ThemesDir, name)
	if err := os.MkdirAll(filepath.Join(root, "layouts", "partials"), 0o755); err != nil {
		t.Fatalf("mkdir theme: %v", err)
	}
	files := map[string]string{
		filepath.Join(root, "theme.yaml"):                         "name: " + name + "\ntitle: " + name + "\nversion: 0.1.0\nlayouts:\n  - base\n  - index\n  - page\n  - post\n  - list\nslots:\n  - head.end\n  - body.start\n  - body.end\n  - page.before_main\n  - page.after_main\n  - page.before_content\n  - page.after_content\n  - post.before_header\n  - post.after_header\n  - post.before_content\n  - post.after_content\n  - post.sidebar.top\n  - post.sidebar.overview\n  - post.sidebar.bottom\n",
		filepath.Join(root, "layouts", "base.html"):               `{{ define "base" }}{{ pluginSlot "body.start" }}{{ pluginSlot "page.before_main" }}{{ template "content" . }}{{ pluginSlot "page.after_main" }}{{ pluginSlot "body.end" }}{{ end }}`,
		filepath.Join(root, "layouts", "index.html"):              `{{ define "content" }}index{{ end }}`,
		filepath.Join(root, "layouts", "page.html"):               `{{ define "content" }}{{ pluginSlot "page.before_content" }}{{ pluginSlot "page.after_content" }}{{ end }}`,
		filepath.Join(root, "layouts", "post.html"):               `{{ define "content" }}{{ pluginSlot "post.before_header" }}{{ pluginSlot "post.after_header" }}{{ pluginSlot "post.before_content" }}{{ pluginSlot "post.after_content" }}{{ pluginSlot "post.sidebar.top" }}{{ pluginSlot "post.sidebar.overview" }}{{ pluginSlot "post.sidebar.bottom" }}{{ end }}`,
		filepath.Join(root, "layouts", "list.html"):               `{{ define "content" }}list{{ end }}`,
		filepath.Join(root, "layouts", "partials", "head.html"):   `{{ define "head" }}{{ pluginSlot "head.end" }}{{ end }}`,
		filepath.Join(root, "layouts", "partials", "header.html"): `{{ define "header" }}{{ end }}`,
		filepath.Join(root, "layouts", "partials", "footer.html"): `{{ define "footer" }}{{ end }}`,
	}
	for path, body := range files {
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatalf("write theme file: %v", err)
		}
	}
}

func writeRouterPluginMetadata(t *testing.T, cfg *config.Config, name string) {
	t.Helper()
	root := filepath.Join(cfg.PluginsDir, name)
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir plugin: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "plugin.yaml"), []byte("name: "+name+"\ntitle: "+name+"\nversion: 0.1.0\nrepo: github.com/acme/"+name+"\nfoundry_api: v1\nmin_foundry_version: 0.1.0\n"), 0o644); err != nil {
		t.Fatalf("write plugin metadata: %v", err)
	}
}

func testPNGBytes() []byte {
	return []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0x00, 0x00, 0x00, 0x0d, 'I', 'H', 'D', 'R'}
}
