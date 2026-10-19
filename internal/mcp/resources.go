package mcp

import (
	"context"
	"fmt"
	"strings"
)

func readResource(name, description string, scope Scope, template string, handler Handler) Capability {
	return Capability{
		Name:          name,
		Version:       DefaultCapabilityVer,
		Kind:          KindResource,
		Description:   description,
		Mutability:    ReadOnly,
		Scope:         scope,
		URITemplate:   template,
		AuditCategory: string(scope),
		Handler:       handler,
	}
}

func coreResources(env *Environment) []Capability {
	return []Capability{
		readResource(
			"content_page",
			"Read a single content page by slug.",
			ScopeReadContent,
			Scheme+"://content/pages/{slug}",
			handleResourceContentPage(env),
		),
		readResource(
			"content_collection",
			"List content documents of a given type.",
			ScopeReadContent,
			Scheme+"://content/collections/{type}",
			handleResourceContentCollection(env),
		),
		readResource(
			"theme_manifest",
			"Read a theme manifest.",
			ScopeReadThemes,
			Scheme+"://themes/{name}/manifest",
			handleResourceThemeManifest(env),
		),
		readResource(
			"theme_security_report",
			"Read a theme external asset security report.",
			ScopeReadThemes,
			Scheme+"://themes/{name}/security-report",
			handleResourceThemeSecurity(env),
		),
		readResource(
			"plugins",
			"List installed plugins and their status.",
			ScopeReadPlugins,
			Scheme+"://plugins",
			handleResourcePlugins(env),
		),
		readResource(
			"runtime_health",
			"Read runtime health and readiness information.",
			ScopeReadOps,
			Scheme+"://runtime/health",
			handleResourceRuntimeHealth(env),
		),
		readResource(
			"docs",
			"Read project documentation by relative path.",
			ScopeReadDocs,
			Scheme+"://docs/{path...}",
			handleResourceDocs(env),
		),
	}
}

func handleResourceContentPage(env *Environment) Handler {
	return func(ctx context.Context, inv Invocation) (any, error) {
		slug := strings.TrimSpace(inv.Vars["slug"])
		if slug == "" {
			return nil, fmt.Errorf("mcp: resource slug is required")
		}
		graph, err := env.LoadGraph(ctx)
		if err != nil {
			return nil, err
		}
		doc, ok := findDocument(graph, slug)
		if !ok {
			return nil, fmt.Errorf("mcp: document not found: %s", slug)
		}
		return detailDocument(doc), nil
	}
}

func handleResourceContentCollection(env *Environment) Handler {
	return func(ctx context.Context, inv Invocation) (any, error) {
		docType := strings.TrimSpace(inv.Vars["type"])
		if docType == "" {
			return nil, fmt.Errorf("mcp: collection type is required")
		}
		graph, err := env.LoadGraph(ctx)
		if err != nil {
			return nil, err
		}
		documents := filterDocuments(graph, docType, "", "")
		if len(documents) > MaxPageSize {
			documents = documents[:MaxPageSize]
		}
		items := make([]DocumentSummary, 0, len(documents))
		for _, doc := range documents {
			items = append(items, summarizeDocument(doc))
		}
		return map[string]any{
			"type":  docType,
			"total": len(items),
			"items": items,
		}, nil
	}
}

func handleResourceThemeManifest(env *Environment) Handler {
	return func(ctx context.Context, inv Invocation) (any, error) {
		if env.LoadThemeManifest == nil {
			return nil, fmt.Errorf("mcp: theme service is unavailable")
		}
		return env.LoadThemeManifest(strings.TrimSpace(inv.Vars["name"]))
	}
}

func handleResourceThemeSecurity(env *Environment) Handler {
	return func(ctx context.Context, inv Invocation) (any, error) {
		if env.ThemeSecurity == nil {
			return nil, fmt.Errorf("mcp: theme service is unavailable")
		}
		return env.ThemeSecurity(strings.TrimSpace(inv.Vars["name"]))
	}
}

func handleResourcePlugins(env *Environment) Handler {
	return func(ctx context.Context, inv Invocation) (any, error) {
		if env.ListPlugins == nil {
			return nil, fmt.Errorf("mcp: plugin service is unavailable")
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
		items := make([]pluginSummary, 0, len(metadata))
		for _, meta := range metadata {
			items = append(items, pluginSummary{
				Name:       meta.Name,
				Title:      meta.Title,
				Version:    meta.Version,
				Runtime:    meta.Runtime.Mode,
				Enabled:    pluginEnabled(env.Config, meta.Name),
				Status:     status[meta.Name],
				FoundryAPI: meta.FoundryAPI,
			})
		}
		return map[string]any{"total": len(items), "items": items}, nil
	}
}

func handleResourceRuntimeHealth(env *Environment) Handler {
	return func(ctx context.Context, inv Invocation) (any, error) {
		if env.RuntimeHealth == nil {
			return nil, fmt.Errorf("mcp: runtime service is unavailable")
		}
		return env.RuntimeHealth(), nil
	}
}

func handleResourceDocs(env *Environment) Handler {
	return func(ctx context.Context, inv Invocation) (any, error) {
		if env.ReadDoc == nil {
			return nil, fmt.Errorf("mcp: documentation service is unavailable")
		}
		rel := strings.TrimSpace(inv.Vars["path"])
		if rel == "" {
			return nil, fmt.Errorf("mcp: documentation path is required")
		}
		data, err := env.ReadDoc(rel)
		if err != nil {
			return nil, err
		}
		return documentationOutput{
			Path:    rel,
			Format:  documentationFormat(rel),
			Content: string(data),
		}, nil
	}
}
