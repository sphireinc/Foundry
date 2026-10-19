package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func testServer(t *testing.T, actor Actor, policy *Policy) *Server {
	t.Helper()
	registry := NewRegistry()
	if err := RegisterCapabilities(registry, stubEnvironment(t)); err != nil {
		t.Fatalf("register capabilities: %v", err)
	}
	server, err := NewServer(ServerOptions{
		Registry:   registry,
		Authorizer: NewAuthorizer(policy),
		Actor:      actor,
		Info:       Implementation{Name: "foundry", Version: "test"},
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	return server
}

func serve(t *testing.T, server *Server, input string) []rpcResponse {
	t.Helper()
	var out bytes.Buffer
	if err := server.ServeStdio(context.Background(), strings.NewReader(input), &out); err != nil {
		t.Fatalf("serve stdio: %v", err)
	}

	responses := []rpcResponse{}
	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var resp rpcResponse
		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			t.Fatalf("stdout line is not valid JSON-RPC: %q (%v)", line, err)
		}
		responses = append(responses, resp)
	}
	return responses
}

func decodeResult[T any](t *testing.T, resp rpcResponse) T {
	t.Helper()
	var out T
	data, err := json.Marshal(resp.Result)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	return out
}

func TestInitializeNegotiatesProtocolVersion(t *testing.T) {
	server := testServer(t, LocalActor(), DefaultPolicy())

	responses := serve(t, server, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","clientInfo":{"name":"test","version":"1"}}}`+"\n")
	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}

	result := decodeResult[struct {
		ProtocolVersion string `json:"protocolVersion"`
		ServerInfo      struct {
			Name string `json:"name"`
		} `json:"serverInfo"`
	}](t, responses[0])

	if result.ProtocolVersion != "2025-06-18" {
		t.Fatalf("expected negotiated version, got %q", result.ProtocolVersion)
	}
	if result.ServerInfo.Name != "foundry" {
		t.Fatalf("expected foundry server info, got %q", result.ServerInfo.Name)
	}
}

func TestInitializeFallsBackToLatestVersion(t *testing.T) {
	server := testServer(t, LocalActor(), DefaultPolicy())
	responses := serve(t, server, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"1999-01-01"}}`+"\n")

	result := decodeResult[struct {
		ProtocolVersion string `json:"protocolVersion"`
	}](t, responses[0])
	if result.ProtocolVersion != ProtocolVersion {
		t.Fatalf("expected %q, got %q", ProtocolVersion, result.ProtocolVersion)
	}
}

func TestToolsListAndCall(t *testing.T) {
	server := testServer(t, LocalActor(), DefaultPolicy())
	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"search_content","arguments":{"query":"hello"}}}`,
	}, "\n") + "\n"

	responses := serve(t, server, input)
	if len(responses) != 3 {
		t.Fatalf("expected 3 responses (notification produces none), got %d", len(responses))
	}

	list := decodeResult[struct {
		Tools []struct {
			Name        string          `json:"name"`
			InputSchema json.RawMessage `json:"inputSchema"`
		} `json:"tools"`
	}](t, responses[1])
	if len(list.Tools) != 13 {
		t.Fatalf("expected 13 tools, got %d", len(list.Tools))
	}
	for _, tool := range list.Tools {
		if len(tool.InputSchema) == 0 {
			t.Errorf("tool %s is missing an input schema", tool.Name)
		}
	}

	call := decodeResult[struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}](t, responses[2])
	if call.IsError || len(call.Content) != 1 {
		t.Fatalf("unexpected tool call response: %+v", call)
	}

	var payload struct {
		Total int `json:"total"`
		Items []struct {
			URL string `json:"url"`
		} `json:"items"`
	}
	if err := json.Unmarshal([]byte(call.Content[0].Text), &payload); err != nil {
		t.Fatalf("tool payload is not JSON: %v", err)
	}
	if payload.Total != 1 || payload.Items[0].URL != "/posts/hello/" {
		t.Fatalf("unexpected search payload: %s", call.Content[0].Text)
	}
}

func TestUnknownMethodAndTool(t *testing.T) {
	server := testServer(t, LocalActor(), DefaultPolicy())
	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"does/not/exist"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"missing_tool"}}`,
	}, "\n") + "\n"

	responses := serve(t, server, input)
	if len(responses) != 2 {
		t.Fatalf("expected 2 responses, got %d", len(responses))
	}
	for _, resp := range responses {
		if resp.Error == nil || resp.Error.Code != codeMethodNotFound {
			t.Fatalf("expected method-not-found error, got %+v", resp.Error)
		}
	}
}

func TestParseErrorProducesProtocolError(t *testing.T) {
	server := testServer(t, LocalActor(), DefaultPolicy())
	responses := serve(t, server, "{not json}\n")
	if len(responses) != 1 || responses[0].Error == nil || responses[0].Error.Code != codeParseError {
		t.Fatalf("expected parse error, got %+v", responses)
	}
}

func TestMutatingToolDeniedByDefault(t *testing.T) {
	registry := NewRegistry()
	if err := RegisterCapabilities(registry, stubEnvironment(t)); err != nil {
		t.Fatalf("register capabilities: %v", err)
	}
	mutating := Capability{
		Name:        "create_draft",
		Kind:        KindTool,
		Scope:       ScopeWriteContent,
		Mutability:  Mutating,
		Description: "create a draft",
		Handler: func(context.Context, Invocation) (any, error) {
			return map[string]any{"ok": true}, nil
		},
	}
	if err := registry.Register(mutating); err != nil {
		t.Fatalf("register mutating tool: %v", err)
	}

	server, err := NewServer(ServerOptions{Registry: registry, Authorizer: NewAuthorizer(DefaultPolicy()), Actor: LocalActor()})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	responses := serve(t, server, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"create_draft"}}`+"\n")
	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}
	call := decodeResult[struct {
		IsError bool `json:"isError"`
	}](t, responses[0])
	if !call.IsError {
		t.Fatal("expected mutating tool call to be denied")
	}
}

func TestSensitiveValuesRedacted(t *testing.T) {
	registry := NewRegistry()
	secretTool := readTool("leaky_tool", "returns secrets", ScopeReadContent, nil, func(context.Context, Invocation) (any, error) {
		return map[string]any{
			"password_hash": "super-secret-hash",
			"nested": map[string]any{
				"access_token": "token-value",
				"safe":         "public",
			},
		}, nil
	})
	if err := registry.Register(secretTool); err != nil {
		t.Fatalf("register tool: %v", err)
	}

	server, err := NewServer(ServerOptions{Registry: registry, Authorizer: NewAuthorizer(DefaultPolicy()), Actor: LocalActor()})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	responses := serve(t, server, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"leaky_tool"}}`+"\n")
	call := decodeResult[struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}](t, responses[0])
	text := call.Content[0].Text
	if strings.Contains(text, "super-secret-hash") || strings.Contains(text, "token-value") {
		t.Fatalf("expected secrets to be redacted, got %s", text)
	}
	if !strings.Contains(text, Redacted) || !strings.Contains(text, "public") {
		t.Fatalf("expected redaction markers and safe values, got %s", text)
	}
}

func TestResourcesListAndRead(t *testing.T) {
	server := testServer(t, LocalActor(), DefaultPolicy())
	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"resources/templates/list"}`,
		`{"jsonrpc":"2.0","id":2,"method":"resources/read","params":{"uri":"foundry://content/pages/about"}}`,
	}, "\n") + "\n"

	responses := serve(t, server, input)
	if len(responses) != 2 {
		t.Fatalf("expected 2 responses, got %d", len(responses))
	}

	templates := decodeResult[struct {
		ResourceTemplates []struct {
			URITemplate string `json:"uriTemplate"`
		} `json:"resourceTemplates"`
	}](t, responses[0])
	if len(templates.ResourceTemplates) == 0 {
		t.Fatal("expected resource templates")
	}

	read := decodeResult[struct {
		Contents []struct {
			URI  string `json:"uri"`
			Text string `json:"text"`
		} `json:"contents"`
	}](t, responses[1])
	if len(read.Contents) != 1 || read.Contents[0].URI != "foundry://content/pages/about" {
		t.Fatalf("unexpected resource read: %+v", read)
	}
}

func TestStdioEOFShutsDownCleanly(t *testing.T) {
	server := testServer(t, LocalActor(), DefaultPolicy())
	var out bytes.Buffer
	if err := server.ServeStdio(context.Background(), strings.NewReader(""), &out); err != nil {
		t.Fatalf("expected clean shutdown, got %v", err)
	}
	if out.Len() != 0 {
		t.Fatalf("expected no stdout output, got %q", out.String())
	}
}
