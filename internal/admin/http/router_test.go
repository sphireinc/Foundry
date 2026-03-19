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
	if _, err := part.Write([]byte("png")); err != nil {
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
	if uploadResp.Reference != "media:images/posts/about/diagram.png" {
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

	doReq := func(method, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.RemoteAddr = "127.0.0.1:10000"
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		req.AddCookie(cookie)
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
	if rr := doReq(http.MethodPost, "/__admin/api/users/save", `{"username":"editor","name":"Editor User","email":"editor@example.com","password":"secret-password"}`); rr.Code != http.StatusOK {
		t.Fatalf("expected save user 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if rr := doReq(http.MethodPost, "/__admin/api/users/save", `{"username":"editor","name":"Updated Editor","email":"updated@example.com"}`); rr.Code != http.StatusOK {
		t.Fatalf("expected update user 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if rr := doReq(http.MethodGet, "/__admin/api/config", ""); rr.Code != http.StatusOK {
		t.Fatalf("expected config 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if rr := doReq(http.MethodGet, "/__admin/api/plugins", ""); rr.Code != http.StatusOK {
		t.Fatalf("expected plugins 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if rr := doReq(http.MethodGet, "/__admin/api/themes", ""); rr.Code != http.StatusOK {
		t.Fatalf("expected themes 200, got %d: %s", rr.Code, rr.Body.String())
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
	usersPath := filepath.Join(cfg.ContentDir, "config", "admin-users.yaml")
	_ = os.MkdirAll(filepath.Dir(usersPath), 0o755)
	if err := os.WriteFile(usersPath, []byte("users:\n  - username: admin\n    name: Admin User\n    email: admin@example.com\n    role: admin\n    password_hash: "+hash+"\n"), 0o644); err != nil {
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
