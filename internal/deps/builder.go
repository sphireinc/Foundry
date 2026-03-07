package deps

import (
	"fmt"
	"path/filepath"

	"github.com/sphireinc/foundry/internal/content"
)

func BuildSiteDependencyGraph(site *content.SiteGraph, themeName string) *Graph {
	g := NewGraph()

	baseTemplatePath := filepath.Join(site.Config.ThemesDir, themeName, "layouts", "base.html")
	baseTemplateID := templateNodeID(baseTemplatePath)

	g.AddNode(&Node{
		ID:   baseTemplateID,
		Type: NodeTemplate,
		Meta: map[string]any{"path": filepath.ToSlash(baseTemplatePath)},
	})

	for _, doc := range site.Documents {
		sourceID := sourceNodeID(doc.SourcePath)
		docID := documentNodeID(doc.ID)
		layoutPath := filepath.Join(site.Config.ThemesDir, themeName, "layouts", doc.Layout+".html")
		layoutID := templateNodeID(layoutPath)
		outputID := outputNodeID(doc.URL)

		g.AddNode(&Node{
			ID:   sourceID,
			Type: NodeSource,
			Meta: map[string]any{"path": filepath.ToSlash(doc.SourcePath)},
		})
		g.AddNode(&Node{
			ID:   docID,
			Type: NodeDocument,
			Meta: map[string]any{"document_id": doc.ID, "url": doc.URL, "type": doc.Type},
		})
		g.AddNode(&Node{
			ID:   layoutID,
			Type: NodeTemplate,
			Meta: map[string]any{"path": filepath.ToSlash(layoutPath)},
		})
		g.AddNode(&Node{
			ID:   outputID,
			Type: NodeOutput,
			Meta: map[string]any{"url": doc.URL},
		})

		g.AddEdge(sourceID, docID)
		g.AddEdge(docID, outputID)
		g.AddEdge(layoutID, outputID)
		g.AddEdge(baseTemplateID, outputID)

		for taxonomy, terms := range doc.Taxonomies {
			for _, term := range terms {
				taxID := taxonomyNodeID(taxonomy, term, doc.Lang)
				g.AddNode(&Node{
					ID:   taxID,
					Type: NodeTaxonomy,
					Meta: map[string]any{
						"taxonomy": taxonomy,
						"term":     term,
						"lang":     doc.Lang,
					},
				})
				g.AddEdge(docID, taxID)
			}
		}
	}

	for key := range site.Data {
		dataID := dataNodeID(key)
		g.AddNode(&Node{
			ID:   dataID,
			Type: NodeData,
			Meta: map[string]any{"key": key},
		})

		for _, doc := range site.Documents {
			g.AddEdge(dataID, outputNodeID(doc.URL))
		}
	}

	return g
}

func sourceNodeID(path string) string {
	return "source:" + filepath.ToSlash(path)
}

func documentNodeID(id string) string {
	return "document:" + id
}

func templateNodeID(path string) string {
	return "template:" + filepath.ToSlash(path)
}

func dataNodeID(key string) string {
	return "data:" + key
}

func taxonomyNodeID(taxonomy, term, lang string) string {
	return fmt.Sprintf("taxonomy:%s:%s:%s", taxonomy, term, lang)
}

func outputNodeID(url string) string {
	return "output:" + url
}
