package mcp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	foundryversion "github.com/sphireinc/foundry/internal/commands/version"
	"github.com/sphireinc/foundry/internal/config"
	"github.com/sphireinc/foundry/internal/content"
	"github.com/sphireinc/foundry/internal/managed"
	"github.com/sphireinc/foundry/internal/plugins"
	"github.com/sphireinc/foundry/internal/router"
	"github.com/sphireinc/foundry/internal/safepath"
	"github.com/sphireinc/foundry/internal/site"
	"github.com/sphireinc/foundry/internal/theme"
)

const docsDir = "docs"

type Environment struct {
	Config *config.Config

	LoadGraph         func(ctx context.Context) (*content.SiteGraph, error)
	LoadThemeManifest func(name string) (*theme.Manifest, error)
	ThemeSecurity     func(name string) (*theme.SecurityReport, error)
	ListThemes        func() ([]theme.Info, error)
	ValidateTheme     func(name string) (*theme.ValidationResult, error)
	ListPlugins       func() ([]plugins.Metadata, error)
	PluginStatus      func() (map[string]string, error)
	PluginHealth      func(name string) (plugins.HealthReport, bool)
	RuntimeHealth     func() managed.HealthReport
	ReadDoc           func(rel string) ([]byte, error)
	ListDocs          func() ([]string, error)
}

func NewProjectEnvironment(cfg *config.Config, manager *plugins.Manager) (*Environment, error) {
	if cfg == nil {
		return nil, fmt.Errorf("mcp: config is required")
	}

	var contentHooks content.Hooks
	var routeHooks site.RouteHooks
	if manager != nil {
		contentHooks = manager
		routeHooks = manager
	}

	loader := content.NewLoader(cfg, contentHooks, false)
	resolver := router.NewResolver(cfg)

	env := &Environment{Config: cfg}
	env.LoadGraph = func(ctx context.Context) (*content.SiteGraph, error) {
		return site.LoadGraph(ctx, loader, resolver, routeHooks)
	}
	env.LoadThemeManifest = func(name string) (*theme.Manifest, error) {
		resolved, err := resolveThemeName(cfg, name)
		if err != nil {
			return nil, err
		}
		return theme.LoadManifest(cfg.ThemesDir, resolved)
	}
	env.ThemeSecurity = func(name string) (*theme.SecurityReport, error) {
		resolved, err := resolveThemeName(cfg, name)
		if err != nil {
			return nil, err
		}
		return theme.AnalyzeInstalledSecurity(cfg.ThemesDir, resolved)
	}
	env.ListThemes = func() ([]theme.Info, error) {
		return theme.ListInstalled(cfg.ThemesDir)
	}
	env.ValidateTheme = func(name string) (*theme.ValidationResult, error) {
		resolved, err := resolveThemeName(cfg, name)
		if err != nil {
			return nil, err
		}
		return theme.ValidateInstalledDetailed(cfg.ThemesDir, resolved)
	}
	env.ListPlugins = func() ([]plugins.Metadata, error) {
		return plugins.ListInstalled(cfg.PluginsDir)
	}
	env.PluginStatus = func() (map[string]string, error) {
		return plugins.EnabledPluginStatus(cfg.PluginsDir, cfg.Plugins.Enabled), nil
	}
	env.PluginHealth = func(name string) (plugins.HealthReport, bool) {
		meta, err := plugins.LoadMetadata(cfg.PluginsDir, name)
		if err != nil {
			return plugins.HealthReport{}, false
		}
		return plugins.DiagnoseInstalled(cfg.PluginsDir, meta, pluginEnabled(cfg, name)), true
	}
	env.RuntimeHealth = func() managed.HealthReport {
		meta := foundryversion.Current("")
		return managed.BuildHealthReport(cfg, managed.HealthVersion{
			Version: meta.DisplayVersion,
			Commit:  meta.Commit,
		}, time.Now().UTC())
	}
	env.ReadDoc = func(rel string) ([]byte, error) {
		return readProjectDoc(rel)
	}
	env.ListDocs = func() ([]string, error) {
		return listProjectDocs()
	}

	return env, nil
}

func resolveThemeName(cfg *config.Config, name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" && cfg != nil {
		name = cfg.Theme
	}
	return safepath.ValidatePathComponent("theme name", name)
}

func pluginEnabled(cfg *config.Config, name string) bool {
	if cfg == nil {
		return false
	}
	for _, enabled := range cfg.Plugins.Enabled {
		if strings.EqualFold(strings.TrimSpace(enabled), strings.TrimSpace(name)) {
			return true
		}
	}
	return false
}

func readProjectDoc(rel string) ([]byte, error) {
	rel = strings.TrimSpace(rel)
	if rel == "" {
		return nil, fmt.Errorf("mcp: documentation path is required")
	}
	target, err := safepath.ResolveRelativeUnderRoot(docsDir, rel)
	if err != nil {
		return nil, err
	}
	if err := safepath.EnsureNoSymlinkEscape(docsDir, target); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Clean(target))
	if err != nil {
		return nil, fmt.Errorf("mcp: read documentation %q: %w", rel, err)
	}
	return data, nil
}

func listProjectDocs() ([]string, error) {
	root, err := filepath.Abs(docsDir)
	if err != nil {
		return nil, err
	}
	out := []string{}
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		switch strings.ToLower(filepath.Ext(entry.Name())) {
		case ".md", ".yaml", ".yml":
		default:
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		out = append(out, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("mcp: list documentation: %w", err)
	}
	sort.Strings(out)
	return out, nil
}
