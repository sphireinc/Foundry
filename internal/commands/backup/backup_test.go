package backupcmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/sphireinc/foundry/internal/config"
)

func TestBackupCommandCreateAndList(t *testing.T) {
	cfg := testConfig(t)
	cmd := command{}
	if err := cmd.Run(cfg, []string{"foundry", "backup", "create"}); err != nil {
		t.Fatalf("backup create: %v", err)
	}
	if err := cmd.Run(cfg, []string{"foundry", "backup", "list"}); err != nil {
		t.Fatalf("backup list: %v", err)
	}
}

func TestBackupCommandGitSnapshot(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	cfg := testConfig(t)
	cmd := command{}
	if err := cmd.Run(cfg, []string{"foundry", "backup", "git-snapshot", "snapshot"}); err != nil {
		t.Fatalf("backup git-snapshot: %v", err)
	}
	if err := cmd.Run(cfg, []string{"foundry", "backup", "git-log", "5"}); err != nil {
		t.Fatalf("backup git-log: %v", err)
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
		t.Fatalf("mkdir pages: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cfg.ContentDir, "pages", "home.md"), []byte("body"), 0o644); err != nil {
		t.Fatalf("write content: %v", err)
	}
	return cfg
}
