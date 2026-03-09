package main

import (
	"context"
	"fmt"
	"os"
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
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}

	pluginManager, err := plugins.NewManager(cfg.PluginsDir, cfg.Plugins.Enabled)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load plugins: %v\n", err)
		os.Exit(1)
	}

	if err := pluginManager.OnConfigLoaded(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "plugin config hook failed: %v\n", err)
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
			fmt.Fprintf(os.Stderr, "build start hook failed: %v\n", err)
			os.Exit(1)
		}

		loader := content.NewLoader(cfg, pluginManager, cfg.Build.IncludeDrafts)
		graph, err := loader.Load(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "load content: %v\n", err)
			os.Exit(1)
		}

		if err := routeResolver.AssignURLs(graph); err != nil {
			fmt.Fprintf(os.Stderr, "assign urls: %v\n", err)
			os.Exit(1)
		}

		if err := pluginManager.OnRoutesAssigned(graph); err != nil {
			fmt.Fprintf(os.Stderr, "route hook failed: %v\n", err)
			os.Exit(1)
		}

		if err := rendererEngine.Build(ctx, graph); err != nil {
			fmt.Fprintf(os.Stderr, "build: %v\n", err)
			os.Exit(1)
		}

		if err := pluginManager.OnBuildCompleted(graph); err != nil {
			fmt.Fprintf(os.Stderr, "build completed hook failed: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("build complete")

	case "serve":
		loader := content.NewLoader(cfg, pluginManager, false)
		srv := server.New(cfg, loader, routeResolver, rendererEngine, pluginManager, false)
		if err := srv.ListenAndServe(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "serve: %v\n", err)
			os.Exit(1)
		}

	case "serve-preview":
		loader := content.NewLoader(cfg, pluginManager, true)
		srv := server.New(cfg, loader, routeResolver, rendererEngine, pluginManager, true)
		if err := srv.ListenAndServe(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "serve preview: %v\n", err)
			os.Exit(1)
		}

	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func handlePluginCLI(pm *plugins.Manager, args []string) (bool, int) {
	if len(args) == 0 {
		return false, 0
	}

	switch args[0] {
	case "plugin":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: cms plugin [list|info <name>]")
			return true, 1
		}

		switch args[1] {
		case "list":
			for _, meta := range pm.MetadataList() {
				fmt.Printf("%s\t%s\t%s\n", meta.Name, meta.Version, meta.Title)
			}
			return true, 0

		case "info":
			if len(args) < 3 {
				fmt.Fprintln(os.Stderr, "usage: cms plugin info <name>")
				return true, 1
			}

			meta, ok := pm.MetadataFor(args[2])
			if !ok {
				fmt.Fprintf(os.Stderr, "plugin not found: %s\n", args[2])
				return true, 1
			}

			fmt.Printf("Name:        %s\n", meta.Name)
			fmt.Printf("Title:       %s\n", meta.Title)
			fmt.Printf("Version:     %s\n", meta.Version)
			fmt.Printf("Description: %s\n", meta.Description)
			fmt.Printf("Author:      %s\n", meta.Author)
			fmt.Printf("Homepage:    %s\n", meta.Homepage)
			fmt.Printf("License:     %s\n", meta.License)
			fmt.Printf("Directory:   %s\n", meta.Directory)
			return true, 0

		default:
			fmt.Fprintf(os.Stderr, "unknown plugin subcommand: %s\n", args[1])
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

		fmt.Fprintln(os.Stderr, err)
		return true, 1
	}
}

func printUsage() {
	fmt.Println("usage: cms [serve|serve-preview|build|plugin]")
	fmt.Println("")
	fmt.Println("plugin commands:")
	fmt.Println("  cms plugin list")
	fmt.Println("  cms plugin info <name>")
	fmt.Println("  cms <plugin-command> [args...]")
}
