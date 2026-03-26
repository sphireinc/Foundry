package types

import "time"

type DocumentSummary struct {
	ID         string              `json:"id"`
	Type       string              `json:"type"`
	Lang       string              `json:"lang"`
	Status     string              `json:"status"`
	Title      string              `json:"title"`
	Slug       string              `json:"slug"`
	URL        string              `json:"url"`
	Layout     string              `json:"layout"`
	SourcePath string              `json:"source_path"`
	Summary    string              `json:"summary"`
	Draft      bool                `json:"draft"`
	Archived   bool                `json:"archived,omitempty"`
	Date       *time.Time          `json:"date,omitempty"`
	CreatedAt  *time.Time          `json:"created_at,omitempty"`
	UpdatedAt  *time.Time          `json:"updated_at,omitempty"`
	Author     string              `json:"author,omitempty"`
	LastEditor string              `json:"last_editor,omitempty"`
	Taxonomies map[string][]string `json:"taxonomies,omitempty"`
}

type DocumentDetail struct {
	DocumentSummary
	RawBody     string         `json:"raw_body"`
	HTMLBody    string         `json:"html_body"`
	Params      map[string]any `json:"params,omitempty"`
	Fields      map[string]any `json:"fields,omitempty"`
	FieldSchema []FieldSchema  `json:"field_schema,omitempty"`
	Lock        *DocumentLock  `json:"lock,omitempty"`
}

type DocumentListOptions struct {
	IncludeDrafts bool
	Type          string
	Lang          string
	Query         string
}

type DocumentSaveRequest struct {
	SourcePath     string         `json:"source_path"`
	Raw            string         `json:"raw"`
	Fields         map[string]any `json:"fields,omitempty"`
	VersionComment string         `json:"version_comment,omitempty"`
	Actor          string         `json:"-"`
	Username       string         `json:"-"`
	LockToken      string         `json:"lock_token,omitempty"`
}

type DocumentSaveResponse struct {
	SourcePath string `json:"source_path"`
	Size       int64  `json:"size"`
	Created    bool   `json:"created"`
	Raw        string `json:"raw,omitempty"`
}

type DocumentPreviewRequest struct {
	SourcePath string         `json:"source_path"`
	Raw        string         `json:"raw"`
	Fields     map[string]any `json:"fields,omitempty"`
}

type DocumentPreviewResponse struct {
	Title       string     `json:"title"`
	Slug        string     `json:"slug"`
	Layout      string     `json:"layout"`
	Summary     string     `json:"summary"`
	Status      string     `json:"status"`
	Draft       bool       `json:"draft"`
	Archived    bool       `json:"archived"`
	Date        *time.Time `json:"date,omitempty"`
	CreatedAt   *time.Time `json:"created_at,omitempty"`
	UpdatedAt   *time.Time `json:"updated_at,omitempty"`
	Author      string     `json:"author,omitempty"`
	LastEditor  string     `json:"last_editor,omitempty"`
	HTML        string     `json:"html"`
	WordCount   int        `json:"word_count"`
	FieldErrors []string   `json:"field_errors,omitempty"`
}

type DocumentCreateRequest struct {
	Kind      string `json:"kind"`
	Slug      string `json:"slug"`
	Lang      string `json:"lang,omitempty"`
	Archetype string `json:"archetype,omitempty"`
	LockToken string `json:"lock_token,omitempty"`
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
	SourcePath           string `json:"source_path"`
	Status               string `json:"status"`
	ScheduledPublishAt   string `json:"scheduled_publish_at,omitempty"`
	ScheduledUnpublishAt string `json:"scheduled_unpublish_at,omitempty"`
	EditorialNote        string `json:"editorial_note,omitempty"`
	LockToken            string `json:"lock_token,omitempty"`
}

type DocumentStatusResponse struct {
	SourcePath           string     `json:"source_path"`
	Status               string     `json:"status"`
	Draft                bool       `json:"draft"`
	Archived             bool       `json:"archived"`
	ScheduledPublishAt   *time.Time `json:"scheduled_publish_at,omitempty"`
	ScheduledUnpublishAt *time.Time `json:"scheduled_unpublish_at,omitempty"`
	EditorialNote        string     `json:"editorial_note,omitempty"`
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
	LockToken  string `json:"lock_token,omitempty"`
}

type DocumentDeleteResponse struct {
	SourcePath string `json:"source_path"`
	TrashPath  string `json:"trash_path"`
	Operation  string `json:"operation"`
}

type LifecycleState string

const (
	LifecycleStateCurrent LifecycleState = "current"
	LifecycleStateVersion LifecycleState = "version"
	LifecycleStateTrash   LifecycleState = "trash"
)

type DocumentHistoryEntry struct {
	Path           string         `json:"path"`
	OriginalPath   string         `json:"original_path"`
	State          LifecycleState `json:"state"`
	Timestamp      *time.Time     `json:"timestamp,omitempty"`
	VersionComment string         `json:"version_comment,omitempty"`
	Actor          string         `json:"actor,omitempty"`
	Status         string         `json:"status,omitempty"`
	Title          string         `json:"title"`
	Slug           string         `json:"slug"`
	Layout         string         `json:"layout"`
	Summary        string         `json:"summary"`
	Draft          bool           `json:"draft"`
	Archived       bool           `json:"archived"`
	Lang           string         `json:"lang"`
	Author         string         `json:"author,omitempty"`
	LastEditor     string         `json:"last_editor,omitempty"`
	CreatedAt      *time.Time     `json:"created_at,omitempty"`
	UpdatedAt      *time.Time     `json:"updated_at,omitempty"`
	Size           int64          `json:"size"`
}

type FieldSchema struct {
	Name        string        `json:"name"`
	Label       string        `json:"label,omitempty"`
	Type        string        `json:"type"`
	Required    bool          `json:"required,omitempty"`
	Default     any           `json:"default,omitempty"`
	Enum        []string      `json:"enum,omitempty"`
	Fields      []FieldSchema `json:"fields,omitempty"`
	Item        *FieldSchema  `json:"item,omitempty"`
	Help        string        `json:"help,omitempty"`
	Placeholder string        `json:"placeholder,omitempty"`
}

type DocumentHistoryResponse struct {
	SourcePath string                 `json:"source_path"`
	Entries    []DocumentHistoryEntry `json:"entries"`
}

type DocumentDiffRequest struct {
	LeftPath  string `json:"left_path"`
	RightPath string `json:"right_path"`
}

type DocumentDiffResponse struct {
	LeftPath  string `json:"left_path"`
	RightPath string `json:"right_path"`
	LeftRaw   string `json:"left_raw"`
	RightRaw  string `json:"right_raw"`
	Diff      string `json:"diff"`
}

type DocumentLifecycleRequest struct {
	Path string `json:"path"`
}

type DocumentLock struct {
	SourcePath string     `json:"source_path"`
	Username   string     `json:"username"`
	Name       string     `json:"name,omitempty"`
	Role       string     `json:"role,omitempty"`
	OwnedByMe  bool       `json:"owned_by_me,omitempty"`
	Token      string     `json:"token,omitempty"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	LastBeatAt *time.Time `json:"last_beat_at,omitempty"`
}

type DocumentLockRequest struct {
	SourcePath string `json:"source_path"`
	LockToken  string `json:"lock_token,omitempty"`
}

type DocumentLockResponse struct {
	Lock *DocumentLock `json:"lock,omitempty"`
}

type DocumentLifecycleResponse struct {
	Path         string `json:"path"`
	RestoredPath string `json:"restored_path,omitempty"`
	Operation    string `json:"operation"`
}

type MediaItem struct {
	Collection string        `json:"collection"`
	Path       string        `json:"path"`
	Name       string        `json:"name"`
	Reference  string        `json:"reference"`
	PublicURL  string        `json:"public_url"`
	Kind       string        `json:"kind"`
	Size       int64         `json:"size"`
	UsedByCount int          `json:"used_by_count,omitempty"`
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
	Title            string     `json:"title,omitempty" yaml:"title,omitempty"`
	Alt              string     `json:"alt,omitempty" yaml:"alt,omitempty"`
	Caption          string     `json:"caption,omitempty" yaml:"caption,omitempty"`
	Description      string     `json:"description,omitempty" yaml:"description,omitempty"`
	Credit           string     `json:"credit,omitempty" yaml:"credit,omitempty"`
	Tags             []string   `json:"tags,omitempty" yaml:"tags,omitempty"`
	OriginalFilename string     `json:"original_filename,omitempty" yaml:"original_filename,omitempty"`
	StoredFilename   string     `json:"stored_filename,omitempty" yaml:"stored_filename,omitempty"`
	Extension        string     `json:"extension,omitempty" yaml:"extension,omitempty"`
	MIMEType         string     `json:"mime_type,omitempty" yaml:"mime_type,omitempty"`
	Kind             string     `json:"kind,omitempty" yaml:"kind,omitempty"`
	ContentHash      string     `json:"content_hash,omitempty" yaml:"content_hash,omitempty"`
	FileSize         int64      `json:"file_size,omitempty" yaml:"file_size,omitempty"`
	Width            int        `json:"width,omitempty" yaml:"width,omitempty"`
	Height           int        `json:"height,omitempty" yaml:"height,omitempty"`
	FocalX           string     `json:"focal_x,omitempty" yaml:"focal_x,omitempty"`
	FocalY           string     `json:"focal_y,omitempty" yaml:"focal_y,omitempty"`
	UploadedAt       *time.Time `json:"uploaded_at,omitempty" yaml:"uploaded_at,omitempty"`
	UploadedBy       string     `json:"uploaded_by,omitempty" yaml:"uploaded_by,omitempty"`
}

type MediaDetailResponse struct {
	MediaItem
	UsedBy []DocumentSummary `json:"used_by,omitempty"`
}

type MediaReplaceResponse struct {
	MediaItem
	Replaced bool `json:"replaced"`
}

type MediaMetadataSaveRequest struct {
	Reference      string        `json:"reference"`
	Metadata       MediaMetadata `json:"metadata"`
	VersionComment string        `json:"version_comment,omitempty"`
	Actor          string        `json:"-"`
}

type MediaHistoryEntry struct {
	Collection       string         `json:"collection"`
	Path             string         `json:"path"`
	OriginalPath     string         `json:"original_path"`
	Reference        string         `json:"reference,omitempty"`
	CurrentReference string         `json:"current_reference,omitempty"`
	Name             string         `json:"name"`
	PublicURL        string         `json:"public_url"`
	Kind             string         `json:"kind"`
	Size             int64          `json:"size"`
	State            LifecycleState `json:"state"`
	Timestamp        *time.Time     `json:"timestamp,omitempty"`
	VersionComment   string         `json:"version_comment,omitempty"`
	Actor            string         `json:"actor,omitempty"`
	MetadataOnly     bool           `json:"metadata_only,omitempty"`
	Metadata         MediaMetadata  `json:"metadata,omitempty"`
}

type MediaHistoryResponse struct {
	Reference string              `json:"reference,omitempty"`
	Path      string              `json:"path,omitempty"`
	Entries   []MediaHistoryEntry `json:"entries"`
}

type MediaLifecycleRequest struct {
	Path string `json:"path"`
}

type MediaLifecycleResponse struct {
	Path         string `json:"path"`
	RestoredPath string `json:"restored_path,omitempty"`
	Operation    string `json:"operation"`
}
