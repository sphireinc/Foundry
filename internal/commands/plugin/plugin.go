package plugin

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/sphireinc/foundry/internal/commands/registry"
	"github.com/sphireinc/foundry/internal/config"
	"github.com/sphireinc/foundry/internal/plugins"
)

type command struct{}

func (command) Name() string {
	return "plugin"
}

func (command) Summary() string {
	return "Manage plugins"
}

func (command) Group() string {
	return "plugin commands"
}

func (command) Details() []string {
	return []string{
		"foundry plugin list",
		"foundry plugin info <name>",
		"foundry plugin install <git-url|owner/repo> [name]",
		"foundry plugin uninstall <name>",
		"foundry plugin sync",
	}
}

func (command) Run(cfg *config.Config, args []string) error {
	if len(args) < 3 {
		return fmt.Errorf("usage: foundry plugin [list|info <name>|install <git-url|owner/repo> [name]|uninstall <name>]")
	}

	switch args[2] {
	case "list":
		pm, err := plugins.NewManager(cfg.PluginsDir, cfg.Plugins.Enabled)
		if err != nil {
			return err
		}

		metas := pm.MetadataList()
		nameWidth := len("NAME")
		versionWidth := len("VERSION")

		for _, meta := range metas {
			if len(meta.Name) > nameWidth {
				nameWidth = len(meta.Name)
			}
			if len(meta.Version) > versionWidth {
				versionWidth = len(meta.Version)
			}
		}

		fmt.Printf("%-*s  %-*s  %s\n", nameWidth, "NAME", versionWidth, "VERSION", "TITLE")
		for _, meta := range metas {
			fmt.Printf("%-*s  %-*s  %s\n", nameWidth, meta.Name, versionWidth, meta.Version, meta.Title)
		}
		return nil

	case "info":
		if len(args) < 4 {
			return fmt.Errorf("usage: foundry plugin info <name>")
		}

		pm, err := plugins.NewManager(cfg.PluginsDir, cfg.Plugins.Enabled)
		if err != nil {
			return err
		}

		meta, ok := pm.MetadataFor(args[3])
		if !ok {
			return fmt.Errorf("plugin not found: %s", args[3])
		}

		fmt.Printf("Name:        %s\n", meta.Name)
		fmt.Printf("Title:       %s\n", meta.Title)
		fmt.Printf("Version:     %s\n", meta.Version)
		fmt.Printf("Description: %s\n", meta.Description)
		fmt.Printf("Author:      %s\n", meta.Author)
		fmt.Printf("Homepage:    %s\n", meta.Homepage)
		fmt.Printf("License:     %s\n", meta.License)
		fmt.Printf("Directory:   %s\n", meta.Directory)
		fmt.Printf("Repo:        %s\n", meta.Repo)
		if len(meta.Requires) > 0 {
			fmt.Println("Requires:")
			for _, dep := range meta.Requires {
				fmt.Printf("  - %s\n", dep)
			}
		}
		return nil

	case "install":
		if len(args) < 4 {
			return fmt.Errorf("usage: foundry plugin install <git-url|owner/repo> [name]")
		}

		repoURL := strings.TrimSpace(args[3])
		name := ""
		if len(args) >= 5 {
			name = strings.TrimSpace(args[4])
		}

		meta, err := plugins.Install(plugins.InstallOptions{
			PluginsDir: cfg.PluginsDir,
			URL:        repoURL,
			Name:       name,
		})
		if err != nil {
			return err
		}

		fmt.Printf("Installed plugin: %s\n", meta.Name)
		fmt.Printf("Directory: %s\n", meta.Directory)
		fmt.Printf("Version: %s\n", meta.Version)

		missing, warnErr := findMissingPluginDependencies(cfg, meta)
		if warnErr != nil {
			return warnErr
		}

		if len(missing) > 0 {
			fmt.Println("")
			fmt.Println("Dependency warnings:")
			for _, dep := range missing {
				if dep.Installed {
					fmt.Printf("- Required plugin repo %s is installed as %q but not enabled\n", dep.Repo, dep.Name)
				} else {
					fmt.Printf("- Missing required plugin repo: %s\n", dep.Repo)
				}
			}

			fmt.Println("")
			fmt.Println("Suggested next steps:")
			for _, dep := range missing {
				if dep.Installed {
					fmt.Printf("Add %q to content/config/site.yaml under plugins.enabled\n", dep.Name)
				} else {
					fmt.Printf("foundry plugin install %s\n", dep.Repo)
				}
			}
		}

		fmt.Println("")
		fmt.Println("Next steps:")
		if len(missing) > 0 {
			fmt.Println("1. Resolve the dependency warnings above")
			fmt.Printf("2. Add %q to content/config/site.yaml under plugins.enabled\n", meta.Name)
			fmt.Println("3. Run make plugins-sync")
			fmt.Println("4. Run make dev or make build")
		} else {
			fmt.Printf("1. Add %q to content/config/site.yaml under plugins.enabled\n", meta.Name)
			fmt.Println("2. Run make plugins-sync")
			fmt.Println("3. Run make dev or make build")
		}

		return nil

	case "sync":
		if err := plugins.SyncFromConfig(plugins.SyncOptions{
			ConfigPath: plugins.DefaultSyncConfigPath,
			PluginsDir: cfg.PluginsDir,
			OutputPath: plugins.DefaultSyncOutputPath,
			ModulePath: plugins.DefaultSyncModulePath,
		}); err != nil {
			return err
		}

		fmt.Println("plugin imports synced")
		return nil

	case "uninstall":
		if len(args) < 4 {
			return fmt.Errorf("usage: foundry plugin uninstall <name>")
		}

		name := strings.TrimSpace(args[3])
		if err := plugins.Uninstall(cfg.PluginsDir, name); err != nil {
			return err
		}

		fmt.Printf("Uninstalled plugin: %s\n", name)
		fmt.Println("")
		fmt.Println("Next steps:")
		fmt.Printf("1. Remove %q from content/config/site.yaml under plugins.enabled\n", name)
		fmt.Println("2. Run make plugins-sync")
		fmt.Println("3. Run make dev or make build")
		return nil
	}

	return fmt.Errorf("unknown plugin subcommand: %s", args[2])
}

type MissingDependency struct {
	Repo      string
	Installed bool
	Name      string
}

func findMissingPluginDependencies(cfg *config.Config, installed plugins.Metadata) ([]MissingDependency, error) {
	if len(installed.Requires) == 0 {
		return nil, nil
	}

	enabledMetadata, err := plugins.LoadAllMetadata(cfg.PluginsDir, cfg.Plugins.Enabled)
	if err != nil {
		return nil, err
	}

	enabledRepos := make(map[string]string, len(enabledMetadata))
	for name, meta := range enabledMetadata {
		repo := strings.TrimSpace(meta.Repo)
		if repo == "" {
			continue
		}
		enabledRepos[repo] = name
	}

	installedOnDisk, err := scanInstalledPluginRepos(cfg.PluginsDir)
	if err != nil {
		return nil, err
	}

	missing := make([]MissingDependency, 0)
	seen := make(map[string]struct{})

	for _, dep := range installed.Requires {
		dep = strings.TrimSpace(dep)
		if dep == "" {
			continue
		}
		if _, ok := enabledRepos[dep]; ok {
			continue
		}
		if _, dup := seen[dep]; dup {
			continue
		}
		seen[dep] = struct{}{}

		md := MissingDependency{Repo: dep}
		if name, ok := installedOnDisk[dep]; ok {
			md.Installed = true
			md.Name = name
		}

		missing = append(missing, md)
	}

	sort.Slice(missing, func(i, j int) bool {
		return missing[i].Repo < missing[j].Repo
	})

	return missing, nil
}

func scanInstalledPluginRepos(pluginsDir string) (map[string]string, error) {
	entries, err := os.ReadDir(pluginsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]string{}, nil
		}
		return nil, err
	}

	out := make(map[string]string)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		name := entry.Name()
		meta, err := plugins.LoadMetadata(pluginsDir, name)
		if err != nil {
			return nil, err
		}

		repo := strings.TrimSpace(meta.Repo)
		if repo == "" {
			continue
		}

		out[repo] = name
	}

	return out, nil
}

func init() {
	registry.Register(command{})
}
