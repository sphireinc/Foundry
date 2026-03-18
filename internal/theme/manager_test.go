package theme

import (
	"os"
	"path/filepath"
	"testing"
)

func TestManagerLayoutPathAndMustExist(t *testing.T) {
	root := t.TempDir()
	mgr := NewManager(root, "default")

	wantPath := filepath.Join(root, "default", "layouts", "post.html")
	if got := mgr.LayoutPath("post"); got != wantPath {
		t.Fatalf("expected %q, got %q", wantPath, got)
	}

	if err := os.MkdirAll(filepath.Join(root, "default"), 0o755); err != nil {
		t.Fatalf("mkdir theme: %v", err)
	}
	if err := mgr.MustExist(); err != nil {
		t.Fatalf("expected theme to exist: %v", err)
	}
}

func TestManagerMustExistErrors(t *testing.T) {
	root := t.TempDir()
	mgr := NewManager(root, "missing")
	if err := mgr.MustExist(); err == nil {
		t.Fatal("expected missing theme error")
	}

	filePath := filepath.Join(root, "filetheme")
	if err := os.WriteFile(filePath, []byte("x"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	mgr = NewManager(root, "filetheme")
	if err := mgr.MustExist(); err == nil {
		t.Fatal("expected non-directory theme error")
	}
}
