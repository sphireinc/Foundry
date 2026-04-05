package theme

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sphireinc/foundry/internal/installutil"
)

func TestThemeInstallHelpers(t *testing.T) {
	if normalizeInstallURL("acme/demo-theme") != "https://github.com/acme/demo-theme.git" {
		t.Fatal("expected GitHub shorthand normalization")
	}
	if normalizeInstallURL("https://github.com/acme/demo-theme") != "https://github.com/acme/demo-theme.git" {
		t.Fatal("expected GitHub https normalization")
	}
	if normalizeInstallURL("git@github.com:acme/demo-theme.git") != "git@github.com:acme/demo-theme.git" {
		t.Fatal("expected git URL preserved")
	}
	if normalizeInstallURL("") != "" {
		t.Fatal("expected empty install URL to stay empty")
	}

	if name, err := inferThemeName("https://github.com/acme/demo-theme.git"); err != nil || name != "demo-theme" {
		t.Fatalf("unexpected inferred name: %q %v", name, err)
	}
	if name, err := inferThemeName("git@github.com:acme/demo-theme.git"); err != nil || name != "demo-theme" {
		t.Fatalf("unexpected inferred git name: %q %v", name, err)
	}
	if _, err := inferThemeName("https://github.com"); err == nil {
		t.Fatal("expected infer name failure")
	}

	if zipURL, err := installutil.RepoZipURL("https://github.com/acme/demo-theme.git"); err != nil || zipURL != "https://github.com/acme/demo-theme/archive/refs/heads/main.zip" {
		t.Fatalf("unexpected zip URL: %q %v", zipURL, err)
	}
	if zipURL, err := installutil.RepoZipURL("git@github.com:acme/demo-theme.git"); err != nil || zipURL != "https://github.com/acme/demo-theme/archive/refs/heads/main.zip" {
		t.Fatalf("unexpected ssh zip URL: %q %v", zipURL, err)
	}
	if _, err := installutil.RepoZipURL("https://example.com/acme/demo-theme.git"); err == nil {
		t.Fatal("expected non-GitHub zip URL rejection")
	}

	if _, err := Install(InstallOptions{}); err == nil {
		t.Fatal("expected install usage failure")
	}
	if _, err := validateInstallURL("http://github.com/acme/demo-theme.git"); err == nil {
		t.Fatal("expected insecure install URL rejection")
	}
	if _, err := validateInstallURL("https://example.com/acme/demo-theme.git"); err == nil {
		t.Fatal("expected non-GitHub install URL rejection")
	}
	if _, err := validateInstallURL("https://github.com/acme"); err == nil {
		t.Fatal("expected incomplete GitHub repo rejection")
	}
}

func TestValidateThemeInstallName(t *testing.T) {
	if _, err := validateThemeInstallName("demo-theme"); err != nil {
		t.Fatalf("expected valid theme name: %v", err)
	}
	if _, err := validateThemeInstallName("../bad"); err == nil {
		t.Fatal("expected traversal name rejection")
	}
}

func TestListInstalledIgnoresFiles(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "alpha"), 0o755); err != nil {
		t.Fatalf("mkdir alpha: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "README.txt"), []byte("ignore"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	items, err := ListInstalled(root)
	if err != nil {
		t.Fatalf("list installed: %v", err)
	}
	if len(items) != 1 || items[0].Name != "alpha" {
		t.Fatalf("unexpected installed themes: %#v", items)
	}
}
