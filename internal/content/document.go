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
	UpdatedAt  *time.Time
	Draft      bool
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
	UpdatedAt  *time.Time          `yaml:"updated_at"`
	Fields     map[string]any      `yaml:"fields"`
	Tags       []string            `yaml:"tags"`
	Categories []string            `yaml:"categories"`
	Taxonomies map[string][]string `yaml:"taxonomies"`
	Params     map[string]any      `yaml:",inline"`
}
