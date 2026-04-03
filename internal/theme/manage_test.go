package theme

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestListLoadValidateAndScaffoldTheme(t *testing.T) {
	root := t.TempDir()
	scaffolded, err := Scaffold(root, "default-theme")
	if err != nil {
		t.Fatalf("scaffold theme: %v", err)
	}
	if _, err := os.Stat(filepath.Join(scaffolded, "layouts", "base.html")); err != nil {
		t.Fatalf("expected scaffolded layout: %v", err)
	}

	list, err := ListInstalled(root)
	if err != nil || len(list) != 1 || list[0].Name != "default-theme" {
		t.Fatalf("unexpected installed list: %#v %v", list, err)
	}

	manifest, err := LoadManifest(root, "default-theme")
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}
	if manifest.Name != "default-theme" || manifest.Version == "" {
		t.Fatalf("unexpected manifest: %#v", manifest)
	}

	if err := ValidateInstalled(root, "default-theme"); err != nil {
		t.Fatalf("validate installed: %v", err)
	}
}

func TestThemeManagementErrorsAndHelpers(t *testing.T) {
	root := t.TempDir()
	if _, err := ListInstalled(filepath.Join(root, "missing")); err != nil {
		t.Fatalf("expected missing themes dir to return empty list, got %v", err)
	}
	if _, err := LoadManifest(root, ""); err == nil {
		t.Fatal("expected empty theme name error")
	}
	if err := ValidateInstalled(root, ""); err == nil {
		t.Fatal("expected empty theme name error")
	}
	if err := ValidateInstalled(root, ".."); err == nil {
		t.Fatal("expected path traversal theme name error")
	}
	if _, err := Scaffold(root, "bad/name"); err == nil {
		t.Fatal("expected invalid scaffold name error")
	}
	if _, err := Scaffold(root, ".."); err == nil {
		t.Fatal("expected traversal scaffold name error")
	}
	if err := SwitchInConfig(filepath.Join(t.TempDir(), "site.yaml"), ".."); err == nil {
		t.Fatal("expected invalid switch theme name error")
	}

	name := humanizeName("my_theme-name")
	if name != "My Theme Name" {
		t.Fatalf("unexpected humanized name: %q", name)
	}
	if !strings.Contains(scaffoldManifest("hello"), "name: hello") {
		t.Fatal("expected manifest scaffold content")
	}
	for _, slot := range requiredLaunchSlots {
		if !strings.Contains(scaffoldManifest("hello"), slot) {
			t.Fatalf("expected scaffold manifest to include slot %q", slot)
		}
	}
	if !strings.Contains(scaffoldBase(), `define "base"`) ||
		!strings.Contains(scaffoldHead(), `define "head"`) ||
		!strings.Contains(scaffoldHeader(), `define "header"`) ||
		!strings.Contains(scaffoldFooter(), `define "footer"`) ||
		!strings.Contains(scaffoldIndex(), `define "content"`) ||
		!strings.Contains(scaffoldPage(), `safeHTML`) ||
		!strings.Contains(scaffoldPost(), `safeHTML`) ||
		!strings.Contains(scaffoldList(), `No entries found.`) ||
		!strings.Contains(scaffoldCSS(), "font-family") {
		t.Fatal("expected scaffold helper content")
	}
	if !strings.Contains(scaffoldBase(), "window.__foundryReloadSource") ||
		!strings.Contains(scaffoldBase(), "window.__foundryReloadPollTimer") ||
		!strings.Contains(scaffoldBase(), "/__reload/poll") ||
		!strings.Contains(scaffoldBase(), "mode === 'poll'") ||
		!strings.Contains(scaffoldBase(), "pagehide") ||
		!strings.Contains(scaffoldBase(), "beforeunload") {
		t.Fatal("expected scaffold base to close live reload connections")
	}
}

func TestValidateInstalledRequiresLaunchSlotsInManifestAndTemplates(t *testing.T) {
	root := t.TempDir()
	scaffolded, err := Scaffold(root, "launch-theme")
	if err != nil {
		t.Fatalf("scaffold theme: %v", err)
	}

	manifestPath := filepath.Join(scaffolded, "theme.yaml")
	body, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	body = []byte(strings.Replace(string(body), "  - post.sidebar.bottom\n", "", 1))
	if err := os.WriteFile(manifestPath, body, 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	if err := ValidateInstalled(root, "launch-theme"); err == nil {
		t.Fatal("expected validation failure for missing required slot declaration")
	}

	scaffolded, err = Scaffold(root, "template-theme")
	if err != nil {
		t.Fatalf("scaffold theme: %v", err)
	}
	postPath := filepath.Join(scaffolded, "layouts", "post.html")
	postBody, err := os.ReadFile(postPath)
	if err != nil {
		t.Fatalf("read post layout: %v", err)
	}
	postBody = []byte(strings.Replace(string(postBody), `{{ pluginSlot "post.sidebar.overview" }}`, "", 1))
	if err := os.WriteFile(postPath, postBody, 0o644); err != nil {
		t.Fatalf("write post layout: %v", err)
	}
	if err := ValidateInstalled(root, "template-theme"); err == nil {
		t.Fatal("expected validation failure for missing required slot rendering")
	}
}

func TestValidateInstalledDetailedReportsTemplateProblems(t *testing.T) {
	root := t.TempDir()
	scaffolded, err := Scaffold(root, "broken-theme")
	if err != nil {
		t.Fatalf("scaffold theme: %v", err)
	}
	basePath := filepath.Join(scaffolded, "layouts", "base.html")
	if err := os.WriteFile(basePath, []byte(`{{ define "base" }}{{ template "missing-partial" . }}{{ end }}`), 0o644); err != nil {
		t.Fatalf("write broken base: %v", err)
	}

	result, err := ValidateInstalledDetailed(root, "broken-theme")
	if err != nil {
		t.Fatalf("validate detailed: %v", err)
	}
	if result.Valid {
		t.Fatal("expected invalid validation result")
	}
	var foundReference bool
	for _, diagnostic := range result.Diagnostics {
		if strings.Contains(diagnostic.Message, "unknown partial") {
			foundReference = true
			break
		}
	}
	if !foundReference {
		t.Fatalf("expected unknown partial diagnostic, got %#v", result.Diagnostics)
	}
}

func TestValidateInstalledDetailedRejectsUnsupportedSDKVersion(t *testing.T) {
	root := t.TempDir()
	scaffolded, err := Scaffold(root, "sdk-theme")
	if err != nil {
		t.Fatalf("scaffold theme: %v", err)
	}
	manifestPath := filepath.Join(scaffolded, "theme.yaml")
	body, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	body = []byte(strings.Replace(string(body), "sdk_version: v1\n", "sdk_version: v2\n", 1))
	if err := os.WriteFile(manifestPath, body, 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	result, err := ValidateInstalledDetailed(root, "sdk-theme")
	if err != nil {
		t.Fatalf("validate detailed: %v", err)
	}
	if result.Valid {
		t.Fatal("expected unsupported sdk version to invalidate theme")
	}
}

func TestValidateInstalledDetailedChecksThemeSecurity(t *testing.T) {
	root := t.TempDir()
	scaffolded, err := Scaffold(root, "security-theme")
	if err != nil {
		t.Fatalf("scaffold theme: %v", err)
	}
	headPath := filepath.Join(scaffolded, "layouts", "partials", "head.html")
	headBody, err := os.ReadFile(headPath)
	if err != nil {
		t.Fatalf("read head: %v", err)
	}
	updatedHead := strings.Replace(string(headBody), `{{ pluginSlot "head.end" }}`, `{{ pluginSlot "head.end" }}<script src="https://cdn.example.com/theme.js"></script>`, 1)
	if err := os.WriteFile(headPath, []byte(updatedHead), 0o644); err != nil {
		t.Fatalf("write head: %v", err)
	}

	result, err := ValidateInstalledDetailed(root, "security-theme")
	if err != nil {
		t.Fatalf("validate detailed: %v", err)
	}
	if result.Valid {
		t.Fatal("expected undeclared remote asset to invalidate theme")
	}

	manifestPath := filepath.Join(scaffolded, "theme.yaml")
	body, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	updated := strings.Replace(string(body), "  external_assets:\n    allowed: false\n", "  external_assets:\n    allowed: true\n    scripts:\n      - https://cdn.example.com\n", 1)
	if err := os.WriteFile(manifestPath, []byte(updated), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	result, err = ValidateInstalledDetailed(root, "security-theme")
	if err != nil {
		t.Fatalf("validate detailed after allowlist: %v", err)
	}
	if !result.Valid {
		t.Fatalf("expected security declaration to validate, got %#v", result.Diagnostics)
	}
}

func TestDocumentFieldDefinitionsMatchesThemeContracts(t *testing.T) {
	root := t.TempDir()
	themeRoot := filepath.Join(root, "contract-theme")
	if err := os.MkdirAll(themeRoot, 0o755); err != nil {
		t.Fatalf("mkdir theme root: %v", err)
	}
	manifest := `name: contract-theme
title: Contract Theme
version: 0.1.0
min_foundry_version: 0.1.0
sdk_version: v1
compatibility_version: v1
layouts: [base, index, page, post, list]
slots: [head.end, body.start, body.end, page.before_main, page.after_main, page.before_content, page.after_content, post.before_header, post.after_header, post.before_content, post.after_content, post.sidebar.top, post.sidebar.overview, post.sidebar.bottom]
field_contracts:
  - key: marketing-page
    target:
      scope: document
      types: [page]
      layouts: [page]
      slugs: [about]
    fields:
      - name: hero_title
        type: text
  - key: blog-post
    target:
      scope: document
      types: [post]
      layouts: [post]
    fields:
      - name: hero_eyebrow
        type: text
`
	if err := os.WriteFile(filepath.Join(themeRoot, "theme.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	pageDefs := DocumentFieldDefinitions(root, "contract-theme", "page", "page", "about")
	if len(pageDefs) != 1 || pageDefs[0].Name != "hero_title" {
		t.Fatalf("expected page contract fields, got %#v", pageDefs)
	}

	postDefs := DocumentFieldDefinitions(root, "contract-theme", "post", "post", "hello-world")
	if len(postDefs) != 1 || postDefs[0].Name != "hero_eyebrow" {
		t.Fatalf("expected post contract fields, got %#v", postDefs)
	}

	missingDefs := DocumentFieldDefinitions(root, "contract-theme", "page", "page", "pricing")
	if len(missingDefs) != 0 {
		t.Fatalf("expected unmatched slug to return no fields, got %#v", missingDefs)
	}
}
