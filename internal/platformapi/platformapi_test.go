package platformapi

import (
	"encoding/json"
	"html/template"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sphireinc/foundry/internal/config"
	"github.com/sphireinc/foundry/internal/content"
)

func TestPublicAPIEndpoints(t *testing.T) {
	cfg, graph := testGraph(t)
	api := &API{cfg: cfg}
	api.SetGraph(graph)

	mux := http.NewServeMux()
	api.RegisterRoutes(mux)

	assertJSON := func(path string, target any) {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("%s returned %d: %s", path, rr.Code, rr.Body.String())
		}
		if err := json.Unmarshal(rr.Body.Bytes(), target); err != nil {
			t.Fatalf("decode %s: %v", path, err)
		}
	}

	var caps CapabilitiesResponse
	assertJSON(APIBase+"/capabilities", &caps)
	if !caps.Modules["content"] || !caps.Features["search"] {
		t.Fatalf("unexpected capabilities: %#v", caps)
	}

	var site SiteInfoResponse
	assertJSON(APIBase+"/site", &site)
	if site.Title != cfg.Title {
		t.Fatalf("expected site title %q, got %#v", cfg.Title, site)
	}

	var route RouteRecord
	assertJSON(APIBase+"/routes/resolve?path=/posts/hello-world/", &route)
	if route.ContentID != "doc-1" {
		t.Fatalf("expected route to resolve doc-1, got %#v", route)
	}

	assertJSON(APIBase+"/routes/resolve?path=/search/", &route)
	if route.Kind != "search" {
		t.Fatalf("expected search route record, got %#v", route)
	}

	assertJSON(APIBase+"/routes/resolve?path=/authors/jane-editor/", &route)
	if route.Kind != "author" || route.Title != "Jane Editor" {
		t.Fatalf("expected author route record, got %#v", route)
	}

	var detail ContentDetail
	assertJSON(APIBase+"/content?id=doc-1", &detail)
	if detail.Title != "Hello World" || detail.HTMLBody == "" {
		t.Fatalf("unexpected content detail: %#v", detail)
	}

	var collections CollectionResponse
	assertJSON(APIBase+"/collections?type=post", &collections)
	if len(collections.Items) != 1 || collections.Items[0].ID != "doc-1" {
		t.Fatalf("unexpected collections payload: %#v", collections)
	}

	var search struct {
		Items []SearchEntry `json:"items"`
	}
	assertJSON(APIBase+"/search?q=hello", &search)
	if len(search.Items) != 1 || search.Items[0].URL != "/posts/hello-world/" {
		t.Fatalf("unexpected search payload: %#v", search)
	}

	var preview struct {
		Links []map[string]any `json:"links"`
	}
	assertJSON(APIBase+"/preview", &preview)
	if len(preview.Links) != 1 {
		t.Fatalf("expected preview manifest to include one non-published entry, got %#v", preview)
	}
}

func TestWriteStaticArtifacts(t *testing.T) {
	cfg, graph := testGraph(t)
	if err := WriteStaticArtifacts(cfg, graph); err != nil {
		t.Fatalf("write static artifacts: %v", err)
	}

	paths := []string{
		filepath.Join(cfg.PublicDir, "__foundry", "capabilities.json"),
		filepath.Join(cfg.PublicDir, "__foundry", "site.json"),
		filepath.Join(cfg.PublicDir, "__foundry", "navigation.json"),
		filepath.Join(cfg.PublicDir, "__foundry", "routes.json"),
		filepath.Join(cfg.PublicDir, "__foundry", "collections.json"),
		filepath.Join(cfg.PublicDir, "__foundry", "search.json"),
		filepath.Join(cfg.PublicDir, "__foundry", "preview.json"),
		filepath.Join(cfg.PublicDir, "__foundry", "content", "doc-1.json"),
		filepath.Join(cfg.PublicDir, "__foundry", "sdk", "frontend", "index.js"),
		filepath.Join(cfg.PublicDir, "__foundry", "sdk", "admin", "index.js"),
	}
	for _, path := range paths {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected artifact %s: %v", path, err)
		}
	}
}

func testGraph(t *testing.T) (*config.Config, *content.SiteGraph) {
	t.Helper()
	root := t.TempDir()
	cfg := &config.Config{
		Name:        "foundry",
		Title:       "Foundry Test",
		BaseURL:     "https://example.com",
		Theme:       "default",
		Environment: "test",
		DefaultLang: "en",
		ContentDir:  filepath.Join(root, "content"),
		PublicDir:   filepath.Join(root, "public"),
		ThemesDir:   filepath.Join(root, "themes"),
		DataDir:     filepath.Join(root, "data"),
		Menus: map[string][]config.MenuItem{
			"main": {
				{Name: "Home", URL: "/"},
				{Name: "Blog", URL: "/posts/hello-world/"},
			},
		},
	}
	cfg.ApplyDefaults()
	graph := content.NewSiteGraph(cfg)
	now := time.Date(2026, 3, 21, 12, 0, 0, 0, time.UTC)
	graph.Add(&content.Document{
		ID:         "doc-1",
		Type:       "post",
		Lang:       "en",
		Title:      "Hello World",
		Slug:       "hello-world",
		URL:        "/posts/hello-world/",
		Layout:     "post",
		SourcePath: filepath.Join(cfg.ContentDir, "posts", "hello-world.md"),
		RawBody:    "# Hello World\n\nFoundry content body.",
		HTMLBody:   template.HTML("<h1>Hello World</h1><p>Foundry content body.</p>"),
		Summary:    "Foundry content body.",
		Date:       &now,
		Draft:      false,
		Status:     "published",
		Author:     "Jane Editor",
		Taxonomies: map[string][]string{"tags": {"go"}},
	})
	graph.Add(&content.Document{
		ID:         "doc-2",
		Type:       "page",
		Lang:       "en",
		Title:      "Draft Preview",
		Slug:       "draft-preview",
		URL:        "/draft-preview/",
		Layout:     "page",
		SourcePath: filepath.Join(cfg.ContentDir, "pages", "draft-preview.md"),
		RawBody:    "# Draft Preview",
		HTMLBody:   template.HTML("<h1>Draft Preview</h1>"),
		Summary:    "Preview-only document.",
		Date:       &now,
		Draft:      true,
		Status:     "draft",
	})
	return cfg, graph
}
