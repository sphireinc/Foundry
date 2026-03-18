package site

import (
	"context"
	"errors"
	"testing"

	"github.com/sphireinc/foundry/internal/config"
	"github.com/sphireinc/foundry/internal/content"
	"github.com/sphireinc/foundry/internal/diag"
	"github.com/sphireinc/foundry/internal/router"
)

type stubLoader struct {
	graph *content.SiteGraph
	err   error
}

func (s stubLoader) Load(context.Context) (*content.SiteGraph, error) { return s.graph, s.err }

type stubHooks struct{ err error }

func (s stubHooks) OnRoutesAssigned(*content.SiteGraph) error { return s.err }

func TestLoadGraph(t *testing.T) {
	cfg := &config.Config{DefaultLang: "en"}
	graph := content.NewSiteGraph(cfg)
	graph.Documents = []*content.Document{{Type: "page", Slug: "about", Lang: "en", SourcePath: "about.md"}}

	got, err := LoadGraph(context.Background(), stubLoader{graph: graph}, router.NewResolver(cfg), stubHooks{})
	if err != nil {
		t.Fatalf("load graph: %v", err)
	}
	if got.Documents[0].URL != "/about/" {
		t.Fatalf("expected route assignment, got %q", got.Documents[0].URL)
	}
}

func TestLoadGraphErrors(t *testing.T) {
	cfg := &config.Config{DefaultLang: "en"}
	if _, err := LoadGraph(context.Background(), nil, router.NewResolver(cfg), nil); diag.KindOf(err) != diag.KindInternal {
		t.Fatalf("expected internal loader error, got %v", err)
	}
	if _, err := LoadGraph(context.Background(), stubLoader{}, nil, nil); diag.KindOf(err) != diag.KindInternal {
		t.Fatalf("expected internal resolver error, got %v", err)
	}
	if _, err := LoadGraph(context.Background(), stubLoader{err: errors.New("boom")}, router.NewResolver(cfg), nil); diag.KindOf(err) != diag.KindBuild {
		t.Fatalf("expected build error, got %v", err)
	}

	graph := content.NewSiteGraph(cfg)
	graph.Documents = []*content.Document{{Type: "page", Slug: "about", Lang: "en", SourcePath: "about.md"}}
	if _, err := LoadGraph(context.Background(), stubLoader{graph: graph}, router.NewResolver(cfg), stubHooks{err: errors.New("hook")}); diag.KindOf(err) != diag.KindPlugin {
		t.Fatalf("expected plugin error, got %v", err)
	}
}
