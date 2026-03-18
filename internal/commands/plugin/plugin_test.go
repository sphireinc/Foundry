package plugin

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sphireinc/foundry/internal/config"
	"github.com/sphireinc/foundry/internal/plugins"
)

func TestPluginCommandRun(t *testing.T) {
	root := t.TempDir()
	cfg := &config.Config{
		PluginsDir: filepath.Join(root, "plugins"),
		ContentDir: filepath.Join(root, "content"),
	}
	cfg.ApplyDefaults()

	writePluginFixture(t, cfg.PluginsDir, "alpha", "github.com/acme/alpha", nil)
	writePluginFixture(t, cfg.PluginsDir, "beta", "github.com/acme/beta", []string{"github.com/acme/alpha"})
	writeConfigFile(t, root, "plugins:\n  enabled:\n    - alpha\n")
	if err := os.MkdirAll(filepath.Join(root, "internal", "generated"), 0o755); err != nil {
		t.Fatalf("mkdir generated: %v", err)
	}

	chdirTempRoot(t, root)

	cmd := command{}
	if err := cmd.Run(cfg, []string{"foundry", "plugin", "list", "--enabled"}); err != nil {
		t.Fatalf("plugin list enabled: %v", err)
	}
	if err := cmd.Run(cfg, []string{"foundry", "plugin", "list", "--installed"}); err != nil {
		t.Fatalf("plugin list installed: %v", err)
	}
	if err := cmd.Run(cfg, []string{"foundry", "plugin", "info", "alpha"}); err != nil {
		t.Fatalf("plugin info: %v", err)
	}
	if err := cmd.Run(cfg, []string{"foundry", "plugin", "validate"}); err != nil {
		t.Fatalf("plugin validate: %v", err)
	}
	if err := cmd.Run(cfg, []string{"foundry", "plugin", "validate", "alpha"}); err != nil {
		t.Fatalf("plugin validate single: %v", err)
	}
	if err := cmd.Run(cfg, []string{"foundry", "plugin", "deps", "beta"}); err != nil {
		t.Fatalf("plugin deps: %v", err)
	}
	if err := cmd.Run(cfg, []string{"foundry", "plugin", "enable", "beta"}); err != nil {
		t.Fatalf("plugin enable: %v", err)
	}
	if err := cmd.Run(cfg, []string{"foundry", "plugin", "disable", "beta"}); err != nil {
		t.Fatalf("plugin disable: %v", err)
	}
	if err := cmd.Run(cfg, []string{"foundry", "plugin", "sync"}); err != nil {
		t.Fatalf("plugin sync: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "internal", "generated", "plugins_gen.go")); err != nil {
		t.Fatalf("expected synced imports file: %v", err)
	}
	if err := cmd.Run(cfg, []string{"foundry", "plugin"}); err == nil {
		t.Fatal("expected usage error")
	}
	if err := cmd.Run(cfg, []string{"foundry", "plugin", "bad"}); err == nil {
		t.Fatal("expected unknown subcommand error")
	}
}

func TestPluginTableHelpers(t *testing.T) {
	metas := []plugins.Metadata{
		{Name: "a", Version: "1.0.0", FoundryAPI: "v1", Title: "A"},
	}
	if err := printInstalledPluginTable(metas); err != nil {
		t.Fatalf("print installed plugin table: %v", err)
	}
}

func TestPluginCommandMetadataAndUsageHelpers(t *testing.T) {
	cmd := command{}
	if cmd.Name() != "plugin" {
		t.Fatalf("unexpected command name: %q", cmd.Name())
	}
	if cmd.Summary() == "" || cmd.Group() == "" || !cmd.RequiresConfig() || len(cmd.Details()) == 0 {
		t.Fatal("expected populated command metadata")
	}

	project := plugins.NewProject("config.yaml", t.TempDir(), "out.go", "example/module")
	if err := runList(&config.Config{}, project, []string{"foundry", "plugin", "list", "--bad"}); err == nil {
		t.Fatal("expected list usage error")
	}
	if err := runInfo(project, []string{"foundry", "plugin", "info"}); err == nil {
		t.Fatal("expected info usage error")
	}
	if err := runEnable(project, []string{"foundry", "plugin", "enable"}); err == nil {
		t.Fatal("expected enable usage error")
	}
	if err := runDisable(project, []string{"foundry", "plugin", "disable"}); err == nil {
		t.Fatal("expected disable usage error")
	}
	if err := runDeps(&config.Config{}, project, []string{"foundry", "plugin", "deps"}); err == nil {
		t.Fatal("expected deps usage error")
	}
	if err := runUpdate(project, []string{"foundry", "plugin", "update"}); err == nil {
		t.Fatal("expected update usage error")
	}
	if err := runValidate(&config.Config{}, project, []string{"foundry", "plugin", "validate", "missing"}); err == nil {
		t.Fatal("expected validate failure for missing plugin")
	}
}

func TestPluginDepsNoDeclaredDependencies(t *testing.T) {
	root := t.TempDir()
	cfg := &config.Config{
		PluginsDir: filepath.Join(root, "plugins"),
	}
	cfg.ApplyDefaults()
	writePluginFixture(t, cfg.PluginsDir, "alpha", "github.com/acme/alpha", nil)

	project := plugins.NewProject("config.yaml", cfg.PluginsDir, "out.go", "example/module")
	if err := runDeps(cfg, project, []string{"foundry", "plugin", "deps", "alpha"}); err != nil {
		t.Fatalf("deps with no requirements: %v", err)
	}
}

func TestPluginCommandUninstallAndTablePaths(t *testing.T) {
	root := t.TempDir()
	cfg := &config.Config{
		PluginsDir: filepath.Join(root, "plugins"),
		ContentDir: filepath.Join(root, "content"),
	}
	cfg.ApplyDefaults()
	cfg.Plugins.Enabled = []string{"alpha", "alpha", "missing"}
	writePluginFixture(t, cfg.PluginsDir, "alpha", "github.com/acme/alpha", nil)
	writeConfigFile(t, root, "plugins:\n  enabled:\n    - alpha\n")

	project := plugins.NewProject(filepath.Join(root, "content", "config", "site.yaml"), cfg.PluginsDir, filepath.Join(root, "internal", "generated", "plugins_gen.go"), "example/module")
	if err := printEnabledPluginTable(cfg, project); err != nil {
		t.Fatalf("print enabled plugin table: %v", err)
	}
	if err := runUninstall(project, []string{"foundry", "plugin", "uninstall", "alpha"}); err != nil {
		t.Fatalf("run uninstall: %v", err)
	}
	if err := runUninstall(project, []string{"foundry", "plugin", "uninstall"}); err == nil {
		t.Fatal("expected uninstall usage error")
	}
	if err := runInstall(cfg, project, []string{"foundry", "plugin", "install"}); err == nil {
		t.Fatal("expected install usage error")
	}
	if err := runUpdate(project, []string{"foundry", "plugin", "update", "missing"}); err == nil {
		t.Fatal("expected update failure")
	}
}

func writeConfigFile(t *testing.T, root, body string) {
	t.Helper()
	path := filepath.Join(root, "content", "config", "site.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
}

func chdirTempRoot(t *testing.T, root string) {
	t.Helper()
	wd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(wd) })
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir: %v", err)
	}
}

func writePluginFixture(t *testing.T, pluginsDir, name, repo string, requires []string) {
	t.Helper()
	dir := filepath.Join(pluginsDir, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir plugin dir: %v", err)
	}
	body := "name: " + name + "\nrepo: " + repo + "\nfoundry_api: v1\nmin_foundry_version: 0.1.0\n"
	if len(requires) > 0 {
		body += "requires:\n"
		for _, dep := range requires {
			body += "  - " + dep + "\n"
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "plugin.yaml"), []byte(body), 0o644); err != nil {
		t.Fatalf("write plugin metadata: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "plugin.go"), []byte("package "+name+"\n"), 0o644); err != nil {
		t.Fatalf("write plugin code: %v", err)
	}
}
