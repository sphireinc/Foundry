package plugin

import (
	"fmt"
	"sort"
	"strings"

	"github.com/sphireinc/foundry/internal/commands/registry"
	"github.com/sphireinc/foundry/internal/config"
	"github.com/sphireinc/foundry/internal/consts"
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
		"foundry plugin list --installed",
		"foundry plugin list --enabled",
		"foundry plugin info <name>",
		"foundry plugin install <git-url|owner/repo> [name]",
		"foundry plugin uninstall <name>",
		"foundry plugin enable <name>",
		"foundry plugin disable <name>",
		"foundry plugin validate <name>",
		"foundry plugin deps <name>",
		"foundry plugin update <name>",
		"foundry plugin sync",
	}
}

func (command) RequiresConfig() bool {
	return true
}

func (command) Run(cfg *config.Config, args []string) error {
	if len(args) < 3 {
		return fmt.Errorf("usage: foundry plugin [list|info|install|uninstall|enable|disable|validate|deps|update|sync]")
	}

	switch args[2] {
	case "list":
		return runList(cfg, args)
	case "info":
		return runInfo(cfg, args)
	case "install":
		return runInstall(cfg, args)
	case "uninstall":
		return runUninstall(cfg, args)
	case "enable":
		return runEnable(cfg, args)
	case "disable":
		return runDisable(cfg, args)
	case "validate":
		return runValidate(cfg, args)
	case "deps":
		return runDeps(cfg, args)
	case "update":
		return runUpdate(cfg, args)
	case "sync":
		err := plugins.SyncFromConfig(plugins.SyncOptions{
			ConfigPath: consts.ConfigFilePath,
			PluginsDir: cfg.PluginsDir,
			OutputPath: consts.GeneratedPluginsFile,
			ModulePath: plugins.DefaultSyncModulePath,
		})
		if err != nil {
			return err
		}
		fmt.Println("plugin imports synced")
		return nil
	}

	return fmt.Errorf("unknown plugin subcommand: %s", args[2])
}

func runList(cfg *config.Config, args []string) error {
	mode := "--enabled"
	if len(args) >= 4 {
		mode = args[3]
	}

	switch mode {
	case "--enabled":
		pm, err := plugins.NewManager(cfg.PluginsDir, cfg.Plugins.Enabled)
		if err != nil {
			return err
		}
		return printMetadataTable(pm.MetadataList())

	case "--installed":
		metas, err := plugins.ListInstalled(cfg.PluginsDir)
		if err != nil {
			return err
		}
		return printMetadataTable(metas)

	default:
		return fmt.Errorf("usage: foundry plugin list [--installed|--enabled]")
	}
}

func runInfo(cfg *config.Config, args []string) error {
	if len(args) < 4 {
		return fmt.Errorf("usage: foundry plugin info <name>")
	}

	meta, err := plugins.LoadMetadata(cfg.PluginsDir, args[3])
	if err != nil {
		return err
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
}

func runInstall(cfg *config.Config, args []string) error {
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
				fmt.Printf("Add %q to %s under plugins.enabled\n", dep.Name, consts.ConfigFilePath)
			} else {
				fmt.Printf("foundry plugin install %s\n", dep.Repo)
			}
		}
	}

	fmt.Println("")
	fmt.Println("Next steps:")
	if len(missing) > 0 {
		fmt.Println("1. Resolve the dependency warnings above")
		fmt.Printf("2. Add %q to %s under plugins.enabled\n", meta.Name, consts.ConfigFilePath)
		fmt.Println("3. Run foundry plugin sync")
		fmt.Println("4. Run foundry build or foundry serve")
	} else {
		fmt.Printf("1. Add %q to %s under plugins.enabled\n", meta.Name, consts.ConfigFilePath)
		fmt.Println("2. Run foundry plugin sync")
		fmt.Println("3. Run foundry build or foundry serve")
	}

	return nil
}

func runUninstall(cfg *config.Config, args []string) error {
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
	fmt.Printf("1. Remove %q from %s under plugins.enabled\n", name, consts.ConfigFilePath)
	fmt.Println("2. Run foundry plugin sync")
	fmt.Println("3. Run foundry build or foundry serve")

	return nil
}

func runEnable(_ *config.Config, args []string) error {
	if len(args) < 4 {
		return fmt.Errorf("usage: foundry plugin enable <name>")
	}

	name := strings.TrimSpace(args[3])
	if err := plugins.EnableInConfig(consts.ConfigFilePath, name); err != nil {
		return err
	}

	fmt.Printf("Enabled plugin: %s\n", name)
	fmt.Println("Next steps:")
	fmt.Println("1. Run foundry plugin sync")
	fmt.Println("2. Run foundry build or foundry serve")
	return nil
}

func runDisable(_ *config.Config, args []string) error {
	if len(args) < 4 {
		return fmt.Errorf("usage: foundry plugin disable <name>")
	}

	name := strings.TrimSpace(args[3])
	if err := plugins.DisableInConfig(consts.ConfigFilePath, name); err != nil {
		return err
	}

	fmt.Printf("Disabled plugin: %s\n", name)
	fmt.Println("Next steps:")
	fmt.Println("1. Run foundry plugin sync")
	fmt.Println("2. Run foundry build or foundry serve")
	return nil
}

func runValidate(cfg *config.Config, args []string) error {
	if len(args) < 4 {
		return fmt.Errorf("usage: foundry plugin validate <name>")
	}

	name := strings.TrimSpace(args[3])
	if err := plugins.ValidateInstalledPlugin(cfg.PluginsDir, name); err != nil {
		return err
	}

	fmt.Printf("Plugin %q is valid\n", name)
	return nil
}

func runDeps(cfg *config.Config, args []string) error {
	if len(args) < 4 {
		return fmt.Errorf("usage: foundry plugin deps <name>")
	}

	name := strings.TrimSpace(args[3])
	meta, err := plugins.LoadMetadata(cfg.PluginsDir, name)
	if err != nil {
		return err
	}

	if len(meta.Requires) == 0 {
		fmt.Printf("Plugin %q has no declared dependencies\n", name)
		return nil
	}

	enabled, err := plugins.LoadAllMetadata(cfg.PluginsDir, cfg.Plugins.Enabled)
	if err != nil {
		return err
	}

	enabledRepos := map[string]string{}
	for pluginName, m := range enabled {
		if strings.TrimSpace(m.Repo) != "" {
			enabledRepos[m.Repo] = pluginName
		}
	}

	installed, err := plugins.ListInstalled(cfg.PluginsDir)
	if err != nil {
		return err
	}
	installedRepos := map[string]string{}
	for _, m := range installed {
		if strings.TrimSpace(m.Repo) != "" {
			installedRepos[m.Repo] = m.Name
		}
	}

	fmt.Printf("Dependencies for %q:\n", name)
	for _, dep := range meta.Requires {
		switch {
		case enabledRepos[dep] != "":
			fmt.Printf("- %s  [enabled as %s]\n", dep, enabledRepos[dep])
		case installedRepos[dep] != "":
			fmt.Printf("- %s  [installed as %s, not enabled]\n", dep, installedRepos[dep])
		default:
			fmt.Printf("- %s  [missing]\n", dep)
		}
	}

	return nil
}

func runUpdate(cfg *config.Config, args []string) error {
	if len(args) < 4 {
		return fmt.Errorf("usage: foundry plugin update <name>")
	}

	name := strings.TrimSpace(args[3])
	meta, err := plugins.UpdateInstalled(cfg.PluginsDir, name)
	if err != nil {
		return err
	}

	fmt.Printf("Updated plugin: %s\n", meta.Name)
	fmt.Printf("Directory: %s\n", meta.Directory)
	fmt.Printf("Version: %s\n", meta.Version)
	return nil
}

func printMetadataTable(metas []plugins.Metadata) error {
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
	metas, err := plugins.ListInstalled(pluginsDir)
	if err != nil {
		return nil, err
	}

	out := make(map[string]string)
	for _, meta := range metas {
		repo := strings.TrimSpace(meta.Repo)
		if repo == "" {
			continue
		}
		out[repo] = meta.Name
	}

	return out, nil
}

func init() {
	registry.Register(command{})
}
