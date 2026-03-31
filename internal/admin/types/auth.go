package types

import "github.com/sphireinc/foundry/internal/config"

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	TOTPCode string `json:"totp_code,omitempty"`
}

type SessionResponse struct {
	Authenticated bool     `json:"authenticated"`
	Username      string   `json:"username,omitempty"`
	Name          string   `json:"name,omitempty"`
	Email         string   `json:"email,omitempty"`
	Role          string   `json:"role,omitempty"`
	Capabilities  []string `json:"capabilities,omitempty"`
	MFAComplete   bool     `json:"mfa_complete,omitempty"`
	CSRFToken     string   `json:"csrf_token,omitempty"`
	TTLSeconds    int      `json:"ttl_seconds,omitempty"`
}

type CapabilityResponse struct {
	SDKVersion   string           `json:"sdk_version"`
	Capabilities []string         `json:"capabilities,omitempty"`
	Modules      map[string]bool  `json:"modules"`
	Features     map[string]bool  `json:"features"`
	Identity     *SessionResponse `json:"identity,omitempty"`
}

type SettingsSection struct {
	Key         string        `json:"key"`
	Title       string        `json:"title"`
	Capability  string        `json:"capability,omitempty"`
	Description string        `json:"description,omitempty"`
	Writable    bool          `json:"writable"`
	Schema      []FieldSchema `json:"schema,omitempty"`
	Source      string        `json:"source,omitempty"`
}

type AdminExtensionRegistry struct {
	Pages    []AdminExtensionPage    `json:"pages,omitempty"`
	Widgets  []AdminExtensionWidget  `json:"widgets,omitempty"`
	Slots    []AdminExtensionSlot    `json:"slots,omitempty"`
	Settings []AdminExtensionSetting `json:"settings,omitempty"`
}

type AdminExtensionPage struct {
	Plugin      string   `json:"plugin"`
	Key         string   `json:"key"`
	Title       string   `json:"title"`
	Route       string   `json:"route"`
	Capability  string   `json:"capability,omitempty"`
	Description string   `json:"description,omitempty"`
	ModuleURL   string   `json:"module_url,omitempty"`
	StyleURLs   []string `json:"style_urls,omitempty"`
}

type AdminExtensionWidget struct {
	Plugin      string   `json:"plugin"`
	Key         string   `json:"key"`
	Title       string   `json:"title"`
	Slot        string   `json:"slot"`
	Capability  string   `json:"capability,omitempty"`
	Description string   `json:"description,omitempty"`
	ModuleURL   string   `json:"module_url,omitempty"`
	StyleURLs   []string `json:"style_urls,omitempty"`
}

type AdminExtensionSlot struct {
	Plugin      string `json:"plugin"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type AdminExtensionSetting struct {
	Plugin      string        `json:"plugin"`
	Key         string        `json:"key"`
	Title       string        `json:"title"`
	Capability  string        `json:"capability,omitempty"`
	Description string        `json:"description,omitempty"`
	Schema      []FieldSchema `json:"schema,omitempty"`
}

type UserSummary struct {
	Username     string   `json:"username"`
	Name         string   `json:"name"`
	Email        string   `json:"email"`
	Role         string   `json:"role,omitempty"`
	Capabilities []string `json:"capabilities,omitempty"`
	Disabled     bool     `json:"disabled,omitempty"`
	TOTPEnabled  bool     `json:"totp_enabled,omitempty"`
}

type UserSaveRequest struct {
	Username     string   `json:"username"`
	Name         string   `json:"name"`
	Email        string   `json:"email"`
	Role         string   `json:"role,omitempty"`
	Capabilities []string `json:"capabilities,omitempty"`
	Password     string   `json:"password,omitempty"`
	Disabled     bool     `json:"disabled,omitempty"`
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

type CustomCSSDocumentResponse struct {
	Path string `json:"path"`
	Raw  string `json:"raw"`
}

type CustomCSSSaveRequest struct {
	Raw string `json:"raw"`
}

type SettingsFormResponse struct {
	Path  string        `json:"path"`
	Value config.Config `json:"value"`
}

type SettingsFormSaveRequest struct {
	Value config.Config `json:"value"`
}

type ThemeRecord struct {
	Name                 string                 `json:"name"`
	Kind                 string                 `json:"kind,omitempty"`
	Title                string                 `json:"title"`
	Version              string                 `json:"version"`
	Description          string                 `json:"description"`
	Repo                 string                 `json:"repo,omitempty"`
	Current              bool                   `json:"current"`
	Valid                bool                   `json:"valid"`
	AdminAPI             string                 `json:"admin_api,omitempty"`
	SDKVersion           string                 `json:"sdk_version,omitempty"`
	CompatibilityVersion string                 `json:"compatibility_version,omitempty"`
	MinFoundryVersion    string                 `json:"min_foundry_version,omitempty"`
	SupportedLayouts     []string               `json:"supported_layouts,omitempty"`
	Components           []string               `json:"components,omitempty"`
	WidgetSlots          []string               `json:"widget_slots,omitempty"`
	Screenshots          []string               `json:"screenshots,omitempty"`
	ConfigSchema         []FieldSchema          `json:"config_schema,omitempty"`
	Diagnostics          []ValidationDiagnostic `json:"diagnostics,omitempty"`
}

type ThemeSwitchRequest struct {
	Name string `json:"name"`
	Kind string `json:"kind,omitempty"`
}

type ThemeInstallRequest struct {
	URL  string `json:"url"`
	Name string `json:"name,omitempty"`
	Kind string `json:"kind,omitempty"`
}

type BackupRecord struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	SizeBytes int64  `json:"size_bytes"`
	CreatedAt string `json:"created_at"`
}

type BackupCreateRequest struct {
	Name string `json:"name,omitempty"`
}

type BackupRestoreRequest struct {
	Name string `json:"name"`
}

type UpdateStatusResponse struct {
	Repo           string `json:"repo"`
	CurrentVersion string `json:"current_version"`
	LatestVersion  string `json:"latest_version"`
	HasUpdate      bool   `json:"has_update"`
	InstallMode    string `json:"install_mode"`
	ApplySupported bool   `json:"apply_supported"`
	ReleaseURL     string `json:"release_url,omitempty"`
	PublishedAt    string `json:"published_at,omitempty"`
	Body           string `json:"body,omitempty"`
	AssetName      string `json:"asset_name,omitempty"`
	Instructions   string `json:"instructions,omitempty"`
}

type PluginRecord struct {
	Name                 string                 `json:"name"`
	Title                string                 `json:"title"`
	Version              string                 `json:"version"`
	Description          string                 `json:"description,omitempty"`
	Author               string                 `json:"author,omitempty"`
	Repo                 string                 `json:"repo,omitempty"`
	Enabled              bool                   `json:"enabled"`
	Status               string                 `json:"status"`
	Health               string                 `json:"health,omitempty"`
	CanRollback          bool                   `json:"can_rollback,omitempty"`
	CompatibilityVersion string                 `json:"compatibility_version,omitempty"`
	MinFoundryVersion    string                 `json:"min_foundry_version,omitempty"`
	FoundryAPI           string                 `json:"foundry_api,omitempty"`
	Requires             []string               `json:"requires,omitempty"`
	Dependencies         []PluginDependency     `json:"dependencies,omitempty"`
	ConfigSchema         []FieldSchema          `json:"config_schema,omitempty"`
	Diagnostics          []ValidationDiagnostic `json:"diagnostics,omitempty"`
}

type ValidationDiagnostic struct {
	Severity string `json:"severity"`
	Path     string `json:"path,omitempty"`
	Message  string `json:"message"`
}

type PluginDependency struct {
	Name     string `json:"name"`
	Version  string `json:"version,omitempty"`
	Optional bool   `json:"optional,omitempty"`
}

type PluginToggleRequest struct {
	Name string `json:"name"`
}

type PluginInstallRequest struct {
	URL  string `json:"url"`
	Name string `json:"name,omitempty"`
}

type SessionRevokeRequest struct {
	Username string `json:"username,omitempty"`
	All      bool   `json:"all,omitempty"`
}

type SessionRevokeResponse struct {
	Revoked int `json:"revoked"`
}

type PasswordResetStartRequest struct {
	Username string `json:"username"`
}

type PasswordResetStartResponse struct {
	Username   string `json:"username"`
	ResetToken string `json:"reset_token"`
	ExpiresIn  int    `json:"expires_in_seconds"`
}

type PasswordResetCompleteRequest struct {
	Username    string `json:"username"`
	ResetToken  string `json:"reset_token"`
	NewPassword string `json:"new_password"`
	TOTPCode    string `json:"totp_code,omitempty"`
}

type TOTPSetupRequest struct {
	Username string `json:"username,omitempty"`
}

type TOTPSetupResponse struct {
	Username        string `json:"username"`
	Secret          string `json:"secret"`
	ProvisioningURI string `json:"provisioning_uri"`
}

type TOTPEnableRequest struct {
	Username string `json:"username,omitempty"`
	Code     string `json:"code"`
}

type TOTPDisableRequest struct {
	Username string `json:"username,omitempty"`
}
