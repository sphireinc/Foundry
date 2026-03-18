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
	OnAssets(*ViewData, *AssetSet) error
	OnBeforeRender(*ViewData) error
	OnAfterRender(url string, html []byte) ([]byte, error)
	OnAssetsBuilding(*config.Config) error
	OnHTMLSlots(*ViewData, *Slots) error
}

type noopHooks struct{}

func (noopHooks) OnContext(*ViewData) error                           { return nil }
func (noopHooks) OnAssets(*ViewData, *AssetSet) error                 { return nil }
func (noopHooks) OnBeforeRender(*ViewData) error                      { return nil }
func (noopHooks) OnAfterRender(_ string, html []byte) ([]byte, error) { return html, nil }
func (noopHooks) OnAssetsBuilding(*config.Config) error               { return nil }
func (noopHooks) OnHTMLSlots(*ViewData, *Slots) error                 { return nil }

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

type Slots struct {
	values map[string][]template.HTML
}

func NewSlots() *Slots {
	return &Slots{
		values: make(map[string][]template.HTML),
	}
}

func (s *Slots) Add(name string, html template.HTML) {
	if s == nil || strings.TrimSpace(name) == "" || strings.TrimSpace(string(html)) == "" {
		return
	}
	s.values[name] = append(s.values[name], html)
}

func (s *Slots) Render(name string) template.HTML {
	if s == nil {
		return ""
	}
	items := s.values[name]
	if len(items) == 0 {
		return ""
	}

	var sb strings.Builder
	for _, item := range items {
		sb.WriteString(string(item))
		sb.WriteString("\n")
	}
	return template.HTML(sb.String())
}

type ScriptPosition string

const (
	ScriptPositionHead    ScriptPosition = "head"
	ScriptPositionBodyEnd ScriptPosition = "body.end"
)

type AssetSet struct {
	styles      []string
	headScripts []string
	bodyScripts []string
}

func NewAssetSet() *AssetSet {
	return &AssetSet{
		styles:      make([]string, 0),
		headScripts: make([]string, 0),
		bodyScripts: make([]string, 0),
	}
}

func (a *AssetSet) AddStyle(url string) {
	url = strings.TrimSpace(url)
	if url == "" {
		return
	}
	if !containsString(a.styles, url) {
		a.styles = append(a.styles, url)
	}
}

func (a *AssetSet) AddScript(url string, pos ScriptPosition) {
	url = strings.TrimSpace(url)
	if url == "" {
		return
	}

	switch pos {
	case ScriptPositionHead:
		if !containsString(a.headScripts, url) {
			a.headScripts = append(a.headScripts, url)
		}
	default:
		if !containsString(a.bodyScripts, url) {
			a.bodyScripts = append(a.bodyScripts, url)
		}
	}
}

func (a *AssetSet) RenderInto(slots *Slots) {
	if a == nil || slots == nil {
		return
	}

	for _, href := range a.styles {
		slots.Add("head.end", template.HTML(
			`<link rel="stylesheet" href="`+template.HTMLEscapeString(href)+`">`,
		))
	}

	for _, src := range a.headScripts {
		slots.Add("head.end", template.HTML(
			`<script src="`+template.HTMLEscapeString(src)+`"></script>`,
		))
	}

	for _, src := range a.bodyScripts {
		slots.Add("body.end", template.HTML(
			`<script src="`+template.HTMLEscapeString(src)+`"></script>`,
		))
	}
}

func containsString(values []string, target string) bool {
	for _, v := range values {
		if v == target {
			return true
		}
	}
	return false
}

func (r *Renderer) Build(ctx context.Context, graph *content.SiteGraph) error {
	_ = ctx

	if err := r.prepareBuild(true, true); err != nil {
		return err
	}

	for _, doc := range graph.Documents {
		if err := r.renderDocumentToDisk(graph, doc, false); err != nil {
			return err
		}
	}

	if err := r.buildTaxonomyArchives(ctx, graph, false, nil); err != nil {
		return err
	}

	return nil
}

func (r *Renderer) BuildURLs(ctx context.Context, graph *content.SiteGraph, urls []string) error {
	_ = ctx

	if err := r.prepareBuild(false, false); err != nil {
		return err
	}

	for _, url := range urls {
		html, err := r.RenderURL(graph, url, false)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}
		if err := r.writeRenderedURL(url, html); err != nil {
			return err
		}
	}

	return nil
}

func (r *Renderer) BuildTaxonomyArchives(ctx context.Context, graph *content.SiteGraph) error {
	return r.buildTaxonomyArchives(ctx, graph, false, nil)
}

func (r *Renderer) buildTaxonomyArchives(ctx context.Context, graph *content.SiteGraph, liveReload bool, filter map[string]struct{}) error {
	_ = ctx

	if err := r.prepareBuild(false, false); err != nil {
		return err
	}

	for _, taxonomyName := range graph.Taxonomies.OrderedNames() {
		terms := graph.Taxonomies.Values[taxonomyName]
		def := graph.Taxonomies.Definition(taxonomyName)

		for _, term := range graph.Taxonomies.OrderedTerms(taxonomyName) {
			entries := terms[term]
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

				currentURL := r.taxonomyURL(lang, taxonomyName, term)
				if !shouldBuildURL(filter, currentURL) {
					continue
				}

				html, err := r.renderTaxonomyArchive(graph, def.EffectiveTermLayout(), currentURL, taxonomyName, term, lang, docs, liveReload)
				if err != nil {
					return fmt.Errorf("render taxonomy archive %s/%s/%s: %w", lang, taxonomyName, term, err)
				}

				if err := r.writeRenderedURL(currentURL, html); err != nil {
					return fmt.Errorf("write taxonomy archive %s: %w", currentURL, err)
				}
			}
		}
	}

	return nil
}

func (r *Renderer) renderDocumentToDisk(graph *content.SiteGraph, doc *content.Document, liveReload bool) error {
	html, err := r.renderTemplate(doc.Layout, doc.URL, r.documentViewData(graph, doc, liveReload))
	if err != nil {
		return fmt.Errorf("render document %s: %w", doc.SourcePath, err)
	}

	if err := r.writeRenderedURL(doc.URL, html); err != nil {
		return fmt.Errorf("write file for %s: %w", doc.URL, err)
	}

	return nil
}

func (r *Renderer) writeRenderedURL(url string, html []byte) error {
	targetDir := filepath.Join(r.cfg.PublicDir, strings.TrimPrefix(url, "/"))
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
		return r.renderTemplate(doc.Layout, doc.URL, r.documentViewData(graph, doc, liveReload))
	}

	if urlPath == "/" {
		return r.renderTemplate("index", "/", r.indexViewData(graph, graph.Config.DefaultLang, "/", liveReload))
	}

	for lang := range graph.ByLang {
		if urlPath == "/"+lang+"/" {
			return r.renderTemplate("index", urlPath, r.indexViewData(graph, lang, urlPath, liveReload))
		}
	}

	if vd, ok := r.findTaxonomyArchive(graph, urlPath, liveReload); ok {
		vd.Nav = r.resolveNav(graph, urlPath)
		layout := graph.Taxonomies.Definition(vd.TaxonomyName).EffectiveTermLayout()
		return r.renderTemplate(layout, urlPath, vd)
	}

	return nil, os.ErrNotExist
}

func (r *Renderer) prepareBuild(cleanPublicDir, syncAssets bool) error {
	if err := r.themes.MustExist(); err != nil {
		return err
	}
	if cleanPublicDir {
		if err := os.RemoveAll(r.cfg.PublicDir); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(r.cfg.PublicDir, 0o755); err != nil {
		return err
	}
	if syncAssets {
		if err := assets.Sync(r.cfg, r.hooks); err != nil {
			return err
		}
	}
	return nil
}

func (r *Renderer) documentViewData(graph *content.SiteGraph, doc *content.Document, liveReload bool) ViewData {
	return ViewData{
		Site:       graph.Config,
		Page:       doc,
		Data:       graph.Data,
		Lang:       doc.Lang,
		Title:      doc.Title,
		LiveReload: liveReload,
		Nav:        r.resolveNav(graph, doc.URL),
	}
}

func (r *Renderer) indexViewData(graph *content.SiteGraph, lang, currentURL string, liveReload bool) ViewData {
	return ViewData{
		Site:       graph.Config,
		Data:       graph.Data,
		Lang:       lang,
		Title:      graph.Config.Title,
		Documents:  r.documentsForLang(graph, lang),
		LiveReload: liveReload,
		Nav:        r.resolveNav(graph, currentURL),
	}
}

func (r *Renderer) renderTaxonomyArchive(
	graph *content.SiteGraph,
	layout string,
	currentURL string,
	taxonomyName string,
	term string,
	lang string,
	docs []*content.Document,
	liveReload bool,
) ([]byte, error) {
	title := fmt.Sprintf("%s: %s", graph.Taxonomies.Definition(taxonomyName).DisplayTitle(lang), term)
	return r.renderTemplate(layout, currentURL, ViewData{
		Site:         graph.Config,
		Data:         graph.Data,
		Lang:         lang,
		Title:        title,
		Documents:    docs,
		LiveReload:   liveReload,
		TaxonomyName: taxonomyName,
		TaxonomyTerm: term,
		Nav:          r.resolveNav(graph, currentURL),
	})
}

func shouldBuildURL(filter map[string]struct{}, url string) bool {
	if len(filter) == 0 {
		return true
	}
	_, ok := filter[url]
	return ok
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

	def := graph.Taxonomies.Definition(taxonomyName)

	return ViewData{
		Site:         graph.Config,
		Data:         graph.Data,
		Lang:         lang,
		Title:        fmt.Sprintf("%s: %s", def.DisplayTitle(lang), term),
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

	assetSet := NewAssetSet()
	if err := r.hooks.OnAssets(&data, assetSet); err != nil {
		return nil, err
	}

	slots := NewSlots()
	assetSet.RenderInto(slots)

	if err := r.hooks.OnHTMLSlots(&data, slots); err != nil {
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
		"pluginSlot": func(name string) template.HTML {
			return slots.Render(name)
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
