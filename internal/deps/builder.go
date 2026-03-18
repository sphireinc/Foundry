package deps

import (
	"fmt"
	"path/filepath"

	"github.com/sphireinc/foundry/internal/content"
)

func BuildSiteDependencyGraph(site *content.SiteGraph, themeName string) *Graph {
	g := NewGraph()
	outputIDs := make(map[string]struct{})

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
		outputIDs[outputID] = struct{}{}

		g.AddEdge(sourceID, docID)
		g.AddEdge(docID, outputID)
		g.AddEdge(layoutID, outputID)
		g.AddEdge(baseTemplateID, outputID)

		for taxonomy, terms := range doc.Taxonomies {
			def := site.Taxonomies.Definition(taxonomy)
			layoutPath := filepath.Join(site.Config.ThemesDir, themeName, "layouts", def.EffectiveTermLayout()+".html")
			layoutID := templateNodeID(layoutPath)
			g.AddNode(&Node{
				ID:   layoutID,
				Type: NodeTemplate,
				Meta: map[string]any{"path": filepath.ToSlash(layoutPath)},
			})

			for _, term := range terms {
				taxID := taxonomyNodeID(taxonomy, term, doc.Lang)
				taxURL := taxonomyOutputURL(site.Config.DefaultLang, doc.Lang, taxonomy, term)
				taxOutputID := outputNodeID(taxURL)
				g.AddNode(&Node{
					ID:   taxID,
					Type: NodeTaxonomy,
					Meta: map[string]any{
						"taxonomy": taxonomy,
						"term":     term,
						"lang":     doc.Lang,
					},
				})
				g.AddNode(&Node{
					ID:   taxOutputID,
					Type: NodeOutput,
					Meta: map[string]any{"url": taxURL},
				})
				outputIDs[taxOutputID] = struct{}{}
				g.AddEdge(docID, taxID)
				g.AddEdge(taxID, taxOutputID)
				g.AddEdge(layoutID, taxOutputID)
				g.AddEdge(baseTemplateID, taxOutputID)
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

		for outputID := range outputIDs {
			g.AddEdge(dataID, outputID)
		}
	}

	return g
}

func taxonomyOutputURL(defaultLang, lang, taxonomy, term string) string {
	if lang == "" || lang == defaultLang {
		return fmt.Sprintf("/%s/%s/", taxonomy, term)
	}
	return fmt.Sprintf("/%s/%s/%s/", lang, taxonomy, term)
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
