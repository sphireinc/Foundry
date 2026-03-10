package types

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
