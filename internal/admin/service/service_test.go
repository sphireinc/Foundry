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
		Raw: "---\ntitle: Preview\nslug: preview\n---\n\n# Hello\n\n![Clip](media:uploads/demo.mp4)",
	})
	if err != nil || preview.Title != "Preview" || !strings.Contains(preview.HTML, "<h1") {
		t.Fatalf("preview document: %v %#v", err, preview)
	}
	if !strings.Contains(preview.HTML, `<video controls preload="metadata" src="/uploads/demo.mp4" title="Clip" aria-label="Clip"></video>`) {
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

func TestMediaUploadAndListing(t *testing.T) {
	cfg := testServiceConfig(t)
	svc := New(cfg)

	first, err := svc.SaveMedia(context.Background(), "", "posts/hello", "Hero Banner.PNG", "image/png", []byte("image"))
	if err != nil {
		t.Fatalf("save media: %v", err)
	}
	if first.Collection != "images" || first.Path != "posts/hello/hero-banner.png" || first.Reference != media.MustReference("images", "posts/hello/hero-banner.png") {
		t.Fatalf("unexpected first upload response: %#v", first)
	}

	second, err := svc.SaveMedia(context.Background(), "uploads", "posts/hello", "clip.mp4", "video/mp4", []byte("video"))
	if err != nil {
		t.Fatalf("save second media: %v", err)
	}
	if second.Collection != "uploads" || second.Kind != "video" {
		t.Fatalf("unexpected second upload response: %#v", second)
	}

	dupe, err := svc.SaveMedia(context.Background(), "images", "posts/hello", "hero banner.png", "image/png", []byte("dupe"))
	if err != nil {
		t.Fatalf("save duplicate media: %v", err)
	}
	if dupe.Path != "posts/hello/hero-banner.png" || dupe.Created {
		t.Fatalf("expected overwrite with stable path, got %#v", dupe)
	}
	entries, err := os.ReadDir(filepath.Join(cfg.ContentDir, cfg.Content.ImagesDir, "posts", "hello"))
	if err != nil {
		t.Fatalf("read media dir: %v", err)
	}
	var foundVersion bool
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "hero-banner.version.") {
			foundVersion = true
		}
	}
	if !foundVersion {
		t.Fatal("expected media overwrite to create versioned file")
	}

	items, err := svc.ListMedia(context.Background())
	if err != nil {
		t.Fatalf("list media: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 current media items, got %#v", items)
	}
}

func TestSaveMediaErrors(t *testing.T) {
	cfg := testServiceConfig(t)
	svc := New(cfg)

	if _, err := svc.SaveMedia(context.Background(), "bad", "", "clip.mp4", "video/mp4", []byte("x")); err == nil {
		t.Fatal("expected invalid collection error")
	}
	if _, err := svc.SaveMedia(context.Background(), "uploads", "../escape", "clip.mp4", "video/mp4", []byte("x")); err == nil {
		t.Fatal("expected invalid media dir error")
	}
	if _, err := svc.SaveMedia(context.Background(), "uploads", "", "clip.mp4", "video/mp4", nil); err == nil {
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
		Password: "secret-password",
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
	savedConfig, err := svc.SaveConfigDocument(context.Background(), strings.Replace(configDoc.Raw, "theme: default", "theme: alt", 1))
	if err != nil || !strings.Contains(savedConfig.Raw, "theme: alt") {
		t.Fatalf("save config: %v %#v", err, savedConfig)
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
	if err := svc.EnablePlugin(context.Background(), "beta"); err != nil {
		t.Fatalf("enable plugin: %v", err)
	}
	if err := svc.DisablePlugin(context.Background(), "alpha"); err != nil {
		t.Fatalf("disable plugin: %v", err)
	}

	upload, err := svc.SaveMedia(context.Background(), "uploads", "docs", "guide.pdf", "application/pdf", []byte("pdf"))
	if err != nil {
		t.Fatalf("save media for delete: %v", err)
	}
	if err := svc.DeleteMedia(context.Background(), upload.Reference); err != nil {
		t.Fatalf("delete media: %v", err)
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

func TestMediaMetadataServices(t *testing.T) {
	cfg := testServiceConfig(t)
	svc := New(cfg)

	upload, err := svc.SaveMedia(context.Background(), "images", "posts/about", "diagram.png", "image/png", []byte("png"))
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
	})
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
	if _, err := svc.SaveMediaMetadata(context.Background(), upload.Reference, types.MediaMetadata{
		Title: "Updated diagram",
	}); err != nil {
		t.Fatalf("update media metadata: %v", err)
	}
	entries, err := os.ReadDir(filepath.Join(cfg.ContentDir, cfg.Content.ImagesDir, "posts", "about"))
	if err != nil {
		t.Fatalf("read media dir: %v", err)
	}
	var foundMetadataVersion bool
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "diagram.version.") && strings.HasSuffix(entry.Name(), ".png.meta.yaml") {
			foundMetadataVersion = true
		}
	}
	if !foundMetadataVersion {
		t.Fatal("expected metadata update to create versioned sidecar")
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
		if strings.HasPrefix(entry.Name(), "diagram.trash.") && strings.HasSuffix(entry.Name(), ".png") {
			foundTrash = true
		}
		if strings.HasPrefix(entry.Name(), "diagram.trash.") && strings.HasSuffix(entry.Name(), ".png.meta.yaml") {
			foundSidecarTrash = true
		}
	}
	if !foundTrash || !foundSidecarTrash {
		t.Fatalf("expected trashed media and sidecar, got entries %#v", entries)
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
