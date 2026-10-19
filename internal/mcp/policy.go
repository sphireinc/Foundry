package mcp

import (
	"fmt"
	"sort"
	"strings"

	"github.com/sphireinc/foundry/internal/config"
)

const (
	DefaultRatePerMinute  = 600
	DefaultRateBurst      = 120
	DefaultMaxConcurrency = 16
)

type RateLimit struct {
	RequestsPerMinute int
	Burst             int
}

type PolicyOptions struct {
	EnabledScopes  []Scope
	Mutating       bool
	Plugins        []string
	RateLimit      RateLimit
	MaxConcurrency int
}

type Policy struct {
	enabled        map[Scope]struct{}
	mutating       bool
	plugins        map[string]struct{}
	rateLimit      RateLimit
	maxConcurrency int
}

func DefaultPolicy() *Policy {
	return &Policy{
		enabled:        readScopeSet(),
		mutating:       false,
		plugins:        map[string]struct{}{},
		rateLimit:      RateLimit{RequestsPerMinute: DefaultRatePerMinute, Burst: DefaultRateBurst},
		maxConcurrency: DefaultMaxConcurrency,
	}
}

func NewPolicy(opts PolicyOptions) (*Policy, error) {
	policy := &Policy{
		enabled:        map[Scope]struct{}{},
		mutating:       opts.Mutating,
		plugins:        map[string]struct{}{},
		rateLimit:      opts.RateLimit.withDefaults(),
		maxConcurrency: opts.MaxConcurrency,
	}
	if policy.maxConcurrency <= 0 {
		policy.maxConcurrency = DefaultMaxConcurrency
	}

	scopes := opts.EnabledScopes
	if len(scopes) == 0 {
		if opts.Mutating {
			scopes = AllScopes()
		} else {
			scopes = readScopes()
		}
	}

	for _, scope := range scopes {
		if !scope.Valid() {
			return nil, fmt.Errorf("mcp: unknown scope %q", scope)
		}
		if !scope.ReadOnly() && !opts.Mutating {
			return nil, fmt.Errorf("mcp: scope %q requires mutating capabilities to be enabled", scope)
		}
		policy.enabled[scope] = struct{}{}
	}

	for _, name := range opts.Plugins {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		policy.plugins[name] = struct{}{}
	}

	return policy, nil
}

func (r RateLimit) withDefaults() RateLimit {
	if r.RequestsPerMinute <= 0 {
		r.RequestsPerMinute = DefaultRatePerMinute
	}
	if r.Burst <= 0 {
		r.Burst = DefaultRateBurst
	}
	return r
}

func PolicyFromConfig(cfg *config.Config) (*Policy, error) {
	opts := PolicyOptions{}
	if cfg != nil {
		opts.Mutating = !cfg.MCP.ReadOnlyOrDefault()
		for _, scope := range cfg.MCP.Scopes {
			opts.EnabledScopes = append(opts.EnabledScopes, Scope(strings.TrimSpace(scope)))
		}
		opts.Plugins = cfg.MCP.Plugins
		opts.RateLimit = RateLimit{
			RequestsPerMinute: cfg.MCP.RateLimit.RequestsPerMinute,
			Burst:             cfg.MCP.RateLimit.Burst,
		}
		opts.MaxConcurrency = cfg.MCP.MaxConcurrency
	}
	return NewPolicy(opts)
}

func (p *Policy) ScopeEnabled(scope Scope) bool {
	if p == nil {
		return false
	}
	_, ok := p.enabled[scope]
	return ok
}

func (p *Policy) MutatingAllowed() bool {
	return p != nil && p.mutating
}

func (p *Policy) PluginEnabled(name string) bool {
	if p == nil {
		return false
	}
	_, ok := p.plugins[strings.TrimSpace(name)]
	return ok
}

func (p *Policy) EnabledScopes() []Scope {
	if p == nil {
		return nil
	}
	out := make([]Scope, 0, len(p.enabled))
	for scope := range p.enabled {
		out = append(out, scope)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func (p *Policy) RateLimit() RateLimit {
	if p == nil {
		return RateLimit{}
	}
	return p.rateLimit
}

func (p *Policy) MaxConcurrency() int {
	if p == nil {
		return 0
	}
	return p.maxConcurrency
}

func readScopes() []Scope {
	out := make([]Scope, 0, len(allScopes))
	for _, scope := range allScopes {
		if scope.ReadOnly() {
			out = append(out, scope)
		}
	}
	return out
}

func readScopeSet() map[Scope]struct{} {
	out := make(map[Scope]struct{}, len(allScopes))
	for _, scope := range readScopes() {
		out[scope] = struct{}{}
	}
	return out
}
