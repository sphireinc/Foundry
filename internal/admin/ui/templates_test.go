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
	body, err := os.ReadFile(filepath.Join("..", "..", "..", "themes", "admin-themes", "default", "assets", "admin.js"))
	if err != nil {
		t.Fatalf("read default admin theme js: %v", err)
	}
	source := string(body)
	if !strings.Contains(source, "Structured Frontmatter") {
		t.Fatalf("expected structured frontmatter UI in admin.js")
	}
	if !strings.Contains(source, "Insert stable <code>media:</code> references at the cursor.") {
		t.Fatalf("expected media picker UI in admin.js")
	}
}
