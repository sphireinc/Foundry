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
