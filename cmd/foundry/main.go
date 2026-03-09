package main

import (
	"context"
	"fmt"
	"os"

	_ "github.com/sphireinc/foundry/internal/commands/imports"
	"github.com/sphireinc/foundry/internal/commands/registry"
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

	handleCli(cfg, os.Args)

	pluginManager, err := plugins.NewManager(cfg.PluginsDir, cfg.Plugins.Enabled)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "load plugins: %v\n", err)
		os.Exit(1)
	}

	if err := pluginManager.OnConfigLoaded(cfg); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "plugin config hook failed: %v\n", err)
		os.Exit(1)
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

func handleCli(cfg *config.Config, args []string) {
	handled, err := registry.Run(cfg, args)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	if handled {
		os.Exit(0)
	}
}

func printUsage() {
	fmt.Println(registry.Usage())
}
