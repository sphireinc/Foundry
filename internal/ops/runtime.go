package ops

import (
	"context"

	"github.com/sphireinc/foundry/internal/assets"
	"github.com/sphireinc/foundry/internal/config"
	"github.com/sphireinc/foundry/internal/content"
	"github.com/sphireinc/foundry/internal/deps"
	"github.com/sphireinc/foundry/internal/diag"
	"github.com/sphireinc/foundry/internal/router"
)

type GraphLoader interface {
	Load(context.Context) (*content.SiteGraph, error)
}

type RouteHookRunner interface {
	OnRoutesAssigned(*content.SiteGraph) error
}

type AssetHookRunner interface {
	OnAssetsBuilding(*config.Config) error
}

type PreparedGraph struct {
	Graph    *content.SiteGraph
	DepGraph *deps.Graph
}

func LoadPreparedGraph(ctx context.Context, loader GraphLoader, resolver *router.Resolver, hooks RouteHookRunner, activeTheme string) (*PreparedGraph, error) {
	if loader == nil {
		return nil, diag.New(diag.KindInternal, "loader is nil")
	}
	if resolver == nil {
		return nil, diag.New(diag.KindInternal, "resolver is nil")
	}

	graph, err := loader.Load(ctx)
	if err != nil {
		return nil, diag.Wrap(diag.KindBuild, "load site graph", err)
	}
	if err := resolver.AssignURLs(graph); err != nil {
		return nil, diag.Wrap(diag.KindBuild, "assign urls", err)
	}
	if hooks != nil {
		if err := hooks.OnRoutesAssigned(graph); err != nil {
			return nil, diag.Wrap(diag.KindPlugin, "run route hooks", err)
		}
	}

	return &PreparedGraph{
		Graph:    graph,
		DepGraph: deps.BuildSiteDependencyGraph(graph, activeTheme),
	}, nil
}

func SyncAssets(cfg *config.Config, hooks AssetHookRunner) error {
	if err := assets.Sync(cfg, hooks); err != nil {
		return diag.Wrap(diag.KindIO, "sync assets", err)
	}
	return nil
}
