package main

import (
	"context"
	"fmt"
	"os"

	_ "github.com/sphireinc/foundry/internal/commands/imports"
	"github.com/sphireinc/foundry/internal/commands/registry"
	"github.com/sphireinc/foundry/internal/consts"
	"github.com/sphireinc/foundry/internal/content"
	"github.com/sphireinc/foundry/internal/diag"
	_ "github.com/sphireinc/foundry/internal/generated"
	"github.com/sphireinc/foundry/internal/logx"

	adminhttp "github.com/sphireinc/foundry/internal/admin/http"
	adminservice "github.com/sphireinc/foundry/internal/admin/service"

	"github.com/sphireinc/foundry/internal/config"
	"github.com/sphireinc/foundry/internal/plugins"
	"github.com/sphireinc/foundry/internal/renderer"
	"github.com/sphireinc/foundry/internal/router"
	"github.com/sphireinc/foundry/internal/server"
	"github.com/sphireinc/foundry/internal/site"
	"github.com/sphireinc/foundry/internal/theme"
)

func main() {
	logx.InitFromEnv()

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	if handled := handleConfigFreeCLI(os.Args); handled {
		return
	}

	cfg, err := config.Load(consts.ConfigFilePath)
	if err != nil {
		exitWithError(diag.Wrap(diag.KindConfig, "load config", err))
	}

	handleConfigBoundCLI(cfg, os.Args)

	pluginManager, err := plugins.NewManager(cfg.PluginsDir, cfg.Plugins.Enabled)
	if err != nil {
		exitWithError(diag.Wrap(diag.KindPlugin, "load plugins", err))
	}

	if err := pluginManager.OnConfigLoaded(cfg); err != nil {
		exitWithError(diag.Wrap(diag.KindPlugin, "run plugin config hooks", err))
	}

	routeResolver := router.NewResolver(cfg)
	themeManager := theme.NewManager(cfg.ThemesDir, cfg.Theme)
	rendererEngine := renderer.New(cfg, themeManager, pluginManager)
	ctx := context.Background()

	switch os.Args[1] {
	case "build":
		if err := pluginManager.OnBuildStarted(); err != nil {
			exitWithError(diag.Wrap(diag.KindBuild, "run build start hooks", err))
		}

		graph, err := site.LoadGraphWithManager(ctx, cfg, pluginManager, cfg.Build.IncludeDrafts)
		if err != nil {
			exitWithError(err)
		}

		if err := rendererEngine.Build(ctx, graph); err != nil {
			exitWithError(diag.Wrap(diag.KindBuild, "build site", err))
		}

		if err := pluginManager.OnBuildCompleted(graph); err != nil {
			exitWithError(diag.Wrap(diag.KindBuild, "run build completed hooks", err))
		}

		_, _ = fmt.Println("build complete")

	case "serve":
		loader := content.NewLoader(cfg, pluginManager, false)
		adminSvc := adminservice.New(cfg)
		adminRouter := adminhttp.New(cfg, adminSvc)
		hooks := adminhttp.WrapHooks(pluginManager, adminRouter)

		srv := server.New(cfg, loader, routeResolver, rendererEngine, hooks, false)
		if err := srv.ListenAndServe(ctx); err != nil {
			exitWithError(diag.Wrap(diag.KindServe, "serve site", err))
		}

	case "serve-preview":
		loader := content.NewLoader(cfg, pluginManager, true)
		adminSvc := adminservice.New(cfg)
		adminRouter := adminhttp.New(cfg, adminSvc)
		hooks := adminhttp.WrapHooks(pluginManager, adminRouter)

		srv := server.New(cfg, loader, routeResolver, rendererEngine, hooks, true)
		if err := srv.ListenAndServe(ctx); err != nil {
			exitWithError(diag.Wrap(diag.KindServe, "serve preview site", err))
		}

	default:
		exitWithError(diag.New(diag.KindUsage, fmt.Sprintf("unknown command: %s", os.Args[1])))
	}
}

func handleConfigFreeCLI(args []string) bool {
	cmd, ok := registry.Lookup(args)
	if !ok {
		return false
	}
	if cmd.RequiresConfig() {
		return false
	}

	if err := cmd.Run(nil, args); err != nil {
		exitWithError(diag.Wrap(diag.KindUsage, "run command", err))
	}
	return true
}

func handleConfigBoundCLI(cfg *config.Config, args []string) {
	cmd, ok := registry.Lookup(args)
	if !ok {
		return
	}
	if !cmd.RequiresConfig() {
		return
	}

	if err := cmd.Run(cfg, args); err != nil {
		exitWithError(err)
	}
	os.Exit(0)
}

func printUsage() {
	fmt.Println(registry.Usage())
}

func exitWithError(err error) {
	if err == nil {
		return
	}

	logx.Error("command failed", "kind", diag.KindOf(err), "error", err)
	_, _ = fmt.Fprintln(os.Stderr, diag.Present(err))
	os.Exit(diag.ExitCode(err))
}
