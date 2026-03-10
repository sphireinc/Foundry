package types

import "time"

type DocumentSummary struct {
	ID         string              `json:"id"`
	Type       string              `json:"type"`
	Lang       string              `json:"lang"`
	Title      string              `json:"title"`
	Slug       string              `json:"slug"`
	URL        string              `json:"url"`
	Layout     string              `json:"layout"`
	SourcePath string              `json:"source_path"`
	Summary    string              `json:"summary"`
	Draft      bool                `json:"draft"`
	Date       *time.Time          `json:"date,omitempty"`
	UpdatedAt  *time.Time          `json:"updated_at,omitempty"`
	Taxonomies map[string][]string `json:"taxonomies,omitempty"`
}

type DocumentDetail struct {
	DocumentSummary
	RawBody  string         `json:"raw_body"`
	HTMLBody string         `json:"html_body"`
	Params   map[string]any `json:"params,omitempty"`
	Fields   map[string]any `json:"fields,omitempty"`
}

type DocumentListOptions struct {
	IncludeDrafts bool
	Type          string
	Lang          string
	Query         string
}

type DocumentSaveRequest struct {
	SourcePath string `json:"source_path"`
	Raw        string `json:"raw"`
}

type DocumentSaveResponse struct {
	SourcePath string `json:"source_path"`
	Size       int64  `json:"size"`
	Created    bool   `json:"created"`
}

type DocumentPreviewRequest struct {
	SourcePath string `json:"source_path"`
	Raw        string `json:"raw"`
}

type DocumentPreviewResponse struct {
	Title     string     `json:"title"`
	Slug      string     `json:"slug"`
	Layout    string     `json:"layout"`
	Summary   string     `json:"summary"`
	Draft     bool       `json:"draft"`
	Date      *time.Time `json:"date,omitempty"`
	UpdatedAt *time.Time `json:"updated_at,omitempty"`
	HTML      string     `json:"html"`
	WordCount int        `json:"word_count"`
}
