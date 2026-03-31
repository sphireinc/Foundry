package backup

import (
	"archive/zip"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/sphireinc/foundry/internal/config"
)

func TestCreateZipSnapshot(t *testing.T) {
	cfg := testConfig(t)
	target := filepath.Join(t.TempDir(), "snapshot.zip")

	snapshot, err := CreateZipSnapshot(cfg, target)
	if err != nil {
		t.Fatalf("create snapshot: %v", err)
	}
	if snapshot.SizeBytes <= 0 {
		t.Fatalf("expected archive size, got %#v", snapshot)
	}

	reader, err := zip.OpenReader(target)
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	defer reader.Close()

	foundContent := false
	foundManifest := false
	for _, file := range reader.File {
		switch file.Name {
		case "content/pages/index.md":
			foundContent = true
		case "backup-manifest.json":
			foundManifest = true
		}
	}
	if !foundContent || !foundManifest {
		t.Fatalf("expected content file and manifest in archive")
	}
}

func TestCreateManagedSnapshotAndPrune(t *testing.T) {
	cfg := testConfig(t)
	if _, err := CreateManagedSnapshot(cfg); err != nil {
		t.Fatalf("create managed snapshot 1: %v", err)
	}
	if _, err := CreateManagedSnapshot(cfg); err != nil {
		t.Fatalf("create managed snapshot 2: %v", err)
	}
	items, err := List(cfg.Backup.Dir)
	if err != nil {
		t.Fatalf("list backups: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 backups, got %d", len(items))
	}
	if err := Prune(cfg.Backup.Dir, 1); err != nil {
		t.Fatalf("prune: %v", err)
	}
	items, err = List(cfg.Backup.Dir)
	if err != nil {
		t.Fatalf("list after prune: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 backup after prune, got %d", len(items))
	}
}

func TestRequiredFreeBytes(t *testing.T) {
	required := requiredFreeBytes(100, 125, 1)
	if required <= 100 {
		t.Fatalf("expected headroom requirement, got %d", required)
	}
}

func TestCreateGitSnapshot(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	cfg := testConfig(t)
	snapshot, err := CreateGitSnapshot(cfg, "initial snapshot", false)
	if err != nil {
		t.Fatalf("create git snapshot: %v", err)
	}
	if snapshot.Revision == "" {
		t.Fatalf("expected revision, got %#v", snapshot)
	}
	items, err := ListGitSnapshots(cfg, 5)
	if err != nil {
		t.Fatalf("list git snapshots: %v", err)
	}
	if len(items) == 0 {
		t.Fatal("expected git snapshot history")
	}
}

func testConfig(t *testing.T) *config.Config {
	t.Helper()
	root := t.TempDir()
	cfg := &config.Config{
		ContentDir: filepath.Join(root, "content"),
		Backup: config.BackupConfig{
			Enabled:         true,
			Dir:             filepath.Join(root, ".foundry", "backups"),
			RetentionCount:  5,
			DebounceSeconds: 1,
			MinFreeMB:       1,
			HeadroomPercent: 100,
		},
	}
	cfg.ApplyDefaults()
	if err := os.MkdirAll(filepath.Join(cfg.ContentDir, "pages"), 0o755); err != nil {
		t.Fatalf("mkdir content: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cfg.ContentDir, "pages", "index.md"), []byte("hello"), 0o644); err != nil {
		t.Fatalf("write content: %v", err)
	}
	return cfg
}
