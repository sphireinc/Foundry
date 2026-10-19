package mcp

import (
	"context"
	"encoding/json"
	"sort"
	"testing"

	"github.com/sphireinc/foundry/internal/config"
	"github.com/sphireinc/foundry/internal/content"
	"github.com/sphireinc/foundry/internal/theme"
)

func stubEnvironment(t *testing.T) *Environment {
	t.Helper()

	cfg := &config.Config{Theme: "default"}
	cfg.ApplyDefaults()

	graph := content.NewSiteGraph(cfg)
	graph.Add(&content.Document{
		ID:      "page-about",
		Type:    "page",
		Lang:    "en",
		Title:   "About",
		Slug:    "about",
		URL:     "/about/",
		RawBody: "About the project.",
		Status:  "published",
	})
	graph.Add(&content.Document{
		ID:      "page-contact",
		Type:    "page",
		Lang:    "en",
		Title:   "Contact",
		Slug:    "contact",
		URL:     "/contact/",
		RawBody: "Contact us.",
		Status:  "published",
	})
	graph.Add(&content.Document{
		ID:      "post-hello",
		Type:    "post",
		Lang:    "en",
		Title:   "Hello",
		Slug:    "hello",
		URL:     "/posts/hello/",
		RawBody: "Hello world.",
		Status:  "published",
	})

	return &Environment{
		Config: cfg,
		LoadGraph: func(context.Context) (*content.SiteGraph, error) {
			return graph, nil
		},
		LoadThemeManifest: func(string) (*theme.Manifest, error) {
			return &theme.Manifest{Name: "default", Version: "1.0.0"}, nil
		},
	}
}

func TestRegisterCapabilitiesShape(t *testing.T) {
	reg := NewRegistry()
	if err := RegisterCapabilities(reg, stubEnvironment(t)); err != nil {
		t.Fatalf("register capabilities: %v", err)
	}

	expectedTools := []string{
		"get_api_documentation",
		"get_document",
		"get_navigation",
		"get_plugin_status",
		"get_project_documentation",
		"get_runtime_health",
		"get_theme_manifest",
		"get_theme_security_report",
		"list_documents",
		"list_plugins",
		"search_content",
		"validate_configuration",
		"validate_theme",
	}
	actualTools := make([]string, 0)
	for _, cap := range reg.Tools() {
		actualTools = append(actualTools, cap.Name)
	}
	if !equalStringSlices(expectedTools, actualTools) {
		t.Fatalf("tool set mismatch:\nwant %v\ngot  %v", expectedTools, actualTools)
	}

	expectedTemplates := []string{
		"foundry://content/collections/{type}",
		"foundry://content/pages/{slug}",
		"foundry://docs/{path...}",
		"foundry://plugins",
		"foundry://runtime/health",
		"foundry://themes/{name}/manifest",
		"foundry://themes/{name}/security-report",
	}
	actualTemplates := make([]string, 0)
	for _, cap := range reg.Resources() {
		actualTemplates = append(actualTemplates, cap.URITemplate)
	}
	if !equalStringSlices(expectedTemplates, actualTemplates) {
		t.Fatalf("resource set mismatch:\nwant %v\ngot  %v", expectedTemplates, actualTemplates)
	}

	for _, cap := range append(reg.Tools(), reg.Resources()...) {
		if cap.Mutability != ReadOnly {
			t.Errorf("capability %s is not read-only", cap.Name)
		}
		if !cap.Scope.ReadOnly() {
			t.Errorf("capability %s has a non-read scope %s", cap.Name, cap.Scope)
		}
	}
}

func TestSearchContentHandler(t *testing.T) {
	reg := NewRegistry()
	if err := RegisterCapabilities(reg, stubEnvironment(t)); err != nil {
		t.Fatalf("register capabilities: %v", err)
	}

	cap, ok := reg.Tool("search_content")
	if !ok {
		t.Fatal("search_content not registered")
	}

	result, err := cap.Handler(context.Background(), Invocation{
		Input: json.RawMessage(`{"query":"about"}`),
	})
	if err != nil {
		t.Fatalf("search handler: %v", err)
	}

	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal search result: %v", err)
	}
	var decoded struct {
		Query string `json:"query"`
		Total int    `json:"total"`
		Items []struct {
			URL string `json:"url"`
		} `json:"items"`
	}
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal search result: %v", err)
	}
	if decoded.Total != 1 || len(decoded.Items) != 1 || decoded.Items[0].URL != "/about/" {
		t.Fatalf("unexpected search output: %s", string(encoded))
	}
}

func TestGetDocumentHandlerUnknown(t *testing.T) {
	reg := NewRegistry()
	if err := RegisterCapabilities(reg, stubEnvironment(t)); err != nil {
		t.Fatalf("register capabilities: %v", err)
	}
	cap, _ := reg.Tool("get_document")
	if _, err := cap.Handler(context.Background(), Invocation{
		Input: json.RawMessage(`{"slug":"missing"}`),
	}); err == nil {
		t.Fatal("expected not-found error")
	}
}

func TestContentPageResourceMatching(t *testing.T) {
	reg := NewRegistry()
	if err := RegisterCapabilities(reg, stubEnvironment(t)); err != nil {
		t.Fatalf("register capabilities: %v", err)
	}
	cap, vars, ok := reg.LookupURI("foundry://content/pages/about")
	if !ok {
		t.Fatal("expected resource match")
	}
	if cap.Name != "content_page" || vars["slug"] != "about" {
		t.Fatalf("unexpected match: %s %v", cap.Name, vars)
	}

	result, err := cap.Handler(context.Background(), Invocation{URI: "foundry://content/pages/about", Vars: vars})
	if err != nil {
		t.Fatalf("resource handler: %v", err)
	}
	encoded, _ := json.Marshal(result)
	var decoded DocumentDetail
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal detail: %v", err)
	}
	if decoded.Slug != "about" || decoded.Body == "" {
		t.Fatalf("unexpected detail: %s", string(encoded))
	}
}

func TestInvalidArgumentsRejected(t *testing.T) {
	reg := NewRegistry()
	if err := RegisterCapabilities(reg, stubEnvironment(t)); err != nil {
		t.Fatalf("register capabilities: %v", err)
	}
	cap, _ := reg.Tool("search_content")
	if _, err := cap.Handler(context.Background(), Invocation{
		Input: json.RawMessage(`{"unknown_field":"x"}`),
	}); err == nil {
		t.Fatal("expected unknown field to be rejected")
	}
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	left := append([]string(nil), a...)
	right := append([]string(nil), b...)
	sort.Strings(left)
	sort.Strings(right)
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
