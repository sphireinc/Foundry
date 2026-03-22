package plugin

import (
	"fmt"
	"sort"
	"strings"

	"github.com/sphireinc/foundry/internal/cliout"
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
		"foundry plugin validate",
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

	project := plugins.NewProject(
		consts.ConfigFilePath,
		cfg.PluginsDir,
		consts.GeneratedPluginsFile,
		plugins.DefaultSyncModulePath,
	)

	switch args[2] {
	case "list":
		return runList(cfg, project, args)
	case "info":
		return runInfo(project, args)
	case "install":
		return runInstall(cfg, project, args)
	case "uninstall":
		return runUninstall(project, args)
	case "enable":
		return runEnable(project, args)
	case "disable":
		return runDisable(project, args)
	case "validate":
		return runValidate(cfg, project, args)
	case "deps":
		return runDeps(cfg, project, args)
	case "update":
		return runUpdate(project, args)
	case "sync":
		if err := project.Sync(); err != nil {
			return err
		}
		cliout.Successf("plugin imports synced")
		return nil
	}

	return fmt.Errorf("unknown plugin subcommand: %s", args[2])
}

func runList(cfg *config.Config, project plugins.Project, args []string) error {
	mode := "--enabled"
	if len(args) >= 4 {
		mode = args[3]
	}

	switch mode {
	case "--enabled":
		return printEnabledPluginTable(cfg, project)
	case "--installed":
		metas, err := project.ListInstalled()
		if err != nil {
			return err
		}
		return printInstalledPluginTable(metas)
	default:
		return fmt.Errorf("usage: foundry plugin list [--installed|--enabled]")
	}
}

func runInfo(project plugins.Project, args []string) error {
	if len(args) < 4 {
		return fmt.Errorf("usage: foundry plugin info <name>")
	}

	meta, err := project.Metadata(args[3])
	if err != nil {
		return err
	}

	fmt.Printf("Name:        %s\n", meta.Name)
	fmt.Printf("Title:       %s\n", meta.Title)
	fmt.Printf("Version:     %s\n", meta.Version)
	fmt.Printf("Foundry API:  %s\n", meta.FoundryAPI)
	fmt.Printf("Min Foundry:  %s\n", meta.MinFoundryVersion)
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

func runInstall(cfg *config.Config, project plugins.Project, args []string) error {
	if len(args) < 4 {
		return fmt.Errorf("usage: foundry plugin install <git-url|owner/repo> [name]")
	}

	repoURL := strings.TrimSpace(args[3])
	name := ""
	if len(args) >= 5 {
		name = strings.TrimSpace(args[4])
	}

	meta, err := project.Install(repoURL, name)
	if err != nil {
		return err
	}

	cliout.Successf("Installed plugin: %s", meta.Name)
	fmt.Printf("%s %s\n", cliout.Label("Directory:"), meta.Directory)
	fmt.Printf("%s %s\n", cliout.Label("Version:"), meta.Version)

	missing, warnErr := project.MissingDependencies(meta, cfg.Plugins.Enabled)
	if warnErr != nil {
		return warnErr
	}

	if len(missing) > 0 {
		fmt.Println("")
		cliout.Println(cliout.Warning("Dependency warnings:"))
		for _, dep := range missing {
			if dep.Installed {
				fmt.Printf("- Required plugin repo %s is installed as %q but not enabled\n", dep.Repo, dep.Name)
			} else {
				fmt.Printf("- Missing required plugin repo: %s\n", dep.Repo)
			}
		}

		fmt.Println("")
		cliout.Println(cliout.Heading("Suggested next steps:"))
		for _, dep := range missing {
			if dep.Installed {
				fmt.Printf("Add %q to %s under plugins.enabled\n", dep.Name, consts.ConfigFilePath)
			} else {
				fmt.Printf("foundry plugin install %s\n", dep.Repo)
			}
		}
	}

	fmt.Println("")
	cliout.Println(cliout.Heading("Next steps:"))
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

func runUninstall(project plugins.Project, args []string) error {
	if len(args) < 4 {
		return fmt.Errorf("usage: foundry plugin uninstall <name>")
	}

	name := strings.TrimSpace(args[3])
	if err := project.Uninstall(name); err != nil {
		return err
	}

	cliout.Successf("Uninstalled plugin: %s", name)
	fmt.Println("")
	cliout.Println(cliout.Heading("Next steps:"))
	fmt.Printf("1. Remove %q from %s under plugins.enabled\n", name, consts.ConfigFilePath)
	fmt.Println("2. Run foundry plugin sync")
	fmt.Println("3. Run foundry build or foundry serve")

	return nil
}

func runEnable(project plugins.Project, args []string) error {
	if len(args) < 4 {
		return fmt.Errorf("usage: foundry plugin enable <name>")
	}

	name := strings.TrimSpace(args[3])
	if err := project.Enable(name); err != nil {
		return err
	}

	cliout.Successf("Enabled plugin: %s", name)
	cliout.Println(cliout.Heading("Next steps:"))
	fmt.Println("1. Run foundry plugin sync")
	fmt.Println("2. Run foundry build or foundry serve")
	return nil
}

func runDisable(project plugins.Project, args []string) error {
	if len(args) < 4 {
		return fmt.Errorf("usage: foundry plugin disable <name>")
	}

	name := strings.TrimSpace(args[3])
	if err := project.Disable(name); err != nil {
		return err
	}

	cliout.Successf("Disabled plugin: %s", name)
	cliout.Println(cliout.Heading("Next steps:"))
	fmt.Println("1. Run foundry plugin sync")
	fmt.Println("2. Run foundry build or foundry serve")
	return nil
}

func runValidate(cfg *config.Config, project plugins.Project, args []string) error {
	if len(args) >= 4 {
		name := strings.TrimSpace(args[3])
		if err := project.Validate(name); err != nil {
			return err
		}
		cliout.Println(cliout.Heading("Plugin validation"))
		fmt.Println("")
		fmt.Println("Legend:")
		fmt.Printf("  %s    valid and loadable\n", cliout.OK("OK"))
		fmt.Printf("  %s  invalid or not loadable\n", cliout.Fail("FAIL"))
		fmt.Println("")
		fmt.Printf("[%s]   %s\n", cliout.OK("OK"), name)
		return nil
	}

	report := project.ValidateEnabled(cfg.Plugins.Enabled)

	cliout.Println(cliout.Heading("Plugin validation"))
	fmt.Println("")
	fmt.Println("Legend:")
	fmt.Printf("  %s    valid and loadable\n", cliout.OK("OK"))
	fmt.Printf("  %s  invalid or not loadable\n", cliout.Fail("FAIL"))
	fmt.Println("")

	for _, name := range report.Passed {
		fmt.Printf("[%s]   %s\n", cliout.OK("OK"), name)
	}
	for _, issue := range report.Issues {
		fmt.Printf("[%s] %s\n", cliout.Fail("FAIL"), issue.String())
	}

	if len(report.Issues) == 0 {
		fmt.Printf("\n%s %d enabled plugin(s) are valid\n", cliout.OK("All"), len(report.Passed))
		return nil
	}

	return fmt.Errorf("plugin validation failed with %d issue(s)", len(report.Issues))
}

func runDeps(cfg *config.Config, project plugins.Project, args []string) error {
	if len(args) < 4 {
		return fmt.Errorf("usage: foundry plugin deps <name>")
	}

	name := strings.TrimSpace(args[3])
	statuses, err := project.DependencyStatuses(name, cfg.Plugins.Enabled)
	if err != nil {
		return err
	}

	if len(statuses) == 0 {
		fmt.Printf("Plugin %q has no declared dependencies\n", name)
		return nil
	}

	fmt.Printf("Dependencies for %q:\n", name)
	for _, dep := range statuses {
		switch dep.Status {
		case "enabled":
			fmt.Printf("- %s  [enabled as %s]\n", dep.Repo, dep.Name)
		case "installed":
			fmt.Printf("- %s  [installed as %s, not enabled]\n", dep.Repo, dep.Name)
		default:
			fmt.Printf("- %s  [missing]\n", dep.Repo)
		}
	}

	return nil
}

func runUpdate(project plugins.Project, args []string) error {
	if len(args) < 4 {
		return fmt.Errorf("usage: foundry plugin update <name>")
	}

	name := strings.TrimSpace(args[3])
	meta, err := project.Update(name)
	if err != nil {
		return err
	}

	fmt.Printf("Updated plugin: %s\n", meta.Name)
	fmt.Printf("Directory: %s\n", meta.Directory)
	fmt.Printf("Version: %s\n", meta.Version)
	return nil
}

func printEnabledPluginTable(cfg *config.Config, project plugins.Project) error {
	names := make([]string, 0, len(cfg.Plugins.Enabled))
	seen := make(map[string]struct{}, len(cfg.Plugins.Enabled))

	for _, name := range cfg.Plugins.Enabled {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}

	sort.Strings(names)

	statuses := project.EnabledStatuses(names)

	type row struct {
		Name    string
		Status  string
		Version string
		Title   string
	}

	rows := make([]row, 0, len(names))
	nameWidth := len("NAME")
	statusWidth := len("STATUS")
	versionWidth := len("VERSION")

	for _, name := range names {
		status := statuses[name]
		if status == "" {
			status = "enabled"
		}

		title := "-"
		version := "-"

		meta, err := project.Metadata(name)
		if err == nil {
			if strings.TrimSpace(meta.Title) != "" {
				title = meta.Title
			}
			if strings.TrimSpace(meta.Version) != "" {
				version = meta.Version
			}
		}

		rows = append(rows, row{
			Name:    name,
			Status:  status,
			Version: version,
			Title:   title,
		})

		if len(name) > nameWidth {
			nameWidth = len(name)
		}
		if len(status) > statusWidth {
			statusWidth = len(status)
		}
		if len(version) > versionWidth {
			versionWidth = len(version)
		}
	}

	fmt.Printf("%-*s  %-*s  %-*s  %s\n", nameWidth, "NAME", statusWidth, "STATUS", versionWidth, "VERSION", "TITLE")
	for _, row := range rows {
		fmt.Printf("%-*s  %-*s  %-*s  %s\n", nameWidth, row.Name, statusWidth, row.Status, versionWidth, row.Version, row.Title)
	}

	return nil
}

func printInstalledPluginTable(metas []plugins.Metadata) error {
	nameWidth := len("NAME")
	versionWidth := len("VERSION")
	apiWidth := len("API")

	for _, meta := range metas {
		if len(meta.Name) > nameWidth {
			nameWidth = len(meta.Name)
		}
		if len(meta.Version) > versionWidth {
			versionWidth = len(meta.Version)
		}
		if len(meta.FoundryAPI) > apiWidth {
			apiWidth = len(meta.FoundryAPI)
		}
	}

	fmt.Printf("%-*s  %-*s  %-*s  %s\n", nameWidth, "NAME", versionWidth, "VERSION", apiWidth, "API", "TITLE")
	for _, meta := range metas {
		fmt.Printf("%-*s  %-*s  %-*s  %s\n", nameWidth, meta.Name, versionWidth, meta.Version, apiWidth, meta.FoundryAPI, meta.Title)
	}

	return nil
}

func init() {
	registry.Register(command{})
}
