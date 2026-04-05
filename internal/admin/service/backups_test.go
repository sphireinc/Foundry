package service

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sphireinc/foundry/internal/config"
)

func TestBackupPathRejectsTraversalName(t *testing.T) {
	root := t.TempDir()
	svc := New(&config.Config{
		Backup: config.BackupConfig{Dir: filepath.Join(root, "backups")},
	})
	if err := os.MkdirAll(svc.cfg.Backup.Dir, 0o755); err != nil {
		t.Fatalf("mkdir backup dir: %v", err)
	}

	if _, err := svc.BackupPath("../escape.zip"); err == nil {
		t.Fatal("expected traversal backup name to be rejected")
	}
}

func TestBackupPathAcceptsSingleFileComponent(t *testing.T) {
	root := t.TempDir()
	svc := New(&config.Config{
		Backup: config.BackupConfig{Dir: filepath.Join(root, "backups")},
	})
	if err := os.MkdirAll(svc.cfg.Backup.Dir, 0o755); err != nil {
		t.Fatalf("mkdir backup dir: %v", err)
	}
	target := filepath.Join(svc.cfg.Backup.Dir, "snapshot.zip")
	if err := os.WriteFile(target, []byte("zip"), 0o644); err != nil {
		t.Fatalf("write backup file: %v", err)
	}

	got, err := svc.BackupPath("snapshot.zip")
	if err != nil {
		t.Fatalf("BackupPath valid file: %v", err)
	}
	if got != target {
		t.Fatalf("BackupPath = %q, want %q", got, target)
	}
}
