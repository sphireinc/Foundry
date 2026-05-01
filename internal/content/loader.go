package content

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sphireinc/foundry/internal/config"
	"github.com/sphireinc/foundry/internal/customfields"
	"github.com/sphireinc/foundry/internal/data"
	"github.com/sphireinc/foundry/internal/fields"
	"github.com/sphireinc/foundry/internal/i18n"
	"github.com/sphireinc/foundry/internal/lifecycle"
	"github.com/sphireinc/foundry/internal/markup"
	"github.com/sphireinc/foundry/internal/theme"
)

// Hooks exposes the content-loading lifecycle to plugins and other integrators.
//
// The hook order for a full load is:
//  1. OnDataLoaded
//  2. OnGraphBuilding
//  3. For each discovered document:
//     a. OnContentDiscovered
//     b. OnFrontmatterParsed
//     c. OnMarkdownRendered
//     d. OnDocumentParsed
//  4. OnTaxonomyBuilt
//  5. OnGraphBuilt
//
// Hook implementations may mutate Document and SiteGraph values, but should do
// so carefully because later phases depend on normalized data.
type Hooks interface {
	OnContentDiscovered(path string) error
	OnFrontmatterParsed(*Document) error
	OnMarkdownRendered(*Document) error
	OnDocumentParsed(*Document) error
	OnDataLoaded(map[string]any) error
	OnGraphBuilding(*SiteGraph) error
	OnGraphBuilt(*SiteGraph) error
	OnTaxonomyBuilt(*SiteGraph) error
}

// noopHooks provides a type-safe no-op Hooks implementation.
type noopHooks struct{}

func (noopHooks) OnContentDiscovered(path string) error { _ = path; return nil }
func (noopHooks) OnFrontmatterParsed(*Document) error   { return nil }
func (noopHooks) OnMarkdownRendered(*Document) error    { return nil }
func (noopHooks) OnDocumentParsed(*Document) error      { return nil }
func (noopHooks) OnDataLoaded(map[string]any) error     { return nil }
func (noopHooks) OnGraphBuilding(*SiteGraph) error      { return nil }
func (noopHooks) OnGraphBuilt(*SiteGraph) error         { return nil }
func (noopHooks) OnTaxonomyBuilt(*SiteGraph) error      { return nil }

// Loader reads content files and data files from disk, normalizes them into
// Documents, and assembles the SiteGraph used by rendering and serving.
type Loader struct {
	cfg           *config.Config
	hooks         Hooks
	includeDrafts bool
	themeManifest *theme.Manifest
}

// NewLoader constructs a loader for the current configuration.
//
// When includeDrafts is false, draft and scheduled-unpublished documents are
// omitted from the resulting graph.
func NewLoader(cfg *config.Config, hooks Hooks, includeDrafts bool) *Loader {
	if hooks == nil {
		hooks = noopHooks{}
	}

	var manifest *theme.Manifest
	if cfg != nil {
		if loaded, err := theme.LoadManifest(cfg.ThemesDir, cfg.Theme); err == nil {
			manifest = loaded
		}
	}

	return &Loader{
		cfg:           cfg,
		hooks:         hooks,
		includeDrafts: includeDrafts,
		themeManifest: manifest,
	}
}

// Load reads content and data from disk and returns a fully assembled SiteGraph.
func (l *Loader) Load(ctx context.Context) (*SiteGraph, error) {
	_ = ctx

	graph := NewSiteGraph(l.cfg)

	store, err := data.LoadDir(l.cfg.DataDir)
	if err != nil {
		return nil, fmt.Errorf("load data dir: %w", err)
	}
	graph.Data = store.All()
	if customFieldStore, err := customfields.Load(l.cfg); err == nil {
		graph.Data["custom_fields"] = customFieldStore.Values
	} else {
		return nil, fmt.Errorf("load custom fields: %w", err)
	}

	if err := l.hooks.OnDataLoaded(graph.Data); err != nil {
		return nil, err
	}

	if err := l.hooks.OnGraphBuilding(graph); err != nil {
		return nil, err
	}

	if err := l.loadSection(graph, "page", filepath.Join(l.cfg.ContentDir, l.cfg.Content.PagesDir)); err != nil {
		return nil, err
	}
	if err := l.loadSection(graph, "post", filepath.Join(l.cfg.ContentDir, l.cfg.Content.PostsDir)); err != nil {
		return nil, err
	}

	if err := l.hooks.OnTaxonomyBuilt(graph); err != nil {
		return nil, err
	}

	if err := l.hooks.OnGraphBuilt(graph); err != nil {
		return nil, err
	}

	return graph, nil
}

// loadSection walks a content section root and adds valid Markdown documents to
// the graph.
func (l *Loader) loadSection(graph *SiteGraph, docType, root string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("walk section: %w", err)
		}
		if info.IsDir() || filepath.Ext(path) != ".md" || lifecycle.IsDerivedPath(path) {
			return nil
		}

		if err := l.hooks.OnContentDiscovered(path); err != nil {
			return err
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}

		lang, relDocPath, isDefault := l.resolveLanguage(rel)
		doc, err := l.loadDocument(path, relDocPath, lang, isDefault, docType)
		if err != nil {
			return err
		}
		if doc == nil {
			return nil
		}

		if doc.Draft && !l.includeDrafts {
			return nil
		}

		if err := l.hooks.OnDocumentParsed(doc); err != nil {
			return err
		}

		graph.Add(doc)
		return nil
	})
}

// resolveLanguage splits a section-relative path into language and document path
// components using Foundry's language directory convention.
func (l *Loader) resolveLanguage(rel string) (lang, relDocPath string, isDefault bool) {
	return i18n.SplitLeadingLang(rel, l.cfg.DefaultLang)
}

// loadDocument reads, parses, normalizes, and renders a single Markdown file
// into a Document.
func (l *Loader) loadDocument(path, relPath, lang string, isDefault bool, docType string) (*Document, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read document %s: %w", path, err)
	}

	fm, body, err := ParseDocument(b)
	if err != nil {
		return nil, fmt.Errorf("parse document %s: %w", path, err)
	}

	slug := fm.Slug
	if slug == "" {
		base := filepath.Base(relPath)
		slug = strings.TrimSuffix(base, filepath.Ext(base))
	}

	layout := fm.Layout
	if layout == "" {
		if docType == "post" {
			layout = l.cfg.Content.DefaultLayoutPost
		} else {
			layout = l.cfg.Content.DefaultLayoutPage
		}
	}

	taxes := make(map[string][]string)
	if len(fm.Tags) > 0 {
		taxes["tags"] = append([]string{}, fm.Tags...)
	}
	if len(fm.Categories) > 0 {
		taxes["categories"] = append([]string{}, fm.Categories...)
	}
	for k, v := range fm.Taxonomies {
		taxes[k] = append([]string{}, v...)
	}

	doc := &Document{
		ID:         lang + ":" + docType + ":" + strings.TrimSuffix(relPath, ".md"),
		Type:       docType,
		Lang:       lang,
		IsDefault:  isDefault,
		Title:      fm.Title,
		Slug:       slug,
		Layout:     layout,
		SourcePath: filepath.ToSlash(path),
		RelPath:    relPath,
		RawBody:    body,
		Summary:    buildSummary(fm.Summary, body),
		Date:       fm.Date,
		CreatedAt:  fm.CreatedAt,
		UpdatedAt:  fm.UpdatedAt,
		Draft:      fm.Draft,
		Author:     strings.TrimSpace(fm.Author),
		LastEditor: strings.TrimSpace(fm.LastEditor),
		Params:     fm.Params,
		Fields:     fields.ApplyDefaults(fields.Normalize(fm.Fields), l.fieldDefinitionsForDocument(docType, layout, slug)),
		Taxonomies: taxes,
	}
	workflow := WorkflowFromFrontMatter(fm, time.Now().UTC())
	doc.Status = workflow.Status
	if workflow.Status == "scheduled" && !l.includeDrafts {
		return nil, nil
	}
	if workflow.ScheduledUnpublish != nil && time.Now().UTC().After(*workflow.ScheduledUnpublish) && !l.includeDrafts {
		return nil, nil
	}

	if doc.Title == "" {
		doc.Title = slug
	}

	if err := l.hooks.OnFrontmatterParsed(doc); err != nil {
		return nil, err
	}

	htmlBody, err := markup.MarkdownToHTML(doc.RawBody, l.cfg.Security.AllowUnsafeHTML)
	if err != nil {
		return nil, fmt.Errorf("render markdown %s: %w", path, err)
	}
	doc.HTMLBody = htmlBody

	if err := l.hooks.OnMarkdownRendered(doc); err != nil {
		return nil, err
	}

	return doc, nil
}

func (l *Loader) fieldDefinitionsForDocument(docType, layout, slug string) []fields.Definition {
	defs := theme.DocumentFieldDefinitions(l.cfg.ThemesDir, l.cfg.Theme, docType, layout, slug)
	if len(defs) > 0 {
		return defs
	}
	return fields.DefinitionsFor(l.cfg, docType)
}

func buildSummary(explicit, body string) string {
	if strings.TrimSpace(explicit) != "" {
		return strings.TrimSpace(explicit)
	}
	return ""
}
