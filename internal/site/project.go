package site

import (
	"context"
	"fmt"

	"github.com/sphireinc/foundry/internal/config"
	"github.com/sphireinc/foundry/internal/content"
	"github.com/sphireinc/foundry/internal/plugins"
	"github.com/sphireinc/foundry/internal/router"
)

func NewPluginManager(cfg *config.Config) (*plugins.Manager, error) {
	pm, err := plugins.NewManager(cfg.PluginsDir, cfg.Plugins.Enabled)
	if err != nil {
		return nil, fmt.Errorf("load plugins: %w", err)
	}

	if err := pm.OnConfigLoaded(cfg); err != nil {
		return nil, fmt.Errorf("plugin config hook failed: %w", err)
	}

	return pm, nil
}

func LoadGraphWithManager(ctx context.Context, cfg *config.Config, pm *plugins.Manager, includeDrafts bool) (*content.SiteGraph, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}
	if pm == nil {
		return nil, fmt.Errorf("plugin manager is nil")
	}

	loader := content.NewLoader(cfg, pm, includeDrafts)
	resolver := router.NewResolver(cfg)

	graph, err := LoadGraph(ctx, loader, resolver, pm)
	if err != nil {
		return nil, err
	}

	return graph, nil
}

func LoadConfiguredGraph(ctx context.Context, cfg *config.Config, includeDrafts bool) (*content.SiteGraph, *plugins.Manager, error) {
	pm, err := NewPluginManager(cfg)
	if err != nil {
		return nil, nil, err
	}

	graph, err := LoadGraphWithManager(ctx, cfg, pm, includeDrafts)
	if err != nil {
		return nil, nil, err
	}

	return graph, pm, nil
}
