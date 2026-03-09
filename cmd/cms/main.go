package main

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	_ "github.com/sphireinc/foundry/internal/generated"

	"github.com/sphireinc/foundry/internal/config"
	"github.com/sphireinc/foundry/internal/content"
	"github.com/sphireinc/foundry/internal/plugins"
	"github.com/sphireinc/foundry/internal/renderer"
	"github.com/sphireinc/foundry/internal/router"
	"github.com/sphireinc/foundry/internal/server"
	"github.com/sphireinc/foundry/internal/theme"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cfg, err := config.Load("content/config/site.yaml")
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}

	if handled, exitCode := handlePluginInstallOrUninstall(cfg, os.Args[1:]); handled {
		os.Exit(exitCode)
	}

	pluginManager, err := plugins.NewManager(cfg.PluginsDir, cfg.Plugins.Enabled)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "load plugins: %v\n", err)
		os.Exit(1)
	}

	if err := pluginManager.OnConfigLoaded(cfg); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "plugin config hook failed: %v\n", err)
		os.Exit(1)
	}

	if handled, exitCode := handlePluginCLI(pluginManager, os.Args[1:]); handled {
		os.Exit(exitCode)
	}

	routeResolver := router.NewResolver(cfg)
	themeManager := theme.NewManager(cfg.ThemesDir, cfg.Theme)
	rendererEngine := renderer.New(cfg, themeManager, pluginManager)
	ctx := context.Background()

	switch os.Args[1] {
	case "build":
		if err := pluginManager.OnBuildStarted(); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "build start hook failed: %v\n", err)
			os.Exit(1)
		}

		loader := content.NewLoader(cfg, pluginManager, cfg.Build.IncludeDrafts)
		graph, err := loader.Load(ctx)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "load content: %v\n", err)
			os.Exit(1)
		}

		if err := routeResolver.AssignURLs(graph); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "assign urls: %v\n", err)
			os.Exit(1)
		}

		if err := pluginManager.OnRoutesAssigned(graph); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "route hook failed: %v\n", err)
			os.Exit(1)
		}

		if err := rendererEngine.Build(ctx, graph); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "build: %v\n", err)
			os.Exit(1)
		}

		if err := pluginManager.OnBuildCompleted(graph); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "build completed hook failed: %v\n", err)
			os.Exit(1)
		}

		_, _ = fmt.Println("build complete")

	case "serve":
		loader := content.NewLoader(cfg, pluginManager, false)
		srv := server.New(cfg, loader, routeResolver, rendererEngine, pluginManager, false)
		if err := srv.ListenAndServe(ctx); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "serve: %v\n", err)
			os.Exit(1)
		}

	case "serve-preview":
		loader := content.NewLoader(cfg, pluginManager, true)
		srv := server.New(cfg, loader, routeResolver, rendererEngine, pluginManager, true)
		if err := srv.ListenAndServe(ctx); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "serve preview: %v\n", err)
			os.Exit(1)
		}

	default:
		_, _ = fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func handlePluginInstallOrUninstall(cfg *config.Config, args []string) (bool, int) {
	if len(args) < 2 || args[0] != "plugin" {
		return false, 0
	}

	switch args[1] {
	case "install":
		if len(args) < 3 {
			_, _ = fmt.Fprintln(os.Stderr, "usage: cms plugin install <git-url|owner/repo> [name]")
			return true, 1
		}

		repoURL := strings.TrimSpace(args[2])
		name := ""
		if len(args) >= 4 {
			name = strings.TrimSpace(args[3])
		}

		meta, err := plugins.Install(plugins.InstallOptions{
			PluginsDir: cfg.PluginsDir,
			URL:        repoURL,
			Name:       name,
		})
		if err != nil {
			_, _ = fmt.Fprintln(os.Stderr, err)
			return true, 1
		}

		_, _ = fmt.Fprintf(os.Stdout, "Installed plugin: %s\n", meta.Name)
		_, _ = fmt.Fprintf(os.Stdout, "Directory: %s\n", meta.Directory)
		_, _ = fmt.Fprintf(os.Stdout, "Version: %s\n", meta.Version)

		missing, warnErr := findMissingPluginDependencies(cfg, meta)
		if warnErr != nil {
			_, _ = fmt.Fprintln(os.Stderr, warnErr)
			return true, 1
		}

		if len(missing) > 0 {
			_, _ = fmt.Fprintln(os.Stdout, "")
			_, _ = fmt.Fprintln(os.Stdout, "Dependency warnings:")
			for _, dep := range missing {
				if dep.Installed {
					_, _ = fmt.Fprintf(os.Stdout, "- Required plugin repo %s is installed as %q but not enabled\n", dep.Repo, dep.Name)
				} else {
					_, _ = fmt.Fprintf(os.Stdout, "- Missing required plugin repo: %s\n", dep.Repo)
				}
			}

			_, _ = fmt.Fprintln(os.Stdout, "")
			_, _ = fmt.Fprintln(os.Stdout, "Suggested next steps:")
			for _, dep := range missing {
				if dep.Installed {
					_, _ = fmt.Fprintf(os.Stdout, `Add %q to content/config/site.yaml under plugins.enabled`+"\n", dep.Name)
				} else {
					_, _ = fmt.Fprintf(os.Stdout, "cms plugin install %s\n", dep.Repo)
				}
			}
		}

		_, _ = fmt.Fprintln(os.Stdout, "")
		_, _ = fmt.Fprintln(os.Stdout, "Next steps:")
		if len(missing) > 0 {
			_, _ = fmt.Fprintln(os.Stdout, "1. Resolve the dependency warnings above")
			_, _ = fmt.Fprintf(os.Stdout, "2. Add %q to content/config/site.yaml under plugins.enabled\n", meta.Name)
			_, _ = fmt.Fprintln(os.Stdout, "3. Run make plugins-sync")
			_, _ = fmt.Fprintln(os.Stdout, "4. Run make dev or make build")
		} else {
			_, _ = fmt.Fprintf(os.Stdout, "1. Add %q to content/config/site.yaml under plugins.enabled\n", meta.Name)
			_, _ = fmt.Fprintln(os.Stdout, "2. Run make plugins-sync")
			_, _ = fmt.Fprintln(os.Stdout, "3. Run make dev or make build")
		}

		return true, 0

	case "uninstall":
		if len(args) < 3 {
			_, _ = fmt.Fprintln(os.Stderr, "usage: cms plugin uninstall <name>")
			return true, 1
		}

		name := strings.TrimSpace(args[2])
		if err := plugins.Uninstall(cfg.PluginsDir, name); err != nil {
			_, _ = fmt.Fprintln(os.Stderr, err)
			return true, 1
		}

		_, _ = fmt.Fprintf(os.Stdout, "Uninstalled plugin: %s\n", name)
		_, _ = fmt.Fprintln(os.Stdout, "")
		_, _ = fmt.Fprintln(os.Stdout, "Next steps:")
		_, _ = fmt.Fprintf(os.Stdout, "1. Remove %q from content/config/site.yaml under plugins.enabled\n", name)
		_, _ = fmt.Fprintln(os.Stdout, "2. Run make plugins-sync")
		_, _ = fmt.Fprintln(os.Stdout, "3. Run make dev or make build")

		return true, 0
	}

	return false, 0
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

func handlePluginCLI(pm *plugins.Manager, args []string) (bool, int) {
	if len(args) == 0 {
		return false, 0
	}

	switch args[0] {
	case "plugin":
		if len(args) < 2 {
			_, _ = fmt.Fprintln(os.Stderr, "usage: cms plugin [list|info <name>|install <git-url|owner/repo> [name]|uninstall <name>]")
			return true, 1
		}

		switch args[1] {
		case "list":
			for _, meta := range pm.MetadataList() {
				_, _ = fmt.Printf("%s\t%s\t%s\n", meta.Name, meta.Version, meta.Title)
			}
			return true, 0

		case "info":
			if len(args) < 3 {
				_, _ = fmt.Fprintln(os.Stderr, "usage: cms plugin info <name>")
				return true, 1
			}

			meta, ok := pm.MetadataFor(args[2])
			if !ok {
				_, _ = fmt.Fprintf(os.Stderr, "plugin not found: %s\n", args[2])
				return true, 1
			}

			_, _ = fmt.Printf("Name:        %s\n", meta.Name)
			_, _ = fmt.Printf("Title:       %s\n", meta.Title)
			_, _ = fmt.Printf("Version:     %s\n", meta.Version)
			_, _ = fmt.Printf("Description: %s\n", meta.Description)
			_, _ = fmt.Printf("Author:      %s\n", meta.Author)
			_, _ = fmt.Printf("Homepage:    %s\n", meta.Homepage)
			_, _ = fmt.Printf("License:     %s\n", meta.License)
			_, _ = fmt.Printf("Directory:   %s\n", meta.Directory)
			return true, 0

		case "install", "uninstall":
			return false, 0

		default:
			_, _ = fmt.Fprintf(os.Stderr, "unknown plugin subcommand: %s\n", args[1])
			return true, 1
		}

	default:
		name := strings.TrimSpace(args[0])
		if name == "" {
			return false, 0
		}

		err := pm.RunCommand(name, plugins.CommandContext{
			Args:   args[1:],
			Stdout: os.Stdout,
			Stderr: os.Stderr,
		})
		if err == nil {
			return true, 0
		}

		if strings.HasPrefix(err.Error(), "unknown plugin command:") {
			return false, 0
		}

		_, _ = fmt.Fprintln(os.Stderr, err)
		return true, 1
	}
}

func printUsage() {
	fmt.Println("usage: cms [serve|serve-preview|build|plugin]")
	fmt.Println("")
	fmt.Println("plugin commands:")
	fmt.Println("  cms plugin list")
	fmt.Println("  cms plugin info <name>")
	fmt.Println("  cms plugin install <git-url|owner/repo> [name]")
	fmt.Println("  cms plugin uninstall <name>")
	fmt.Println("  cms <plugin-command> [args...]")
}
