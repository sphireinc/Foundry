package types

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type SessionResponse struct {
	Authenticated bool   `json:"authenticated"`
	Username      string `json:"username,omitempty"`
	Name          string `json:"name,omitempty"`
	Email         string `json:"email,omitempty"`
	Role          string `json:"role,omitempty"`
	TTLSeconds    int    `json:"ttl_seconds,omitempty"`
}

type UserSummary struct {
	Username string `json:"username"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	Role     string `json:"role,omitempty"`
	Disabled bool   `json:"disabled,omitempty"`
}

type UserSaveRequest struct {
	Username string `json:"username"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	Role     string `json:"role,omitempty"`
	Password string `json:"password,omitempty"`
	Disabled bool   `json:"disabled,omitempty"`
}

type UserDeleteRequest struct {
	Username string `json:"username"`
}

type ConfigDocumentResponse struct {
	Path string `json:"path"`
	Raw  string `json:"raw"`
}

type ConfigSaveRequest struct {
	Raw string `json:"raw"`
}

type ThemeRecord struct {
	Name        string `json:"name"`
	Title       string `json:"title"`
	Version     string `json:"version"`
	Description string `json:"description"`
	Current     bool   `json:"current"`
	Valid       bool   `json:"valid"`
}

type ThemeSwitchRequest struct {
	Name string `json:"name"`
}

type PluginRecord struct {
	Name    string `json:"name"`
	Title   string `json:"title"`
	Version string `json:"version"`
	Enabled bool   `json:"enabled"`
	Status  string `json:"status"`
}

type PluginToggleRequest struct {
	Name string `json:"name"`
}
