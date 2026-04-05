package plugins

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sphireinc/foundry/internal/installutil"
)

func TestSafeArchivePathRejectsTraversal(t *testing.T) {
	root := t.TempDir()

	if _, err := installutil.SafeArchivePath(root, "../escape.txt"); err == nil {
		t.Fatal("expected path traversal entry to be rejected")
	}
}

func TestSafeArchivePathAcceptsNestedPath(t *testing.T) {
	root := t.TempDir()

	path, err := installutil.SafeArchivePath(root, "plugin/assets/app.css")
	if err != nil {
		t.Fatalf("expected nested path to be accepted: %v", err)
	}
	if path == "" {
		t.Fatal("expected non-empty resolved path")
	}
}

func TestValidateInstallURL(t *testing.T) {
	if got, err := validateInstallURL("acme/demo"); err != nil || got != "https://github.com/acme/demo.git" {
		t.Fatalf("expected shorthand GitHub URL, got %q %v", got, err)
	}
	if _, err := validateInstallURL("http://github.com/acme/demo.git"); err == nil {
		t.Fatal("expected insecure URL rejection")
	}
	if _, err := validateInstallURL("https://example.com/acme/demo.git"); err == nil {
		t.Fatal("expected non-GitHub URL rejection")
	}
}

func TestRemoveInstalledPluginDirRemovesValidatedChild(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "demo-plugin")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("mkdir target: %v", err)
	}
	if err := os.WriteFile(filepath.Join(target, "plugin.yaml"), []byte("name: demo-plugin\n"), 0o644); err != nil {
		t.Fatalf("write target file: %v", err)
	}

	if err := removeInstalledPluginDir(root, "demo-plugin"); err != nil {
		t.Fatalf("remove installed plugin dir: %v", err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("expected plugin directory to be removed, stat err=%v", err)
	}
}

func TestRemoveInstalledPluginDirRejectsTraversal(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatalf("mkdir outside: %v", err)
	}

	if err := removeInstalledPluginDir(root, "../outside"); err == nil {
		t.Fatal("expected traversal name to be rejected")
	}
}

func TestRemoveInstalledPluginDirRejectsNestedSeparators(t *testing.T) {
	root := t.TempDir()

	for _, bad := range []string{"nested/plugin", `nested\\plugin`} {
		if err := removeInstalledPluginDir(root, bad); err == nil {
			t.Fatalf("expected %q to be rejected", bad)
		}
	}
}
