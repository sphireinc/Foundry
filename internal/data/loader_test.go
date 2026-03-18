package data

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDirAndNormalizeKey(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "nav.yaml"), []byte("title: Home\n"), 0o644); err != nil {
		t.Fatalf("write yaml: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "posts"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "posts", "meta.json"), []byte(`{"count":2}`), 0o644); err != nil {
		t.Fatalf("write json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "ignore.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write txt: %v", err)
	}

	store, err := LoadDir(root)
	if err != nil {
		t.Fatalf("load dir: %v", err)
	}
	if _, ok := store.Get("nav"); !ok {
		t.Fatal("expected yaml key to be loaded")
	}
	if _, ok := store.Get("posts/meta"); !ok {
		t.Fatal("expected nested json key to be loaded")
	}
	if normalizeKey("posts/meta.json") != "posts/meta" {
		t.Fatalf("unexpected normalized key")
	}
}

func TestLoadDirAndLoadFileErrors(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(filePath, []byte("x"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if _, err := LoadDir(filePath); err == nil {
		t.Fatal("expected non-directory error")
	}

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "bad.yaml"), []byte(":\n"), 0o644); err != nil {
		t.Fatalf("write yaml: %v", err)
	}
	if _, err := LoadDir(root); err == nil {
		t.Fatal("expected bad yaml error")
	}

	if _, err := loadFile(filepath.Join(root, "missing"), ".txt"); err == nil {
		t.Fatal("expected unsupported extension/read error")
	}
}
