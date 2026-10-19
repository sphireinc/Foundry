package mcp

import (
	"errors"
	"fmt"

	adminauth "github.com/sphireinc/foundry/internal/admin/auth"
)

var ErrDenied = errors.New("mcp: access denied")

type Actor struct {
	ID            string
	Local         bool
	Authenticated bool
	Role          string
	Capabilities  []string
}

func LocalActor() Actor {
	return Actor{ID: "local", Local: true}
}

func ActorFromIdentity(identity *adminauth.Identity) Actor {
	if identity == nil {
		return LocalActor()
	}
	return Actor{
		ID:            identity.Username,
		Authenticated: true,
		Role:          identity.Role,
		Capabilities:  append([]string(nil), identity.Capabilities...),
	}
}

var scopeCapabilities = map[Scope][]string{
	ScopeReadContent:    {"documents.read", "dashboard.read"},
	ScopeReadDocs:       {"dashboard.read"},
	ScopeReadThemes:     {"themes.manage"},
	ScopeReadPlugins:    {"plugins.manage"},
	ScopeReadOps:        {"dashboard.read"},
	ScopeWriteContent:   {"documents.write"},
	ScopePublishContent: {"documents.publish"},
	ScopeManagePlugins:  {"plugins.manage"},
	ScopeManageThemes:   {"themes.manage"},
}

func RequiredCapabilities(scope Scope) []string {
	out := scopeCapabilities[scope]
	result := make([]string, len(out))
	copy(result, out)
	return result
}

type Authorizer struct {
	policy *Policy
}

func NewAuthorizer(policy *Policy) *Authorizer {
	if policy == nil {
		policy = DefaultPolicy()
	}
	return &Authorizer{policy: policy}
}

func (a *Authorizer) Policy() *Policy {
	if a == nil {
		return nil
	}
	return a.policy
}

func (a *Authorizer) Authorize(cap Capability, actor Actor) error {
	if a == nil || a.policy == nil {
		return fmt.Errorf("%w: no policy configured", ErrDenied)
	}

	cap = cap.withDefaults()
	if err := cap.Validate(); err != nil {
		return err
	}
	if !a.policy.ScopeEnabled(cap.Scope) {
		return fmt.Errorf("%w: scope %s is not enabled", ErrDenied, cap.Scope)
	}
	if cap.Mutability == Mutating && !a.policy.MutatingAllowed() {
		return fmt.Errorf("%w: mutating capabilities are disabled", ErrDenied)
	}
	if cap.Plugin != "" && !a.policy.PluginEnabled(cap.Plugin) {
		return fmt.Errorf("%w: plugin %s is not registered for MCP", ErrDenied, cap.Plugin)
	}
	if actor.Authenticated && !actorAllowed(actor, cap.Scope) {
		return fmt.Errorf("%w: actor %s lacks a Foundry capability for scope %s", ErrDenied, actor.ID, cap.Scope)
	}
	return nil
}

func actorAllowed(actor Actor, scope Scope) bool {
	required := scopeCapabilities[scope]
	if len(required) == 0 {
		return false
	}
	for _, capability := range required {
		if adminauth.CapabilityAllowed(actor.Capabilities, capability) {
			return true
		}
	}
	return false
}
