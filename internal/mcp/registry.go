package mcp

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

type Registry struct {
	mu        sync.RWMutex
	tools     map[string]Capability
	resources map[string]Capability
}

func NewRegistry() *Registry {
	return &Registry{
		tools:     map[string]Capability{},
		resources: map[string]Capability{},
	}
}

func (r *Registry) Register(cap Capability) error {
	cap = cap.withDefaults()
	if err := cap.Validate(); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	switch cap.Kind {
	case KindTool:
		if _, exists := r.tools[cap.Name]; exists {
			return fmt.Errorf("mcp: duplicate tool capability %q", cap.Name)
		}
		r.tools[cap.Name] = cap
	case KindResource:
		if _, exists := r.resources[cap.URITemplate]; exists {
			return fmt.Errorf("mcp: duplicate resource capability %q", cap.URITemplate)
		}
		r.resources[cap.URITemplate] = cap
	}
	return nil
}

func (r *Registry) Tools() []Capability {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]Capability, 0, len(r.tools))
	for _, cap := range r.tools {
		out = append(out, cap)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func (r *Registry) Resources() []Capability {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]Capability, 0, len(r.resources))
	for _, cap := range r.resources {
		out = append(out, cap)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].URITemplate < out[j].URITemplate })
	return out
}

func (r *Registry) Tool(name string) (Capability, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	cap, ok := r.tools[name]
	return cap, ok
}

func (r *Registry) Resource(template string) (Capability, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	cap, ok := r.resources[template]
	return cap, ok
}

func (r *Registry) LookupURI(uri string) (Capability, map[string]string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	templates := make([]string, 0, len(r.resources))
	for tmpl := range r.resources {
		templates = append(templates, tmpl)
	}
	sort.Slice(templates, func(i, j int) bool {
		if len(templates[i]) != len(templates[j]) {
			return len(templates[i]) > len(templates[j])
		}
		return templates[i] < templates[j]
	})

	for _, tmpl := range templates {
		if vars, ok := matchTemplate(tmpl, uri); ok {
			return r.resources[tmpl], vars, true
		}
	}
	return Capability{}, nil, false
}

func (r *Registry) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return len(r.tools) + len(r.resources)
}

func (r *Registry) Empty() bool {
	return r.Len() == 0
}

func matchTemplate(template, uri string) (map[string]string, bool) {
	tSegs := strings.Split(template, "/")
	uSegs := strings.Split(uri, "/")

	if len(tSegs) == 0 || len(uSegs) < len(tSegs) {
		return nil, false
	}

	if len(uSegs) > len(tSegs) {
		if _, isRest, ok := parseVar(tSegs[len(tSegs)-1]); !ok || !isRest {
			return nil, false
		}
	}

	vars := map[string]string{}
	for i, seg := range tSegs {
		if name, isRest, ok := parseVar(seg); ok {
			if isRest {
				vars[name] = strings.Join(uSegs[i:], "/")
				return vars, true
			}
			if i >= len(uSegs) || uSegs[i] == "" {
				return nil, false
			}
			vars[name] = uSegs[i]
			continue
		}
		if i >= len(uSegs) || seg != uSegs[i] {
			return nil, false
		}
	}
	return vars, true
}
