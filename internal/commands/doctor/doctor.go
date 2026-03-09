package doctor

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/sphireinc/foundry/internal/commands/registry"
	foundryconfig "github.com/sphireinc/foundry/internal/config"
	"github.com/sphireinc/foundry/internal/plugins"
	"github.com/sphireinc/foundry/internal/theme"
)

type command struct{}

func (command) Name() string {
	return "doctor"
}

func (command) Summary() string {
	return "Check project and environment health"
}

func (command) Group() string {
	return "core commands"
}

func (command) Details() []string {
	return nil
}

func (command) RequiresConfig() bool {
	return true
}

func (command) Run(cfg *foundryconfig.Config, _ []string) error {
	type result struct {
		label string
		ok    bool
		msg   string
	}

	results := make([]result, 0)
	failures := 0

	add := func(label string, ok bool, msg string) {
		results = append(results, result{
			label: label,
			ok:    ok,
			msg:   msg,
		})
		if !ok {
			failures++
		}
	}

	if errs := foundryconfig.Validate(cfg); len(errs) == 0 {
		add("config", true, "valid")
	} else {
		add("config", false, fmt.Sprintf("%d validation error(s)", len(errs)))
	}

	checkDir := func(label, path string) {
		info, err := os.Stat(path)
		if err != nil {
			if os.IsNotExist(err) {
				add(label, false, fmt.Sprintf("%s does not exist", path))
				return
			}
			add(label, false, err.Error())
			return
		}
		if !info.IsDir() {
			add(label, false, fmt.Sprintf("%s is not a directory", path))
			return
		}
		add(label, true, path)
	}

	checkDir("content_dir", cfg.ContentDir)
	checkDir("themes_dir", cfg.ThemesDir)
	checkDir("data_dir", cfg.DataDir)
	checkDir("plugins_dir", cfg.PluginsDir)

	if err := theme.NewManager(cfg.ThemesDir, cfg.Theme).MustExist(); err != nil {
		add("theme", false, err.Error())
	} else {
		add("theme", true, cfg.Theme)
	}

	if _, err := plugins.NewManager(cfg.PluginsDir, cfg.Plugins.Enabled); err != nil {
		add("plugins", false, err.Error())
	} else {
		add("plugins", true, fmt.Sprintf("%d enabled", len(cfg.Plugins.Enabled)))
	}

	genPath := filepath.Join("internal", "generated", "plugins_gen.go")
	if _, err := os.Stat(genPath); err != nil {
		if os.IsNotExist(err) {
			add("plugin_sync", false, genPath+" not found")
		} else {
			add("plugin_sync", false, err.Error())
		}
	} else {
		add("plugin_sync", true, genPath)
	}

	for _, r := range results {
		status := "OK"
		if !r.ok {
			status = "FAIL"
		}
		fmt.Printf("[%s] %-14s %s\n", status, r.label, r.msg)
	}

	if failures > 0 {
		return fmt.Errorf("doctor found %d problem(s)", failures)
	}

	fmt.Println("doctor OK")
	return nil
}

func init() {
	registry.Register(command{})
}
