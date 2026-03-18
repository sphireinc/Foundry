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
}
