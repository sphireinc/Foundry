package service

import (
	"context"
	"encoding/json"
	"html/template"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	adminauth "github.com/sphireinc/foundry/internal/admin/auth"
	"github.com/sphireinc/foundry/internal/admin/types"
	"github.com/sphireinc/foundry/internal/admin/users"
	"github.com/sphireinc/foundry/internal/config"
	"github.com/sphireinc/foundry/internal/content"
	"github.com/sphireinc/foundry/internal/media"
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
	_, err = svc.SaveDocument(context.Background(), types.DocumentSaveRequest{
		SourcePath: filepath.Join("pages", "cache-test.md"),
		Raw:        "---\ntitle: Cache Test\nslug: cache-test\n---\n\nUpdated Body",
	})
	if err != nil {
		t.Fatalf("save document second version failed: %v", err)
	}
	entries, err := os.ReadDir(filepath.Join(cfg.ContentDir, "pages"))
	if err != nil {
		t.Fatalf("read page dir: %v", err)
	}
	var foundVersion bool
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "cache-test.version.") {
			foundVersion = true
		}
	}
	if !foundVersion {
		t.Fatal("expected versioned cache-test document after overwrite")
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
	if err := os.WriteFile(filepath.Join(cfg.ContentDir, "pages", "about.md"), []byte("---\ntitle: About\nslug: about\nlayout: page\n---\n\n# Hello"), 0o644); err != nil {
		t.Fatalf("write about page: %v", err)
	}

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
	if !strings.Contains(detail.RawBody, "title: About") || !strings.Contains(detail.RawBody, "# Hello") {
		t.Fatalf("expected full raw document with frontmatter, got %q", detail.RawBody)
	}

	preview, err := svc.PreviewDocument(context.Background(), types.DocumentPreviewRequest{
		Raw: "---\ntitle: Preview\nslug: preview\n---\n\n# Hello\n\n![Clip](media:videos/demo.mp4)",
	})
	if err != nil || preview.Title != "Preview" || !strings.Contains(preview.HTML, "<h1") {
		t.Fatalf("preview document: %v %#v", err, preview)
	}
	if !strings.Contains(preview.HTML, `<video controls preload="metadata" src="/videos/demo.mp4" title="Clip" aria-label="Clip"></video>`) {
		t.Fatalf("expected preview html to rewrite media reference, got %q", preview.HTML)
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

func TestPreviewDocumentIncludesFieldValidationErrors(t *testing.T) {
	cfg := testServiceConfig(t)
	writeServiceThemeContracts(t, cfg, `
field_contracts:
  - key: post-workflow
    title: Post Workflow
    target:
      scope: document
      types: [post]
    fields:
      - name: stage
        type: select
        enum: [draft, review]
`)
	svc := New(cfg)

	preview, err := svc.PreviewDocument(context.Background(), types.DocumentPreviewRequest{
		SourcePath: "posts/invalid.md",
		Raw:        "---\ntitle: Invalid\nslug: invalid\nlayout: post\n---\n\nBody",
		Fields:     map[string]any{"stage": "published"},
	})
	if err != nil {
		t.Fatalf("preview document: %v", err)
	}
	if len(preview.FieldErrors) == 0 {
		t.Fatalf("expected preview field errors, got %#v", preview)
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

func TestDocumentPathResolutionRejectsSymlinkEscape(t *testing.T) {
	cfg := testServiceConfig(t)
	outsidePath := filepath.Join(filepath.Dir(cfg.ContentDir), "outside.md")
	if err := os.WriteFile(outsidePath, []byte("outside"), 0o644); err != nil {
		t.Fatalf("write outside file: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(cfg.ContentDir, "pages"), 0o755); err != nil {
		t.Fatalf("mkdir pages: %v", err)
	}

	link := filepath.Join(cfg.ContentDir, "pages", "linked.md")
	if err := os.Symlink(outsidePath, link); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}

	svc := New(cfg)
	if _, err := svc.PreviewDocument(context.Background(), types.DocumentPreviewRequest{SourcePath: "pages/linked.md"}); err == nil {
		t.Fatal("expected preview symlink escape rejection")
	}
	if _, err := svc.SaveDocument(context.Background(), types.DocumentSaveRequest{
		SourcePath: "pages/linked.md",
		Raw:        "---\ntitle: Linked\nslug: linked\n---\n\nBody",
	}); err == nil {
		t.Fatal("expected save symlink escape rejection")
	}
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

func TestStatusIncludesAuthSecurityCheckFailures(t *testing.T) {
	cfg := testServiceConfig(t)
	cfg.Admin.Enabled = true
	cfg.Admin.LocalOnly = false
	cfg.Admin.SessionSecret = ""
	if err := os.WriteFile(cfg.Admin.UsersFile, []byte("users:\n  - username: admin\n    name: Admin User\n    email: admin@example.com\n    role: admin\n    password_hash: argon2id$dummy\n    totp_enabled: true\n    totp_secret: PLAINTEXTSECRET\n"), 0o644); err != nil {
		t.Fatalf("write users file: %v", err)
	}
	svc := New(cfg, WithGraphLoader(func(context.Context, *config.Config, bool) (*content.SiteGraph, error) {
		return content.NewSiteGraph(cfg), nil
	}))

	status, err := svc.GetSystemStatus(context.Background())
	if err != nil {
		t.Fatalf("status failed: %v", err)
	}
	found := false
	for _, check := range status.Checks {
		if check.Name == "auth-security" {
			found = true
			if check.Status != "fail" {
				t.Fatalf("expected auth-security to fail, got %#v", check)
			}
			if !strings.Contains(check.Message, "session_secret") || !strings.Contains(check.Message, "plaintext TOTP secrets") {
				t.Fatalf("unexpected auth-security message: %#v", check)
			}
		}
	}
	if !found {
		t.Fatal("expected auth-security check to be present")
	}
}

func TestGetRuntimeStatusIncludesInventoryStorageIntegrityAndActivity(t *testing.T) {
	cfg := testServiceConfig(t)
	writeServiceTheme(t, cfg, cfg.Theme)

	graph := content.NewSiteGraph(cfg)
	now := time.Now().UTC()
	page := &content.Document{
		ID:         "page-1",
		Type:       "page",
		Lang:       "en",
		Status:     "published",
		Title:      "Home",
		Slug:       "home",
		URL:        "/",
		Layout:     "page",
		SourcePath: filepath.ToSlash(filepath.Join(cfg.ContentDir, "pages", "index.md")),
		RawBody:    "# Home\n\n![Hero](media:images/hero.png)",
		HTMLBody:   template.HTML("<h1>Home</h1>"),
		Date:       &now,
	}
	post := &content.Document{
		ID:         "post-1",
		Type:       "post",
		Lang:       "en",
		Status:     "draft",
		Title:      "Draft Post",
		Slug:       "draft-post",
		URL:        "/posts/draft-post/",
		Layout:     "post",
		SourcePath: filepath.ToSlash(filepath.Join(cfg.ContentDir, "posts", "draft-post.md")),
		RawBody:    "# Draft\n\n[Missing](/missing/)",
		HTMLBody:   template.HTML("<h1>Draft</h1>"),
		Date:       &now,
		Draft:      true,
	}
	graph.Add(page)
	graph.Add(post)
	graph.Taxonomies.Values = map[string]map[string][]taxonomy.Entry{
		"tags": {
			"intro": {{DocumentID: page.ID, URL: page.URL, Lang: page.Lang, Type: page.Type, Title: page.Title, Slug: page.Slug}},
		},
	}

	if err := os.WriteFile(filepath.Join(cfg.ContentDir, "pages", "index.md"), []byte("---\ntitle: Home\nslug: home\nlayout: page\n---\n\n# Home"), 0o644); err != nil {
		t.Fatalf("write page: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cfg.ContentDir, "posts", "draft-post.md"), []byte("---\ntitle: Draft Post\nslug: draft-post\nlayout: post\ndraft: true\n---\n\n# Draft"), 0o644); err != nil {
		t.Fatalf("write post: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(cfg.ContentDir, cfg.Content.ImagesDir), 0o755); err != nil {
		t.Fatalf("mkdir images: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cfg.ContentDir, cfg.Content.ImagesDir, "hero.png"), testPNGBytes(), 0o644); err != nil {
		t.Fatalf("write hero image: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cfg.ContentDir, cfg.Content.ImagesDir, "old.version.20260321T120000Z.png"), testPNGBytes(), 0o644); err != nil {
		t.Fatalf("write versioned image: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cfg.ContentDir, cfg.Content.ImagesDir, "trashed.trash.20260321T120500Z.png"), testPNGBytes(), 0o644); err != nil {
		t.Fatalf("write trashed image: %v", err)
	}
	if err := os.MkdirAll(cfg.PublicDir, 0o755); err != nil {
		t.Fatalf("mkdir public: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cfg.PublicDir, "index.html"), []byte("<html></html>"), 0o644); err != nil {
		t.Fatalf("write public file: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(cfg.Admin.SessionStoreFile), 0o755); err != nil {
		t.Fatalf("mkdir session dir: %v", err)
	}
	if err := os.WriteFile(cfg.Admin.SessionStoreFile, []byte("sessions:\n  - token: active\n    expires_at: "+now.Add(10*time.Minute).Format(time.RFC3339)+"\n"), 0o600); err != nil {
		t.Fatalf("write session file: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(cfg.Admin.LockFile), 0o755); err != nil {
		t.Fatalf("mkdir lock dir: %v", err)
	}
	if err := os.WriteFile(cfg.Admin.LockFile, []byte("locks:\n  - source_path: "+filepath.ToSlash(filepath.Join(cfg.ContentDir, "pages", "index.md"))+"\n    username: admin\n    token: abc\n    last_beat_at: "+now.Format(time.RFC3339)+"\n    expires_at: "+now.Add(time.Minute).Format(time.RFC3339)+"\n"), 0o600); err != nil {
		t.Fatalf("write lock file: %v", err)
	}
	auditEntry, err := json.Marshal(types.AuditEntry{
		Timestamp: now,
		Action:    "login",
		Outcome:   "fail",
		Actor:     "admin",
	})
	if err != nil {
		t.Fatalf("marshal audit entry: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(cfg.DataDir, "admin"), 0o755); err != nil {
		t.Fatalf("mkdir audit dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cfg.DataDir, "admin", "audit.jsonl"), append(auditEntry, '\n'), 0o644); err != nil {
		t.Fatalf("write audit file: %v", err)
	}
	buildReport := `{"generated_at":"` + now.Format(time.RFC3339) + `","environment":"preview","target":"production","preview":true,"document_count":2,"route_count":2,"stats":{"prepare":1000000,"assets":2000000,"documents":3000000,"taxonomies":4000000,"search":5000000}}`
	if err := os.WriteFile(filepath.Join(cfg.DataDir, "admin", "build-report.json"), []byte(buildReport), 0o644); err != nil {
		t.Fatalf("write build report: %v", err)
	}

	svc := New(cfg, WithGraphLoader(func(context.Context, *config.Config, bool) (*content.SiteGraph, error) {
		return graph, nil
	}))

	status, err := svc.GetRuntimeStatus(context.Background())
	if err != nil {
		t.Fatalf("get runtime status: %v", err)
	}
	if status.Content.DocumentCount != 2 || status.Content.RouteCount != 2 {
		t.Fatalf("unexpected content totals: %#v", status.Content)
	}
	if status.Content.ByStatus["published"] != 1 || status.Content.ByStatus["draft"] != 1 {
		t.Fatalf("unexpected content status counts: %#v", status.Content.ByStatus)
	}
	if status.Content.MediaCounts["images"] != 1 {
		t.Fatalf("expected one current image, got %#v", status.Content.MediaCounts)
	}
	if status.Activity.ActiveSessions != 1 {
		t.Fatalf("expected one active session, got %#v", status.Activity)
	}
	if status.Storage.DerivedVersionCount != 1 || status.Storage.DerivedTrashCount != 1 {
		t.Fatalf("unexpected derived file counts: %#v", status.Storage)
	}
	if status.Storage.MediaBytes["images"] <= 0 || status.Storage.ContentBytes <= 0 {
		t.Fatalf("expected storage sizes, got %#v", status.Storage)
	}
	if status.Integrity.BrokenInternalLinks != 1 {
		t.Fatalf("expected one broken internal link, got %#v", status.Integrity)
	}
	if status.Activity.ActiveSessions != 1 || status.Activity.ActiveDocumentLocks != 1 {
		t.Fatalf("unexpected activity counts: %#v", status.Activity)
	}
	if status.Activity.RecentFailedLogins != 1 || status.Activity.RecentAuditEvents != 1 {
		t.Fatalf("unexpected audit activity counts: %#v", status.Activity)
	}
	if status.LastBuild == nil || status.LastBuild.Environment != "preview" || status.LastBuild.DocumentsMS != 3 {
		t.Fatalf("unexpected build report: %#v", status.LastBuild)
	}
	if len(status.Storage.LargestFiles) == 0 {
		t.Fatalf("expected largest files, got %#v", status.Storage)
	}
}

func TestRuntimeStatusCountsHashedSessions(t *testing.T) {
	cfg := testServiceConfig(t)
	now := time.Now().UTC()
	if err := os.MkdirAll(filepath.Dir(cfg.Admin.SessionStoreFile), 0o755); err != nil {
		t.Fatalf("mkdir session dir: %v", err)
	}
	if err := os.WriteFile(cfg.Admin.SessionStoreFile, []byte("sessions:\n  - token_hash: abc123\n    expires_at: "+now.Add(10*time.Minute).Format(time.RFC3339)+"\n"), 0o600); err != nil {
		t.Fatalf("write hashed session file: %v", err)
	}
	svc := New(cfg, WithGraphLoader(func(context.Context, *config.Config, bool) (*content.SiteGraph, error) {
		return content.NewSiteGraph(cfg), nil
	}))

	status, err := svc.GetRuntimeStatus(context.Background())
	if err != nil {
		t.Fatalf("get runtime status: %v", err)
	}
	if status.Activity.ActiveSessions != 1 {
		t.Fatalf("expected hashed session to be counted, got %#v", status.Activity)
	}
}

func TestRuntimeStatusComputesSessionRiskSignals(t *testing.T) {
	cfg := testServiceConfig(t)
	now := time.Now().UTC()
	if err := os.MkdirAll(filepath.Dir(cfg.Admin.SessionStoreFile), 0o755); err != nil {
		t.Fatalf("mkdir session dir: %v", err)
	}
	body := "sessions:\n" +
		"  - token_hash: a1\n    username: admin\n    remote_addr: 127.0.0.1\n    issued_at: " + now.Add(-13*time.Hour).Format(time.RFC3339) + "\n    last_seen: " + now.Add(-45*time.Minute).Format(time.RFC3339) + "\n    expires_at: " + now.Add(10*time.Minute).Format(time.RFC3339) + "\n" +
		"  - token_hash: a2\n    username: admin\n    remote_addr: 10.0.0.2\n    issued_at: " + now.Add(-2*time.Hour).Format(time.RFC3339) + "\n    last_seen: " + now.Add(-5*time.Minute).Format(time.RFC3339) + "\n    expires_at: " + now.Add(10*time.Minute).Format(time.RFC3339) + "\n"
	if err := os.WriteFile(cfg.Admin.SessionStoreFile, []byte(body), 0o600); err != nil {
		t.Fatalf("write session file: %v", err)
	}
	svc := New(cfg, WithGraphLoader(func(context.Context, *config.Config, bool) (*content.SiteGraph, error) {
		return content.NewSiteGraph(cfg), nil
	}))

	status, err := svc.GetRuntimeStatus(context.Background())
	if err != nil {
		t.Fatalf("get runtime status: %v", err)
	}
	if status.Activity.ConcurrentUsers != 1 {
		t.Fatalf("expected one concurrent user, got %#v", status.Activity)
	}
	if status.Activity.AddressSpreadUsers != 1 {
		t.Fatalf("expected one address-spread user, got %#v", status.Activity)
	}
	if status.Activity.LongLivedSessions != 1 {
		t.Fatalf("expected one long-lived session, got %#v", status.Activity)
	}
	if status.Activity.IdleSessions != 1 {
		t.Fatalf("expected one idle session, got %#v", status.Activity)
	}
}

func TestDocumentHistoryRestorePurgeAndDiff(t *testing.T) {
	cfg := testServiceConfig(t)
	svc := New(cfg)

	created, err := svc.CreateDocument(context.Background(), types.DocumentCreateRequest{
		Kind: "page",
		Slug: "history-page",
		Lang: "en",
	})
	if err != nil {
		t.Fatalf("create document: %v", err)
	}

	if _, err := svc.SaveDocument(context.Background(), types.DocumentSaveRequest{
		SourcePath:     created.SourcePath,
		Raw:            "---\ntitle: History Page\nslug: history-page\nlayout: page\ndraft: false\n---\n\n# Updated\n",
		VersionComment: "Refine the page copy",
		Actor:          "Admin User",
	}); err != nil {
		t.Fatalf("save document update: %v", err)
	}

	history, err := svc.GetDocumentHistory(context.Background(), created.SourcePath)
	if err != nil {
		t.Fatalf("get document history: %v", err)
	}
	if len(history.Entries) != 2 {
		t.Fatalf("expected current and one version, got %#v", history.Entries)
	}
	if history.Entries[0].State != types.LifecycleStateCurrent || history.Entries[1].State != types.LifecycleStateVersion {
		t.Fatalf("unexpected history states: %#v", history.Entries)
	}
	if history.Entries[1].VersionComment != "Refine the page copy" {
		t.Fatalf("expected version comment, got %#v", history.Entries[1])
	}
	if history.Entries[1].Actor != "Admin User" {
		t.Fatalf("expected version actor, got %#v", history.Entries[1])
	}

	diff, err := svc.DiffDocument(context.Background(), types.DocumentDiffRequest{
		LeftPath:  history.Entries[1].Path,
		RightPath: history.Entries[0].Path,
	})
	if err != nil {
		t.Fatalf("diff document: %v", err)
	}
	if !strings.Contains(diff.Diff, "-# History Page") || !strings.Contains(diff.Diff, "+# Updated") {
		t.Fatalf("unexpected diff output: %s", diff.Diff)
	}

	deleted, err := svc.DeleteDocument(context.Background(), types.DocumentDeleteRequest{SourcePath: created.SourcePath})
	if err != nil {
		t.Fatalf("delete document: %v", err)
	}
	trash, err := svc.ListDocumentTrash(context.Background())
	if err != nil {
		t.Fatalf("list document trash: %v", err)
	}
	if len(trash) != 1 || trash[0].State != types.LifecycleStateTrash {
		t.Fatalf("unexpected document trash: %#v", trash)
	}

	restored, err := svc.RestoreDocument(context.Background(), types.DocumentLifecycleRequest{Path: deleted.TrashPath})
	if err != nil {
		t.Fatalf("restore document: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(cfg.ContentDir, "pages", "history-page.md"))
	if err != nil {
		t.Fatalf("read restored document: %v", err)
	}
	if !strings.Contains(string(raw), "# Updated") {
		t.Fatalf("expected restored document contents, got %q", string(raw))
	}

	history, err = svc.GetDocumentHistory(context.Background(), restored.RestoredPath)
	if err != nil {
		t.Fatalf("get restored document history: %v", err)
	}
	if len(history.Entries) < 2 {
		t.Fatalf("expected history after restore, got %#v", history.Entries)
	}

	versionPath := ""
	for _, entry := range history.Entries {
		if entry.State == types.LifecycleStateVersion {
			versionPath = entry.Path
			break
		}
	}
	if versionPath == "" {
		t.Fatalf("expected version entry after restore, got %#v", history.Entries)
	}
	if _, err := svc.PurgeDocument(context.Background(), types.DocumentLifecycleRequest{Path: versionPath}); err != nil {
		t.Fatalf("purge document version: %v", err)
	}
	history, err = svc.GetDocumentHistory(context.Background(), restored.RestoredPath)
	if err != nil {
		t.Fatalf("get document history after purge: %v", err)
	}
	for _, entry := range history.Entries {
		if entry.Path == versionPath {
			t.Fatalf("expected version to be purged, history=%#v", history.Entries)
		}
	}
}

func TestMediaHistoryRestoreAndPurge(t *testing.T) {
	cfg := testServiceConfig(t)
	svc := New(cfg)

	upload, err := svc.SaveMedia(context.Background(), "images", "posts/history", "diagram.png", "image/png", testPNGBytes())
	if err != nil {
		t.Fatalf("save media: %v", err)
	}
	if _, err := svc.SaveMediaMetadata(context.Background(), upload.Reference, types.MediaMetadata{Title: "Diagram v1"}, "initial metadata", "Admin User"); err != nil {
		t.Fatalf("save media metadata: %v", err)
	}
	if _, err := svc.ReplaceMedia(context.Background(), upload.Reference, "image/png", testPNGBytes()); err != nil {
		t.Fatalf("replace media: %v", err)
	}
	if _, err := svc.SaveMediaMetadata(context.Background(), upload.Reference, types.MediaMetadata{Title: "Diagram current"}, "refresh metadata", "Admin User"); err != nil {
		t.Fatalf("save current media metadata: %v", err)
	}
	items, err := svc.ListMedia(context.Background())
	if err != nil {
		t.Fatalf("list media: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected one current media item, got %#v", items)
	}

	history, err := svc.GetMediaHistory(context.Background(), filepath.Join("content", "images", items[0].Path))
	if err != nil {
		t.Fatalf("get media history: %v", err)
	}
	if len(history.Entries) < 2 {
		t.Fatalf("expected media history entries, got %#v", history.Entries)
	}
	var metadataHistoryFound bool
	for _, entry := range history.Entries {
		if entry.MetadataOnly {
			metadataHistoryFound = true
		}
	}
	if !metadataHistoryFound {
		t.Fatalf("expected metadata history entry, got %#v", history.Entries)
	}

	if err := svc.DeleteMedia(context.Background(), upload.Reference); err != nil {
		t.Fatalf("delete media: %v", err)
	}
	trash, err := svc.ListMediaTrash(context.Background())
	if err != nil {
		t.Fatalf("list media trash: %v", err)
	}
	if len(trash) != 1 || trash[0].State != types.LifecycleStateTrash {
		t.Fatalf("unexpected media trash entries: %#v", trash)
	}

	restored, err := svc.RestoreMedia(context.Background(), types.MediaLifecycleRequest{Path: trash[0].Path})
	if err != nil {
		t.Fatalf("restore media: %v", err)
	}
	restoredPath := filepath.Join(filepath.Dir(cfg.ContentDir), restored.RestoredPath)
	if _, err := os.Stat(restoredPath); err != nil {
		t.Fatalf("expected restored media file to exist: %v", err)
	}
	sidecarBody, err := os.ReadFile(restoredPath + ".meta.yaml")
	if err != nil {
		t.Fatalf("expected restored media metadata sidecar: %v", err)
	}
	if !strings.Contains(string(sidecarBody), "Diagram current") {
		t.Fatalf("expected restored metadata sidecar contents, got %q", string(sidecarBody))
	}

	history, err = svc.GetMediaHistory(context.Background(), filepath.Join("content", "images", items[0].Path))
	if err != nil {
		t.Fatalf("get media history after restore: %v", err)
	}
	versionPath := ""
	for _, entry := range history.Entries {
		if entry.State == types.LifecycleStateVersion {
			versionPath = entry.Path
			break
		}
	}
	if versionPath == "" {
		t.Fatalf("expected version entry after restore, got %#v", history.Entries)
	}
	if _, err := svc.PurgeMedia(context.Background(), types.MediaLifecycleRequest{Path: versionPath}); err != nil {
		t.Fatalf("purge media: %v", err)
	}
	history, err = svc.GetMediaHistory(context.Background(), upload.Reference)
	if err != nil {
		t.Fatalf("get media history after purge: %v", err)
	}
	for _, entry := range history.Entries {
		if entry.Path == versionPath {
			t.Fatalf("expected media version to be purged, history=%#v", history.Entries)
		}
	}
}

func TestListMediaTrashForRootUploadWithoutMetadata(t *testing.T) {
	cfg := testServiceConfig(t)
	svc := New(cfg)

	upload, err := svc.SaveMedia(context.Background(), "images", "", "1768917471861.png", "image/png", testPNGBytes())
	if err != nil {
		t.Fatalf("save media: %v", err)
	}

	if err := svc.DeleteMedia(context.Background(), upload.Reference); err != nil {
		t.Fatalf("delete media: %v", err)
	}

	trash, err := svc.ListMediaTrash(context.Background())
	if err != nil {
		t.Fatalf("list media trash: %v", err)
	}
	if len(trash) != 1 {
		t.Fatalf("expected one media trash entry, got %#v", trash)
	}
	if trash[0].State != types.LifecycleStateTrash {
		t.Fatalf("expected trash state, got %#v", trash[0])
	}
	if trash[0].Collection != "images" {
		t.Fatalf("expected images collection, got %#v", trash[0])
	}
	if trash[0].CurrentReference != upload.Reference {
		t.Fatalf("expected current reference %q, got %#v", upload.Reference, trash[0])
	}
	if !strings.Contains(trash[0].Path, ".trash.") || !strings.HasSuffix(trash[0].Path, ".png") {
		t.Fatalf("expected trash path for numeric root upload, got %#v", trash[0])
	}
}

func TestGetMediaDetailIncludesUsage(t *testing.T) {
	cfg := testServiceConfig(t)
	uploadPath := filepath.Join(cfg.ContentDir, cfg.Content.ImagesDir, "posts", "about", "diagram.png")
	if err := os.MkdirAll(filepath.Dir(uploadPath), 0o755); err != nil {
		t.Fatalf("mkdir media dir: %v", err)
	}
	if err := os.WriteFile(uploadPath, testPNGBytes(), 0o644); err != nil {
		t.Fatalf("write media: %v", err)
	}
	docPath := filepath.Join(cfg.ContentDir, cfg.Content.PagesDir, "about.md")
	if err := os.WriteFile(docPath, []byte("---\ntitle: About\nslug: about\nlayout: page\n---\n\n![Diagram](media:images/posts/about/diagram.png)\n"), 0o644); err != nil {
		t.Fatalf("write document: %v", err)
	}

	graph := content.NewSiteGraph(cfg)
	graph.Add(&content.Document{
		ID:         "doc-1",
		Type:       "page",
		Lang:       "en",
		Title:      "About",
		Slug:       "about",
		URL:        "/about/",
		Layout:     "page",
		SourcePath: filepath.ToSlash(docPath),
	})
	svc := New(cfg, WithGraphLoader(func(context.Context, *config.Config, bool) (*content.SiteGraph, error) {
		return graph, nil
	}))

	detail, err := svc.GetMediaDetail(context.Background(), "media:images/posts/about/diagram.png")
	if err != nil {
		t.Fatalf("get media detail: %v", err)
	}
	if len(detail.UsedBy) != 1 || detail.UsedBy[0].SourcePath != "content/pages/about.md" {
		t.Fatalf("expected usage entry, got %#v", detail.UsedBy)
	}
}

func TestMediaUploadAndListing(t *testing.T) {
	cfg := testServiceConfig(t)
	svc := New(cfg)

	first, err := svc.SaveMedia(context.Background(), "", "posts/hello", "Hero Banner.PNG", "image/png", testPNGBytes())
	if err != nil {
		t.Fatalf("save media: %v", err)
	}
	if first.Collection != "images" || !strings.HasPrefix(first.Path, "posts/hello/hero-banner-") || !strings.HasSuffix(first.Path, ".png") || first.Reference != media.MustReference("images", first.Path) {
		t.Fatalf("unexpected first upload response: %#v", first)
	}
	if first.Metadata.OriginalFilename != "Hero Banner.PNG" || first.Metadata.StoredFilename != first.Name || first.Metadata.ContentHash == "" || first.Metadata.MIMEType != "image/png" {
		t.Fatalf("expected upload metadata to be populated, got %#v", first.Metadata)
	}

	second, err := svc.SaveMedia(context.Background(), "videos", "posts/hello", "clip.mp4", "video/mp4", testMP4Bytes())
	if err != nil {
		t.Fatalf("save second media: %v", err)
	}
	if second.Collection != "videos" || second.Kind != "video" {
		t.Fatalf("unexpected second upload response: %#v", second)
	}

	dupe, err := svc.SaveMedia(context.Background(), "images", "posts/hello", "hero banner.png", "image/png", testPNGBytes())
	if err != nil {
		t.Fatalf("save duplicate media: %v", err)
	}
	if dupe.Path == first.Path || !dupe.Created {
		t.Fatalf("expected unique path for duplicate upload, got %#v", dupe)
	}
	entries, err := os.ReadDir(filepath.Join(cfg.ContentDir, cfg.Content.ImagesDir, "posts", "hello"))
	if err != nil {
		t.Fatalf("read media dir: %v", err)
	}
	var pngCount int
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".png") {
			pngCount++
		}
	}
	if pngCount != 2 {
		t.Fatalf("expected two distinct uploaded png files, got entries %#v", entries)
	}

	items, err := svc.ListMedia(context.Background())
	if err != nil {
		t.Fatalf("list media: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("expected 3 current media items, got %#v", items)
	}
}

func TestMediaUploadNormalizesCollectionRootDirectoryInput(t *testing.T) {
	cfg := testServiceConfig(t)
	svc := New(cfg)

	upload, err := svc.SaveMedia(context.Background(), "images", "content/images", "Hero Banner.PNG", "image/png", testPNGBytes())
	if err != nil {
		t.Fatalf("save media with collection root dir: %v", err)
	}
	if strings.Contains(upload.Path, "content/images") || strings.Contains(upload.Path, "images/") {
		t.Fatalf("expected collection-root dir input to normalize away, got path %q", upload.Path)
	}
	if !strings.HasPrefix(upload.Path, "hero-banner-") || !strings.HasSuffix(upload.Path, ".png") {
		t.Fatalf("unexpected normalized upload path: %q", upload.Path)
	}
}

func TestMediaUploadWritesMetadataWithRelativeContentDir(t *testing.T) {
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
		ContentDir: "content",
		PublicDir:  "public",
		ThemesDir:  "themes",
		PluginsDir: "plugins",
		DataDir:    "data",
		Content: config.ContentConfig{
			PagesDir:          "pages",
			PostsDir:          "posts",
			ImagesDir:         "images",
			DefaultLayoutPage: "page",
			DefaultLayoutPost: "post",
		},
	}
	cfg.ApplyDefaults()
	if err := os.MkdirAll(filepath.Join(cfg.ContentDir, "config"), 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	hash, err := users.HashPassword("secret-password")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	cfg.Admin.UsersFile = filepath.Join(cfg.ContentDir, "config", "admin-users.yaml")
	if err := os.WriteFile(cfg.Admin.UsersFile, []byte("users:\n  - username: admin\n    name: Admin User\n    email: admin@example.com\n    role: admin\n    password_hash: "+hash+"\n"), 0o644); err != nil {
		t.Fatalf("write users file: %v", err)
	}

	svc := New(cfg)
	upload, err := svc.SaveMedia(context.Background(), "images", "", "Hero Banner.PNG", "image/png", testPNGBytes())
	if err != nil {
		t.Fatalf("save media with relative content dir: %v", err)
	}
	sidecar := filepath.Join(cfg.ContentDir, cfg.Content.ImagesDir, upload.Name+".meta.yaml")
	if _, err := os.Stat(sidecar); err != nil {
		t.Fatalf("expected metadata sidecar to exist at %s: %v", sidecar, err)
	}
}

func TestSaveMediaErrors(t *testing.T) {
	cfg := testServiceConfig(t)
	svc := New(cfg)

	if _, err := svc.SaveMedia(context.Background(), "bad", "", "clip.mp4", "video/mp4", []byte("x")); err == nil {
		t.Fatal("expected invalid collection error")
	}
	if _, err := svc.SaveMedia(context.Background(), "videos", "../escape", "clip.mp4", "video/mp4", []byte("x")); err == nil {
		t.Fatal("expected invalid media dir error")
	}
	if _, err := svc.SaveMedia(context.Background(), "videos", "", "clip.mp4", "video/mp4", nil); err == nil {
		t.Fatal("expected empty media body error")
	}
}

func TestManagementServices(t *testing.T) {
	cfg := testServiceConfig(t)
	prevWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(filepath.Dir(cfg.ContentDir)); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(prevWD)
	})
	if err := os.WriteFile(filepath.Join(cfg.ContentDir, "config", "site.yaml"), []byte("theme: default\ncontent_dir: content\npublic_dir: public\nthemes_dir: themes\ndata_dir: data\nplugins_dir: plugins\nserver:\n  addr: :8080\nfeed:\n  rss_path: /rss.xml\n  sitemap_path: /sitemap.xml\n"), 0o644); err != nil {
		t.Fatalf("write site config: %v", err)
	}
	writeServiceTheme(t, cfg, "default")
	writeServiceTheme(t, cfg, "alt")
	writePluginMetadata(t, cfg, "alpha")
	writePluginMetadata(t, cfg, "beta")
	cfg.Plugins.Enabled = []string{"alpha"}

	svc := New(cfg)

	user, err := svc.SaveUser(context.Background(), types.UserSaveRequest{
		Username: "editor",
		Name:     "Editor User",
		Email:    "editor@example.com",
		Role:     "editor",
		Password: "Secret-password1!",
	})
	if err != nil || user.Username != "editor" {
		t.Fatalf("save user: %v %#v", err, user)
	}
	updatedUser, err := svc.SaveUser(context.Background(), types.UserSaveRequest{
		Username: "editor",
		Name:     "Updated Editor",
		Email:    "updated@example.com",
		Role:     "admin",
	})
	if err != nil || updatedUser.Name != "Updated Editor" {
		t.Fatalf("update user: %v %#v", err, updatedUser)
	}
	users, err := svc.ListUsers(context.Background())
	if err != nil || len(users) < 2 {
		t.Fatalf("list users: %v %#v", err, users)
	}
	if err := svc.DeleteUser(context.Background(), "editor"); err != nil {
		t.Fatalf("delete user: %v", err)
	}

	configDoc, err := svc.LoadConfigDocument(context.Background())
	if err != nil || !strings.Contains(configDoc.Raw, "theme: default") {
		t.Fatalf("load config: %v %#v", err, configDoc)
	}
	settingsForm, err := svc.LoadSettingsForm(context.Background())
	if err != nil || settingsForm.Value.Theme != "default" {
		t.Fatalf("load settings form: %v %#v", err, settingsForm)
	}
	settingsForm.Value.Title = "Updated Foundry"
	savedSettings, err := svc.SaveSettingsForm(context.Background(), settingsForm.Value)
	if err != nil || savedSettings.Value.Title != "Updated Foundry" {
		t.Fatalf("save settings form: %v %#v", err, savedSettings)
	}
	savedConfig, err := svc.SaveConfigDocument(context.Background(), strings.Replace(configDoc.Raw, "theme: default", "theme: alt", 1))
	if err != nil || !strings.Contains(savedConfig.Raw, "theme: alt") {
		t.Fatalf("save config: %v %#v", err, savedConfig)
	}
	customCSS, err := svc.LoadCustomCSSDocument(context.Background())
	if err != nil {
		t.Fatalf("load custom css: %v", err)
	}
	if customCSS.Path != filepath.Join("content", "assets", "css", "custom.css") {
		t.Fatalf("unexpected custom css path: %#v", customCSS)
	}
	savedCustomCSS, err := svc.SaveCustomCSSDocument(context.Background(), "body { color: #123456; }")
	if err != nil || !strings.Contains(savedCustomCSS.Raw, "color: #123456") {
		t.Fatalf("save custom css: %v %#v", err, savedCustomCSS)
	}

	themes, err := svc.ListThemes(context.Background())
	if err != nil || len(themes) < 2 {
		t.Fatalf("list themes: %v %#v", err, themes)
	}
	if err := svc.SwitchTheme(context.Background(), "alt"); err != nil {
		t.Fatalf("switch theme: %v", err)
	}

	pluginsList, err := svc.ListPlugins(context.Background())
	if err != nil || len(pluginsList) == 0 {
		t.Fatalf("list plugins: %v %#v", err, pluginsList)
	}
	if err := svc.EnablePlugin(context.Background(), "beta", true, true); err != nil {
		t.Fatalf("enable plugin: %v", err)
	}
	if err := svc.DisablePlugin(context.Background(), "alpha"); err != nil {
		t.Fatalf("disable plugin: %v", err)
	}

	upload, err := svc.SaveMedia(context.Background(), "documents", "docs", "guide.pdf", "application/pdf", testPDFBytes())
	if err != nil {
		t.Fatalf("save media for delete: %v", err)
	}
	if err := svc.DeleteMedia(context.Background(), upload.Reference); err != nil {
		t.Fatalf("delete media: %v", err)
	}
}

func TestSaveUserRevokesExistingSessionsOnSecurityChange(t *testing.T) {
	cfg := testServiceConfig(t)
	cfg.Admin.Enabled = true
	cfg.Admin.AccessToken = ""
	svc := New(cfg)

	auth := adminauth.New(cfg)
	loginReq := httptest.NewRequest(http.MethodPost, "/__admin/api/login", nil)
	loginReq.RemoteAddr = "127.0.0.1:12345"
	loginRR := httptest.NewRecorder()
	if _, err := auth.Login(loginRR, loginReq, "admin", "secret-password", ""); err != nil {
		t.Fatalf("login failed: %v", err)
	}
	cookies := loginRR.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected session cookie")
	}

	if _, err := svc.SaveUser(context.Background(), types.UserSaveRequest{
		Username: "admin",
		Name:     "Admin User",
		Email:    "admin@example.com",
		Role:     "admin",
		Password: "New-secret-password1!",
	}); err != nil {
		t.Fatalf("save user with new password: %v", err)
	}

	authAfter := adminauth.New(cfg)
	req := httptest.NewRequest(http.MethodGet, "/__admin/api/status", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	req.AddCookie(cookies[0])
	if err := authAfter.Authorize(req); err == nil {
		t.Fatal("expected password change to revoke existing session")
	}
}

func TestDocumentLifecycleServices(t *testing.T) {
	cfg := testServiceConfig(t)
	svc := New(cfg)

	created, err := svc.CreateDocument(context.Background(), types.DocumentCreateRequest{
		Kind:      "post",
		Slug:      "launch-notes",
		Lang:      "fr",
		Archetype: "post",
	})
	if err != nil {
		t.Fatalf("create document: %v", err)
	}
	if created.SourcePath != "content/posts/fr/launch-notes.md" {
		t.Fatalf("unexpected created source path: %#v", created)
	}
	createdFullPath := filepath.Join(filepath.Dir(cfg.ContentDir), filepath.FromSlash(created.SourcePath))

	status, err := svc.UpdateDocumentStatus(context.Background(), types.DocumentStatusRequest{
		SourcePath: created.SourcePath,
		Status:     "archived",
	})
	if err != nil {
		t.Fatalf("archive document: %v", err)
	}
	if !status.Draft || !status.Archived {
		t.Fatalf("expected archived draft status, got %#v", status)
	}

	body, err := os.ReadFile(createdFullPath)
	if err != nil {
		t.Fatalf("read archived document: %v", err)
	}
	if !strings.Contains(string(body), "archived: true") {
		t.Fatalf("expected archived frontmatter, got %q", string(body))
	}

	restore, err := svc.UpdateDocumentStatus(context.Background(), types.DocumentStatusRequest{
		SourcePath: created.SourcePath,
		Status:     "published",
	})
	if err != nil {
		t.Fatalf("publish document: %v", err)
	}
	if restore.Draft || restore.Archived {
		t.Fatalf("expected published document, got %#v", restore)
	}

	deleted, err := svc.DeleteDocument(context.Background(), types.DocumentDeleteRequest{SourcePath: created.SourcePath})
	if err != nil {
		t.Fatalf("soft delete document: %v", err)
	}
	if deleted.Operation != "soft_delete" || !strings.Contains(deleted.TrashPath, ".trash.") {
		t.Fatalf("unexpected delete response: %#v", deleted)
	}
	if _, err := os.Stat(createdFullPath); !os.IsNotExist(err) {
		t.Fatalf("expected source path to move to trash, got err=%v", err)
	}
	deletedFullPath := filepath.Join(filepath.Dir(cfg.ContentDir), filepath.FromSlash(deleted.TrashPath))
	if _, err := os.Stat(deletedFullPath); err != nil {
		t.Fatalf("expected trash path to exist: %v", err)
	}
}

func TestDocumentWorkflowAndSchemaFields(t *testing.T) {
	cfg := testServiceConfig(t)
	writeServiceThemeContracts(t, cfg, `
field_contracts:
  - key: post-launch
    title: Post Launch
    target:
      scope: document
      types: [post]
    fields:
      - name: hero
        type: text
        required: true
        default: launch
      - name: stage
        type: select
        enum: [draft, review]
        default: draft
`)
	svc := New(cfg)

	saved, err := svc.SaveDocument(context.Background(), types.DocumentSaveRequest{
		SourcePath: "posts/editorial-workflow.md",
		Raw:        "---\ntitle: Editorial Workflow\nslug: editorial-workflow\nlayout: post\nworkflow: in_review\neditorial_note: Needs fact check\ndraft: true\n---\n\nBody",
		Fields:     map[string]any{"stage": "review"},
		Username:   "editor",
	})
	if err != nil {
		t.Fatalf("save document with workflow: %v", err)
	}
	if !strings.Contains(saved.Raw, "author: editor") || !strings.Contains(saved.Raw, "last_editor: editor") {
		t.Fatalf("expected author attribution in raw document, got %q", saved.Raw)
	}
	if !strings.Contains(saved.Raw, "fields:") || !strings.Contains(saved.Raw, "hero: launch") {
		t.Fatalf("expected schema defaults in raw document, got %q", saved.Raw)
	}

	status, err := svc.UpdateDocumentStatus(context.Background(), types.DocumentStatusRequest{
		SourcePath:           saved.SourcePath,
		Status:               "scheduled",
		ScheduledPublishAt:   "2026-03-25T10:00:00Z",
		ScheduledUnpublishAt: "2026-03-30T10:00:00Z",
		EditorialNote:        "Publish with launch post",
	})
	if err != nil {
		t.Fatalf("schedule document: %v", err)
	}
	if status.Status != "scheduled" || status.ScheduledPublishAt == nil || status.ScheduledUnpublishAt == nil {
		t.Fatalf("expected scheduled status response, got %#v", status)
	}

	detail, err := svc.GetDocument(context.Background(), saved.SourcePath, true)
	if err != nil {
		t.Fatalf("get saved document: %v", err)
	}
	if detail.Status != "scheduled" || detail.Author != "editor" || detail.LastEditor != "editor" {
		t.Fatalf("expected workflow attribution on detail, got %#v", detail)
	}
	if len(detail.FieldSchema) != 2 || detail.Fields["hero"] != "launch" {
		t.Fatalf("expected field schema and defaults on detail, got %#v %#v", detail.FieldSchema, detail.Fields)
	}
}

func TestMediaMetadataServices(t *testing.T) {
	cfg := testServiceConfig(t)
	svc := New(cfg)

	upload, err := svc.SaveMedia(context.Background(), "images", "posts/about", "diagram.png", "image/png", testPNGBytes())
	if err != nil {
		t.Fatalf("save media: %v", err)
	}

	detail, err := svc.SaveMediaMetadata(context.Background(), upload.Reference, types.MediaMetadata{
		Title:       "Architecture diagram",
		Alt:         "Diagram alt",
		Caption:     "System overview",
		Description: "Longer description",
		Credit:      "Team",
		Tags:        []string{"diagram", "architecture"},
	}, "annotate media", "Admin User")
	if err != nil {
		t.Fatalf("save media metadata: %v", err)
	}
	if detail.Metadata.Title != "Architecture diagram" || len(detail.Metadata.Tags) != 2 {
		t.Fatalf("unexpected saved metadata: %#v", detail)
	}

	items, err := svc.ListMedia(context.Background())
	if err != nil {
		t.Fatalf("list media: %v", err)
	}
	if len(items) != 1 || items[0].Metadata.Alt != "Diagram alt" {
		t.Fatalf("expected metadata in media list, got %#v", items)
	}

	gotDetail, err := svc.GetMediaDetail(context.Background(), upload.Reference)
	if err != nil {
		t.Fatalf("get media detail: %v", err)
	}
	if gotDetail.Metadata.Caption != "System overview" {
		t.Fatalf("unexpected detail metadata: %#v", gotDetail)
	}
	if gotDetail.Metadata.OriginalFilename != "diagram.png" || gotDetail.Metadata.ContentHash == "" {
		t.Fatalf("expected technical media metadata, got %#v", gotDetail.Metadata)
	}
	if len(gotDetail.UsedBy) != 0 {
		t.Fatalf("expected no media usage, got %#v", gotDetail.UsedBy)
	}
	filtered, err := svc.ListMedia(context.Background(), "diagram alt")
	if err != nil {
		t.Fatalf("list filtered media: %v", err)
	}
	if len(filtered) != 1 || filtered[0].Reference != upload.Reference {
		t.Fatalf("expected filtered media result, got %#v", filtered)
	}
	if _, err := svc.SaveMediaMetadata(context.Background(), upload.Reference, types.MediaMetadata{
		Title: "Updated diagram",
	}, "update title", "Admin User"); err != nil {
		t.Fatalf("update media metadata: %v", err)
	}
	entries, err := os.ReadDir(filepath.Join(cfg.ContentDir, cfg.Content.ImagesDir, "posts", "about"))
	if err != nil {
		t.Fatalf("read media dir: %v", err)
	}
	var foundMetadataVersion bool
	var foundMetadataComment bool
	baseName := strings.TrimSuffix(filepath.Base(upload.Path), filepath.Ext(upload.Path))
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), baseName+".version.") && strings.HasSuffix(entry.Name(), ".png.meta.yaml") {
			foundMetadataVersion = true
			body, err := os.ReadFile(filepath.Join(cfg.ContentDir, cfg.Content.ImagesDir, "posts", "about", entry.Name()))
			if err != nil {
				t.Fatalf("read metadata version sidecar: %v", err)
			}
			if strings.Contains(string(body), "version_comment: update title") {
				foundMetadataComment = true
			}
		}
	}
	if !foundMetadataVersion {
		t.Fatal("expected metadata update to create versioned sidecar")
	}
	if !foundMetadataComment {
		t.Fatal("expected metadata update to store version comment in the versioned sidecar")
	}

	if err := svc.DeleteMedia(context.Background(), upload.Reference); err != nil {
		t.Fatalf("delete media: %v", err)
	}
	entries, err = os.ReadDir(filepath.Join(cfg.ContentDir, cfg.Content.ImagesDir, "posts", "about"))
	if err != nil {
		t.Fatalf("read media dir: %v", err)
	}
	var foundTrash, foundSidecarTrash bool
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), baseName+".trash.") && strings.HasSuffix(entry.Name(), ".png") {
			foundTrash = true
		}
		if strings.HasPrefix(entry.Name(), baseName+".trash.") && strings.HasSuffix(entry.Name(), ".png.meta.yaml") {
			foundSidecarTrash = true
		}
	}
	if !foundTrash || !foundSidecarTrash {
		t.Fatalf("expected trashed media and sidecar, got entries %#v", entries)
	}
}

func TestReplaceMediaPreservesCanonicalReference(t *testing.T) {
	cfg := testServiceConfig(t)
	svc := New(cfg)

	upload, err := svc.SaveMedia(context.Background(), "images", "posts/about", "diagram.png", "image/png", testPNGBytes())
	if err != nil {
		t.Fatalf("save media: %v", err)
	}

	replaced, err := svc.ReplaceMedia(context.Background(), upload.Reference, "image/png", testPNGBytes())
	if err != nil {
		t.Fatalf("replace media: %v", err)
	}
	if !replaced.Replaced || replaced.Reference != upload.Reference {
		t.Fatalf("expected canonical reference to be preserved, got %#v", replaced)
	}

	body, err := os.ReadFile(filepath.Join(cfg.ContentDir, cfg.Content.ImagesDir, filepath.FromSlash(upload.Path)))
	if err != nil {
		t.Fatalf("read replaced media: %v", err)
	}
	if len(body) == 0 {
		t.Fatalf("expected replaced media body, got %q", string(body))
	}

	entries, err := os.ReadDir(filepath.Join(cfg.ContentDir, cfg.Content.ImagesDir, "posts", "about"))
	if err != nil {
		t.Fatalf("read media dir: %v", err)
	}
	foundVersion := false
	baseName := strings.TrimSuffix(filepath.Base(upload.Path), filepath.Ext(upload.Path))
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), baseName+".version.") && strings.HasSuffix(entry.Name(), ".png") {
			foundVersion = true
			break
		}
	}
	if !foundVersion {
		t.Fatal("expected replaced media to create a versioned file")
	}
}

func TestVersionRetentionLimit(t *testing.T) {
	cfg := testServiceConfig(t)
	cfg.Content.MaxVersionsPerFile = 2
	svc := New(cfg)

	for i := 0; i < 4; i++ {
		_, err := svc.SaveDocument(context.Background(), types.DocumentSaveRequest{
			SourcePath: "pages/retention.md",
			Raw:        "---\ntitle: Retention\nslug: retention\n---\n\nVersion " + strings.Repeat("x", i+1),
		})
		if err != nil {
			t.Fatalf("save document %d: %v", i, err)
		}
	}

	entries, err := os.ReadDir(filepath.Join(cfg.ContentDir, "pages"))
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	var versions int
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "retention.version.") {
			versions++
		}
	}
	if versions != 2 {
		t.Fatalf("expected 2 retained versions, got %d", versions)
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
	if err := os.MkdirAll(filepath.Join(cfg.ContentDir, cfg.Content.PostsDir), 0o755); err != nil {
		t.Fatalf("mkdir posts dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(cfg.ContentDir, "config"), 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	hash, err := users.HashPassword("secret-password")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	cfg.Admin.UsersFile = filepath.Join(cfg.ContentDir, "config", "admin-users.yaml")
	if err := os.WriteFile(cfg.Admin.UsersFile, []byte("users:\n  - username: admin\n    name: Admin User\n    email: admin@example.com\n    role: admin\n    password_hash: "+hash+"\n"), 0o644); err != nil {
		t.Fatalf("write users file: %v", err)
	}
	return cfg
}

func writeServiceTheme(t *testing.T, cfg *config.Config, name string) {
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
			t.Fatalf("write %s: %v", path, err)
		}
	}
}

func writeServiceThemeContracts(t *testing.T, cfg *config.Config, extra string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(cfg.ThemesDir, cfg.Theme, "theme.yaml")); err != nil {
		writeServiceTheme(t, cfg, cfg.Theme)
	}
	path := filepath.Join(cfg.ThemesDir, cfg.Theme, "theme.yaml")
	base, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read theme manifest: %v", err)
	}
	content := strings.TrimRight(string(base), "\n") + "\n" + strings.TrimSpace(extra) + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write theme manifest: %v", err)
	}
}

func writePluginMetadata(t *testing.T, cfg *config.Config, name string) {
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

func testMP4Bytes() []byte {
	return []byte{0x00, 0x00, 0x00, 0x18, 'f', 't', 'y', 'p', 'm', 'p', '4', '2', 0x00, 0x00, 0x00, 0x00, 'm', 'p', '4', '2', 'i', 's', 'o', 'm'}
}

func testPDFBytes() []byte {
	return []byte("%PDF-1.4\n")
}
