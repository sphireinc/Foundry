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
