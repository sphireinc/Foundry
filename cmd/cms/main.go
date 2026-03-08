package main

import (
	"context"
	"fmt"
	"os"

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
		fmt.Println("usage: cms [serve|serve-preview|build]")
		os.Exit(1)
	}

	cfg, err := config.Load("content/config/site.yaml")
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}

	pluginManager := plugins.NewManager(cfg.Plugins.Enabled)
	if err := pluginManager.OnConfigLoaded(cfg); err != nil {
		os.Exit(1)
	}

	routeResolver := router.NewResolver(cfg)
	themeManager := theme.NewManager(cfg.ThemesDir, cfg.Theme)
	rendererEngine := renderer.New(cfg, themeManager)
	ctx := context.Background()

	switch os.Args[1] {
	case "build":
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
			os.Exit(1)
		}

		if err := rendererEngine.Build(ctx, graph); err != nil {
			fmt.Fprintf(os.Stderr, "build: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("build complete")

	case "serve":
		loader := content.NewLoader(cfg, pluginManager, false)
		srv := server.New(cfg, loader, routeResolver, rendererEngine, false)
		if err := srv.ListenAndServe(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "serve: %v\n", err)
			os.Exit(1)
		}

	case "serve-preview":
		loader := content.NewLoader(cfg, pluginManager, true)
		srv := server.New(cfg, loader, routeResolver, rendererEngine, true)
		if err := srv.ListenAndServe(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "serve preview: %v\n", err)
			os.Exit(1)
		}

	default:
		fmt.Println("usage: cms [serve|serve-preview|build]")
		os.Exit(1)
	}
}
