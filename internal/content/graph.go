package content

import (
	"github.com/sphireinc/foundry/internal/config"
	"github.com/sphireinc/foundry/internal/taxonomy"
)

type SiteGraph struct {
	Config     *config.Config
	Documents  []*Document
	ByURL      map[string]*Document
	ByType     map[string][]*Document
	ByLang     map[string][]*Document
	Taxonomies *taxonomy.Index
	Data       map[string]any
}

func NewSiteGraph(cfg *config.Config) *SiteGraph {
	return &SiteGraph{
		Config:     cfg,
		ByURL:      make(map[string]*Document),
		ByType:     make(map[string][]*Document),
		ByLang:     make(map[string][]*Document),
		Taxonomies: taxonomy.New(buildTaxonomyDefinitions(cfg)),
		Data:       make(map[string]any),
	}
}

func (g *SiteGraph) Add(doc *Document) {
	g.Documents = append(g.Documents, doc)
	g.ByType[doc.Type] = append(g.ByType[doc.Type], doc)
	g.ByLang[doc.Lang] = append(g.ByLang[doc.Lang], doc)

	if doc.URL != "" {
		g.ByURL[doc.URL] = doc
	}

	g.Taxonomies.AddDocument(
		doc.ID,
		doc.URL,
		doc.Lang,
		doc.Type,
		doc.Title,
		doc.Slug,
		doc.Taxonomies,
	)
}

func buildTaxonomyDefinitions(cfg *config.Config) map[string]taxonomy.Definition {
	out := make(map[string]taxonomy.Definition)

	if cfg == nil {
		return out
	}

	for _, name := range cfg.Taxonomies.DefaultSet {
		if name == "" {
			continue
		}
		out[name] = taxonomy.Definition{Name: name}
	}

	for name, def := range cfg.Taxonomies.Definitions {
		out[name] = taxonomy.Definition{
			Name:          name,
			Title:         def.Title,
			Labels:        def.Labels,
			ArchiveLayout: def.ArchiveLayout,
			TermLayout:    def.TermLayout,
			Order:         def.Order,
		}
	}

	return out
}
