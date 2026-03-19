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
	Archived   bool                `json:"archived,omitempty"`
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

type DocumentCreateRequest struct {
	Kind      string `json:"kind"`
	Slug      string `json:"slug"`
	Lang      string `json:"lang,omitempty"`
	Archetype string `json:"archetype,omitempty"`
}

type DocumentCreateResponse struct {
	Kind       string `json:"kind"`
	Slug       string `json:"slug"`
	Lang       string `json:"lang"`
	Archetype  string `json:"archetype"`
	SourcePath string `json:"source_path"`
	Created    bool   `json:"created"`
	Raw        string `json:"raw,omitempty"`
}

type DocumentStatusRequest struct {
	SourcePath string `json:"source_path"`
	Status     string `json:"status"`
}

type DocumentStatusResponse struct {
	SourcePath string `json:"source_path"`
	Status     string `json:"status"`
	Draft      bool   `json:"draft"`
	Archived   bool   `json:"archived"`
}

type DocumentMoveRequest struct {
	SourcePath      string `json:"source_path"`
	DestinationPath string `json:"destination_path"`
}

type DocumentMoveResponse struct {
	SourcePath      string `json:"source_path"`
	DestinationPath string `json:"destination_path"`
	Operation       string `json:"operation"`
}

type DocumentDeleteRequest struct {
	SourcePath string `json:"source_path"`
}

type DocumentDeleteResponse struct {
	SourcePath string `json:"source_path"`
	TrashPath  string `json:"trash_path"`
	Operation  string `json:"operation"`
}

type MediaItem struct {
	Collection string        `json:"collection"`
	Path       string        `json:"path"`
	Name       string        `json:"name"`
	Reference  string        `json:"reference"`
	PublicURL  string        `json:"public_url"`
	Kind       string        `json:"kind"`
	Size       int64         `json:"size"`
	Metadata   MediaMetadata `json:"metadata,omitempty"`
}

type MediaUploadResponse struct {
	MediaItem
	Created bool `json:"created"`
}

type MediaDeleteRequest struct {
	Reference string `json:"reference"`
}

type MediaMetadata struct {
	Title       string   `json:"title,omitempty" yaml:"title,omitempty"`
	Alt         string   `json:"alt,omitempty" yaml:"alt,omitempty"`
	Caption     string   `json:"caption,omitempty" yaml:"caption,omitempty"`
	Description string   `json:"description,omitempty" yaml:"description,omitempty"`
	Credit      string   `json:"credit,omitempty" yaml:"credit,omitempty"`
	Tags        []string `json:"tags,omitempty" yaml:"tags,omitempty"`
}

type MediaDetailResponse struct {
	MediaItem
}

type MediaMetadataSaveRequest struct {
	Reference string        `json:"reference"`
	Metadata  MediaMetadata `json:"metadata"`
}
