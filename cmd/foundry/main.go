package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	_ "github.com/sphireinc/foundry/internal/commands/imports"
	"github.com/sphireinc/foundry/internal/commands/registry"
	"github.com/sphireinc/foundry/internal/consts"
	"github.com/sphireinc/foundry/internal/content"
	"github.com/sphireinc/foundry/internal/diag"
	_ "github.com/sphireinc/foundry/internal/generated"
	"github.com/sphireinc/foundry/internal/logx"

	adminhttp "github.com/sphireinc/foundry/internal/admin/http"
	"github.com/sphireinc/foundry/internal/cliout"
	"github.com/sphireinc/foundry/internal/config"
	"github.com/sphireinc/foundry/internal/ops"
	"github.com/sphireinc/foundry/internal/platformapi"
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

	args, loadOpts, err := extractGlobalConfigFlags(os.Args)
	if err != nil {
		exitWithError(diag.Wrap(diag.KindUsage, "parse global flags", err))
	}

	if handled := handleConfigFreeCLI(args); handled {
		return
	}

	cfg, err := config.LoadWithOptions(consts.ConfigFilePath, loadOpts)
	if err != nil {
		exitWithError(diag.Wrap(diag.KindConfig, "load config", err))
	}

	handleConfigBoundCLI(cfg, args)

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

	switch args[1] {
	case "build":
		buildOpts, err := parseBuildFlags(args[2:])
		if err != nil {
			exitWithError(diag.Wrap(diag.KindUsage, "parse build flags", err))
		}
		if buildOpts.preview {
			cfg.Build.IncludeDrafts = true
		}

		if err := pluginManager.OnBuildStarted(); err != nil {
			exitWithError(diag.Wrap(diag.KindBuild, "run build start hooks", err))
		}

		graph, err := site.LoadGraphWithManager(ctx, cfg, pluginManager, cfg.Build.IncludeDrafts)
		if err != nil {
			exitWithError(err)
		}

		stats, err := rendererEngine.BuildWithStats(ctx, graph)
		if err != nil {
			exitWithError(diag.Wrap(diag.KindBuild, "build site", err))
		}
		if err := ops.WritePreviewManifest(cfg, graph, loadOpts.Target, buildOpts.preview); err != nil {
			exitWithError(diag.Wrap(diag.KindBuild, "write preview manifest", err))
		}
		if err := ops.WriteBuildReport(cfg, graph, loadOpts.Target, buildOpts.preview, stats); err != nil {
			exitWithError(diag.Wrap(diag.KindBuild, "write build report", err))
		}

		if err := pluginManager.OnBuildCompleted(graph); err != nil {
			exitWithError(diag.Wrap(diag.KindBuild, "run build completed hooks", err))
		}

		cliout.Successf("build complete")

	case "serve":
		serveDebug, err := parseServeDebugFlag(args[2:])
		if err != nil {
			exitWithError(diag.Wrap(diag.KindUsage, "parse serve flags", err))
		}

		loader := content.NewLoader(cfg, pluginManager, false)
		hooks := adminhttp.NewHooks(cfg, platformapi.NewHooks(cfg, pluginManager))
		srv := server.New(cfg, loader, routeResolver, rendererEngine, hooks, false, server.WithDebugMode(serveDebug))
		if err := srv.ListenAndServe(ctx); err != nil {
			exitWithError(diag.Wrap(diag.KindServe, "serve site", err))
		}

	case "serve-preview":
		serveDebug, err := parseServeDebugFlag(args[2:])
		if err != nil {
			exitWithError(diag.Wrap(diag.KindUsage, "parse serve-preview flags", err))
		}

		loader := content.NewLoader(cfg, pluginManager, true)
		hooks := adminhttp.NewHooks(cfg, platformapi.NewHooks(cfg, pluginManager))
		srv := server.New(cfg, loader, routeResolver, rendererEngine, hooks, true, server.WithDebugMode(serveDebug))
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
	cliout.Println(registry.Usage())
}

func parseServeDebugFlag(args []string) (bool, error) {
	debug := false

	for _, arg := range args {
		switch strings.TrimSpace(arg) {
		case "":
			continue
		case "--debug":
			debug = true
		default:
			return false, fmt.Errorf("unknown serve flag: %s", arg)
		}
	}

	return debug, nil
}

type buildFlags struct {
	preview bool
}

func parseBuildFlags(args []string) (buildFlags, error) {
	var flags buildFlags
	for _, arg := range args {
		switch strings.TrimSpace(arg) {
		case "":
			continue
		case "--preview":
			flags.preview = true
		default:
			return buildFlags{}, fmt.Errorf("unknown build flag: %s", arg)
		}
	}
	return flags, nil
}

func extractGlobalConfigFlags(args []string) ([]string, config.LoadOptions, error) {
	if len(args) == 0 {
		return nil, config.LoadOptions{}, nil
	}

	filtered := []string{args[0]}
	var opts config.LoadOptions

	for i := 1; i < len(args); i++ {
		arg := strings.TrimSpace(args[i])
		switch {
		case arg == "--env":
			if i+1 >= len(args) {
				return nil, config.LoadOptions{}, fmt.Errorf("--env requires a value")
			}
			opts.Environment = strings.TrimSpace(args[i+1])
			i++
		case strings.HasPrefix(arg, "--env="):
			opts.Environment = strings.TrimSpace(strings.TrimPrefix(arg, "--env="))
		case arg == "--target":
			if i+1 >= len(args) {
				return nil, config.LoadOptions{}, fmt.Errorf("--target requires a value")
			}
			opts.Target = strings.TrimSpace(args[i+1])
			i++
		case strings.HasPrefix(arg, "--target="):
			opts.Target = strings.TrimSpace(strings.TrimPrefix(arg, "--target="))
		case arg == "--config-overlay":
			if i+1 >= len(args) {
				return nil, config.LoadOptions{}, fmt.Errorf("--config-overlay requires a value")
			}
			opts.OverlayPaths = append(opts.OverlayPaths, strings.TrimSpace(args[i+1]))
			i++
		case strings.HasPrefix(arg, "--config-overlay="):
			opts.OverlayPaths = append(opts.OverlayPaths, strings.TrimSpace(strings.TrimPrefix(arg, "--config-overlay=")))
		default:
			filtered = append(filtered, args[i])
		}
	}

	return filtered, opts, nil
}

func exitWithError(err error) {
	if err == nil {
		return
	}

	logx.Error("command failed", "kind", diag.KindOf(err), "error", err)
	cliout.Stderr(cliout.Fail(diag.Present(err)))
	os.Exit(diag.ExitCode(err))
}
