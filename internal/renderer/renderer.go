package renderer

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/sphireinc/foundry/internal/assets"
	"github.com/sphireinc/foundry/internal/config"
	"github.com/sphireinc/foundry/internal/content"
	"github.com/sphireinc/foundry/internal/platformapi"
	"github.com/sphireinc/foundry/internal/theme"
)

var stripHTMLTagsRE = regexp.MustCompile(`<[^>]+>`)

// Hooks lets plugins participate in the render pipeline.
//
// The hook order for a single page render is:
//  1. OnContext
//  2. OnAssets
//  3. OnHTMLSlots
//  4. OnBeforeRender
//  5. >> template execution
//  6. OnAfterRender
//
// Implementations should keep work fast and deterministic because these hooks
// run for every rendered output.
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

// Renderer turns the site graph into HTML output using the active frontend
// theme and the provided render hooks.
type Renderer struct {
	cfg    *config.Config
	themes *theme.Manager
	hooks  Hooks
}

// BuildStats records coarse timing breakdowns for a render/build pass.
type BuildStats struct {
	Prepare    time.Duration
	Assets     time.Duration
	Documents  time.Duration
	Taxonomies time.Duration
	Search     time.Duration
}

// New constructs a renderer for the active theme.
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

// NavItem is a normalized menu item exposed to templates.
type NavItem struct {
	Name   string
	URL    string
	Active bool
}

// ViewData is the template context passed to frontend theme layouts.
//
// Theme authors can rely on these fields in templates, and render hooks may
// enrich or modify them before execution.
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
	AuthorName   string
	SearchQuery  string
	RequestPath  string
	StatusCode   int
	Nav          []NavItem
}

// Slots collects named HTML fragments that themes expose via pluginSlot in
// templates.
//
// Asset hooks and HTML slot hooks populate Slots before template execution.
type Slots struct {
	values map[string][]template.HTML
}

// NewSlots creates an empty slot registry for a single render pass.
func NewSlots() *Slots {
	return &Slots{
		values: make(map[string][]template.HTML),
	}
}

// Add appends an HTML fragment to a named slot.
//
// Slot names should match the theme manifest's declared slots. Empty names and
// empty HTML fragments are ignored.
func (s *Slots) Add(name string, html template.HTML) {
	if s == nil || strings.TrimSpace(name) == "" || strings.TrimSpace(string(html)) == "" {
		return
	}
	s.values[name] = append(s.values[name], html)
}

// Render concatenates all fragments for a slot in insertion order.
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

// ScriptPosition controls where a script asset is rendered in theme slots.
type ScriptPosition string

const (
	ScriptPositionHead    ScriptPosition = "head"
	ScriptPositionBodyEnd ScriptPosition = "body.end"
)

// AssetSet accumulates CSS and JS assets for a single render pass.
//
// Asset hooks append to the set, and the renderer publishes them into standard
// theme slots such as head.end and body.end.
type AssetSet struct {
	styles      []string
	headScripts []string
	bodyScripts []string
}

// NewAssetSet creates an empty asset accumulator for a render pass.
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
	_, err := r.BuildWithStats(ctx, graph)
	return err
}

func (r *Renderer) BuildWithStats(ctx context.Context, graph *content.SiteGraph) (BuildStats, error) {
	_ = ctx
	var stats BuildStats
	r.syncActiveTheme()

	if err := r.prepareBuild(true, true, &stats); err != nil {
		return stats, err
	}

	start := time.Now()
	for _, doc := range graph.Documents {
		if err := r.renderDocumentToDisk(graph, doc, false); err != nil {
			return stats, err
		}
	}
	stats.Documents = time.Since(start)

	start = time.Now()
	if err := r.buildTaxonomyArchives(ctx, graph, false, nil); err != nil {
		return stats, err
	}
	stats.Taxonomies = time.Since(start)

	if err := r.buildCoreRoutes(graph, false, nil); err != nil {
		return stats, err
	}

	start = time.Now()
	if err := r.writeSearchIndex(graph); err != nil {
		return stats, err
	}
	stats.Search = time.Since(start)

	if err := platformapi.WriteStaticArtifacts(r.cfg, graph); err != nil {
		return stats, err
	}

	return stats, nil
}

func (r *Renderer) BuildURLs(ctx context.Context, graph *content.SiteGraph, urls []string) error {
	_ = ctx

	if err := r.prepareBuild(false, false, nil); err != nil {
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

	if err := r.buildCoreRoutes(graph, false, nil); err != nil {
		return err
	}

	if err := r.writeSearchIndex(graph); err != nil {
		return err
	}
	if err := platformapi.WriteStaticArtifacts(r.cfg, graph); err != nil {
		return err
	}

	return nil
}

func (r *Renderer) BuildTaxonomyArchives(ctx context.Context, graph *content.SiteGraph) error {
	return r.buildTaxonomyArchives(ctx, graph, false, nil)
}

func (r *Renderer) buildTaxonomyArchives(ctx context.Context, graph *content.SiteGraph, liveReload bool, filter map[string]struct{}) error {
	_ = ctx

	if err := r.prepareBuild(false, false, nil); err != nil {
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
	return r.RenderURLWithQuery(graph, urlPath, "", liveReload)
}

func (r *Renderer) RenderURLWithQuery(graph *content.SiteGraph, urlPath string, rawQuery string, liveReload bool) ([]byte, error) {
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

	if vd, ok := r.findSearchPage(graph, urlPath, rawQuery, liveReload); ok {
		vd.Nav = r.resolveNav(graph, urlPath)
		return r.renderTemplate("search", urlPath, vd)
	}

	if vd, ok := r.findAuthorArchive(graph, urlPath, liveReload); ok {
		vd.Nav = r.resolveNav(graph, urlPath)
		return r.renderTemplate("author", urlPath, vd)
	}

	return nil, os.ErrNotExist
}

type searchIndexEntry struct {
	Title      string              `json:"title"`
	URL        string              `json:"url"`
	Summary    string              `json:"summary,omitempty"`
	Snippet    string              `json:"snippet,omitempty"`
	Content    string              `json:"content,omitempty"`
	Type       string              `json:"type"`
	Lang       string              `json:"lang"`
	Layout     string              `json:"layout,omitempty"`
	Tags       []string            `json:"tags,omitempty"`
	Categories []string            `json:"categories,omitempty"`
	Taxonomies map[string][]string `json:"taxonomies,omitempty"`
}

func (r *Renderer) writeSearchIndex(graph *content.SiteGraph) error {
	if r == nil || r.cfg == nil || graph == nil {
		return nil
	}
	items := make([]searchIndexEntry, 0, len(graph.Documents))
	for _, doc := range graph.Documents {
		if doc == nil || doc.Draft || documentArchived(doc) {
			continue
		}
		items = append(items, searchIndexEntry{
			Title:      doc.Title,
			URL:        doc.URL,
			Summary:    doc.Summary,
			Snippet:    buildSearchSnippet(doc.Summary, normalizeSearchContent(doc)),
			Content:    normalizeSearchContent(doc),
			Type:       doc.Type,
			Lang:       doc.Lang,
			Layout:     doc.Layout,
			Tags:       append([]string{}, doc.Taxonomies["tags"]...),
			Categories: append([]string{}, doc.Taxonomies["categories"]...),
			Taxonomies: cloneTaxonomies(doc.Taxonomies),
		})
	}
	body, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal search index: %w", err)
	}
	path := filepath.Join(r.cfg.PublicDir, "search.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir search index dir: %w", err)
	}
	if err := os.WriteFile(path, append(body, '\n'), 0o644); err != nil {
		return fmt.Errorf("write search index: %w", err)
	}
	return nil
}

func normalizeSearchContent(doc *content.Document) string {
	if doc == nil {
		return ""
	}
	text := strings.TrimSpace(doc.RawBody)
	if text == "" {
		text = strings.TrimSpace(stripHTMLTagsRE.ReplaceAllString(string(doc.HTMLBody), " "))
	}
	return strings.Join(strings.Fields(text), " ")
}

func buildSearchSnippet(summary, content string) string {
	summary = strings.TrimSpace(summary)
	if summary != "" {
		return summary
	}
	runes := []rune(strings.TrimSpace(content))
	if len(runes) <= 180 {
		return string(runes)
	}
	return strings.TrimSpace(string(runes[:180])) + "..."
}

func cloneTaxonomies(in map[string][]string) map[string][]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string][]string, len(in))
	for key, values := range in {
		out[key] = append([]string{}, values...)
	}
	return out
}

func documentArchived(doc *content.Document) bool {
	if doc == nil || doc.Params == nil {
		return false
	}
	value, ok := doc.Params["archived"]
	if !ok {
		return false
	}
	flag, ok := value.(bool)
	return ok && flag
}

func (r *Renderer) prepareBuild(cleanPublicDir, syncAssets bool, stats *BuildStats) error {
	start := time.Now()
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
		assetsStart := time.Now()
		if err := assets.Sync(r.cfg, r.hooks); err != nil {
			return err
		}
		if stats != nil {
			stats.Assets = time.Since(assetsStart)
		}
	}
	if stats != nil {
		stats.Prepare = time.Since(start)
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

func (r *Renderer) buildCoreRoutes(graph *content.SiteGraph, liveReload bool, filter map[string]struct{}) error {
	if graph == nil {
		return nil
	}

	searchLangs := r.knownLangs(graph)
	for _, lang := range searchLangs {
		searchURL := content.SearchPageURL(r.cfg.DefaultLang, lang)
		if !shouldBuildURL(filter, searchURL) {
			continue
		}
		html, err := r.renderTemplate("search", searchURL, r.searchViewData(graph, lang, "", searchURL, liveReload))
		if err != nil {
			return fmt.Errorf("render search page %s: %w", searchURL, err)
		}
		if err := r.writeRenderedURL(searchURL, html); err != nil {
			return fmt.Errorf("write search page %s: %w", searchURL, err)
		}
	}

	for _, author := range r.authorArchives(graph) {
		currentURL := content.AuthorArchiveURL(r.cfg.DefaultLang, author.lang, author.name)
		if currentURL == "" || !shouldBuildURL(filter, currentURL) {
			continue
		}
		html, err := r.renderTemplate("author", currentURL, ViewData{
			Site:       graph.Config,
			Data:       graph.Data,
			Lang:       author.lang,
			Title:      fmt.Sprintf("Author: %s", author.name),
			Documents:  author.documents,
			LiveReload: liveReload,
			AuthorName: author.name,
			Nav:        r.resolveNav(graph, currentURL),
		})
		if err != nil {
			return fmt.Errorf("render author archive %s: %w", currentURL, err)
		}
		if err := r.writeRenderedURL(currentURL, html); err != nil {
			return fmt.Errorf("write author archive %s: %w", currentURL, err)
		}
	}

	if len(filter) == 0 || shouldBuildURL(filter, "/404/") || shouldBuildURL(filter, "/404.html") {
		html, err := r.RenderNotFoundPage(graph, "/", false)
		if err != nil {
			return fmt.Errorf("render 404 page: %w", err)
		}
		if err := r.writeNotFoundPage(html); err != nil {
			return fmt.Errorf("write 404 page: %w", err)
		}
	}

	return nil
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

func (r *Renderer) RenderNotFoundPage(graph *content.SiteGraph, requestPath string, liveReload bool) ([]byte, error) {
	lang := r.langForPath(graph, requestPath)
	return r.renderTemplate("404", requestPath, ViewData{
		Site:        graph.Config,
		Data:        graph.Data,
		Lang:        lang,
		Title:       "Page not found",
		Documents:   r.documentsForLang(graph, lang),
		LiveReload:  liveReload,
		RequestPath: requestPath,
		StatusCode:  http.StatusNotFound,
		Nav:         r.resolveNav(graph, requestPath),
	})
}

func (r *Renderer) writeNotFoundPage(html []byte) error {
	path := filepath.Join(r.cfg.PublicDir, "404.html")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir 404 dir: %w", err)
	}
	if err := os.WriteFile(path, html, 0o644); err != nil {
		return fmt.Errorf("write 404 file: %w", err)
	}
	return nil
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

func (r *Renderer) findSearchPage(graph *content.SiteGraph, urlPath string, rawQuery string, liveReload bool) (ViewData, bool) {
	lang := r.searchLangForPath(graph, urlPath)
	if lang == "" {
		return ViewData{}, false
	}

	queryValues, _ := url.ParseQuery(rawQuery)
	query := strings.TrimSpace(queryValues.Get("q"))
	return r.searchViewData(graph, lang, query, urlPath, liveReload), true
}

func (r *Renderer) searchViewData(graph *content.SiteGraph, lang, query, currentURL string, liveReload bool) ViewData {
	docs := r.documentsForSearch(graph, lang, query)
	title := "Search"
	if query != "" {
		title = fmt.Sprintf("Search: %s", query)
	}
	return ViewData{
		Site:        graph.Config,
		Data:        graph.Data,
		Lang:        lang,
		Title:       title,
		Documents:   docs,
		LiveReload:  liveReload,
		SearchQuery: query,
	}
}

func (r *Renderer) findAuthorArchive(graph *content.SiteGraph, urlPath string, liveReload bool) (ViewData, bool) {
	clean := strings.Trim(urlPath, "/")
	if clean == "" {
		return ViewData{}, false
	}

	parts := strings.Split(clean, "/")
	var lang, slug string
	switch len(parts) {
	case 2:
		if parts[0] != "authors" {
			return ViewData{}, false
		}
		lang = r.cfg.DefaultLang
		slug = parts[1]
	case 3:
		if parts[1] != "authors" {
			return ViewData{}, false
		}
		lang = parts[0]
		slug = parts[2]
	default:
		return ViewData{}, false
	}

	for _, author := range r.authorArchives(graph) {
		if author.lang != lang || content.AuthorSlug(author.name) != slug {
			continue
		}
		return ViewData{
			Site:       graph.Config,
			Data:       graph.Data,
			Lang:       lang,
			Title:      fmt.Sprintf("Author: %s", author.name),
			Documents:  author.documents,
			LiveReload: liveReload,
			AuthorName: author.name,
		}, true
	}

	return ViewData{}, false
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

func (r *Renderer) documentsForSearch(graph *content.SiteGraph, lang, query string) []*content.Document {
	docs := make([]*content.Document, 0)
	needle := strings.ToLower(strings.TrimSpace(query))
	for _, doc := range graph.Documents {
		if doc == nil || doc.Draft || documentArchived(doc) {
			continue
		}
		if lang != "" && doc.Lang != lang {
			continue
		}
		if needle != "" {
			haystack := strings.ToLower(strings.Join([]string{
				doc.Title,
				doc.Slug,
				doc.URL,
				doc.Summary,
				normalizeSearchContent(doc),
			}, " "))
			if !strings.Contains(haystack, needle) {
				continue
			}
		}
		docs = append(docs, doc)
	}
	sort.Slice(docs, func(i, j int) bool {
		return docs[i].URL < docs[j].URL
	})
	return docs
}

type authorArchive struct {
	name      string
	lang      string
	documents []*content.Document
}

func (r *Renderer) authorArchives(graph *content.SiteGraph) []authorArchive {
	type key struct {
		lang string
		name string
	}
	grouped := make(map[key][]*content.Document)
	for _, doc := range graph.Documents {
		if doc == nil || doc.Draft || documentArchived(doc) {
			continue
		}
		name := strings.TrimSpace(doc.Author)
		if name == "" {
			continue
		}
		k := key{lang: doc.Lang, name: name}
		grouped[k] = append(grouped[k], doc)
	}

	out := make([]authorArchive, 0, len(grouped))
	for k, docs := range grouped {
		sort.Slice(docs, func(i, j int) bool {
			return docs[i].URL < docs[j].URL
		})
		out = append(out, authorArchive{name: k.name, lang: k.lang, documents: docs})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].lang == out[j].lang {
			return out[i].name < out[j].name
		}
		return out[i].lang < out[j].lang
	})
	return out
}

func (r *Renderer) searchLangForPath(graph *content.SiteGraph, urlPath string) string {
	trimmed := strings.Trim(urlPath, "/")
	switch trimmed {
	case "search":
		return r.cfg.DefaultLang
	case "":
		return ""
	}

	parts := strings.Split(trimmed, "/")
	if len(parts) == 2 && parts[1] == "search" {
		if _, ok := graph.ByLang[parts[0]]; ok {
			return parts[0]
		}
	}

	return ""
}

func (r *Renderer) langForPath(graph *content.SiteGraph, path string) string {
	trimmed := strings.Trim(path, "/")
	if trimmed == "" {
		return r.cfg.DefaultLang
	}
	parts := strings.Split(trimmed, "/")
	if _, ok := graph.ByLang[parts[0]]; ok {
		return parts[0]
	}
	return r.cfg.DefaultLang
}

func (r *Renderer) knownLangs(graph *content.SiteGraph) []string {
	if graph == nil || len(graph.ByLang) == 0 {
		return []string{r.cfg.DefaultLang}
	}
	langs := make([]string, 0, len(graph.ByLang))
	for lang := range graph.ByLang {
		if strings.TrimSpace(lang) == "" {
			continue
		}
		langs = append(langs, lang)
	}
	if !containsString(langs, r.cfg.DefaultLang) {
		langs = append(langs, r.cfg.DefaultLang)
	}
	sort.Strings(langs)
	return langs
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
	// pure thievery lol
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
	r.syncActiveTheme()
	manifest, _ := theme.LoadManifest(r.cfg.ThemesDir, r.cfg.Theme)
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
	data = sanitizeViewDataForTheme(data, manifest)

	basePath := r.themes.LayoutPath("base")
	pagePath := r.layoutPathWithFallback(name)

	partials, err := filepath.Glob(filepath.Join(r.cfg.ThemesDir, r.cfg.Theme, "layouts", "partials", "*.html"))
	if err != nil {
		return nil, fmt.Errorf("glob partials: %w", err)
	}

	files := []string{basePath, pagePath}
	files = append(files, partials...)

	// These template functions are the stable extension helpers exposed to
	// frontend themes:
	//   - safeHTML returns trusted template.HTML values unchanged.
	//   - field reads schema/custom fields from the current document.
	//   - data reads from the shared site data map loaded from content/data.
	//   - pluginSlot renders accumulated HTML for a declared theme slot.
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

func (r *Renderer) ContentSecurityPolicy() string {
	manifest, err := theme.LoadManifest(r.cfg.ThemesDir, r.cfg.Theme)
	if err != nil {
		return theme.ContentSecurityPolicy(nil)
	}
	return theme.ContentSecurityPolicy(manifest)
}

func sanitizeViewDataForTheme(data ViewData, manifest *theme.Manifest) ViewData {
	security := theme.ThemeSecurity{}
	if manifest != nil {
		security = manifest.Security
	}
	out := data
	out.Site = sanitizePublicSiteConfig(data.Site, security.TemplateContext)
	out.Page = sanitizeDocumentForTheme(data.Page, security.TemplateContext)
	out.Documents = sanitizeDocumentsForTheme(data.Documents, security.TemplateContext)
	out.Data = sanitizeThemeDataMap(data.Data, security.TemplateContext)
	return out
}

func sanitizePublicSiteConfig(site *config.Config, ctx theme.ThemeTemplateContext) *config.Config {
	if site == nil {
		return nil
	}
	copySite := *site
	copySite.Admin = config.AdminConfig{
		Enabled:   site.Admin.Enabled,
		Addr:      site.Admin.Addr,
		Path:      site.Admin.Path,
		LocalOnly: site.Admin.LocalOnly,
		Theme:     site.Admin.Theme,
		Debug:     config.AdminDebugConfig{Pprof: false},
	}
	copySite.Backup = config.BackupConfig{}
	copySite.ContentDir = ""
	copySite.PublicDir = ""
	copySite.ThemesDir = ""
	copySite.DataDir = ""
	copySite.PluginsDir = ""
	copySite.Build = config.BuildConfig{}
	copySite.Content = config.ContentConfig{}
	copySite.Plugins = config.PluginConfig{}
	copySite.Fields = config.FieldsConfig{}
	copySite.Cache = config.CacheConfig{}
	copySite.Security = config.SecurityConfig{}
	copySite.Deploy = config.DeployConfig{}
	copySite.Params = nil
	if themeTemplateBool(ctx.AllowSiteParams) {
		copySite.Params = cloneMap(site.Params)
	}
	copySite.Menus = cloneMenus(site.Menus)
	return &copySite
}

func sanitizeDocumentForTheme(doc *content.Document, ctx theme.ThemeTemplateContext) *content.Document {
	if doc == nil {
		return nil
	}
	cloned := *doc
	cloned.Params = cloneMap(doc.Params)
	cloned.Fields = nil
	if themeTemplateBool(ctx.AllowContentFields) {
		cloned.Fields = cloneMap(doc.Fields)
	}
	return &cloned
}

func sanitizeDocumentsForTheme(items []*content.Document, ctx theme.ThemeTemplateContext) []*content.Document {
	if len(items) == 0 {
		return nil
	}
	out := make([]*content.Document, 0, len(items))
	for _, item := range items {
		out = append(out, sanitizeDocumentForTheme(item, ctx))
	}
	return out
}

func sanitizeThemeDataMap(in map[string]any, ctx theme.ThemeTemplateContext) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for key, value := range in {
		normalized := strings.ToLower(strings.TrimSpace(key))
		if normalized == "custom_fields" && !themeTemplateBool(ctx.AllowSharedFields) {
			continue
		}
		if !themeTemplateBool(ctx.AllowAdminState) && (normalized == "admin" || normalized == "auth" || normalized == "sessions") {
			continue
		}
		if !themeTemplateBool(ctx.AllowRuntimeState) && (normalized == "runtime" || normalized == "debug" || normalized == "diagnostics") {
			continue
		}
		out[key] = value
	}
	return out
}

func cloneMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func cloneMenus(in map[string][]config.MenuItem) map[string][]config.MenuItem {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string][]config.MenuItem, len(in))
	for key, items := range in {
		out[key] = append([]config.MenuItem(nil), items...)
	}
	return out
}

func themeTemplateBool(v *bool) bool {
	return v != nil && *v
}

func (r *Renderer) syncActiveTheme() {
	if r == nil || r.themes == nil || r.cfg == nil {
		return
	}
	r.themes.SetActiveTheme(r.cfg.Theme)
}

func (r *Renderer) layoutPathWithFallback(name string) string {
	candidates := []string{name}
	switch name {
	case "404":
		candidates = append(candidates, "index")
	case "search", "author":
		candidates = append(candidates, "list", "index")
	}
	for _, candidate := range candidates {
		path := r.themes.LayoutPath(candidate)
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return r.themes.LayoutPath(name)
}
