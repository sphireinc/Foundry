package safepath

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestEnsureNoSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "content", "posts")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("mkdir nested: %v", err)
	}

	target := filepath.Join(nested, "hello.md")
	if err := os.WriteFile(target, []byte("hello"), 0o644); err != nil {
		t.Fatalf("write target: %v", err)
	}
	if err := EnsureNoSymlinkEscape(filepath.Join(root, "content"), target); err != nil {
		t.Fatalf("expected safe path to pass: %v", err)
	}

	outside := filepath.Join(root, "secret.txt")
	if err := os.WriteFile(outside, []byte("secret"), 0o600); err != nil {
		t.Fatalf("write outside: %v", err)
	}
	link := filepath.Join(nested, "linked.md")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlink not supported on %s: %v", runtime.GOOS, err)
	}
	if err := EnsureNoSymlinkEscape(filepath.Join(root, "content"), link); err == nil {
		t.Fatal("expected symlink escape to be rejected")
	}
}

func TestEnsureNoSymlinkEscapeRejectsSymlinkedRoot(t *testing.T) {
	root := t.TempDir()
	realRoot := filepath.Join(root, "real-content")
	if err := os.MkdirAll(realRoot, 0o755); err != nil {
		t.Fatalf("mkdir real root: %v", err)
	}
	linkedRoot := filepath.Join(root, "content")
	if err := os.Symlink(realRoot, linkedRoot); err != nil {
		t.Skipf("symlink not supported on %s: %v", runtime.GOOS, err)
	}

	if err := EnsureNoSymlinkEscape(linkedRoot, filepath.Join(linkedRoot, "posts", "hello.md")); err == nil {
		t.Fatal("expected symlinked root to be rejected")
	}
}

func TestEnsureNoSymlinkEscapeRejectsSymlinkedDirectoryComponent(t *testing.T) {
	root := t.TempDir()
	contentRoot := filepath.Join(root, "content")
	if err := os.MkdirAll(filepath.Join(contentRoot, "posts"), 0o755); err != nil {
		t.Fatalf("mkdir content root: %v", err)
	}
	outside := filepath.Join(t.TempDir(), "outside")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatalf("mkdir outside root: %v", err)
	}
	if err := os.WriteFile(filepath.Join(outside, "hello.md"), []byte("hello"), 0o600); err != nil {
		t.Fatalf("write outside file: %v", err)
	}
	linkedDir := filepath.Join(contentRoot, "posts")
	if err := os.RemoveAll(linkedDir); err != nil {
		t.Fatalf("remove posts dir: %v", err)
	}
	if err := os.Symlink(outside, linkedDir); err != nil {
		t.Skipf("symlink not supported on %s: %v", runtime.GOOS, err)
	}

	if err := EnsureNoSymlinkEscape(contentRoot, filepath.Join(linkedDir, "hello.md")); err == nil {
		t.Fatal("expected symlinked directory component to be rejected")
	}
}
