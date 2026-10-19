package mcp

import "testing"

func TestDefaultPolicyIsReadOnly(t *testing.T) {
	policy := DefaultPolicy()
	for _, scope := range readScopes() {
		if !policy.ScopeEnabled(scope) {
			t.Errorf("expected read scope %s to be enabled", scope)
		}
	}
	for _, scope := range allScopes {
		if scope.ReadOnly() {
			continue
		}
		if policy.ScopeEnabled(scope) {
			t.Errorf("expected non-read scope %s to be disabled", scope)
		}
	}
	if policy.MutatingAllowed() {
		t.Fatal("expected default policy to forbid mutating capabilities")
	}
}

func TestNewPolicyRejectsMutatingScopeWithoutOptIn(t *testing.T) {
	_, err := NewPolicy(PolicyOptions{EnabledScopes: []Scope{ScopeWriteContent}})
	if err == nil {
		t.Fatal("expected error for mutating scope without opt-in")
	}
}

func TestNewPolicyRejectsUnknownScope(t *testing.T) {
	_, err := NewPolicy(PolicyOptions{EnabledScopes: []Scope{"read:everything"}})
	if err == nil {
		t.Fatal("expected error for unknown scope")
	}
}

func TestNewPolicyMutatingOptIn(t *testing.T) {
	policy, err := NewPolicy(PolicyOptions{Mutating: true})
	if err != nil {
		t.Fatalf("new policy: %v", err)
	}
	if !policy.MutatingAllowed() {
		t.Fatal("expected mutating to be allowed")
	}
	for _, scope := range AllScopes() {
		if !policy.ScopeEnabled(scope) {
			t.Errorf("expected scope %s to be enabled", scope)
		}
	}
}

func TestPolicyFromConfig(t *testing.T) {
	policy, err := PolicyFromConfig(nil)
	if err != nil {
		t.Fatalf("policy from nil config: %v", err)
	}
	if policy.MutatingAllowed() {
		t.Fatal("expected read-only default")
	}

	cfg := testConfig()
	cfg.MCP.Scopes = []string{"read:content", "read:docs"}
	policy, err = PolicyFromConfig(cfg)
	if err != nil {
		t.Fatalf("policy from config: %v", err)
	}
	if !policy.ScopeEnabled(ScopeReadContent) || !policy.ScopeEnabled(ScopeReadDocs) {
		t.Fatal("expected configured scopes to be enabled")
	}
	if policy.ScopeEnabled(ScopeReadOps) {
		t.Fatal("expected unconfigured scope to be disabled")
	}
}
