package mcp

import (
	"errors"
	"testing"

	adminauth "github.com/sphireinc/foundry/internal/admin/auth"
	"github.com/sphireinc/foundry/internal/config"
)

func testConfig() *config.Config {
	return &config.Config{}
}

func TestAuthorizeLocalReadAllowed(t *testing.T) {
	authz := NewAuthorizer(DefaultPolicy())
	if err := authz.Authorize(makeReadTool("search_content"), LocalActor()); err != nil {
		t.Fatalf("expected local read to be allowed, got %v", err)
	}
}

func TestAuthorizeMutatingDeniedByDefault(t *testing.T) {
	authz := NewAuthorizer(DefaultPolicy())
	mut := Capability{
		Name:       "create_draft",
		Kind:       KindTool,
		Scope:      ScopeWriteContent,
		Mutability: Mutating,
		Handler:    noopHandler,
	}
	err := authz.Authorize(mut, LocalActor())
	if !errors.Is(err, ErrDenied) {
		t.Fatalf("expected ErrDenied, got %v", err)
	}
}

func TestAuthorizeDisabledScopeDenied(t *testing.T) {
	policy, err := NewPolicy(PolicyOptions{EnabledScopes: []Scope{ScopeReadDocs}})
	if err != nil {
		t.Fatalf("new policy: %v", err)
	}
	authz := NewAuthorizer(policy)
	if err := authz.Authorize(makeReadTool("search_content"), LocalActor()); !errors.Is(err, ErrDenied) {
		t.Fatalf("expected ErrDenied for disabled scope, got %v", err)
	}
}

func TestAuthorizeAuthenticatedActorRequiresCapability(t *testing.T) {
	authz := NewAuthorizer(DefaultPolicy())

	denied := authActor("bob", nil)
	if err := authz.Authorize(makeReadTool("search_content"), denied); !errors.Is(err, ErrDenied) {
		t.Fatalf("expected ErrDenied for actor without capability, got %v", err)
	}

	allowed := authActor("alice", []string{"documents.read"})
	if err := authz.Authorize(makeReadTool("search_content"), allowed); err != nil {
		t.Fatalf("expected docs.read actor to be allowed, got %v", err)
	}

	admin := authActor("root", []string{"*"})
	if err := authz.Authorize(makeReadTool("search_content"), admin); err != nil {
		t.Fatalf("expected wildcard actor to be allowed, got %v", err)
	}
}

func authActor(username string, capabilities []string) Actor {
	return Actor{
		ID:            username,
		Authenticated: true,
		Capabilities:  capabilities,
	}
}

func TestActorFromIdentity(t *testing.T) {
	actor := ActorFromIdentity(&adminauth.Identity{
		Username:     "alice",
		Role:         "editor",
		Capabilities: []string{"documents.read"},
	})
	if !actor.Authenticated || actor.ID != "alice" || actor.Role != "editor" {
		t.Fatalf("unexpected actor: %+v", actor)
	}
	if actor.Local {
		t.Fatal("expected identity actor not to be local")
	}

	if local := ActorFromIdentity(nil); !local.Local || local.Authenticated {
		t.Fatalf("expected nil identity to map to local actor, got %+v", local)
	}
}
