package mcp

import (
	"context"
	"encoding/json"
	"testing"
)

func noopHandler(context.Context, Invocation) (any, error) { return nil, nil }

func makeReadTool(name string) Capability {
	return Capability{
		Name:       name,
		Kind:       KindTool,
		Scope:      ScopeReadContent,
		Mutability: ReadOnly,
		Handler:    noopHandler,
	}
}

func resource(template string) Capability {
	return Capability{
		Name:        "content_page",
		Kind:        KindResource,
		Scope:       ScopeReadContent,
		Mutability:  ReadOnly,
		URITemplate: template,
		Handler:     noopHandler,
	}
}

func TestRegisterToolAndList(t *testing.T) {
	reg := NewRegistry()
	if err := reg.Register(makeReadTool("search_content")); err != nil {
		t.Fatalf("register: %v", err)
	}
	if err := reg.Register(makeReadTool("get_document")); err != nil {
		t.Fatalf("register: %v", err)
	}

	tools := reg.Tools()
	if len(tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(tools))
	}
	if tools[0].Name != "get_document" || tools[1].Name != "search_content" {
		t.Fatalf("tools not sorted by name: %v", []string{tools[0].Name, tools[1].Name})
	}
	if tools[0].Timeout != DefaultTimeout || tools[0].MaxResultBytes != DefaultMaxResultBytes {
		t.Fatalf("defaults not applied: %+v", tools[0])
	}
	if tools[0].Version != DefaultCapabilityVer {
		t.Fatalf("expected default version, got %q", tools[0].Version)
	}

	if _, ok := reg.Tool("search_content"); !ok {
		t.Fatal("expected to find search_content")
	}
	if _, ok := reg.Tool("missing"); ok {
		t.Fatal("did not expect to find missing tool")
	}
}

func TestRegisterRejectsDuplicatesAndInvalid(t *testing.T) {
	reg := NewRegistry()
	if err := reg.Register(makeReadTool("search_content")); err != nil {
		t.Fatalf("register: %v", err)
	}
	if err := reg.Register(makeReadTool("search_content")); err == nil {
		t.Fatal("expected duplicate tool error")
	}

	cases := map[string]Capability{
		"invalid name": {Name: "Bad Name", Kind: KindTool, Scope: ScopeReadContent, Mutability: ReadOnly, Handler: noopHandler},
		"no handler":   {Name: "no_handler", Kind: KindTool, Scope: ScopeReadContent, Mutability: ReadOnly},
		"bad scope":    {Name: "bad_scope", Kind: KindTool, Scope: "read:everything", Mutability: ReadOnly, Handler: noopHandler},
		"readonly write scope": {
			Name: "contradiction", Kind: KindTool, Scope: ScopeWriteContent, Mutability: ReadOnly, Handler: noopHandler,
		},
		"mutating read scope": {
			Name: "contradiction2", Kind: KindTool, Scope: ScopeReadContent, Mutability: Mutating, Handler: noopHandler,
		},
		"tool with template": {
			Name: "weird", Kind: KindTool, Scope: ScopeReadContent, Mutability: ReadOnly,
			URITemplate: "foundry://content/pages", Handler: noopHandler,
		},
		"resource without template": {
			Name: "no_template", Kind: KindResource, Scope: ScopeReadContent, Mutability: ReadOnly, Handler: noopHandler,
		},
		"resource bad scheme": {
			Name: "bad_scheme", Kind: KindResource, Scope: ScopeReadContent, Mutability: ReadOnly,
			URITemplate: "http://example.com", Handler: noopHandler,
		},
	}
	for name, cap := range cases {
		if err := reg.Register(cap); err == nil {
			t.Errorf("%s: expected registration error", name)
		}
	}
}

func TestLookupURITemplateMatching(t *testing.T) {
	reg := NewRegistry()
	if err := reg.Register(resource("foundry://content/pages/{slug}")); err != nil {
		t.Fatalf("register: %v", err)
	}
	docs := resource("foundry://docs/{path...}")
	docs.Name = "docs"
	if err := reg.Register(docs); err != nil {
		t.Fatalf("register: %v", err)
	}

	cap, vars, ok := reg.LookupURI("foundry://content/pages/about")
	if !ok || cap.Name != "content_page" || vars["slug"] != "about" {
		t.Fatalf("page lookup failed: ok=%v cap=%q vars=%v", ok, cap.Name, vars)
	}

	cap, vars, ok = reg.LookupURI("foundry://docs/guides/getting-started")
	if !ok || cap.Name != "docs" || vars["path"] != "guides/getting-started" {
		t.Fatalf("docs lookup failed: ok=%v cap=%q vars=%v", ok, cap.Name, vars)
	}

	if _, _, ok := reg.LookupURI("foundry://content/pages"); ok {
		t.Fatal("expected empty slug not to match single-segment variable")
	}
	if _, _, ok := reg.LookupURI("foundry://unrelated"); ok {
		t.Fatal("expected unrelated uri to miss")
	}
}

func TestLookupURIPrefersLongestMatch(t *testing.T) {
	reg := NewRegistry()
	short := resource("foundry://content/pages/{slug}")
	short.Name = "short"
	long := resource("foundry://content/pages/{slug}/revisions")
	long.Name = "long"
	if err := reg.Register(short); err != nil {
		t.Fatalf("register short: %v", err)
	}
	if err := reg.Register(long); err != nil {
		t.Fatalf("register long: %v", err)
	}

	cap, _, ok := reg.LookupURI("foundry://content/pages/about/revisions")
	if !ok || cap.Name != "long" {
		t.Fatalf("expected longest match, got ok=%v cap=%q", ok, cap.Name)
	}
}

func TestCapabilityInputSchemaMarshals(t *testing.T) {
	cap := makeReadTool("search_content")
	cap.InputSchema = ObjectSchema(map[string]*Schema{
		"query": StringSchema("search terms"),
		"limit": IntSchema("max results"),
	}, "query")

	data, err := json.Marshal(cap.InputSchema)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded["type"] != "object" {
		t.Fatalf("expected object schema, got %v", decoded["type"])
	}
}
