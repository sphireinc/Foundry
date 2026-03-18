package httpadmin

import (
	"bytes"
	"context"
	"encoding/json"
	"html/template"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sphireinc/foundry/internal/admin/service"
	admintypes "github.com/sphireinc/foundry/internal/admin/types"
	"github.com/sphireinc/foundry/internal/config"
	"github.com/sphireinc/foundry/internal/content"
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
	if !strings.Contains(detail.RawBody, "Hello") {
		t.Fatalf("expected raw body to contain Hello, got %q", detail.RawBody)
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

func newTestRouter(t *testing.T, cfg *config.Config) *Router {
	t.Helper()
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
			Enabled:     true,
			LocalOnly:   true,
			AccessToken: "test-token",
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
	return cfg
}
