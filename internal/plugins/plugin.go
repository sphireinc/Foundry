package plugins

import "github.com/sphireinc/foundry/internal/content"

type Plugin interface {
	Name() string
}

type Factory func() Plugin

type DocumentParsedHook interface {
	OnDocumentParsed(*content.Document) error
}

type GraphBuiltHook interface {
	OnGraphBuilt(*content.SiteGraph) error
}

type Manager struct {
	plugins []Plugin
}

func NewManager(enabled []string) *Manager {
	m := &Manager{
		plugins: make([]Plugin, 0),
	}

	for _, name := range enabled {
		if factory, ok := registry[name]; ok {
			m.plugins = append(m.plugins, factory())
		}
	}

	return m
}

func (m *Manager) OnDocumentParsed(doc *content.Document) error {
	for _, p := range m.plugins {
		if hook, ok := p.(DocumentParsedHook); ok {
			if err := hook.OnDocumentParsed(doc); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *Manager) OnGraphBuilt(graph *content.SiteGraph) error {
	for _, p := range m.plugins {
		if hook, ok := p.(GraphBuiltHook); ok {
			if err := hook.OnGraphBuilt(graph); err != nil {
				return err
			}
		}
	}
	return nil
}
