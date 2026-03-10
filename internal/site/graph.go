package site

import (
	"context"

	"github.com/sphireinc/foundry/internal/content"
	"github.com/sphireinc/foundry/internal/diag"
	"github.com/sphireinc/foundry/internal/router"
)

type Loader interface {
	Load(context.Context) (*content.SiteGraph, error)
}

type RouteHooks interface {
	OnRoutesAssigned(*content.SiteGraph) error
}

func LoadGraph(ctx context.Context, loader Loader, resolver *router.Resolver, hooks RouteHooks) (*content.SiteGraph, error) {
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

	return graph, nil
}
