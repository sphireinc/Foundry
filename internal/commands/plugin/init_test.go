package plugin

import (
	"github.com/sphireinc/foundry/internal/config"
	"github.com/sphireinc/foundry/internal/plugins"
	"os"
	"path/filepath"
	"testing"
)

func TestPluginInitDispatchAndValidation(t *testing.T) {
	cfg := &config.Config{PluginsDir: filepath.Join(t.TempDir(), "plugins")}
	for _, args := range [][]string{{"foundry", "plugin", "init", "rpc-starter"}, {"foundry", "plugin", "init", "compiled-starter", "--runtime", "compiled"}} {
		if err := (command{}).Run(cfg, args); err != nil {
			t.Fatal(err)
		}
		if err := plugins.ValidateInstalledPlugin(cfg.PluginsDir, args[3]); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{{"foundry", "plugin", "init"}, {"foundry", "plugin", "init", "bad", "--oops"}, {"foundry", "plugin", "init", "bad", "--runtime=invalid"}} {
		if err := (command{}).Run(cfg, args); err == nil {
			t.Fatal("invalid arguments accepted")
		}
	}
	if _, err := os.Stat(filepath.Join(cfg.PluginsDir, "bad")); !os.IsNotExist(err) {
		t.Fatal("failed init created plugin")
	}
}
