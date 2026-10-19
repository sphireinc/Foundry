package mcp

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	admintypes "github.com/sphireinc/foundry/internal/admin/types"
)

type recordingAuditor struct {
	mu      sync.Mutex
	entries []admintypes.AuditEntry
}

func (a *recordingAuditor) Log(entry admintypes.AuditEntry) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.entries = append(a.entries, entry)
	return nil
}

func (a *recordingAuditor) snapshot() []admintypes.AuditEntry {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]admintypes.AuditEntry, len(a.entries))
	copy(out, a.entries)
	return out
}

func newTestServerWithOptions(t *testing.T, opts ServerOptions) *Server {
	t.Helper()
	registry := NewRegistry()
	if err := RegisterCapabilities(registry, stubEnvironment(t)); err != nil {
		t.Fatalf("register capabilities: %v", err)
	}
	opts.Registry = registry
	if strings.TrimSpace(opts.Info.Name) == "" {
		opts.Info = Implementation{Name: "foundry", Version: "test"}
	}
	server, err := NewServer(opts)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	return server
}

func requestBatch(requests ...string) string {
	return strings.Join(requests, "\n") + "\n"
}

func TestAuditRecordsSuccess(t *testing.T) {
	recorder := &recordingAuditor{}
	registry := NewRegistry()
	tool := readTool("search_content", "search", ScopeReadContent, nil, func(context.Context, Invocation) (any, error) {
		return map[string]any{"ok": true}, nil
	})
	if err := registry.Register(tool); err != nil {
		t.Fatalf("register tool: %v", err)
	}

	server, err := NewServer(ServerOptions{
		Registry:   registry,
		Authorizer: NewAuthorizer(DefaultPolicy()),
		Auditor:    recorder,
		Actor:      LocalActor(),
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	serve(t, server, requestBatch(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"search_content"}}`))

	entries := recorder.snapshot()
	outcomes := map[string]bool{}
	for _, entry := range entries {
		outcomes[entry.Outcome] = true
		if entry.Action != "mcp.read:content" {
			t.Fatalf("expected mcp.read:content action, got %q", entry.Action)
		}
		if entry.Metadata["capability"] != "search_content" {
			t.Fatalf("expected capability metadata, got %+v", entry.Metadata)
		}
	}
	if !outcomes["started"] || !outcomes["success"] {
		t.Fatalf("expected started and success audit events, got %+v", entries)
	}
}

func TestAuditRecordsDenied(t *testing.T) {
	recorder := &recordingAuditor{}
	registry := NewRegistry()
	tool := readTool("search_content", "search", ScopeReadContent, nil, func(context.Context, Invocation) (any, error) {
		return map[string]any{"ok": true}, nil
	})
	if err := registry.Register(tool); err != nil {
		t.Fatalf("register tool: %v", err)
	}

	server, err := NewServer(ServerOptions{
		Registry:   registry,
		Authorizer: NewAuthorizer(DefaultPolicy()),
		Auditor:    recorder,
		Actor:      Actor{ID: "viewer", Authenticated: true, Role: "viewer"},
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	responses := serve(t, server, requestBatch(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"search_content"}}`))
	call := decodeResult[struct {
		IsError bool `json:"isError"`
	}](t, responses[0])
	if !call.IsError {
		t.Fatal("expected denied call to report an error")
	}

	entries := recorder.snapshot()
	if len(entries) == 0 || entries[0].Outcome != "denied" {
		t.Fatalf("expected a denied audit event, got %+v", entries)
	}
}

func TestToolTimeoutReturnsError(t *testing.T) {
	registry := NewRegistry()
	tool := readTool("slow_tool", "blocks", ScopeReadContent, nil, func(ctx context.Context, _ Invocation) (any, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	})
	tool.Timeout = 20 * time.Millisecond
	if err := registry.Register(tool); err != nil {
		t.Fatalf("register tool: %v", err)
	}

	server, err := NewServer(ServerOptions{
		Registry:   registry,
		Authorizer: NewAuthorizer(DefaultPolicy()),
		Actor:      LocalActor(),
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	responses := serve(t, server, requestBatch(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"slow_tool"}}`))
	call := decodeResult[struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}](t, responses[0])
	if !call.IsError {
		t.Fatal("expected timeout to produce an error result")
	}
	if len(call.Content) == 0 || !strings.Contains(call.Content[0].Text, "deadline") {
		t.Fatalf("expected deadline error text, got %+v", call.Content)
	}
}
