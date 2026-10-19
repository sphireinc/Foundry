package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"path"
	"strings"

	"github.com/sphireinc/foundry/internal/config"
	"github.com/sphireinc/foundry/internal/ops"
	"github.com/sphireinc/foundry/internal/sitesearch"
	"github.com/sphireinc/foundry/internal/theme"
)

func RegisterCapabilities(reg *Registry, env *Environment) error {
	if reg == nil {
		return fmt.Errorf("mcp: registry is required")
	}
	if env == nil {
		return fmt.Errorf("mcp: environment is required")
	}

	for _, cap := range coreTools(env) {
		if err := reg.Register(cap); err != nil {
			return err
		}
	}
	for _, cap := range coreResources(env) {
		if err := reg.Register(cap); err != nil {
			return err
		}
	}
	return nil
}

func readTool(name, description string, scope Scope, schema *Schema, handler Handler) Capability {
	return Capability{
		Name:          name,
		Version:       DefaultCapabilityVer,
		Kind:          KindTool,
		Description:   description,
		InputSchema:   schema,
		Mutability:    ReadOnly,
		Scope:         scope,
		AuditCategory: string(scope),
		Handler:       handler,
	}
}

func coreTools(env *Environment) []Capability {
	return []Capability{
		readTool(
			"search_content",
			"Search published and draft content for the active site.",
			ScopeReadContent,
			ObjectSchema(map[string]*Schema{
				"query":  StringSchema("Search terms to match against titles, summaries, and body text."),
				"type":   StringSchema("Restrict results to a document type such as page or post."),
				"lang":   StringSchema("Restrict results to a content language."),
				"limit":  IntSchema("Maximum number of results to return."),
				"cursor": StringSchema("Opaque pagination cursor returned by a previous call."),
			}, "query"),
			handleSearchContent(env),
		),
		readTool(
			"list_documents",
			"List content documents with optional type, language, and status filters.",
			ScopeReadContent,
			ObjectSchema(map[string]*Schema{
				"type":   StringSchema("Restrict results to a document type."),
				"lang":   StringSchema("Restrict results to a content language."),
				"status": StringSchema("Restrict results to a workflow status such as draft or published."),
				"limit":  IntSchema("Maximum number of results to return."),
				"cursor": StringSchema("Opaque pagination cursor returned by a previous call."),
			}),
			handleListDocuments(env),
		),
		readTool(
			"get_document",
			"Fetch a single content document by slug, URL, or id.",
			ScopeReadContent,
			ObjectSchema(map[string]*Schema{
				"slug": StringSchema("Document slug."),
				"url":  StringSchema("Document URL path."),
				"id":   StringSchema("Stable document id."),
			}),
			handleGetDocument(env),
		),
		readTool(
			"get_navigation",
			"Read the site navigation menus and published page index.",
			ScopeReadContent,
			ObjectSchema(map[string]*Schema{
				"lang": StringSchema("Restrict navigation to a content language."),
			}),
			handleGetNavigation(env),
		),
		readTool(
			"get_project_documentation",
			"List or read Foundry project documentation files.",
			ScopeReadDocs,
			ObjectSchema(map[string]*Schema{
				"path": StringSchema("Relative documentation path, for example guides/getting-started.md."),
			}),
			handleProjectDocumentation(env),
		),
		readTool(
			"get_api_documentation",
			"Read the Foundry API documentation definition.",
			ScopeReadDocs,
			ObjectSchema(map[string]*Schema{}),
			handleAPIDocumentation(env),
		),
		readTool(
			"get_theme_manifest",
			"Read a theme manifest, defaulting to the active theme.",
			ScopeReadThemes,
			ObjectSchema(map[string]*Schema{
				"name": StringSchema("Theme name. Defaults to the active theme."),
			}),
			handleThemeManifest(env),
		),
		readTool(
			"get_theme_security_report",
			"Read the external asset and CSP security report for a theme.",
			ScopeReadThemes,
			ObjectSchema(map[string]*Schema{
				"name": StringSchema("Theme name. Defaults to the active theme."),
			}),
			handleThemeSecurity(env),
		),
		readTool(
			"list_plugins",
			"List installed plugins with their enabled status.",
			ScopeReadPlugins,
			ObjectSchema(map[string]*Schema{
				"limit":  IntSchema("Maximum number of results to return."),
				"cursor": StringSchema("Opaque pagination cursor returned by a previous call."),
			}),
			handleListPlugins(env),
		),
		readTool(
			"get_plugin_status",
			"Read the health and diagnostics report for a single plugin.",
			ScopeReadPlugins,
			ObjectSchema(map[string]*Schema{
				"name": StringSchema("Plugin name."),
			}, "name"),
			handlePluginStatus(env),
		),
		readTool(
			"get_runtime_health",
			"Read runtime health and readiness information.",
			ScopeReadOps,
			ObjectSchema(map[string]*Schema{}),
			handleRuntimeHealth(env),
		),
		readTool(
			"validate_theme",
			"Validate a theme manifest and its assets, defaulting to the active theme.",
			ScopeReadThemes,
			ObjectSchema(map[string]*Schema{
				"name": StringSchema("Theme name. Defaults to the active theme."),
			}),
			handleValidateTheme(env),
		),
		readTool(
			"validate_configuration",
			"Validate the Foundry configuration, active theme, and site graph without changing anything.",
			ScopeReadOps,
			ObjectSchema(map[string]*Schema{}),
			handleValidateConfiguration(env),
		),
	}
}

func decodeInput(input json.RawMessage, target any) error {
	if len(bytes.TrimSpace(input)) == 0 {
		return nil
	}
	decoder := json.NewDecoder(bytes.NewReader(input))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("mcp: invalid arguments: %w", err)
	}
	return nil
}

type searchContentInput struct {
	Query  string `json:"query"`
	Type   string `json:"type"`
	Lang   string `json:"lang"`
	Limit  int    `json:"limit"`
	Cursor string `json:"cursor"`
}

type contentSearchItem struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Type    string `json:"type"`
	Lang    string `json:"lang"`
	Summary string `json:"summary,omitempty"`
	Snippet string `json:"snippet,omitempty"`
}

type searchContentOutput struct {
	Query      string              `json:"query"`
	Total      int                 `json:"total"`
	Limit      int                 `json:"limit"`
	More       bool                `json:"more"`
	NextCursor string              `json:"next_cursor,omitempty"`
	Items      []contentSearchItem `json:"items"`
}

func handleSearchContent(env *Environment) Handler {
	return func(ctx context.Context, inv Invocation) (any, error) {
		var input searchContentInput
		if err := decodeInput(inv.Input, &input); err != nil {
			return nil, err
		}

		graph, err := env.LoadGraph(ctx)
		if err != nil {
			return nil, err
		}

		limit := input.Limit
		if limit <= 0 {
			limit = DefaultPageSize
		}
		if limit > MaxPageSize {
			limit = MaxPageSize
		}
		offset, err := parseCursor(input.Cursor)
		if err != nil {
			return nil, err
		}

		fetch := offset + limit
		if fetch > sitesearch.MaxLimit {
			fetch = sitesearch.MaxLimit
		}

		result := sitesearch.Query(sitesearch.Entries(graph), input.Query, sitesearch.Options{
			Lang:  input.Lang,
			Type:  input.Type,
			Limit: fetch,
		})

		start := offset
		if start > len(result.Items) {
			start = len(result.Items)
		}
		end := start + limit
		if end > len(result.Items) {
			end = len(result.Items)
		}

		items := make([]contentSearchItem, 0, end-start)
		for _, entry := range result.Items[start:end] {
			items = append(items, contentSearchItem{
				Title:   entry.Title,
				URL:     entry.URL,
				Type:    entry.Type,
				Lang:    entry.Lang,
				Summary: entry.Summary,
				Snippet: entry.Snippet,
			})
		}

		out := searchContentOutput{
			Query: input.Query,
			Total: result.Total,
			Limit: limit,
			More:  result.Total > end,
			Items: items,
		}
		if out.More {
			out.NextCursor = fmt.Sprintf("%d", end)
		}
		return out, nil
	}
}

type listDocumentsInput struct {
	Type   string `json:"type"`
	Lang   string `json:"lang"`
	Status string `json:"status"`
	Limit  int    `json:"limit"`
	Cursor string `json:"cursor"`
}

type listDocumentsOutput struct {
	Total      int               `json:"total"`
	Limit      int               `json:"limit"`
	More       bool              `json:"more"`
	NextCursor string            `json:"next_cursor,omitempty"`
	Items      []DocumentSummary `json:"items"`
}

func handleListDocuments(env *Environment) Handler {
	return func(ctx context.Context, inv Invocation) (any, error) {
		var input listDocumentsInput
		if err := decodeInput(inv.Input, &input); err != nil {
			return nil, err
		}

		graph, err := env.LoadGraph(ctx)
		if err != nil {
			return nil, err
		}

		documents := filterDocuments(graph, input.Type, input.Lang, input.Status)
		window, err := Paginate(len(documents), input.Limit, input.Cursor)
		if err != nil {
			return nil, err
		}

		items := make([]DocumentSummary, 0, window.End-window.Start)
		for _, doc := range documents[window.Start:window.End] {
			items = append(items, summarizeDocument(doc))
		}

		return listDocumentsOutput{
			Total:      window.Total,
			Limit:      inputLimit(input.Limit),
			More:       window.More,
			NextCursor: window.NextCursor,
			Items:      items,
		}, nil
	}
}

type getDocumentInput struct {
	Slug string `json:"slug"`
	URL  string `json:"url"`
	ID   string `json:"id"`
}

func handleGetDocument(env *Environment) Handler {
	return func(ctx context.Context, inv Invocation) (any, error) {
		var input getDocumentInput
		if err := decodeInput(inv.Input, &input); err != nil {
			return nil, err
		}

		key := firstNonEmpty(input.ID, input.Slug, input.URL)
		if key == "" {
			return nil, fmt.Errorf("mcp: one of slug, url, or id is required")
		}

		graph, err := env.LoadGraph(ctx)
		if err != nil {
			return nil, err
		}
		doc, ok := findDocument(graph, key)
		if !ok {
			return nil, fmt.Errorf("mcp: document not found: %s", key)
		}
		return detailDocument(doc), nil
	}
}

type getNavigationInput struct {
	Lang string `json:"lang"`
}

func handleGetNavigation(env *Environment) Handler {
	return func(ctx context.Context, inv Invocation) (any, error) {
		var input getNavigationInput
		if err := decodeInput(inv.Input, &input); err != nil {
			return nil, err
		}
		graph, err := env.LoadGraph(ctx)
		if err != nil {
			return nil, err
		}
		return buildNavigation(env.Config, graph, input.Lang), nil
	}
}

type documentationInput struct {
	Path string `json:"path"`
}

type documentationOutput struct {
	Path    string   `json:"path,omitempty"`
	Format  string   `json:"format,omitempty"`
	Content string   `json:"content,omitempty"`
	Entries []string `json:"entries,omitempty"`
}

func handleProjectDocumentation(env *Environment) Handler {
	return func(ctx context.Context, inv Invocation) (any, error) {
		var input documentationInput
		if err := decodeInput(inv.Input, &input); err != nil {
			return nil, err
		}

		if strings.TrimSpace(input.Path) == "" {
			if env.ListDocs == nil {
				return nil, fmt.Errorf("mcp: documentation service is unavailable")
			}
			entries, err := env.ListDocs()
			if err != nil {
				return nil, err
			}
			return documentationOutput{Entries: entries}, nil
		}

		if env.ReadDoc == nil {
			return nil, fmt.Errorf("mcp: documentation service is unavailable")
		}
		data, err := env.ReadDoc(input.Path)
		if err != nil {
			return nil, err
		}
		return documentationOutput{
			Path:    input.Path,
			Format:  documentationFormat(input.Path),
			Content: string(data),
		}, nil
	}
}

func handleAPIDocumentation(env *Environment) Handler {
	return func(ctx context.Context, inv Invocation) (any, error) {
		if env.ReadDoc == nil {
			return nil, fmt.Errorf("mcp: documentation service is unavailable")
		}
		for _, candidate := range []string{"api.yaml", "api-documentation.md"} {
			data, err := env.ReadDoc(candidate)
			if err == nil {
				return documentationOutput{
					Path:    candidate,
					Format:  documentationFormat(candidate),
					Content: string(data),
				}, nil
			}
		}
		return nil, fmt.Errorf("mcp: API documentation is unavailable")
	}
}

func handleThemeManifest(env *Environment) Handler {
	return func(ctx context.Context, inv Invocation) (any, error) {
		if env.LoadThemeManifest == nil {
			return nil, fmt.Errorf("mcp: theme service is unavailable")
		}
		var input themeNameInput
		if err := decodeInput(inv.Input, &input); err != nil {
			return nil, err
		}
		name := ""
		if input.Name != nil {
			name = *input.Name
		}
		return env.LoadThemeManifest(name)
	}
}

func handleThemeSecurity(env *Environment) Handler {
	return func(ctx context.Context, inv Invocation) (any, error) {
		if env.ThemeSecurity == nil {
			return nil, fmt.Errorf("mcp: theme service is unavailable")
		}
		var input themeNameInput
		if err := decodeInput(inv.Input, &input); err != nil {
			return nil, err
		}
		name := ""
		if input.Name != nil {
			name = *input.Name
		}
		return env.ThemeSecurity(name)
	}
}

type themeNameInput struct {
	Name *string `json:"name"`
}

type listPluginsInput struct {
	Limit  int    `json:"limit"`
	Cursor string `json:"cursor"`
}

type pluginSummary struct {
	Name        string `json:"name"`
	Title       string `json:"title,omitempty"`
	Version     string `json:"version,omitempty"`
	Runtime     string `json:"runtime,omitempty"`
	Enabled     bool   `json:"enabled"`
	Status      string `json:"status,omitempty"`
	FoundryAPI  string `json:"foundry_api,omitempty"`
	Permissions any    `json:"permissions,omitempty"`
}

type listPluginsOutput struct {
	Total      int             `json:"total"`
	Limit      int             `json:"limit"`
	More       bool            `json:"more"`
	NextCursor string          `json:"next_cursor,omitempty"`
	Items      []pluginSummary `json:"items"`
}

func handleListPlugins(env *Environment) Handler {
	return func(ctx context.Context, inv Invocation) (any, error) {
		if env.ListPlugins == nil {
			return nil, fmt.Errorf("mcp: plugin service is unavailable")
		}
		var input listPluginsInput
		if err := decodeInput(inv.Input, &input); err != nil {
			return nil, err
		}

		metadata, err := env.ListPlugins()
		if err != nil {
			return nil, err
		}
		status := map[string]string{}
		if env.PluginStatus != nil {
			if computed, err := env.PluginStatus(); err == nil {
				status = computed
			}
		}

		window, err := Paginate(len(metadata), input.Limit, input.Cursor)
		if err != nil {
			return nil, err
		}

		items := make([]pluginSummary, 0, window.End-window.Start)
		for _, meta := range metadata[window.Start:window.End] {
			items = append(items, pluginSummary{
				Name:        meta.Name,
				Title:       meta.Title,
				Version:     meta.Version,
				Runtime:     meta.Runtime.Mode,
				Enabled:     pluginEnabled(env.Config, meta.Name),
				Status:      status[meta.Name],
				FoundryAPI:  meta.FoundryAPI,
				Permissions: meta.Permissions,
			})
		}

		return listPluginsOutput{
			Total:      window.Total,
			Limit:      inputLimit(input.Limit),
			More:       window.More,
			NextCursor: window.NextCursor,
			Items:      items,
		}, nil
	}
}

type pluginStatusInput struct {
	Name string `json:"name"`
}

type pluginStatusOutput struct {
	Name        string `json:"name"`
	Found       bool   `json:"found"`
	Healthy     bool   `json:"healthy"`
	Status      string `json:"status,omitempty"`
	Diagnostics any    `json:"diagnostics,omitempty"`
	Security    any    `json:"security,omitempty"`
}

func handlePluginStatus(env *Environment) Handler {
	return func(ctx context.Context, inv Invocation) (any, error) {
		if env.PluginHealth == nil {
			return nil, fmt.Errorf("mcp: plugin service is unavailable")
		}
		var input pluginStatusInput
		if err := decodeInput(inv.Input, &input); err != nil {
			return nil, err
		}
		name := strings.TrimSpace(input.Name)
		if name == "" {
			return nil, fmt.Errorf("mcp: plugin name is required")
		}

		report, ok := env.PluginHealth(name)
		if !ok {
			return pluginStatusOutput{Name: name, Found: false}, nil
		}
		return pluginStatusOutput{
			Name:        name,
			Found:       true,
			Healthy:     report.Healthy,
			Status:      report.Status,
			Diagnostics: report.Diagnostics,
			Security:    report.Security,
		}, nil
	}
}

func handleRuntimeHealth(env *Environment) Handler {
	return func(ctx context.Context, inv Invocation) (any, error) {
		if env.RuntimeHealth == nil {
			return nil, fmt.Errorf("mcp: runtime service is unavailable")
		}
		return env.RuntimeHealth(), nil
	}
}

func handleValidateTheme(env *Environment) Handler {
	return func(ctx context.Context, inv Invocation) (any, error) {
		if env.ValidateTheme == nil {
			return nil, fmt.Errorf("mcp: theme service is unavailable")
		}
		var input themeNameInput
		if err := decodeInput(inv.Input, &input); err != nil {
			return nil, err
		}
		name := ""
		if input.Name != nil {
			name = *input.Name
		}
		result, err := env.ValidateTheme(name)
		if err != nil {
			return nil, err
		}
		return result, nil
	}
}

type configValidationOutput struct {
	Valid      bool     `json:"valid"`
	Errors     []string `json:"errors,omitempty"`
	SiteIssues []string `json:"site_issues,omitempty"`
}

func handleValidateConfiguration(env *Environment) Handler {
	return func(ctx context.Context, inv Invocation) (any, error) {
		if env.Config == nil {
			return nil, fmt.Errorf("mcp: config is unavailable")
		}

		out := configValidationOutput{Valid: true}
		for _, err := range config.Validate(env.Config) {
			out.Errors = append(out.Errors, err.Error())
		}

		if env.ListThemes != nil {
			themes, err := env.ListThemes()
			if err != nil {
				out.Errors = append(out.Errors, fmt.Sprintf("theme: %v", err))
			} else if !themeInstalled(themes, env.Config.Theme) {
				out.Errors = append(out.Errors, fmt.Sprintf("theme: active theme %q is not installed", env.Config.Theme))
			}
		}

		if env.LoadGraph != nil {
			if graph, err := env.LoadGraph(ctx); err != nil {
				out.SiteIssues = append(out.SiteIssues, fmt.Sprintf("site: %v", err))
			} else {
				out.SiteIssues = append(out.SiteIssues, ops.AnalyzeSite(env.Config, graph).Messages()...)
			}
		}

		out.Valid = len(out.Errors) == 0 && len(out.SiteIssues) == 0
		return out, nil
	}
}

func themeInstalled(themes []theme.Info, name string) bool {
	for _, info := range themes {
		if strings.EqualFold(info.Name, strings.TrimSpace(name)) {
			return true
		}
	}
	return false
}

func documentationFormat(name string) string {
	switch strings.ToLower(path.Ext(name)) {
	case ".md":
		return "markdown"
	case ".yaml", ".yml":
		return "yaml"
	case ".json":
		return "json"
	case ".html":
		return "html"
	default:
		return strings.TrimPrefix(strings.ToLower(path.Ext(name)), ".")
	}
}

func inputLimit(limit int) int {
	if limit <= 0 {
		return DefaultPageSize
	}
	if limit > MaxPageSize {
		return MaxPageSize
	}
	return limit
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
