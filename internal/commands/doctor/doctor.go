package doctor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	adminusers "github.com/sphireinc/foundry/internal/admin/users"
	"github.com/sphireinc/foundry/internal/cliout"
	"github.com/sphireinc/foundry/internal/commands/registry"
	foundryconfig "github.com/sphireinc/foundry/internal/config"
	"github.com/sphireinc/foundry/internal/content"
	"github.com/sphireinc/foundry/internal/ops"
	"github.com/sphireinc/foundry/internal/plugins"
	"github.com/sphireinc/foundry/internal/renderer"
	"github.com/sphireinc/foundry/internal/router"
	"github.com/sphireinc/foundry/internal/theme"
)

type command struct{}

func (command) Name() string {
	return "doctor"
}

func (command) Summary() string {
	return "Check project and environment health"
}

func (command) Group() string {
	return "core commands"
}

func (command) Details() []string {
	return nil
}

func (command) RequiresConfig() bool {
	return true
}

func (command) Run(cfg *foundryconfig.Config, _ []string) error {
	type result struct {
		label string
		ok    bool
		msg   string
	}

	results := make([]result, 0)
	failures := 0

	add := func(label string, ok bool, msg string) {
		results = append(results, result{
			label: label,
			ok:    ok,
			msg:   msg,
		})
		if !ok {
			failures++
		}
	}

	if errs := foundryconfig.Validate(cfg); len(errs) == 0 {
		add("config", true, "valid")
	} else {
		add("config", false, fmt.Sprintf("%d validation error(s)", len(errs)))
	}

	checkDir := func(label, path string) {
		info, err := os.Stat(path)
		if err != nil {
			if os.IsNotExist(err) {
				add(label, false, fmt.Sprintf("%s does not exist", path))
				return
			}
			add(label, false, err.Error())
			return
		}
		if !info.IsDir() {
			add(label, false, fmt.Sprintf("%s is not a directory", path))
			return
		}
		add(label, true, path)
	}

	checkDir("content_dir", cfg.ContentDir)
	checkDir("themes_dir", cfg.ThemesDir)
	checkDir("data_dir", cfg.DataDir)
	checkDir("plugins_dir", cfg.PluginsDir)

	if err := theme.NewManager(cfg.ThemesDir, cfg.Theme).MustExist(); err != nil {
		add("theme", false, err.Error())
	} else {
		add("theme", true, cfg.Theme)
	}

	if _, err := plugins.NewManager(cfg.PluginsDir, cfg.Plugins.Enabled); err != nil {
		add("plugins", false, err.Error())
	} else {
		add("plugins", true, fmt.Sprintf("%d enabled", len(cfg.Plugins.Enabled)))
	}

	pmStart := time.Now()
	pm, err := plugins.NewManager(cfg.PluginsDir, cfg.Plugins.Enabled)
	if err == nil {
		err = pm.OnConfigLoaded(cfg)
	}
	pluginTiming := time.Since(pmStart)
	if err != nil {
		add("timing.plugin_config", false, err.Error())
	} else {
		add("timing.plugin_config", true, pluginTiming.String())

		tempCfg := *cfg
		tempCfg.PublicDir = filepath.Join(cfg.DataDir, ".doctor-public")
		loader := content.NewLoader(&tempCfg, pm, true)
		resolver := router.NewResolver(&tempCfg)
		graph, timing, graphErr := ops.LoadGraphWithTiming(context.Background(), loader, resolver, pm)
		if graphErr != nil {
			add("timing.loader_router", false, graphErr.Error())
		} else {
			add("timing.loader", true, timing.Loader.String())
			add("timing.router", true, timing.Router.String())
			add("timing.route_hooks", true, timing.RouteHooks.String())

			rendererEngine := renderer.New(&tempCfg, theme.NewManager(tempCfg.ThemesDir, tempCfg.Theme), pm)
			renderMetrics, renderErr := ops.BuildRendererWithTiming(context.Background(), rendererEngine, graph)
			if renderErr != nil {
				add("timing.renderer", false, renderErr.Error())
			} else {
				add("timing.assets", true, renderMetrics.Assets.String())
				add("timing.renderer", true, renderMetrics.Renderer.String())
			}
			feedMetrics, feedErr := ops.BuildFeedsWithTiming(&tempCfg, graph)
			if feedErr != nil {
				add("timing.feed", false, feedErr.Error())
			} else {
				add("timing.feed", true, feedMetrics.Feed.String())
			}

			report := ops.AnalyzeSite(&tempCfg, graph)
			add("diagnostics.broken_links", len(report.BrokenInternalLinks) == 0, fmt.Sprintf("%d issue(s)", len(report.BrokenInternalLinks)))
			add("diagnostics.broken_media", len(report.BrokenMediaRefs) == 0, fmt.Sprintf("%d issue(s)", len(report.BrokenMediaRefs)))
			add("diagnostics.templates", len(report.MissingTemplates) == 0, fmt.Sprintf("%d issue(s)", len(report.MissingTemplates)))
			add("diagnostics.orphaned_media", len(report.OrphanedMedia) == 0, fmt.Sprintf("%d issue(s)", len(report.OrphanedMedia)))
			add("diagnostics.duplicate_routes", len(report.DuplicateURLs) == 0, fmt.Sprintf("%d issue(s)", len(report.DuplicateURLs)))
			add("diagnostics.duplicate_slugs", len(report.DuplicateSlugs) == 0, fmt.Sprintf("%d issue(s)", len(report.DuplicateSlugs)))
			add("diagnostics.taxonomies", len(report.TaxonomyInconsistency) == 0, fmt.Sprintf("%d issue(s)", len(report.TaxonomyInconsistency)))
		}
	}

	genPath := filepath.Join("internal", "generated", "plugins_gen.go")
	if _, err := os.Stat(genPath); err != nil {
		if os.IsNotExist(err) {
			add("plugin_sync", false, genPath+" not found")
		} else {
			add("plugin_sync", false, err.Error())
		}
	} else {
		add("plugin_sync", true, genPath)
	}

	if cfg.Admin.Enabled {
		userEntries, userErr := adminusers.Load(cfg.Admin.UsersFile)
		if userErr != nil {
			add("auth.users_file", false, userErr.Error())
		} else {
			legacyPBKDF2 := 0
			plaintextTOTP := 0
			for _, user := range userEntries {
				if strings.HasPrefix(strings.TrimSpace(user.PasswordHash), "pbkdf2_sha256$") {
					legacyPBKDF2++
				}
				secret := strings.TrimSpace(user.TOTPSecret)
				if secret != "" && !strings.HasPrefix(secret, "enc:v1:") {
					plaintextTOTP++
				}
			}
			add("auth.legacy_password_hashes", legacyPBKDF2 == 0, fmt.Sprintf("%d remaining", legacyPBKDF2))
			add("auth.plaintext_totp", plaintextTOTP == 0, fmt.Sprintf("%d remaining", plaintextTOTP))
		}

		sessionFile, sessionErr := os.ReadFile(cfg.Admin.SessionStoreFile)
		if sessionErr != nil && !os.IsNotExist(sessionErr) {
			add("auth.session_store", false, sessionErr.Error())
		} else {
			legacySessions := strings.Count(string(sessionFile), "\n  - token:")
			add("auth.legacy_sessions", legacySessions == 0, fmt.Sprintf("%d remaining", legacySessions))
		}
	}

	for _, r := range results {
		fmt.Printf("[%s] %-20s %s\n", cliout.StatusLabel(r.ok), cliout.Label(r.label), r.msg)
	}

	if failures > 0 {
		return fmt.Errorf("doctor found %d problem(s)", failures)
	}

	cliout.Successf("doctor OK")
	return nil
}

func init() {
	registry.Register(command{})
}
