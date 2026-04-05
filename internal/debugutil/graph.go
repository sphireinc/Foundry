package debugutil

import (
	"context"

	"github.com/sphireinc/foundry/internal/config"
	"github.com/sphireinc/foundry/internal/content"
	"github.com/sphireinc/foundry/internal/site"
)

func LoadGraph(cfg *config.Config) (*content.SiteGraph, error) {
	graph, _, err := site.LoadConfiguredGraph(context.Background(), cfg, true)
	if err != nil {
		return nil, err
	}
	return graph, nil
}
