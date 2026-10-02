package themecmd

import (
	"github.com/sphireinc/foundry/internal/config"
	"github.com/sphireinc/foundry/internal/theme"
	"os"
	"path/filepath"
	"testing"
)

func TestThemeInitReusesValidatedScaffold(t *testing.T) {
	cfg := &config.Config{ThemesDir: filepath.Join(t.TempDir(), "themes"), Theme: "existing"}
	if err := (command{}).Run(cfg, []string{"foundry", "theme", "init", "author-theme"}); err != nil {
		t.Fatal(err)
	}
	if cfg.Theme != "existing" {
		t.Fatal("init changed active theme")
	}
	if err := theme.ValidateInstalled(cfg.ThemesDir, "author-theme"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(cfg.ThemesDir, "author-theme", "README.md")); err != nil {
		t.Fatal(err)
	}
	if err := (command{}).Run(cfg, []string{"foundry", "theme", "init", "author-theme"}); err == nil {
		t.Fatal("existing theme overwritten")
	}
	if err := os.Remove(filepath.Join(cfg.ThemesDir, "author-theme", "layouts", "page.html")); err != nil {
		t.Fatal(err)
	}
	result, err := theme.ValidateInstalledDetailed(cfg.ThemesDir, "author-theme")
	if err != nil {
		t.Fatal(err)
	}
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Message == "missing required theme layout" && diagnostic.Hint != "" {
			return
		}
	}
	t.Fatal("missing layout did not provide remediation")
}
