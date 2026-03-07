package content

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/sphireinc/foundry/internal/config"
	"github.com/sphireinc/foundry/internal/data"
	"github.com/sphireinc/foundry/internal/fields"
	"github.com/sphireinc/foundry/internal/markup"
)

type Hooks interface {
	OnDocumentParsed(*Document) error
	OnGraphBuilt(*SiteGraph) error
}

// these below are purley for type safety
type noopHooks struct{}

func (noopHooks) OnDocumentParsed(*Document) error { return nil }
func (noopHooks) OnGraphBuilt(*SiteGraph) error    { return nil }

type Loader struct {
	cfg           *config.Config
	hooks         Hooks
	includeDrafts bool
}

func NewLoader(cfg *config.Config, hooks Hooks, includeDrafts bool) *Loader {
	if hooks == nil {
		hooks = noopHooks{}
	}

	return &Loader{
		cfg:           cfg,
		hooks:         hooks,
		includeDrafts: includeDrafts,
	}
}

func (l *Loader) Load(ctx context.Context) (*SiteGraph, error) {
	_ = ctx

	graph := NewSiteGraph(l.cfg)

	store, err := data.LoadDir(l.cfg.DataDir)
	if err != nil {
		return nil, fmt.Errorf("load data dir: %w", err)
	}
	graph.Data = store.All()

	if err := l.loadSection(graph, "page", filepath.Join(l.cfg.ContentDir, l.cfg.Content.PagesDir)); err != nil {
		return nil, err
	}
	if err := l.loadSection(graph, "post", filepath.Join(l.cfg.ContentDir, l.cfg.Content.PostsDir)); err != nil {
		return nil, err
	}

	if err := l.hooks.OnGraphBuilt(graph); err != nil {
		return nil, err
	}

	return graph, nil
}

func (l *Loader) loadSection(graph *SiteGraph, docType, root string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("walk section: %w", err)
		}
		if info.IsDir() || filepath.Ext(path) != ".md" {
			return nil
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

func (l *Loader) resolveLanguage(rel string) (lang, relDocPath string, isDefault bool) {
	parts := strings.Split(filepath.ToSlash(rel), "/")
	if len(parts) > 1 && len(parts[0]) == 2 {
		return parts[0], strings.Join(parts[1:], "/"), false
	}
	return l.cfg.DefaultLang, filepath.ToSlash(rel), true
}

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

	htmlBody, err := markup.MarkdownToHTML(body)
	if err != nil {
		return nil, fmt.Errorf("render markdown %s: %w", path, err)
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
		HTMLBody:   htmlBody,
		Summary:    buildSummary(fm.Summary, body),
		Date:       fm.Date,
		UpdatedAt:  fm.UpdatedAt,
		Draft:      fm.Draft,
		Params:     fm.Params,
		Fields:     fields.Normalize(fm.Fields),
		Taxonomies: taxes,
	}

	if doc.Title == "" {
		doc.Title = slug
	}

	return doc, nil
}

func buildSummary(explicit, body string) string {
	if strings.TrimSpace(explicit) != "" {
		return strings.TrimSpace(explicit)
	}

	body = strings.TrimSpace(body)
	body = strings.ReplaceAll(body, "\n", " ")
	body = strings.ReplaceAll(body, "\r", " ")
	body = strings.Join(strings.Fields(body), " ")

	const maxLen = 180
	if utf8.RuneCountInString(body) <= maxLen {
		return body
	}

	runes := []rune(body)
	return strings.TrimSpace(string(runes[:maxLen])) + "..."
}
