package mcp

import (
	"sort"
	"strings"
	"time"

	"github.com/sphireinc/foundry/internal/config"
	"github.com/sphireinc/foundry/internal/content"
)

type DocumentSummary struct {
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	Slug      string   `json:"slug"`
	URL       string   `json:"url"`
	Type      string   `json:"type"`
	Lang      string   `json:"lang"`
	Status    string   `json:"status"`
	Draft     bool     `json:"draft"`
	Summary   string   `json:"summary,omitempty"`
	Author    string   `json:"author,omitempty"`
	Tags      []string `json:"tags,omitempty"`
	UpdatedAt string   `json:"updated_at,omitempty"`
}

type DocumentDetail struct {
	DocumentSummary
	Layout     string              `json:"layout,omitempty"`
	SourcePath string              `json:"source_path,omitempty"`
	Body       string              `json:"body"`
	Fields     map[string]any      `json:"fields,omitempty"`
	Params     map[string]any      `json:"params,omitempty"`
	Taxonomies map[string][]string `json:"taxonomies,omitempty"`
}

type NavigationEntry struct {
	Title string `json:"title"`
	URL   string `json:"url"`
	Type  string `json:"type"`
}

type Navigation struct {
	Lang  string                       `json:"lang,omitempty"`
	Menus map[string][]NavigationEntry `json:"menus,omitempty"`
	Pages []NavigationEntry            `json:"pages"`
}

func statusOf(doc *content.Document) string {
	if doc == nil {
		return ""
	}
	if strings.TrimSpace(doc.Status) != "" {
		return doc.Status
	}
	if doc.Draft {
		return "draft"
	}
	return "published"
}

func formatTime(value *time.Time) string {
	if value == nil || value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}

func summarizeDocument(doc *content.Document) DocumentSummary {
	summary := DocumentSummary{
		ID:        doc.ID,
		Title:     doc.Title,
		Slug:      doc.Slug,
		URL:       doc.URL,
		Type:      doc.Type,
		Lang:      doc.Lang,
		Status:    statusOf(doc),
		Draft:     doc.Draft,
		Summary:   doc.Summary,
		Author:    doc.Author,
		Tags:      doc.Taxonomies["tags"],
		UpdatedAt: formatTime(doc.UpdatedAt),
	}
	return summary
}

func detailDocument(doc *content.Document) DocumentDetail {
	body := doc.RawBody
	if strings.TrimSpace(body) == "" {
		body = string(doc.HTMLBody)
	}
	return DocumentDetail{
		DocumentSummary: summarizeDocument(doc),
		Layout:          doc.Layout,
		SourcePath:      doc.RelPath,
		Body:            body,
		Fields:          doc.Fields,
		Params:          doc.Params,
		Taxonomies:      doc.Taxonomies,
	}
}

func findDocument(graph *content.SiteGraph, key string) (*content.Document, bool) {
	key = strings.TrimSpace(key)
	if graph == nil || key == "" {
		return nil, false
	}
	trimmed := strings.Trim(key, "/")
	lowered := strings.ToLower(key)

	var fallback *content.Document
	for _, doc := range graph.Documents {
		if doc == nil {
			continue
		}
		if doc.ID == key || doc.Slug == key || strings.Trim(doc.URL, "/") == trimmed {
			return doc, true
		}
		if fallback == nil && strings.ToLower(doc.Slug) == lowered {
			fallback = doc
		}
	}
	if fallback != nil {
		return fallback, true
	}
	return nil, false
}

func filterDocuments(graph *content.SiteGraph, docType, lang, status string) []*content.Document {
	out := []*content.Document{}
	if graph == nil {
		return out
	}
	docType = strings.TrimSpace(docType)
	lang = strings.TrimSpace(lang)
	status = strings.TrimSpace(status)

	for _, doc := range graph.Documents {
		if doc == nil {
			continue
		}
		if docType != "" && doc.Type != docType {
			continue
		}
		if lang != "" && doc.Lang != lang {
			continue
		}
		if status != "" && statusOf(doc) != status {
			continue
		}
		out = append(out, doc)
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].URL != out[j].URL {
			return out[i].URL < out[j].URL
		}
		return out[i].Title < out[j].Title
	})
	return out
}

func buildNavigation(cfg *config.Config, graph *content.SiteGraph, lang string) Navigation {
	nav := Navigation{Lang: strings.TrimSpace(lang), Pages: []NavigationEntry{}}
	if cfg != nil && len(cfg.Menus) > 0 {
		nav.Menus = map[string][]NavigationEntry{}
		for name, items := range cfg.Menus {
			entries := make([]NavigationEntry, 0, len(items))
			for _, item := range items {
				entries = append(entries, NavigationEntry{Title: item.Name, URL: item.URL})
			}
			nav.Menus[name] = entries
		}
	}

	lang = strings.TrimSpace(lang)
	for _, doc := range filterDocuments(graph, "", lang, "published") {
		if doc.Draft {
			continue
		}
		nav.Pages = append(nav.Pages, NavigationEntry{
			Title: doc.Title,
			URL:   doc.URL,
			Type:  doc.Type,
		})
	}
	return nav
}
