package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
)

type Scope string

const (
	ScopeReadContent    Scope = "read:content"
	ScopeReadDocs       Scope = "read:docs"
	ScopeReadThemes     Scope = "read:themes"
	ScopeReadPlugins    Scope = "read:plugins"
	ScopeReadOps        Scope = "read:ops"
	ScopeWriteContent   Scope = "write:content"
	ScopePublishContent Scope = "publish:content"
	ScopeManagePlugins  Scope = "manage:plugins"
	ScopeManageThemes   Scope = "manage:themes"
)

var allScopes = []Scope{
	ScopeReadContent,
	ScopeReadDocs,
	ScopeReadThemes,
	ScopeReadPlugins,
	ScopeReadOps,
	ScopeWriteContent,
	ScopePublishContent,
	ScopeManagePlugins,
	ScopeManageThemes,
}

func AllScopes() []Scope {
	out := make([]Scope, len(allScopes))
	copy(out, allScopes)
	return out
}

func (s Scope) Valid() bool {
	for _, known := range allScopes {
		if s == known {
			return true
		}
	}
	return false
}

func (s Scope) ReadOnly() bool {
	return strings.HasPrefix(string(s), "read:")
}

type Kind string

const (
	KindTool     Kind = "tool"
	KindResource Kind = "resource"
)

type Mutability string

const (
	ReadOnly Mutability = "read_only"
	Mutating Mutability = "mutating"
)

const (
	DefaultTimeout        = 10 * time.Second
	DefaultMaxResultBytes = 256 * 1024
	MaxResultBytesLimit   = 4 * 1024 * 1024
	DefaultPageSize       = 20
	MaxPageSize           = 100
	DefaultCapabilityVer  = "1"
)

const (
	Scheme       = "foundry"
	PluginScheme = "foundry-plugin"
)

type Page struct {
	Limit  int
	Cursor string
}

func (p Page) Normalize() Page {
	if p.Limit <= 0 {
		p.Limit = DefaultPageSize
	}
	if p.Limit > MaxPageSize {
		p.Limit = MaxPageSize
	}
	return p
}

type Invocation struct {
	Capability string
	URI        string
	Vars       map[string]string
	Input      json.RawMessage
	Page       Page
}

type Handler func(ctx context.Context, inv Invocation) (any, error)

type Schema struct {
	Type                 string             `json:"type,omitempty"`
	Description          string             `json:"description,omitempty"`
	Properties           map[string]*Schema `json:"properties,omitempty"`
	Required             []string           `json:"required,omitempty"`
	Items                *Schema            `json:"items,omitempty"`
	Enum                 []string           `json:"enum,omitempty"`
	AdditionalProperties *bool              `json:"additionalProperties,omitempty"`
}

func ObjectSchema(props map[string]*Schema, required ...string) *Schema {
	return &Schema{
		Type:       "object",
		Properties: props,
		Required:   required,
	}
}

func StringSchema(description string) *Schema {
	return &Schema{Type: "string", Description: description}
}

func IntSchema(description string) *Schema {
	return &Schema{Type: "integer", Description: description}
}

func BoolSchema(description string) *Schema {
	return &Schema{Type: "boolean", Description: description}
}

type Capability struct {
	Name           string
	Version        string
	Kind           Kind
	Description    string
	InputSchema    *Schema
	OutputSchema   *Schema
	Mutability     Mutability
	Scope          Scope
	Plugin         string
	AuditCategory  string
	Timeout        time.Duration
	MaxResultBytes int
	DefaultEnabled bool
	URITemplate    string
	Handler        Handler
}

var (
	capabilityNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	pluginNamePattern     = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
)

func (c Capability) withDefaults() Capability {
	if strings.TrimSpace(c.Version) == "" {
		c.Version = DefaultCapabilityVer
	}
	if c.Timeout <= 0 {
		c.Timeout = DefaultTimeout
	}
	if c.MaxResultBytes <= 0 {
		c.MaxResultBytes = DefaultMaxResultBytes
	}
	if c.AuditCategory == "" {
		c.AuditCategory = string(c.Scope)
	}
	return c
}

func (c Capability) Validate() error {
	if !capabilityNamePattern.MatchString(c.Name) {
		return fmt.Errorf("mcp: invalid capability name %q", c.Name)
	}
	switch c.Kind {
	case KindTool, KindResource:
	default:
		return fmt.Errorf("mcp: capability %q has invalid kind %q", c.Name, c.Kind)
	}
	if !c.Scope.Valid() {
		return fmt.Errorf("mcp: capability %q has invalid scope %q", c.Name, c.Scope)
	}
	switch c.Mutability {
	case ReadOnly:
		if !c.Scope.ReadOnly() {
			return fmt.Errorf("mcp: read-only capability %q requires a read scope, got %q", c.Name, c.Scope)
		}
	case Mutating:
		if c.Scope.ReadOnly() {
			return fmt.Errorf("mcp: mutating capability %q requires a write scope, got %q", c.Name, c.Scope)
		}
	default:
		return fmt.Errorf("mcp: capability %q has invalid mutability %q", c.Name, c.Mutability)
	}
	if c.Handler == nil {
		return fmt.Errorf("mcp: capability %q has no handler", c.Name)
	}
	if c.MaxResultBytes > MaxResultBytesLimit {
		return fmt.Errorf("mcp: capability %q max result bytes %d exceeds limit %d", c.Name, c.MaxResultBytes, MaxResultBytesLimit)
	}
	if c.Plugin != "" && !pluginNamePattern.MatchString(c.Plugin) {
		return fmt.Errorf("mcp: capability %q has invalid plugin namespace %q", c.Name, c.Plugin)
	}
	switch c.Kind {
	case KindResource:
		if strings.TrimSpace(c.URITemplate) == "" {
			return fmt.Errorf("mcp: resource capability %q missing template", c.Name)
		}
		if err := validateURITemplate(c.URITemplate); err != nil {
			return fmt.Errorf("mcp: resource capability %q: %w", c.Name, err)
		}
	case KindTool:
		if strings.TrimSpace(c.URITemplate) != "" {
			return fmt.Errorf("mcp: tool capability %q must not declare a template", c.Name)
		}
	}
	return nil
}

func validateURITemplate(tmpl string) error {
	scheme := ""
	switch {
	case strings.HasPrefix(tmpl, Scheme+"://"):
		scheme = Scheme
	case strings.HasPrefix(tmpl, PluginScheme+"://"):
		scheme = PluginScheme
	default:
		return fmt.Errorf("template %q must use the %s:// or %s:// scheme", tmpl, Scheme, PluginScheme)
	}
	rest := strings.TrimPrefix(tmpl, scheme+"://")
	segments := strings.Split(rest, "/")
	seen := map[string]bool{}
	for i, seg := range segments {
		name, isRest, ok := parseVar(seg)
		if !ok {
			continue
		}
		if !capabilityNamePattern.MatchString(name) {
			return fmt.Errorf("template %q has invalid variable %q", tmpl, seg)
		}
		if seen[name] {
			return fmt.Errorf("template %q repeats variable %q", tmpl, name)
		}
		seen[name] = true
		if isRest && i != len(segments)-1 {
			return fmt.Errorf("template %q rest variable %q must be last", tmpl, seg)
		}
	}
	return nil
}

func parseVar(seg string) (name string, rest bool, ok bool) {
	if !strings.HasPrefix(seg, "{") || !strings.HasSuffix(seg, "}") {
		return "", false, false
	}
	inner := seg[1 : len(seg)-1]
	if strings.HasSuffix(inner, "...") {
		return strings.TrimSpace(strings.TrimSuffix(inner, "...")), true, true
	}
	return strings.TrimSpace(inner), false, true
}

func PluginCapabilityName(plugin, name string) string {
	return plugin + "_" + name
}

func PluginURITemplate(plugin, path string) string {
	return PluginScheme + "://" + plugin + "/" + strings.TrimPrefix(path, "/")
}
