package renderer

import (
	"context"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sphireinc/foundry/internal/assets"
	"github.com/sphireinc/foundry/internal/config"
	"github.com/sphireinc/foundry/internal/content"
	"github.com/sphireinc/foundry/internal/theme"
)

type Hooks interface {
	OnContext(*ViewData) error
	OnBeforeRender(*ViewData) error
	OnAfterRender(url string, html []byte) ([]byte, error)
	OnAssetsBuilding(*config.Config) error
}

type noopHooks struct{}

func (noopHooks) OnContext(*ViewData) error                           { return nil }
func (noopHooks) OnBeforeRender(*ViewData) error                      { return nil }
func (noopHooks) OnAfterRender(_ string, html []byte) ([]byte, error) { return html, nil }
func (noopHooks) OnAssetsBuilding(*config.Config) error               { return nil }

type Renderer struct {
	cfg    *config.Config
	themes *theme.Manager
	hooks  Hooks
}

func New(cfg *config.Config, themes *theme.Manager, hooks Hooks) *Renderer {
	if hooks == nil {
		hooks = noopHooks{}
	}

	return &Renderer{
		cfg:    cfg,
		themes: themes,
		hooks:  hooks,
	}
}

type NavItem struct {
	Name   string
	URL    string
	Active bool
}

type ViewData struct {
	Site         *config.Config
	Page         *content.Document
	Documents    []*content.Document
	Data         map[string]any
	Lang         string
	Title        string
	LiveReload   bool
	TaxonomyName string
	TaxonomyTerm string
	Nav          []NavItem
}

func (r *Renderer) Build(ctx context.Context, graph *content.SiteGraph) error {
	_ = ctx

	if err := r.themes.MustExist(); err != nil {
		return err
	}

	if r.cfg.Build.CleanPublicDir {
		if err := os.RemoveAll(r.cfg.PublicDir); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(r.cfg.PublicDir, 0o755); err != nil {
		return err
	}
	if err := assets.Sync(r.cfg, r.hooks); err != nil {
		return err
	}

	for _, doc := range graph.Documents {
		if err := r.buildSingle(graph, doc); err != nil {
			return err
		}
	}

	if err := r.BuildTaxonomyArchives(ctx, graph); err != nil {
		return err
	}

	return nil
}

func (r *Renderer) BuildURLs(ctx context.Context, graph *content.SiteGraph, urls []string) error {
	_ = ctx

	if err := r.themes.MustExist(); err != nil {
		return err
	}

	for _, url := range urls {
		doc, ok := graph.ByURL[url]
		if !ok {
			continue
		}
		if err := r.buildSingle(graph, doc); err != nil {
			return err
		}
	}

	return nil
}

func (r *Renderer) BuildTaxonomyArchives(ctx context.Context, graph *content.SiteGraph) error {
	_ = ctx

	if err := r.themes.MustExist(); err != nil {
		return err
	}

	for taxonomyName, terms := range graph.Taxonomies.Values {
		for term, entries := range terms {
			byLang := make(map[string][]*content.Document)

			for _, entry := range entries {
				doc, ok := r.findDocumentByID(graph, entry.DocumentID)
				if !ok || doc.Draft {
					continue
				}
				byLang[doc.Lang] = append(byLang[doc.Lang], doc)
			}

			for lang, docs := range byLang {
				sort.Slice(docs, func(i, j int) bool {
					return docs[i].URL < docs[j].URL
				})

				title := fmt.Sprintf("%s: %s", taxonomyName, term)
				currentURL := r.taxonomyURL(lang, taxonomyName, term)

				html, err := r.renderTemplate("list", currentURL, ViewData{
					Site:         graph.Config,
					Data:         graph.Data,
					Lang:         lang,
					Title:        title,
					Documents:    docs,
					TaxonomyName: taxonomyName,
					TaxonomyTerm: term,
					Nav:          r.resolveNav(graph, currentURL),
				})
				if err != nil {
					return fmt.Errorf("render taxonomy archive %s/%s/%s: %w", lang, taxonomyName, term, err)
				}

				targetDir := filepath.Join(r.cfg.PublicDir, strings.TrimPrefix(currentURL, "/"))
				if err := os.MkdirAll(targetDir, 0o755); err != nil {
					return fmt.Errorf("mkdir taxonomy target %s: %w", targetDir, err)
				}

				targetFile := filepath.Join(targetDir, "index.html")
				if err := os.WriteFile(targetFile, html, 0o644); err != nil {
					return fmt.Errorf("write taxonomy archive %s: %w", targetFile, err)
				}
			}
		}
	}

	return nil
}

func (r *Renderer) buildSingle(graph *content.SiteGraph, doc *content.Document) error {
	html, err := r.renderTemplate(doc.Layout, doc.URL, ViewData{
		Site:  graph.Config,
		Page:  doc,
		Data:  graph.Data,
		Lang:  doc.Lang,
		Title: doc.Title,
		Nav:   r.resolveNav(graph, doc.URL),
	})
	if err != nil {
		return fmt.Errorf("render document %s: %w", doc.SourcePath, err)
	}

	targetDir := filepath.Join(r.cfg.PublicDir, strings.TrimPrefix(doc.URL, "/"))
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return fmt.Errorf("mkdir target %s: %w", targetDir, err)
	}

	targetFile := filepath.Join(targetDir, "index.html")
	if err := os.WriteFile(targetFile, html, 0o644); err != nil {
		return fmt.Errorf("write file %s: %w", targetFile, err)
	}

	return nil
}

func (r *Renderer) RenderURL(graph *content.SiteGraph, urlPath string, liveReload bool) ([]byte, error) {
	if doc, ok := graph.ByURL[urlPath]; ok {
		return r.renderTemplate(doc.Layout, doc.URL, ViewData{
			Site:       graph.Config,
			Page:       doc,
			Data:       graph.Data,
			Lang:       doc.Lang,
			Title:      doc.Title,
			LiveReload: liveReload,
			Nav:        r.resolveNav(graph, doc.URL),
		})
	}

	if urlPath == "/" {
		return r.renderTemplate("index", "/", ViewData{
			Site:       graph.Config,
			Data:       graph.Data,
			Lang:       graph.Config.DefaultLang,
			Title:      graph.Config.Title,
			Documents:  r.documentsForLang(graph, graph.Config.DefaultLang),
			LiveReload: liveReload,
			Nav:        r.resolveNav(graph, "/"),
		})
	}

	for lang := range graph.ByLang {
		if urlPath == "/"+lang+"/" {
			return r.renderTemplate("index", urlPath, ViewData{
				Site:       graph.Config,
				Data:       graph.Data,
				Lang:       lang,
				Title:      graph.Config.Title,
				Documents:  r.documentsForLang(graph, lang),
				LiveReload: liveReload,
				Nav:        r.resolveNav(graph, urlPath),
			})
		}
	}

	if vd, ok := r.findTaxonomyArchive(graph, urlPath, liveReload); ok {
		vd.Nav = r.resolveNav(graph, urlPath)
		return r.renderTemplate("list", urlPath, vd)
	}

	return nil, os.ErrNotExist
}

func (r *Renderer) findTaxonomyArchive(graph *content.SiteGraph, urlPath string, liveReload bool) (ViewData, bool) {
	clean := strings.Trim(urlPath, "/")
	if clean == "" {
		return ViewData{}, false
	}

	parts := strings.Split(clean, "/")
	var lang, taxonomyName, term string

	switch len(parts) {
	case 2:
		lang = r.cfg.DefaultLang
		taxonomyName = parts[0]
		term = parts[1]
	case 3:
		lang = parts[0]
		taxonomyName = parts[1]
		term = parts[2]
	default:
		return ViewData{}, false
	}

	taxonomyTerms, ok := graph.Taxonomies.Values[taxonomyName]
	if !ok {
		return ViewData{}, false
	}

	entries, ok := taxonomyTerms[term]
	if !ok {
		return ViewData{}, false
	}

	docs := make([]*content.Document, 0)
	for _, entry := range entries {
		if entry.Lang != lang {
			continue
		}
		doc, ok := r.findDocumentByID(graph, entry.DocumentID)
		if !ok || doc.Draft {
			continue
		}
		docs = append(docs, doc)
	}

	if len(docs) == 0 {
		return ViewData{}, false
	}

	sort.Slice(docs, func(i, j int) bool {
		return docs[i].URL < docs[j].URL
	})

	return ViewData{
		Site:         graph.Config,
		Data:         graph.Data,
		Lang:         lang,
		Title:        fmt.Sprintf("%s: %s", taxonomyName, term),
		Documents:    docs,
		LiveReload:   liveReload,
		TaxonomyName: taxonomyName,
		TaxonomyTerm: term,
	}, true
}

func (r *Renderer) taxonomyURL(lang, taxonomyName, term string) string {
	if lang == "" || lang == r.cfg.DefaultLang {
		return fmt.Sprintf("/%s/%s/", taxonomyName, term)
	}
	return fmt.Sprintf("/%s/%s/%s/", lang, taxonomyName, term)
}

func (r *Renderer) findDocumentByID(graph *content.SiteGraph, id string) (*content.Document, bool) {
	for _, doc := range graph.Documents {
		if doc.ID == id {
			return doc, true
		}
	}
	return nil, false
}

func (r *Renderer) documentsForLang(graph *content.SiteGraph, lang string) []*content.Document {
	docs := make([]*content.Document, 0, len(graph.ByLang[lang]))
	for _, doc := range graph.ByLang[lang] {
		docs = append(docs, doc)
	}
	sort.Slice(docs, func(i, j int) bool {
		return docs[i].URL < docs[j].URL
	})
	return docs
}

func (r *Renderer) resolveNav(graph *content.SiteGraph, currentURL string) []NavItem {
	var base []NavItem

	if len(r.cfg.Menus["main"]) > 0 {
		base = make([]NavItem, 0, len(r.cfg.Menus["main"]))
		for _, item := range r.cfg.Menus["main"] {
			base = append(base, NavItem{
				Name: item.Name,
				URL:  normalizeNavURL(item.URL),
			})
		}
	} else if graph != nil && graph.Data != nil {
		if nav := parseNavigationData(graph.Data["navigation"]); len(nav) > 0 {
			base = nav
		}
	}

	if len(base) == 0 {
		base = []NavItem{
			{Name: "Home", URL: "/"},
			{Name: "Sample Post", URL: "/posts/hello-world/"},
			{Name: "Tags", URL: "/tags/go/"},
			{Name: "RSS", URL: normalizeNavURL(r.cfg.Feed.RSSPath)},
		}
	}

	currentURL = normalizeNavURL(currentURL)

	out := make([]NavItem, 0, len(base))
	for _, item := range base {
		item.Active = navItemIsActive(item.URL, currentURL)
		out = append(out, item)
	}

	return out
}

func normalizeNavURL(u string) string {
	u = strings.TrimSpace(u)
	if u == "" {
		return "/"
	}
	if !strings.HasPrefix(u, "/") && !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
		u = "/" + u
	}
	if strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://") {
		return u
	}
	if u != "/" && !strings.Contains(filepath.Base(u), ".") && !strings.HasSuffix(u, "/") {
		u += "/"
	}
	return u
}

func navItemIsActive(itemURL, currentURL string) bool {
	if itemURL == "" || currentURL == "" {
		return false
	}

	if itemURL == currentURL {
		return true
	}

	if strings.HasPrefix(itemURL, "http://") || strings.HasPrefix(itemURL, "https://") {
		return false
	}

	if itemURL == "/" {
		return currentURL == "/"
	}

	return strings.HasPrefix(currentURL, itemURL)
}

func parseNavigationData(v any) []NavItem {
	root, ok := v.(map[string]any)
	if !ok {
		return nil
	}

	raw, ok := root["main"]
	if !ok {
		return nil
	}

	list, ok := raw.([]any)
	if !ok {
		return nil
	}

	items := make([]NavItem, 0, len(list))
	for _, entry := range list {
		m, ok := entry.(map[string]any)
		if !ok {
			continue
		}

		name, _ := m["name"].(string)
		url, _ := m["url"].(string)
		if strings.TrimSpace(name) == "" || strings.TrimSpace(url) == "" {
			continue
		}

		items = append(items, NavItem{
			Name: name,
			URL:  normalizeNavURL(url),
		})
	}

	return items
}

func (r *Renderer) renderTemplate(name string, targetURL string, data ViewData) ([]byte, error) {
	if err := r.hooks.OnContext(&data); err != nil {
		return nil, err
	}

	if err := r.hooks.OnBeforeRender(&data); err != nil {
		return nil, err
	}

	basePath := r.themes.LayoutPath("base")
	pagePath := r.themes.LayoutPath(name)

	partials, err := filepath.Glob(filepath.Join(r.cfg.ThemesDir, r.cfg.Theme, "layouts", "partials", "*.html"))
	if err != nil {
		return nil, fmt.Errorf("glob partials: %w", err)
	}

	files := []string{basePath, pagePath}
	files = append(files, partials...)

	tmpl, err := template.New("base.html").Funcs(template.FuncMap{
		"safeHTML": func(v any) template.HTML {
			if h, ok := v.(template.HTML); ok {
				return h
			}
			return ""
		},
		"field": func(doc *content.Document, key string) any {
			if doc == nil || doc.Fields == nil {
				return nil
			}
			return doc.Fields[key]
		},
		"data": func(key string) any {
			if data.Data == nil {
				return nil
			}
			return data.Data[key]
		},
	}).ParseFiles(files...)
	if err != nil {
		return nil, fmt.Errorf("parse templates: %w", err)
	}

	var sb strings.Builder
	if err := tmpl.ExecuteTemplate(&sb, "base", data); err != nil {
		return nil, fmt.Errorf("execute template: %w", err)
	}

	html := []byte(sb.String())

	html, err = r.hooks.OnAfterRender(targetURL, html)
	if err != nil {
		return nil, err
	}

	return html, nil
}
