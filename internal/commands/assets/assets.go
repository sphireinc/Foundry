package assetscmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sphireinc/foundry/internal/assets"
	"github.com/sphireinc/foundry/internal/commands/registry"
	"github.com/sphireinc/foundry/internal/config"
)

type command struct{}

func (command) Name() string {
	return "assets"
}

func (command) Summary() string {
	return "Build and inspect assets"
}

func (command) Group() string {
	return "asset commands"
}

func (command) Details() []string {
	return []string{
		"foundry assets build",
		"foundry assets clean",
		"foundry assets list",
	}
}

func (command) RequiresConfig() bool {
	return true
}

func (command) Run(cfg *config.Config, args []string) error {
	if len(args) < 3 {
		return fmt.Errorf("usage: foundry assets [build|clean|list]")
	}

	switch args[2] {
	case "build":
		if err := assets.Sync(cfg, nil); err != nil {
			return err
		}
		fmt.Println("assets built")
		return nil

	case "clean":
		targets := []string{
			filepath.Join(cfg.PublicDir, "assets"),
			filepath.Join(cfg.PublicDir, "images"),
			filepath.Join(cfg.PublicDir, "uploads"),
			filepath.Join(cfg.PublicDir, "theme"),
			filepath.Join(cfg.PublicDir, "plugins"),
		}

		for _, target := range targets {
			if _, err := os.Stat(target); err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return err
			}
			if err := os.RemoveAll(target); err != nil {
				return err
			}
			fmt.Printf("removed %s\n", target)
		}
		return nil

	case "list":
		return listAssets(cfg)
	}

	return fmt.Errorf("unknown assets subcommand: %s", args[2])
}

func listAssets(cfg *config.Config) error {
	type row struct {
		Kind string
		Path string
	}

	rows := make([]row, 0)

	addFiles := func(kind, root string) error {
		info, err := os.Stat(root)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if !info.IsDir() {
			return nil
		}

		return filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if info.IsDir() {
				return nil
			}
			rows = append(rows, row{
				Kind: kind,
				Path: filepath.ToSlash(path),
			})
			return nil
		})
	}

	if err := addFiles("content-assets", filepath.Join(cfg.ContentDir, cfg.Content.AssetsDir)); err != nil {
		return err
	}
	if err := addFiles("content-images", filepath.Join(cfg.ContentDir, cfg.Content.ImagesDir)); err != nil {
		return err
	}
	if err := addFiles("content-uploads", filepath.Join(cfg.ContentDir, cfg.Content.UploadsDir)); err != nil {
		return err
	}
	if err := addFiles("theme-assets", filepath.Join(cfg.ThemesDir, cfg.Theme, "assets")); err != nil {
		return err
	}

	for _, pluginName := range cfg.Plugins.Enabled {
		pluginName = strings.TrimSpace(pluginName)
		if pluginName == "" {
			continue
		}
		if err := addFiles("plugin-assets", filepath.Join(cfg.PluginsDir, pluginName, "assets")); err != nil {
			return err
		}
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Kind != rows[j].Kind {
			return rows[i].Kind < rows[j].Kind
		}
		return rows[i].Path < rows[j].Path
	})

	if len(rows) == 0 {
		fmt.Println("no assets found")
		return nil
	}

	kindWidth := len("KIND")
	for _, row := range rows {
		if len(row.Kind) > kindWidth {
			kindWidth = len(row.Kind)
		}
	}

	fmt.Printf("%-*s  %s\n", kindWidth, "KIND", "PATH")
	for _, row := range rows {
		fmt.Printf("%-*s  %s\n", kindWidth, row.Kind, row.Path)
	}

	fmt.Println("")
	fmt.Printf("%d asset file(s)\n", len(rows))
	return nil
}

func init() {
	registry.Register(command{})
}
