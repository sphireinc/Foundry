package service

import (
	"context"
	"html/template"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sphireinc/foundry/internal/admin/types"
	"github.com/sphireinc/foundry/internal/config"
	"github.com/sphireinc/foundry/internal/content"
	"github.com/sphireinc/foundry/internal/taxonomy"
)

func TestLoadCachesGraphsWithinTTL(t *testing.T) {
	cfg := testServiceConfig(t)
	var loads int

	svc := New(cfg, WithGraphLoader(func(context.Context, *config.Config, bool) (*content.SiteGraph, error) {
		loads++
		return content.NewSiteGraph(cfg), nil
	}))

	if _, err := svc.load(context.Background(), true); err != nil {
		t.Fatalf("first load failed: %v", err)
	}
	if _, err := svc.load(context.Background(), true); err != nil {
		t.Fatalf("second load failed: %v", err)
	}

	if loads != 1 {
		t.Fatalf("expected one graph load, got %d", loads)
	}
}

func TestSaveDocumentInvalidatesGraphCache(t *testing.T) {
	cfg := testServiceConfig(t)
	var loads int

	svc := New(cfg, WithGraphLoader(func(context.Context, *config.Config, bool) (*content.SiteGraph, error) {
		loads++
		return content.NewSiteGraph(cfg), nil
	}))

	if _, err := svc.load(context.Background(), true); err != nil {
		t.Fatalf("prime cache failed: %v", err)
	}
	if loads != 1 {
		t.Fatalf("expected one graph load, got %d", loads)
	}

	_, err := svc.SaveDocument(context.Background(), types.DocumentSaveRequest{
		SourcePath: filepath.Join("pages", "cache-test.md"),
		Raw:        "---\ntitle: Cache Test\nslug: cache-test\n---\n\nBody",
	})
	if err != nil {
		t.Fatalf("save document failed: %v", err)
	}

	if _, err := svc.load(context.Background(), true); err != nil {
		t.Fatalf("load after save failed: %v", err)
	}
	if loads != 2 {
		t.Fatalf("expected cache invalidation to force second load, got %d loads", loads)
	}
}

func TestDocumentQueriesPreviewAndStatus(t *testing.T) {
	cfg := testServiceConfig(t)
	now := time.Now()
	graph := content.NewSiteGraph(cfg)
	doc := &content.Document{
		ID:         "doc-1",
		Type:       "page",
		Lang:       "en",
		Title:      "About",
		Slug:       "about",
		URL:        "/about/",
		Layout:     "page",
		SourcePath: filepath.ToSlash(filepath.Join(cfg.ContentDir, "pages", "about.md")),
		RawBody:    "# Hello",
		HTMLBody:   template.HTML("<h1>Hello</h1>"),
		Summary:    "Summary",
		Date:       &now,
		Taxonomies: map[string][]string{"tags": {"intro"}},
	}
	graph.Add(doc)

	svc := New(cfg, WithGraphLoader(func(context.Context, *config.Config, bool) (*content.SiteGraph, error) {
		return graph, nil
	}))

	list, err := svc.ListDocuments(context.Background(), types.DocumentListOptions{Query: "about"})
	if err != nil || len(list) != 1 {
		t.Fatalf("list documents: %v %#v", err, list)
	}
	detail, err := svc.GetDocument(context.Background(), "doc-1", true)
	if err != nil || detail.ID != "doc-1" {
		t.Fatalf("get document: %v %#v", err, detail)
	}

	preview, err := svc.PreviewDocument(context.Background(), types.DocumentPreviewRequest{
		Raw: "---\ntitle: Preview\nslug: preview\n---\n\n# Hello",
	})
	if err != nil || preview.Title != "Preview" || !strings.Contains(preview.HTML, "<h1") {
		t.Fatalf("preview document: %v %#v", err, preview)
	}

	status, err := svc.GetSystemStatus(context.Background())
	if err != nil {
		t.Fatalf("get system status: %v", err)
	}
	if status.Content.DocumentCount != 1 || len(status.Checks) == 0 {
		t.Fatalf("unexpected system status: %#v", status)
	}
	if svc.Config() != cfg {
		t.Fatal("expected config getter")
	}
	if len(svc.providers()) == 0 {
		t.Fatal("expected status providers")
	}
}

func TestServiceHelpersAndErrorPaths(t *testing.T) {
	cfg := testServiceConfig(t)
	svc := New(cfg, WithGraphLoader(func(context.Context, *config.Config, bool) (*content.SiteGraph, error) {
		return nil, os.ErrNotExist
	}))

	if _, err := svc.load(context.Background(), true); err == nil {
		t.Fatal("expected load error")
	}

	if _, err := svc.GetDocument(context.Background(), "", true); err == nil {
		t.Fatal("expected empty document id error")
	}

	if _, err := svc.SaveDocument(context.Background(), types.DocumentSaveRequest{SourcePath: "pages/test.txt", Raw: "x"}); err == nil {
		t.Fatal("expected non-markdown save rejection")
	}
	if _, err := svc.SaveDocument(context.Background(), types.DocumentSaveRequest{SourcePath: "../escape.md", Raw: "x"}); err == nil {
		t.Fatal("expected path traversal rejection")
	}
	if _, err := svc.SaveDocument(context.Background(), types.DocumentSaveRequest{SourcePath: "pages/test.md"}); err == nil {
		t.Fatal("expected empty raw error")
	}

	if _, err := svc.PreviewDocument(context.Background(), types.DocumentPreviewRequest{}); err == nil {
		t.Fatal("expected empty preview error")
	}
	if _, err := svc.PreviewDocument(context.Background(), types.DocumentPreviewRequest{Raw: "---\ntitle: [\n---\nbody"}); err == nil {
		t.Fatal("expected frontmatter parse error")
	}

	if got := countWords(" one  two\nthree "); got != 3 {
		t.Fatalf("unexpected word count: %d", got)
	}
	if !matchesDocumentQuery(&content.Document{Title: "Hello"}, "hell") {
		t.Fatal("expected document query match")
	}
	if matchesDocumentQuery(&content.Document{Title: "Hello"}, "nope") {
		t.Fatal("expected document query miss")
	}

	svc.RegisterStatusProvider(nil)
}

func TestStatusProvidersBranches(t *testing.T) {
	cfg := testServiceConfig(t)
	graph := content.NewSiteGraph(cfg)
	now := time.Now()
	doc := &content.Document{
		ID:         "doc-1",
		Type:       "page",
		Lang:       "en",
		Title:      "About",
		Slug:       "about",
		URL:        "/about/",
		Layout:     "page",
		SourcePath: filepath.ToSlash(filepath.Join(cfg.ContentDir, "pages", "about.md")),
		RawBody:    "# Hello",
		HTMLBody:   template.HTML("<h1>Hello</h1>"),
		Date:       &now,
		Draft:      true,
	}
	graph.Add(doc)
	graph.Taxonomies.Values = map[string]map[string][]taxonomy.Entry{
		"tags": {"intro": {{DocumentID: doc.ID, URL: doc.URL, Lang: doc.Lang, Type: doc.Type, Title: doc.Title, Slug: doc.Slug}}},
	}

	if err := os.MkdirAll(filepath.Join(cfg.ThemesDir, cfg.Theme), 0o755); err != nil {
		t.Fatalf("mkdir theme: %v", err)
	}
	if err := os.MkdirAll(cfg.PluginsDir, 0o755); err != nil {
		t.Fatalf("mkdir plugins: %v", err)
	}
	cfg.Plugins.Enabled = []string{"missing-plugin"}

	svc := New(cfg, WithGraphLoader(func(context.Context, *config.Config, bool) (*content.SiteGraph, error) {
		return graph, nil
	}))
	status, err := svc.GetSystemStatus(context.Background())
	if err != nil {
		t.Fatalf("status failed: %v", err)
	}
	if status.Content.DraftCount != 1 {
		t.Fatalf("expected draft count, got %#v", status.Content)
	}
	if len(status.Taxonomies) != 1 {
		t.Fatalf("expected taxonomy status, got %#v", status.Taxonomies)
	}
	if len(status.Plugins) != 1 || status.Plugins[0].Enabled != true {
		t.Fatalf("expected missing enabled plugin status, got %#v", status.Plugins)
	}
	if len(status.Checks) == 0 {
		t.Fatal("expected health checks")
	}
}

func testServiceConfig(t *testing.T) *config.Config {
	t.Helper()

	root := t.TempDir()
	cfg := &config.Config{
		ContentDir: filepath.Join(root, "content"),
		PublicDir:  filepath.Join(root, "public"),
		ThemesDir:  filepath.Join(root, "themes"),
		PluginsDir: filepath.Join(root, "plugins"),
		DataDir:    filepath.Join(root, "data"),
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
	if err := os.MkdirAll(filepath.Join(cfg.ContentDir, cfg.Content.PagesDir), 0o755); err != nil {
		t.Fatalf("mkdir pages dir: %v", err)
	}
	return cfg
}
