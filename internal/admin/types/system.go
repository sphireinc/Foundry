package types

import "time"

type SystemStatus struct {
	Name           string           `json:"name"`
	Title          string           `json:"title"`
	BaseURL        string           `json:"base_url"`
	DefaultLang    string           `json:"default_lang"`
	PublicDir      string           `json:"public_dir"`
	ContentDir     string           `json:"content_dir"`
	DataDir        string           `json:"data_dir"`
	ThemesDir      string           `json:"themes_dir"`
	PluginsDir     string           `json:"plugins_dir"`
	AdminEnabled   bool             `json:"admin_enabled"`
	AdminLocalOnly bool             `json:"admin_local_only"`
	Content        ContentStatus    `json:"content"`
	Theme          ThemeStatus      `json:"theme"`
	Plugins        []PluginStatus   `json:"plugins"`
	Taxonomies     []TaxonomyStatus `json:"taxonomies"`
	Checks         []HealthCheck    `json:"checks"`
}

type RuntimeStatus struct {
	CapturedAt         time.Time              `json:"captured_at"`
	UptimeSeconds      int64                  `json:"uptime_seconds"`
	GoVersion          string                 `json:"go_version"`
	NumCPU             int                    `json:"num_cpu"`
	LiveReloadMode     string                 `json:"live_reload_mode"`
	HeapAllocBytes     uint64                 `json:"heap_alloc_bytes"`
	HeapInuseBytes     uint64                 `json:"heap_inuse_bytes"`
	HeapObjects        uint64                 `json:"heap_objects"`
	StackInuseBytes    uint64                 `json:"stack_inuse_bytes"`
	SysBytes           uint64                 `json:"sys_bytes"`
	NumGC              uint32                 `json:"num_gc"`
	NextGCBytes        uint64                 `json:"next_gc_bytes"`
	LastGCAt           *time.Time             `json:"last_gc_at,omitempty"`
	Goroutines         int                    `json:"goroutines"`
	ProcessUserCPUMS   int64                  `json:"process_user_cpu_ms"`
	ProcessSystemCPUMS int64                  `json:"process_system_cpu_ms"`
	Content            RuntimeContentStatus   `json:"content"`
	Storage            RuntimeStorageStatus   `json:"storage"`
	Integrity          RuntimeIntegrityStatus `json:"integrity"`
	Activity           RuntimeActivityStatus  `json:"activity"`
	LastBuild          *RuntimeBuildStatus    `json:"last_build,omitempty"`
}

type RuntimeContentStatus struct {
	DocumentCount     int            `json:"document_count"`
	RouteCount        int            `json:"route_count"`
	TaxonomyCount     int            `json:"taxonomy_count"`
	TaxonomyTermCount int            `json:"taxonomy_term_count"`
	ByType            map[string]int `json:"by_type"`
	ByLang            map[string]int `json:"by_lang"`
	ByStatus          map[string]int `json:"by_status"`
	MediaCounts       map[string]int `json:"media_counts"`
}

type RuntimeStorageStatus struct {
	ContentBytes        int64             `json:"content_bytes"`
	PublicBytes         int64             `json:"public_bytes"`
	MediaBytes          map[string]int64  `json:"media_bytes"`
	MediaCounts         map[string]int    `json:"media_counts"`
	DerivedVersionCount int               `json:"derived_version_count"`
	DerivedTrashCount   int               `json:"derived_trash_count"`
	DerivedBytes        int64             `json:"derived_bytes"`
	LargestFiles        []RuntimeFileStat `json:"largest_files"`
}

type RuntimeFileStat struct {
	Path      string `json:"path"`
	SizeBytes int64  `json:"size_bytes"`
}

type RuntimeIntegrityStatus struct {
	BrokenMediaRefs       int `json:"broken_media_refs"`
	BrokenInternalLinks   int `json:"broken_internal_links"`
	MissingTemplates      int `json:"missing_templates"`
	OrphanedMedia         int `json:"orphaned_media"`
	DuplicateURLs         int `json:"duplicate_urls"`
	DuplicateSlugs        int `json:"duplicate_slugs"`
	TaxonomyInconsistency int `json:"taxonomy_inconsistency"`
}

type RuntimeActivityStatus struct {
	ActiveSessions      int            `json:"active_sessions"`
	ConcurrentUsers     int            `json:"concurrent_users"`
	AddressSpreadUsers  int            `json:"address_spread_users"`
	LongLivedSessions   int            `json:"long_lived_sessions"`
	IdleSessions        int            `json:"idle_sessions"`
	ActiveDocumentLocks int            `json:"active_document_locks"`
	RecentAuditEvents   int            `json:"recent_audit_events"`
	RecentFailedLogins  int            `json:"recent_failed_logins"`
	RecentAuditByAction map[string]int `json:"recent_audit_by_action"`
	AuditWindowHours    int            `json:"audit_window_hours"`
}

type RuntimeBuildStatus struct {
	GeneratedAt   time.Time `json:"generated_at"`
	Environment   string    `json:"environment"`
	Target        string    `json:"target,omitempty"`
	Preview       bool      `json:"preview"`
	DocumentCount int       `json:"document_count"`
	RouteCount    int       `json:"route_count"`
	PrepareMS     int64     `json:"prepare_ms"`
	AssetsMS      int64     `json:"assets_ms"`
	DocumentsMS   int64     `json:"documents_ms"`
	TaxonomiesMS  int64     `json:"taxonomies_ms"`
	SearchMS      int64     `json:"search_ms"`
}

type SiteValidationResponse struct {
	BrokenMediaRefs       []string `json:"broken_media_refs,omitempty"`
	BrokenInternalLinks   []string `json:"broken_internal_links,omitempty"`
	MissingTemplates      []string `json:"missing_templates,omitempty"`
	OrphanedMedia         []string `json:"orphaned_media,omitempty"`
	DuplicateURLs         []string `json:"duplicate_urls,omitempty"`
	DuplicateSlugs        []string `json:"duplicate_slugs,omitempty"`
	TaxonomyInconsistency []string `json:"taxonomy_inconsistency,omitempty"`
	MessageCount          int      `json:"message_count"`
}

type ContentStatus struct {
	DocumentCount int            `json:"document_count"`
	RouteCount    int            `json:"route_count"`
	DraftCount    int            `json:"draft_count"`
	ByType        map[string]int `json:"by_type"`
	ByLang        map[string]int `json:"by_lang"`
}

type ThemeStatus struct {
	Current     string `json:"current"`
	Title       string `json:"title"`
	Version     string `json:"version"`
	Valid       bool   `json:"valid"`
	Description string `json:"description,omitempty"`
}

type PluginStatus struct {
	Name    string `json:"name"`
	Title   string `json:"title"`
	Version string `json:"version"`
	Enabled bool   `json:"enabled"`
	Status  string `json:"status"`
}

type TaxonomyStatus struct {
	Name      string `json:"name"`
	TermCount int    `json:"term_count"`
}

type HealthCheck struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}
