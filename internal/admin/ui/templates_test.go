package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sphireinc/foundry/internal/config"
)

func TestRenderIndexEscapesTitle(t *testing.T) {
	cfg := &config.Config{Title: `<script>alert(1)</script>`, ThemesDir: t.TempDir()}
	cfg.ApplyDefaults()

	body, err := NewManager(cfg).RenderIndex()
	if err != nil {
		t.Fatalf("render index: %v", err)
	}
	html := string(body)
	if strings.Contains(html, "<script>alert(1)</script>") {
		t.Fatalf("expected title to be escaped, got %q", html)
	}
	if !strings.Contains(html, "&lt;script&gt;alert(1)&lt;/script&gt; Admin") {
		t.Fatalf("expected escaped title in output, got %q", html)
	}
	if !strings.Contains(html, `data-default-lang="en"`) {
		t.Fatalf("expected default lang data attribute in output, got %q", html)
	}
	if !strings.Contains(html, `type="module"`) {
		t.Fatalf("expected admin script to load as an ES module, got %q", html)
	}
}

func TestManagerLoadsFilesystemThemeWhenPresent(t *testing.T) {
	root := t.TempDir()
	cfg := &config.Config{
		Title:     "Foundry",
		ThemesDir: root,
		Admin:     config.AdminConfig{Theme: "studio"},
	}
	cfg.ApplyDefaults()

	themeRoot := filepath.Join(root, "admin-themes", "studio")
	if err := os.MkdirAll(filepath.Join(themeRoot, "assets"), 0o755); err != nil {
		t.Fatalf("mkdir theme: %v", err)
	}
	if err := os.WriteFile(filepath.Join(themeRoot, "index.html"), []byte(`<!doctype html><title>{{ .Title }}</title><body>studio theme</body>`), 0o644); err != nil {
		t.Fatalf("write index: %v", err)
	}
	if err := os.WriteFile(filepath.Join(themeRoot, "assets", "admin.css"), []byte(`body { color: red; }`), 0o644); err != nil {
		t.Fatalf("write css: %v", err)
	}

	manager := NewManager(cfg)
	body, err := manager.RenderIndex()
	if err != nil {
		t.Fatalf("render custom index: %v", err)
	}
	if !strings.Contains(string(body), "studio theme") {
		t.Fatalf("expected custom theme html, got %q", string(body))
	}
}

func TestDefaultAdminThemeAssetsIncludeStructuredEditor(t *testing.T) {
	indexBody, err := os.ReadFile(filepath.Join("..", "..", "..", "themes", "admin-themes", "default", "index.html"))
	if err != nil {
		t.Fatalf("read default admin theme index: %v", err)
	}
	if !strings.Contains(string(indexBody), `type="module"`) || !strings.Contains(string(indexBody), `data-default-lang="{{ .DefaultLang }}"`) {
		t.Fatalf("expected default admin theme index to load the admin app as a module with default lang data")
	}
	body, err := os.ReadFile(filepath.Join("..", "..", "..", "themes", "admin-themes", "default", "assets", "admin.js"))
	if err != nil {
		t.Fatalf("read default admin theme js: %v", err)
	}
	source := string(body)
	if !strings.Contains(source, "Structured Frontmatter") {
		t.Fatalf("expected structured frontmatter UI in admin.js")
	}
	if !strings.Contains(source, "createAdminClient") || !strings.Contains(source, "/__foundry/sdk/admin/index.js") {
		t.Fatalf("expected default admin theme to consume the official admin sdk")
	}
	if !strings.Contains(source, "foundry:admin-extension-page") || !strings.Contains(source, "window.FoundryAdmin") {
		t.Fatalf("expected default admin theme to expose the admin extension mount contract")
	}
	if !strings.Contains(source, "mountAdminExtensionPage") || !strings.Contains(source, "module_url") {
		t.Fatalf("expected default admin theme to auto-load plugin extension page bundles")
	}
	if !strings.Contains(source, "foundry:admin-extension-widget") || !strings.Contains(source, "mountAdminExtensionWidget") {
		t.Fatalf("expected default admin theme to auto-load plugin extension widgets")
	}
	if !strings.Contains(source, "admin/editor/frontmatter.js") || !strings.Contains(source, "admin/views/shared.js") {
		t.Fatalf("expected admin.js to import modularized editor and view code")
	}
	if !strings.Contains(source, "user-reset-start") || !strings.Contains(source, "user-totp-setup") || !strings.Contains(source, "user-revoke-all-sessions") {
		t.Fatalf("expected default admin theme to expose user security management flows")
	}
	eventBody, err := os.ReadFile(filepath.Join("..", "..", "..", "themes", "admin-themes", "default", "assets", "admin", "events", "dashboard.js"))
	if err != nil {
		t.Fatalf("read dashboard events module: %v", err)
	}
	eventSource := string(eventBody)
	if !strings.Contains(eventSource, "startPasswordReset") || !strings.Contains(eventSource, "enableTOTP") || !strings.Contains(eventSource, "revoke({ all: true })") {
		t.Fatalf("expected modularized admin theme to use official admin sdk security APIs")
	}
	if !strings.Contains(source, "Runtime Profiling") || !strings.Contains(source, "Embedded pprof") || !strings.Contains(source, "Runtime Summary") || !strings.Contains(source, "/api/debug/runtime") {
		t.Fatalf("expected default admin theme to expose the pprof debug surface")
	}
	viewBody, err := os.ReadFile(filepath.Join("..", "..", "..", "themes", "admin-themes", "default", "assets", "admin", "views", "shared.js"))
	if err != nil {
		t.Fatalf("read shared admin view module: %v", err)
	}
	if !strings.Contains(string(viewBody), "Keyboard Shortcuts") || !strings.Contains(string(viewBody), "data-table-sort") {
		t.Fatalf("expected admin polish controls in shared view module")
	}
	editorBody, err := os.ReadFile(filepath.Join("..", "..", "..", "themes", "admin-themes", "default", "assets", "admin", "editor", "frontmatter.js"))
	if err != nil {
		t.Fatalf("read frontmatter module: %v", err)
	}
	if !strings.Contains(string(editorBody), "Insert stable <code>media:</code> references at the cursor.") && !strings.Contains(source, "Insert stable <code>media:</code> references at the cursor.") {
		t.Fatalf("expected media picker UI copy in modularized admin assets")
	}
}

func TestAdminThemeManifestValidation(t *testing.T) {
	root := t.TempDir()
	cfg := &config.Config{ThemesDir: root}
	cfg.ApplyDefaults()

	themeRoot := filepath.Join(root, "admin-themes", "studio")
	if err := os.MkdirAll(filepath.Join(themeRoot, "assets"), 0o755); err != nil {
		t.Fatalf("mkdir theme: %v", err)
	}
	files := map[string]string{
		filepath.Join(themeRoot, "index.html"):          "<!doctype html><div>studio</div>",
		filepath.Join(themeRoot, "assets", "admin.css"): "body{}",
		filepath.Join(themeRoot, "assets", "admin.js"):  "console.log('studio')",
		filepath.Join(themeRoot, "admin-theme.yaml"):    "name: studio\ntitle: Studio\nversion: 1.0.0\nadmin_api: v1\nsdk_version: v1\ncompatibility_version: v1\ncomponents:\n  - shell\n  - login\n  - navigation\n  - documents\n  - media\n  - users\n  - config\n  - plugins\n  - themes\n  - audit\nwidget_slots:\n  - overview.after\n  - documents.sidebar\n  - media.sidebar\n  - plugins.sidebar\n",
	}
	for path, body := range files {
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}

	manifest, err := LoadManifest(root, "studio")
	if err != nil || manifest.Name != "studio" {
		t.Fatalf("load admin manifest: %v %#v", err, manifest)
	}
	result, err := ValidateTheme(root, "studio")
	if err != nil {
		t.Fatalf("validate admin theme: %v", err)
	}
	if !result.Valid {
		t.Fatalf("expected valid admin theme, got %#v", result.Diagnostics)
	}
	if len(manifest.WidgetSlots) != 4 {
		t.Fatalf("expected admin widget slots to load, got %#v", manifest.WidgetSlots)
	}
}

func TestAdminThemeManifestValidationRequiresWidgetSlots(t *testing.T) {
	root := t.TempDir()
	themeRoot := filepath.Join(root, "admin-themes", "studio")
	if err := os.MkdirAll(filepath.Join(themeRoot, "assets"), 0o755); err != nil {
		t.Fatalf("mkdir theme: %v", err)
	}
	files := map[string]string{
		filepath.Join(themeRoot, "index.html"):          "<!doctype html><div>studio</div>",
		filepath.Join(themeRoot, "assets", "admin.css"): "body{}",
		filepath.Join(themeRoot, "assets", "admin.js"):  "console.log('studio')",
		filepath.Join(themeRoot, "admin-theme.yaml"):    "name: studio\ntitle: Studio\nversion: 1.0.0\nadmin_api: v1\nsdk_version: v1\ncompatibility_version: v1\ncomponents:\n  - shell\n  - login\n  - navigation\n  - documents\n  - media\n  - users\n  - config\n  - plugins\n  - themes\n  - audit\nwidget_slots:\n  - overview.after\n  - documents.sidebar\n  - media.sidebar\n",
	}
	for path, body := range files {
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}

	result, err := ValidateTheme(root, "studio")
	if err != nil {
		t.Fatalf("validate admin theme: %v", err)
	}
	if result.Valid {
		t.Fatalf("expected missing widget slot to make admin theme invalid")
	}
}
