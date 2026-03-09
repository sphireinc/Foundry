package relatedposts

import (
	"sort"
	"sync"
	"time"

	"github.com/sphireinc/foundry/internal/content"
	"github.com/sphireinc/foundry/internal/plugins"
	"github.com/sphireinc/foundry/internal/renderer"
)

type Plugin struct {
	mu      sync.RWMutex
	related map[string][]Item
}

type Item struct {
	Title   string
	URL     string
	Summary string
	Lang    string
	Type    string
	Date    *time.Time
	Score   int
}

func (p *Plugin) Name() string {
	return "relatedposts"
}

func (p *Plugin) OnRoutesAssigned(graph *content.SiteGraph) error {
	result := make(map[string][]Item, len(graph.Documents))

	for _, doc := range graph.Documents {
		if doc == nil || doc.Draft {
			continue
		}

		candidates := p.computeRelated(graph, doc)
		result[doc.ID] = candidates
	}

	p.mu.Lock()
	p.related = result
	p.mu.Unlock()

	return nil
}

func (p *Plugin) OnContext(ctx *renderer.ViewData) error {
	if ctx.Page == nil {
		return nil
	}

	p.mu.RLock()
	items := cloneItems(p.related[ctx.Page.ID])
	p.mu.RUnlock()

	if ctx.Data == nil {
		ctx.Data = map[string]any{}
	}

	ctx.Data["related_posts"] = items
	ctx.Data["has_related_posts"] = len(items) > 0

	return nil
}

func (p *Plugin) computeRelated(graph *content.SiteGraph, current *content.Document) []Item {
	type scored struct {
		doc   *content.Document
		score int
	}

	scoredDocs := make([]scored, 0)

	for _, candidate := range graph.Documents {
		if candidate == nil || candidate.Draft {
			continue
		}
		if candidate.ID == current.ID {
			continue
		}
		if candidate.Lang != current.Lang {
			continue
		}
		if candidate.Type != current.Type {
			continue
		}

		score := scoreDocuments(current, candidate)
		if score <= 0 {
			continue
		}

		scoredDocs = append(scoredDocs, scored{
			doc:   candidate,
			score: score,
		})
	}

	sort.Slice(scoredDocs, func(i, j int) bool {
		if scoredDocs[i].score != scoredDocs[j].score {
			return scoredDocs[i].score > scoredDocs[j].score
		}

		di := scoredDocs[i].doc.Date
		dj := scoredDocs[j].doc.Date

		switch {
		case di != nil && dj != nil:
			if !di.Equal(*dj) {
				return di.After(*dj)
			}
		case di != nil && dj == nil:
			return true
		case di == nil && dj != nil:
			return false
		}

		return scoredDocs[i].doc.Title < scoredDocs[j].doc.Title
	})

	// Fallback: if nothing is related by taxonomy, pick latest same-lang same-type docs.
	if len(scoredDocs) == 0 {
		fallback := make([]*content.Document, 0)
		for _, candidate := range graph.Documents {
			if candidate == nil || candidate.Draft {
				continue
			}
			if candidate.ID == current.ID {
				continue
			}
			if candidate.Lang != current.Lang {
				continue
			}
			if candidate.Type != current.Type {
				continue
			}
			fallback = append(fallback, candidate)
		}

		sort.Slice(fallback, func(i, j int) bool {
			di := fallback[i].Date
			dj := fallback[j].Date

			switch {
			case di != nil && dj != nil:
				if !di.Equal(*dj) {
					return di.After(*dj)
				}
			case di != nil && dj == nil:
				return true
			case di == nil && dj != nil:
				return false
			}

			return fallback[i].Title < fallback[j].Title
		})

		limit := 3
		if len(fallback) < limit {
			limit = len(fallback)
		}

		items := make([]Item, 0, limit)
		for _, doc := range fallback[:limit] {
			items = append(items, Item{
				Title:   doc.Title,
				URL:     doc.URL,
				Summary: doc.Summary,
				Lang:    doc.Lang,
				Type:    doc.Type,
				Date:    doc.Date,
				Score:   0,
			})
		}

		return items
	}

	limit := 5
	if len(scoredDocs) < limit {
		limit = len(scoredDocs)
	}

	items := make([]Item, 0, limit)
	for _, entry := range scoredDocs[:limit] {
		doc := entry.doc
		items = append(items, Item{
			Title:   doc.Title,
			URL:     doc.URL,
			Summary: doc.Summary,
			Lang:    doc.Lang,
			Type:    doc.Type,
			Date:    doc.Date,
			Score:   entry.score,
		})
	}

	return items
}

func scoreDocuments(a, b *content.Document) int {
	score := 0

	for taxonomy, termsA := range a.Taxonomies {
		termsB, ok := b.Taxonomies[taxonomy]
		if !ok {
			continue
		}

		shared := countSharedTerms(termsA, termsB)
		if shared == 0 {
			continue
		}

		switch taxonomy {
		case "categories":
			score += shared * 6
		case "tags":
			score += shared * 4
		default:
			score += shared * 2
		}
	}

	return score
}

func countSharedTerms(a, b []string) int {
	set := make(map[string]struct{}, len(a))
	for _, v := range a {
		set[v] = struct{}{}
	}

	count := 0
	seen := make(map[string]struct{})
	for _, v := range b {
		if _, ok := set[v]; ok {
			if _, dup := seen[v]; !dup {
				count++
				seen[v] = struct{}{}
			}
		}
	}

	return count
}

func cloneItems(in []Item) []Item {
	if len(in) == 0 {
		return nil
	}
	out := make([]Item, len(in))
	copy(out, in)
	return out
}

func init() {
	plugins.Register("relatedposts", func() plugins.Plugin {
		return &Plugin{
			related: make(map[string][]Item),
		}
	})
}
