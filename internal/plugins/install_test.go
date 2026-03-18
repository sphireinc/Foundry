package plugins

import "testing"

func TestSafeArchivePathRejectsTraversal(t *testing.T) {
	root := t.TempDir()

	if _, err := safeArchivePath(root, "../escape.txt"); err == nil {
		t.Fatal("expected path traversal entry to be rejected")
	}
}

func TestSafeArchivePathAcceptsNestedPath(t *testing.T) {
	root := t.TempDir()

	path, err := safeArchivePath(root, "plugin/assets/app.css")
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
