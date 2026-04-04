package doctor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sphireinc/foundry/internal/config"
	"github.com/sphireinc/foundry/internal/theme"
)

func TestDoctorRun(t *testing.T) {
	root := t.TempDir()
	cfg := testProjectConfig(t, root)
	if _, err := theme.Scaffold(cfg.ThemesDir, cfg.Theme); err != nil {
		t.Fatalf("scaffold theme: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "internal", "generated"), 0o755); err != nil {
		t.Fatalf("mkdir generated: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "internal", "generated", "plugins_gen.go"), []byte("package generated\n"), 0o644); err != nil {
		t.Fatalf("write generated imports: %v", err)
	}

	wd, _ := os.Getwd()
	defer func() { _ = os.Chdir(wd) }()
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	cmd := command{}
	if err := cmd.Run(cfg, nil); err != nil {
		t.Fatalf("doctor run: %v", err)
	}
}

func TestDoctorRunFailsWhenGeneratedFileMissing(t *testing.T) {
	root := t.TempDir()
	cfg := testProjectConfig(t, root)
	if _, err := theme.Scaffold(cfg.ThemesDir, cfg.Theme); err != nil {
		t.Fatalf("scaffold theme: %v", err)
	}

	wd, _ := os.Getwd()
	defer func() { _ = os.Chdir(wd) }()
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	cmd := command{}
	if err := cmd.Run(cfg, nil); err == nil {
		t.Fatal("expected doctor failure")
	}
}

func TestDoctorRunFailsOnLegacyAuthRecords(t *testing.T) {
	root := t.TempDir()
	cfg := testProjectConfig(t, root)
	cfg.Admin.Enabled = true
	cfg.Admin.UsersFile = filepath.Join(root, "content", "config", "admin-users.yaml")
	cfg.Admin.SessionStoreFile = filepath.Join(root, "data", "admin", "sessions.yaml")
	if _, err := theme.Scaffold(cfg.ThemesDir, cfg.Theme); err != nil {
		t.Fatalf("scaffold theme: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "internal", "generated"), 0o755); err != nil {
		t.Fatalf("mkdir generated: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "internal", "generated", "plugins_gen.go"), []byte("package generated\n"), 0o644); err != nil {
		t.Fatalf("write generated imports: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(cfg.Admin.UsersFile), 0o755); err != nil {
		t.Fatalf("mkdir users file dir: %v", err)
	}
	if err := os.WriteFile(cfg.Admin.UsersFile, []byte("users:\n  - username: admin\n    password_hash: pbkdf2_sha256$legacy\n    totp_secret: PLAINTEXT\n"), 0o644); err != nil {
		t.Fatalf("write users file: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(cfg.Admin.SessionStoreFile), 0o755); err != nil {
		t.Fatalf("mkdir session dir: %v", err)
	}
	if err := os.WriteFile(cfg.Admin.SessionStoreFile, []byte("sessions:\n  - token: legacy\n"), 0o600); err != nil {
		t.Fatalf("write session file: %v", err)
	}

	wd, _ := os.Getwd()
	defer func() { _ = os.Chdir(wd) }()
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	cmd := command{}
	err := cmd.Run(cfg, nil)
	if err == nil {
		t.Fatal("expected doctor to fail on legacy auth records")
	}
	if !strings.Contains(err.Error(), "problem") {
		t.Fatalf("unexpected doctor error: %v", err)
	}
}

func testProjectConfig(t *testing.T, root string) *config.Config {
	t.Helper()
	cfg := &config.Config{
		Title:       "Foundry",
		BaseURL:     "https://example.com",
		Theme:       "default",
		DefaultLang: "en",
		ContentDir:  filepath.Join(root, "content"),
		PublicDir:   filepath.Join(root, "public"),
		ThemesDir:   filepath.Join(root, "themes"),
		PluginsDir:  filepath.Join(root, "plugins"),
		DataDir:     filepath.Join(root, "data"),
		Content: config.ContentConfig{
			PagesDir: "pages",
			PostsDir: "posts",
		},
	}
	cfg.ApplyDefaults()
	for _, dir := range []string{
		filepath.Join(cfg.ContentDir, cfg.Content.PagesDir),
		filepath.Join(cfg.ContentDir, cfg.Content.PostsDir),
		cfg.DataDir,
		cfg.PluginsDir,
		cfg.PublicDir,
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	return cfg
}
