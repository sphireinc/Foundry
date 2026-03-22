package content

import (
	"html/template"
	"time"
)

type Document struct {
	ID         string
	Type       string
	Lang       string
	IsDefault  bool
	Status     string
	Title      string
	Slug       string
	URL        string
	Layout     string
	SourcePath string
	RelPath    string
	RawBody    string
	HTMLBody   template.HTML
	Summary    string
	Date       *time.Time
	CreatedAt  *time.Time
	UpdatedAt  *time.Time
	Draft      bool
	Author     string
	LastEditor string
	Params     map[string]any
	Fields     map[string]any
	Taxonomies map[string][]string
}

type FrontMatter struct {
	Title      string              `yaml:"title"`
	Slug       string              `yaml:"slug"`
	Layout     string              `yaml:"layout"`
	Draft      bool                `yaml:"draft"`
	Summary    string              `yaml:"summary"`
	Date       *time.Time          `yaml:"date"`
	CreatedAt  *time.Time          `yaml:"created_at,omitempty"`
	UpdatedAt  *time.Time          `yaml:"updated_at"`
	Author     string              `yaml:"author,omitempty"`
	LastEditor string              `yaml:"last_editor,omitempty"`
	Fields     map[string]any      `yaml:"fields"`
	Tags       []string            `yaml:"tags"`
	Categories []string            `yaml:"categories"`
	Taxonomies map[string][]string `yaml:"taxonomies"`
	Params     map[string]any      `yaml:",inline"`
}
