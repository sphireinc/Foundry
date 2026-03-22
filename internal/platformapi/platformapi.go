package platformapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/sphireinc/foundry/internal/config"
	"github.com/sphireinc/foundry/internal/consts"
	"github.com/sphireinc/foundry/internal/content"
	sdkassets "github.com/sphireinc/foundry/sdk"
)

const (
	RouteBase = "/__foundry"
	APIBase   = RouteBase + "/api"
	SDKBase   = RouteBase + "/sdk"
)

type Hooks interface {
	RegisterRoutes(*http.ServeMux)
	OnServerStarted(string) error
	OnRoutesAssigned(*content.SiteGraph) error
	OnAssetsBuilding(*config.Config) error
}

type hookSet struct {
	base Hooks
	api  *API
}

type API struct {
	cfg   *config.Config
	mu    sync.RWMutex
	graph *content.SiteGraph
}

type CapabilitiesResponse struct {
	SDKVersion string          `json:"sdk_version"`
	Modules    map[string]bool `json:"modules"`
	Features   map[string]bool `json:"features"`
}

type SiteInfoResponse struct {
	Name        string               `json:"name"`
	Title       string               `json:"title"`
	BaseURL     string               `json:"base_url"`
	Theme       string               `json:"theme"`
	Environment string               `json:"environment"`
	DefaultLang string               `json:"default_lang"`
	Menus       map[string][]NavItem `json:"menus,omitempty"`
	Params      map[string]any       `json:"params,omitempty"`
	Taxonomies  map[string]any       `json:"taxonomies,omitempty"`
}

type NavItem struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type RouteRecord struct {
	Kind         string `json:"kind"`
	ID           string `json:"id,omitempty"`
	URL          string `json:"url"`
	Type         string `json:"type,omitempty"`
	Lang         string `json:"lang,omitempty"`
	Title        string `json:"title,omitempty"`
	ContentID    string `json:"content_id,omitempty"`
	TaxonomyName string `json:"taxonomy_name,omitempty"`
	TaxonomyTerm string `json:"taxonomy_term,omitempty"`
}

type ContentSummary struct {
	ID         string              `json:"id"`
	Type       string              `json:"type"`
	Lang       string              `json:"lang"`
	Title      string              `json:"title"`
	Slug       string              `json:"slug"`
	URL        string              `json:"url"`
	Layout     string              `json:"layout"`
	Summary    string              `json:"summary,omitempty"`
	Date       *time.Time          `json:"date,omitempty"`
	Author     string              `json:"author,omitempty"`
	LastEditor string              `json:"last_editor,omitempty"`
	Taxonomies map[string][]string `json:"taxonomies,omitempty"`
}

type ContentDetail struct {
	ContentSummary
	HTMLBody  string         `json:"html_body,omitempty"`
	RawBody   string         `json:"raw_body,omitempty"`
	Fields    map[string]any `json:"fields,omitempty"`
	Params    map[string]any `json:"params,omitempty"`
	CreatedAt *time.Time     `json:"created_at,omitempty"`
	UpdatedAt *time.Time     `json:"updated_at,omitempty"`
}

type CollectionResponse struct {
	Items    []ContentSummary `json:"items"`
	Page     int              `json:"page"`
	PageSize int              `json:"page_size"`
	Total    int              `json:"total"`
}

type SearchEntry struct {
	Title      string              `json:"title"`
	URL        string              `json:"url"`
	Summary    string              `json:"summary,omitempty"`
	Snippet    string              `json:"snippet,omitempty"`
	Content    string              `json:"content,omitempty"`
	Type       string              `json:"type"`
	Lang       string              `json:"lang"`
	Layout     string              `json:"layout,omitempty"`
	Taxonomies map[string][]string `json:"taxonomies,omitempty"`
}

type PreviewLink struct {
	Title      string `json:"title"`
	Status     string `json:"status"`
	Type       string `json:"type"`
	Lang       string `json:"lang"`
	SourcePath string `json:"source_path"`
	URL        string `json:"url"`
	PreviewURL string `json:"preview_url"`
}

type PreviewManifest struct {
	GeneratedAt time.Time     `json:"generated_at"`
	Environment string        `json:"environment"`
	Target      string        `json:"target,omitempty"`
	Links       []PreviewLink `json:"links"`
}

func NewHooks(cfg *config.Config, base Hooks) Hooks {
	if cfg == nil {
		return base
	}
	return hookSet{
		base: base,
		api:  &API{cfg: cfg},
	}
}

func (h hookSet) RegisterRoutes(mux *http.ServeMux) {
	if h.base != nil {
		h.base.RegisterRoutes(mux)
	}
	if h.api == nil || mux == nil {
		return
	}
	h.api.RegisterRoutes(mux)
}

func (h hookSet) OnServerStarted(addr string) error {
	if h.base == nil {
		return nil
	}
	return h.base.OnServerStarted(addr)
}

func (h hookSet) OnRoutesAssigned(graph *content.SiteGraph) error {
	if h.api != nil {
		h.api.SetGraph(graph)
	}
	if h.base == nil {
		return nil
	}
	return h.base.OnRoutesAssigned(graph)
}

func (h hookSet) OnAssetsBuilding(cfg *config.Config) error {
	if h.base == nil {
		return nil
	}
	return h.base.OnAssetsBuilding(cfg)
}

func (a *API) SetGraph(graph *content.SiteGraph) {
	a.mu.Lock()
	a.graph = graph
	a.mu.Unlock()
}

func (a *API) currentGraph() *content.SiteGraph {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.graph
}

func (a *API) RegisterRoutes(mux *http.ServeMux) {
	mux.Handle(SDKBase+"/", http.StripPrefix(SDKBase+"/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		sdkassets.Handler().ServeHTTP(w, r)
	})))
	mux.HandleFunc(APIBase+"/capabilities", a.handleCapabilities)
	mux.HandleFunc(APIBase+"/site", a.handleSite)
	mux.HandleFunc(APIBase+"/navigation", a.handleNavigation)
	mux.HandleFunc(APIBase+"/routes/resolve", a.handleRouteResolve)
	mux.HandleFunc(APIBase+"/content", a.handleContent)
	mux.HandleFunc(APIBase+"/collections", a.handleCollections)
	mux.HandleFunc(APIBase+"/search", a.handleSearch)
	mux.HandleFunc(APIBase+"/preview", a.handlePreview)
}

func (a *API) handleCapabilities(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, CapabilitiesResponse{
		SDKVersion: consts.SDKVersion,
		Modules: map[string]bool{
			"site":        true,
			"navigation":  true,
			"routes":      true,
			"content":     true,
			"collections": true,
			"search":      true,
			"media":       true,
			"preview":     true,
		},
		Features: map[string]bool{
			"search":           true,
			"preview":          false,
			"preview_manifest": true,
			"live_preview":     false,
			"navigation":       true,
			"taxonomies":       true,
			"media_refs":       true,
		},
	})
}

func (a *API) handleSite(w http.ResponseWriter, req *http.Request) {
	graph := a.requireGraph(w, req)
	if graph == nil {
		return
	}
	writeJSON(w, buildSiteInfo(graph))
}

func (a *API) handleNavigation(w http.ResponseWriter, req *http.Request) {
	graph := a.requireGraph(w, req)
	if graph == nil {
		return
	}
	writeJSON(w, buildNavigation(graph))
}

func (a *API) handleRouteResolve(w http.ResponseWriter, req *http.Request) {
	graph := a.requireGraph(w, req)
	if graph == nil {
		return
	}
	path := normalizeURLPath(req.URL.Query().Get("path"))
	for _, route := range buildRouteRecords(graph) {
		if route.URL == path {
			writeJSON(w, route)
			return
		}
	}
	http.NotFound(w, req)
}

func (a *API) handleContent(w http.ResponseWriter, req *http.Request) {
	graph := a.requireGraph(w, req)
	if graph == nil {
		return
	}
	id := strings.TrimSpace(req.URL.Query().Get("id"))
	if id == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}
	for _, doc := range graph.Documents {
		if doc != nil && doc.ID == id && !doc.Draft && !documentArchived(doc) {
			writeJSON(w, buildContentDetail(doc))
			return
		}
	}
	http.NotFound(w, req)
}

func (a *API) handleCollections(w http.ResponseWriter, req *http.Request) {
	graph := a.requireGraph(w, req)
	if graph == nil {
		return
	}
	items := buildCollectionItems(graph)
	query := strings.TrimSpace(req.URL.Query().Get("q"))
	typ := strings.TrimSpace(req.URL.Query().Get("type"))
	lang := strings.TrimSpace(req.URL.Query().Get("lang"))
	taxonomyName := strings.TrimSpace(req.URL.Query().Get("taxonomy"))
	term := strings.TrimSpace(req.URL.Query().Get("term"))
	if typ != "" {
		items = filterContent(items, func(item ContentSummary) bool { return item.Type == typ })
	}
	if lang != "" {
		items = filterContent(items, func(item ContentSummary) bool { return item.Lang == lang })
	}
	if taxonomyName != "" && term != "" {
		items = filterContent(items, func(item ContentSummary) bool {
			return containsString(item.Taxonomies[taxonomyName], term)
		})
	}
	if query != "" {
		needle := strings.ToLower(query)
		items = filterContent(items, func(item ContentSummary) bool {
			return strings.Contains(strings.ToLower(strings.Join([]string{
				item.Title,
				item.Slug,
				item.URL,
				item.Summary,
			}, " ")), needle)
		})
	}
	page := parsePositiveInt(req.URL.Query().Get("page"), 1)
	pageSize := parsePositiveInt(req.URL.Query().Get("page_size"), 20)
	total := len(items)
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	writeJSON(w, CollectionResponse{
		Items:    items[start:end],
		Page:     page,
		PageSize: pageSize,
		Total:    total,
	})
}

func (a *API) handleSearch(w http.ResponseWriter, req *http.Request) {
	graph := a.requireGraph(w, req)
	if graph == nil {
		return
	}
	query := strings.ToLower(strings.TrimSpace(req.URL.Query().Get("q")))
	items := buildSearchEntries(graph)
	if query != "" {
		items = rankSearchEntries(items, query)
	}
	writeJSON(w, map[string]any{
		"query": query,
		"items": items,
	})
}

func (a *API) handlePreview(w http.ResponseWriter, req *http.Request) {
	graph := a.requireGraph(w, req)
	if graph == nil {
		return
	}
	writeJSON(w, buildPreviewManifest(graph))
}

func (a *API) requireGraph(w http.ResponseWriter, req *http.Request) *content.SiteGraph {
	if req.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return nil
	}
	graph := a.currentGraph()
	if graph == nil {
		http.Error(w, "site graph is unavailable", http.StatusServiceUnavailable)
		return nil
	}
	return graph
}

func buildSiteInfo(graph *content.SiteGraph) SiteInfoResponse {
	var taxonomies map[string]any
	if graph != nil && graph.Config != nil && len(graph.Config.Taxonomies.Definitions) > 0 {
		taxonomies = make(map[string]any, len(graph.Config.Taxonomies.Definitions))
		for name, def := range graph.Config.Taxonomies.Definitions {
			taxonomies[name] = map[string]any{
				"title":          def.Title,
				"labels":         def.Labels,
				"archive_layout": def.ArchiveLayout,
				"term_layout":    def.TermLayout,
				"order":          def.Order,
			}
		}
	}
	return SiteInfoResponse{
		Name:        graph.Config.Name,
		Title:       graph.Config.Title,
		BaseURL:     graph.Config.BaseURL,
		Theme:       graph.Config.Theme,
		Environment: graph.Config.Environment,
		DefaultLang: graph.Config.DefaultLang,
		Menus:       buildNavigation(graph),
		Params:      graph.Config.Params,
		Taxonomies:  taxonomies,
	}
}

func buildNavigation(graph *content.SiteGraph) map[string][]NavItem {
	if graph == nil || graph.Config == nil {
		return map[string][]NavItem{}
	}
	out := map[string][]NavItem{}
	if len(graph.Config.Menus) > 0 {
		for key, items := range graph.Config.Menus {
			for _, item := range items {
				out[key] = append(out[key], NavItem{Name: item.Name, URL: normalizeURLPath(item.URL)})
			}
		}
	}
	if len(out) == 0 {
		if raw, ok := graph.Data["navigation"].(map[string]any); ok {
			for key, value := range raw {
				list, ok := value.([]any)
				if !ok {
					continue
				}
				for _, entry := range list {
					item, ok := entry.(map[string]any)
					if !ok {
						continue
					}
					name, _ := item["name"].(string)
					urlValue, _ := item["url"].(string)
					if strings.TrimSpace(name) == "" || strings.TrimSpace(urlValue) == "" {
						continue
					}
					out[key] = append(out[key], NavItem{Name: name, URL: normalizeURLPath(urlValue)})
				}
			}
		}
	}
	return out
}

func buildRouteRecords(graph *content.SiteGraph) []RouteRecord {
	out := make([]RouteRecord, 0, len(graph.Documents))
	for _, doc := range graph.Documents {
		if doc == nil || doc.Draft || documentArchived(doc) {
			continue
		}
		out = append(out, RouteRecord{
			Kind:      "document",
			ID:        doc.ID,
			URL:       doc.URL,
			Type:      doc.Type,
			Lang:      doc.Lang,
			Title:     doc.Title,
			ContentID: doc.ID,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].URL < out[j].URL })
	return out
}

func buildCollectionItems(graph *content.SiteGraph) []ContentSummary {
	out := make([]ContentSummary, 0, len(graph.Documents))
	for _, doc := range graph.Documents {
		if doc == nil || doc.Draft || documentArchived(doc) {
			continue
		}
		out = append(out, buildContentSummary(doc))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].URL < out[j].URL })
	return out
}

func buildContentSummary(doc *content.Document) ContentSummary {
	return ContentSummary{
		ID:         doc.ID,
		Type:       doc.Type,
		Lang:       doc.Lang,
		Title:      doc.Title,
		Slug:       doc.Slug,
		URL:        doc.URL,
		Layout:     doc.Layout,
		Summary:    doc.Summary,
		Date:       doc.Date,
		Author:     doc.Author,
		LastEditor: doc.LastEditor,
		Taxonomies: cloneTaxonomies(doc.Taxonomies),
	}
}

func buildContentDetail(doc *content.Document) ContentDetail {
	summary := buildContentSummary(doc)
	return ContentDetail{
		ContentSummary: summary,
		HTMLBody:       string(doc.HTMLBody),
		RawBody:        doc.RawBody,
		Fields:         cloneMap(doc.Fields),
		Params:         cloneMap(doc.Params),
		CreatedAt:      doc.CreatedAt,
		UpdatedAt:      doc.UpdatedAt,
	}
}

func buildSearchEntries(graph *content.SiteGraph) []SearchEntry {
	out := make([]SearchEntry, 0, len(graph.Documents))
	for _, doc := range graph.Documents {
		if doc == nil || doc.Draft || documentArchived(doc) {
			continue
		}
		out = append(out, SearchEntry{
			Title:      doc.Title,
			URL:        doc.URL,
			Summary:    doc.Summary,
			Snippet:    deriveSearchSnippet(doc.Title, doc.Summary, normalizeSearchContent(doc), ""),
			Content:    normalizeSearchContent(doc),
			Type:       doc.Type,
			Lang:       doc.Lang,
			Layout:     doc.Layout,
			Taxonomies: cloneTaxonomies(doc.Taxonomies),
		})
	}
	return out
}

func rankSearchEntries(items []SearchEntry, query string) []SearchEntry {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return items
	}
	type scored struct {
		entry SearchEntry
		score int
	}
	scoredItems := make([]scored, 0, len(items))
	for _, item := range items {
		score := 0
		title := strings.ToLower(item.Title)
		summary := strings.ToLower(item.Summary)
		content := strings.ToLower(item.Content)
		url := strings.ToLower(item.URL)
		if strings.Contains(title, query) {
			score += 6
		}
		if strings.Contains(summary, query) {
			score += 4
		}
		if strings.Contains(content, query) {
			score += 2
		}
		if strings.Contains(url, query) {
			score += 1
		}
		if score == 0 {
			continue
		}
		item.Snippet = deriveSearchSnippet(item.Title, item.Summary, item.Content, query)
		scoredItems = append(scoredItems, scored{entry: item, score: score})
	}
	sort.SliceStable(scoredItems, func(i, j int) bool {
		if scoredItems[i].score == scoredItems[j].score {
			return scoredItems[i].entry.Title < scoredItems[j].entry.Title
		}
		return scoredItems[i].score > scoredItems[j].score
	})
	out := make([]SearchEntry, 0, len(scoredItems))
	for _, item := range scoredItems {
		out = append(out, item.entry)
	}
	return out
}

func deriveSearchSnippet(title, summary, content, query string) string {
	if strings.TrimSpace(summary) != "" {
		return strings.TrimSpace(summary)
	}
	body := strings.TrimSpace(content)
	if body == "" {
		return strings.TrimSpace(title)
	}
	if query == "" {
		return firstRunes(body, 180)
	}
	lower := strings.ToLower(body)
	idx := strings.Index(lower, strings.ToLower(strings.TrimSpace(query)))
	if idx < 0 {
		return firstRunes(body, 180)
	}
	start := idx - 60
	if start < 0 {
		start = 0
	}
	end := start + 180
	runes := []rune(body)
	if start > len(runes) {
		start = 0
	}
	if end > len(runes) {
		end = len(runes)
	}
	snippet := strings.TrimSpace(string(runes[start:end]))
	if start > 0 {
		snippet = "..." + snippet
	}
	if end < len(runes) {
		snippet += "..."
	}
	return snippet
}

func firstRunes(value string, max int) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= max {
		return string(runes)
	}
	return strings.TrimSpace(string(runes[:max])) + "..."
}

func WriteStaticArtifacts(cfg *config.Config, graph *content.SiteGraph) error {
	if cfg == nil || graph == nil {
		return nil
	}
	root := filepath.Join(cfg.PublicDir, "__foundry")
	if err := os.MkdirAll(filepath.Join(root, "content"), 0o755); err != nil {
		return err
	}
	if err := writeJSONFile(filepath.Join(root, "capabilities.json"), CapabilitiesResponse{
		SDKVersion: "v1",
		Modules: map[string]bool{
			"site":        true,
			"navigation":  true,
			"routes":      true,
			"content":     true,
			"collections": true,
			"search":      true,
			"media":       true,
			"preview":     true,
		},
		Features: map[string]bool{
			"search":           true,
			"preview":          false,
			"preview_manifest": true,
			"live_preview":     false,
			"navigation":       true,
			"taxonomies":       true,
			"media_refs":       true,
		},
	}); err != nil {
		return err
	}
	if err := writeJSONFile(filepath.Join(root, "site.json"), buildSiteInfo(graph)); err != nil {
		return err
	}
	if err := writeJSONFile(filepath.Join(root, "navigation.json"), buildNavigation(graph)); err != nil {
		return err
	}
	if err := writeJSONFile(filepath.Join(root, "routes.json"), buildRouteRecords(graph)); err != nil {
		return err
	}
	if err := writeJSONFile(filepath.Join(root, "collections.json"), map[string]any{"items": buildCollectionItems(graph)}); err != nil {
		return err
	}
	if err := writeJSONFile(filepath.Join(root, "search.json"), buildSearchEntries(graph)); err != nil {
		return err
	}
	if err := writeJSONFile(filepath.Join(root, "preview.json"), buildPreviewManifest(graph)); err != nil {
		return err
	}
	for _, doc := range graph.Documents {
		if doc == nil || doc.Draft || documentArchived(doc) {
			continue
		}
		filename := filepath.Join(root, "content", url.PathEscape(doc.ID)+".json")
		if err := writeJSONFile(filename, buildContentDetail(doc)); err != nil {
			return err
		}
	}
	return sdkassets.CopyToDir(filepath.Join(root, "sdk"))
}

func buildPreviewManifest(graph *content.SiteGraph) PreviewManifest {
	manifest := PreviewManifest{
		GeneratedAt: time.Now().UTC(),
		Environment: graph.Config.Environment,
		Links:       make([]PreviewLink, 0),
	}
	for _, doc := range graph.Documents {
		if doc == nil {
			continue
		}
		if doc.Status == "published" && !doc.Draft {
			continue
		}
		manifest.Links = append(manifest.Links, PreviewLink{
			Title:      doc.Title,
			Status:     doc.Status,
			Type:       doc.Type,
			Lang:       doc.Lang,
			SourcePath: doc.SourcePath,
			URL:        doc.URL,
			PreviewURL: doc.URL,
		})
	}
	sort.Slice(manifest.Links, func(i, j int) bool {
		if manifest.Links[i].Status != manifest.Links[j].Status {
			return manifest.Links[i].Status < manifest.Links[j].Status
		}
		return manifest.Links[i].URL < manifest.Links[j].URL
	})
	return manifest
}

func writeJSONFile(path string, v any) error {
	body, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(body, '\n'), 0o644)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(v)
}

func normalizeURLPath(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "/"
	}
	if strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") {
		return value
	}
	if !strings.HasPrefix(value, "/") {
		value = "/" + value
	}
	if value != "/" && !strings.Contains(filepath.Base(value), ".") && !strings.HasSuffix(value, "/") {
		value += "/"
	}
	return value
}

func parsePositiveInt(value string, fallback int) int {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	n := fallback
	if _, err := fmt.Sscanf(value, "%d", &n); err != nil || n <= 0 {
		return fallback
	}
	return n
}

func normalizeSearchContent(doc *content.Document) string {
	if doc == nil {
		return ""
	}
	if strings.TrimSpace(doc.RawBody) != "" {
		return strings.Join(strings.Fields(doc.RawBody), " ")
	}
	return strings.Join(strings.Fields(string(doc.HTMLBody)), " ")
}

func cloneTaxonomies(in map[string][]string) map[string][]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string][]string, len(in))
	for key, values := range in {
		out[key] = append([]string(nil), values...)
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

func filterContent(items []ContentSummary, fn func(ContentSummary) bool) []ContentSummary {
	out := make([]ContentSummary, 0, len(items))
	for _, item := range items {
		if fn(item) {
			out = append(out, item)
		}
	}
	return out
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func documentArchived(doc *content.Document) bool {
	if doc == nil || doc.Params == nil {
		return false
	}
	value, ok := doc.Params["archived"].(bool)
	return ok && value
}
