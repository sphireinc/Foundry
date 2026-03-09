package site

import (
	"context"
	"fmt"

	"github.com/sphireinc/foundry/internal/content"
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
		return nil, fmt.Errorf("loader is nil")
	}
	if resolver == nil {
		return nil, fmt.Errorf("resolver is nil")
	}

	graph, err := loader.Load(ctx)
	if err != nil {
		return nil, fmt.Errorf("load site graph: %w", err)
	}

	if err := resolver.AssignURLs(graph); err != nil {
		return nil, fmt.Errorf("assign urls: %w", err)
	}

	if hooks != nil {
		if err := hooks.OnRoutesAssigned(graph); err != nil {
			return nil, fmt.Errorf("route hook failed: %w", err)
		}
	}

	return graph, nil
}
