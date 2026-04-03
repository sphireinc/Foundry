package plugins

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sphireinc/foundry/internal/config"
	"gopkg.in/yaml.v3"
)

// Metadata describes a plugin's declarative contract from plugin.yaml.
//
// Foundry uses this metadata for validation, dependency checks, admin UI
// display, and extension discovery. Runtime hooks still come from the plugin's
// implementation.
type Metadata struct {
	Name                 string                   `yaml:"name"`
	Title                string                   `yaml:"title"`
	Version              string                   `yaml:"version"`
	Description          string                   `yaml:"description"`
	Author               string                   `yaml:"author"`
	Homepage             string                   `yaml:"homepage"`
	License              string                   `yaml:"license"`
	Repo                 string                   `yaml:"repo"`
	Requires             []string                 `yaml:"requires"`
	Dependencies         []Dependency             `yaml:"dependencies,omitempty"`
	FoundryAPI           string                   `yaml:"foundry_api"`
	MinFoundryVersion    string                   `yaml:"min_foundry_version"`
	CompatibilityVersion string                   `yaml:"compatibility_version,omitempty"`
	ConfigSchema         []config.FieldDefinition `yaml:"config_schema,omitempty"`
	Permissions          PermissionSet            `yaml:"permissions,omitempty"`
	Runtime              RuntimeConfig            `yaml:"runtime,omitempty"`
	AdminExtensions      AdminExtensions          `yaml:"admin,omitempty"`
	Screenshots          []string                 `yaml:"screenshots,omitempty"`
	Directory            string                   `yaml:"-"`
}

type PermissionSet struct {
	Filesystem   FilesystemPermissions  `yaml:"filesystem,omitempty" json:"filesystem,omitempty"`
	Network      NetworkPermissions     `yaml:"network,omitempty" json:"network,omitempty"`
	Process      ProcessPermissions     `yaml:"process,omitempty" json:"process,omitempty"`
	Environment  EnvironmentPermissions `yaml:"environment,omitempty" json:"environment,omitempty"`
	Config       ConfigPermissions      `yaml:"config,omitempty" json:"config,omitempty"`
	Content      ContentPermissions     `yaml:"content,omitempty" json:"content,omitempty"`
	Render       RenderPermissions      `yaml:"render,omitempty" json:"render,omitempty"`
	Graph        GraphPermissions       `yaml:"graph,omitempty" json:"graph,omitempty"`
	Admin        AdminPermissions       `yaml:"admin,omitempty" json:"admin,omitempty"`
	Runtime      RuntimePermissions     `yaml:"runtime,omitempty" json:"runtime,omitempty"`
	Secrets      SecretPermissions      `yaml:"secrets,omitempty" json:"secrets,omitempty"`
	Capabilities CapabilityPermissions  `yaml:"capabilities,omitempty" json:"capabilities,omitempty"`
}

type FilesystemPermissions struct {
	Read   FilesystemReadPermissions   `yaml:"read,omitempty" json:"read,omitempty"`
	Write  FilesystemWritePermissions  `yaml:"write,omitempty" json:"write,omitempty"`
	Delete FilesystemDeletePermissions `yaml:"delete,omitempty" json:"delete,omitempty"`
}

type FilesystemReadPermissions struct {
	Content bool     `yaml:"content,omitempty" json:"content,omitempty"`
	Data    bool     `yaml:"data,omitempty" json:"data,omitempty"`
	Public  bool     `yaml:"public,omitempty" json:"public,omitempty"`
	Themes  bool     `yaml:"themes,omitempty" json:"themes,omitempty"`
	Plugins bool     `yaml:"plugins,omitempty" json:"plugins,omitempty"`
	Config  bool     `yaml:"config,omitempty" json:"config,omitempty"`
	Custom  []string `yaml:"custom,omitempty" json:"custom,omitempty"`
}

type FilesystemWritePermissions struct {
	Content bool     `yaml:"content,omitempty" json:"content,omitempty"`
	Data    bool     `yaml:"data,omitempty" json:"data,omitempty"`
	Public  bool     `yaml:"public,omitempty" json:"public,omitempty"`
	Cache   bool     `yaml:"cache,omitempty" json:"cache,omitempty"`
	Backups bool     `yaml:"backups,omitempty" json:"backups,omitempty"`
	Custom  []string `yaml:"custom,omitempty" json:"custom,omitempty"`
}

type FilesystemDeletePermissions struct {
	Content bool     `yaml:"content,omitempty" json:"content,omitempty"`
	Data    bool     `yaml:"data,omitempty" json:"data,omitempty"`
	Public  bool     `yaml:"public,omitempty" json:"public,omitempty"`
	Cache   bool     `yaml:"cache,omitempty" json:"cache,omitempty"`
	Backups bool     `yaml:"backups,omitempty" json:"backups,omitempty"`
	Custom  []string `yaml:"custom,omitempty" json:"custom,omitempty"`
}

type NetworkPermissions struct {
	Outbound NetworkOutboundPermissions `yaml:"outbound,omitempty" json:"outbound,omitempty"`
	Inbound  NetworkInboundPermissions  `yaml:"inbound,omitempty" json:"inbound,omitempty"`
}

type NetworkOutboundPermissions struct {
	HTTP          bool     `yaml:"http,omitempty" json:"http,omitempty"`
	HTTPS         bool     `yaml:"https,omitempty" json:"https,omitempty"`
	WebSocket     bool     `yaml:"websocket,omitempty" json:"websocket,omitempty"`
	GRPC          bool     `yaml:"grpc,omitempty" json:"grpc,omitempty"`
	CustomSchemes []string `yaml:"custom_schemes,omitempty" json:"custom_schemes,omitempty"`
	Domains       []string `yaml:"domains,omitempty" json:"domains,omitempty"`
	Methods       []string `yaml:"methods,omitempty" json:"methods,omitempty"`
}

type NetworkInboundPermissions struct {
	RegisterRoutes       bool `yaml:"register_routes,omitempty" json:"register_routes,omitempty"`
	AdminRoutes          bool `yaml:"admin_routes,omitempty" json:"admin_routes,omitempty"`
	PublicRoutes         bool `yaml:"public_routes,omitempty" json:"public_routes,omitempty"`
	BindExternalServices bool `yaml:"bind_external_services,omitempty" json:"bind_external_services,omitempty"`
}

type ProcessPermissions struct {
	Exec            ProcessExecPermissions `yaml:"exec,omitempty" json:"exec,omitempty"`
	Shell           AllowedPermission      `yaml:"shell,omitempty" json:"shell,omitempty"`
	SpawnBackground AllowedPermission      `yaml:"spawn_background,omitempty" json:"spawn_background,omitempty"`
}

type ProcessExecPermissions struct {
	Allowed  bool     `yaml:"allowed,omitempty" json:"allowed,omitempty"`
	Commands []string `yaml:"commands,omitempty" json:"commands,omitempty"`
}

type AllowedPermission struct {
	Allowed bool `yaml:"allowed,omitempty" json:"allowed,omitempty"`
}

type EnvironmentPermissions struct {
	Read EnvironmentReadPermissions `yaml:"read,omitempty" json:"read,omitempty"`
}

type EnvironmentReadPermissions struct {
	Allowed   bool     `yaml:"allowed,omitempty" json:"allowed,omitempty"`
	Variables []string `yaml:"variables,omitempty" json:"variables,omitempty"`
}

type ConfigPermissions struct {
	Read  ConfigReadPermissions  `yaml:"read,omitempty" json:"read,omitempty"`
	Write ConfigWritePermissions `yaml:"write,omitempty" json:"write,omitempty"`
}

type ConfigReadPermissions struct {
	Site          bool `yaml:"site,omitempty" json:"site,omitempty"`
	PluginConfig  bool `yaml:"plugin_config,omitempty" json:"plugin_config,omitempty"`
	ThemeManifest bool `yaml:"theme_manifest,omitempty" json:"theme_manifest,omitempty"`
	RawFiles      bool `yaml:"raw_files,omitempty" json:"raw_files,omitempty"`
}

type ConfigWritePermissions struct {
	Site          bool `yaml:"site,omitempty" json:"site,omitempty"`
	PluginConfig  bool `yaml:"plugin_config,omitempty" json:"plugin_config,omitempty"`
	ThemeManifest bool `yaml:"theme_manifest,omitempty" json:"theme_manifest,omitempty"`
}

type ContentPermissions struct {
	Documents    DocumentPermissions    `yaml:"documents,omitempty" json:"documents,omitempty"`
	Media        MediaPermissions       `yaml:"media,omitempty" json:"media,omitempty"`
	Taxonomies   TaxonomyPermissions    `yaml:"taxonomies,omitempty" json:"taxonomies,omitempty"`
	SharedFields SharedFieldPermissions `yaml:"shared_fields,omitempty" json:"shared_fields,omitempty"`
}

type DocumentPermissions struct {
	Read     bool `yaml:"read,omitempty" json:"read,omitempty"`
	Write    bool `yaml:"write,omitempty" json:"write,omitempty"`
	Delete   bool `yaml:"delete,omitempty" json:"delete,omitempty"`
	Workflow bool `yaml:"workflow,omitempty" json:"workflow,omitempty"`
	Versions bool `yaml:"versions,omitempty" json:"versions,omitempty"`
}

type MediaPermissions struct {
	Read     bool `yaml:"read,omitempty" json:"read,omitempty"`
	Write    bool `yaml:"write,omitempty" json:"write,omitempty"`
	Delete   bool `yaml:"delete,omitempty" json:"delete,omitempty"`
	Metadata bool `yaml:"metadata,omitempty" json:"metadata,omitempty"`
	Versions bool `yaml:"versions,omitempty" json:"versions,omitempty"`
}

type TaxonomyPermissions struct {
	Read  bool `yaml:"read,omitempty" json:"read,omitempty"`
	Write bool `yaml:"write,omitempty" json:"write,omitempty"`
}

type SharedFieldPermissions struct {
	Read  bool `yaml:"read,omitempty" json:"read,omitempty"`
	Write bool `yaml:"write,omitempty" json:"write,omitempty"`
}

type RenderPermissions struct {
	Context     RenderContextPermissions    `yaml:"context,omitempty" json:"context,omitempty"`
	HTMLSlots   RenderHTMLSlotPermissions   `yaml:"html_slots,omitempty" json:"html_slots,omitempty"`
	Assets      RenderAssetPermissions      `yaml:"assets,omitempty" json:"assets,omitempty"`
	AfterRender RenderAfterRenderPermission `yaml:"after_render,omitempty" json:"after_render,omitempty"`
}

type RenderContextPermissions struct {
	Read  bool `yaml:"read,omitempty" json:"read,omitempty"`
	Write bool `yaml:"write,omitempty" json:"write,omitempty"`
}

type RenderHTMLSlotPermissions struct {
	Inject bool `yaml:"inject,omitempty" json:"inject,omitempty"`
}

type RenderAssetPermissions struct {
	InjectCSS          bool `yaml:"inject_css,omitempty" json:"inject_css,omitempty"`
	InjectJS           bool `yaml:"inject_js,omitempty" json:"inject_js,omitempty"`
	InjectRemoteAssets bool `yaml:"inject_remote_assets,omitempty" json:"inject_remote_assets,omitempty"`
}

type RenderAfterRenderPermission struct {
	MutateHTML bool `yaml:"mutate_html,omitempty" json:"mutate_html,omitempty"`
}

type GraphPermissions struct {
	Read       bool                     `yaml:"read,omitempty" json:"read,omitempty"`
	Mutate     bool                     `yaml:"mutate,omitempty" json:"mutate,omitempty"`
	Routes     GraphRoutePermissions    `yaml:"routes,omitempty" json:"routes,omitempty"`
	Taxonomies GraphTaxonomyPermissions `yaml:"taxonomies,omitempty" json:"taxonomies,omitempty"`
}

type GraphRoutePermissions struct {
	Inspect bool `yaml:"inspect,omitempty" json:"inspect,omitempty"`
	Mutate  bool `yaml:"mutate,omitempty" json:"mutate,omitempty"`
}

type GraphTaxonomyPermissions struct {
	Inspect bool `yaml:"inspect,omitempty" json:"inspect,omitempty"`
	Mutate  bool `yaml:"mutate,omitempty" json:"mutate,omitempty"`
}

type AdminPermissions struct {
	Extensions  AdminExtensionPermissions   `yaml:"extensions,omitempty" json:"extensions,omitempty"`
	Users       AdminUserPermissions        `yaml:"users,omitempty" json:"users,omitempty"`
	Audit       AdminAuditPermissions       `yaml:"audit,omitempty" json:"audit,omitempty"`
	Diagnostics AdminDiagnosticsPermissions `yaml:"diagnostics,omitempty" json:"diagnostics,omitempty"`
	Operations  AdminOperationsPermissions  `yaml:"operations,omitempty" json:"operations,omitempty"`
}

type AdminExtensionPermissions struct {
	Pages            bool `yaml:"pages,omitempty" json:"pages,omitempty"`
	Widgets          bool `yaml:"widgets,omitempty" json:"widgets,omitempty"`
	SettingsSections bool `yaml:"settings_sections,omitempty" json:"settings_sections,omitempty"`
	Slots            bool `yaml:"slots,omitempty" json:"slots,omitempty"`
}

type AdminUserPermissions struct {
	Read           bool `yaml:"read,omitempty" json:"read,omitempty"`
	Write          bool `yaml:"write,omitempty" json:"write,omitempty"`
	RevokeSessions bool `yaml:"revoke_sessions,omitempty" json:"revoke_sessions,omitempty"`
	ResetPasswords bool `yaml:"reset_passwords,omitempty" json:"reset_passwords,omitempty"`
}

type AdminAuditPermissions struct {
	Read bool `yaml:"read,omitempty" json:"read,omitempty"`
}

type AdminDiagnosticsPermissions struct {
	Read     bool `yaml:"read,omitempty" json:"read,omitempty"`
	Validate bool `yaml:"validate,omitempty" json:"validate,omitempty"`
}

type AdminOperationsPermissions struct {
	Rebuild    bool `yaml:"rebuild,omitempty" json:"rebuild,omitempty"`
	ClearCache bool `yaml:"clear_cache,omitempty" json:"clear_cache,omitempty"`
	Backups    bool `yaml:"backups,omitempty" json:"backups,omitempty"`
	Updates    bool `yaml:"updates,omitempty" json:"updates,omitempty"`
}

type RuntimePermissions struct {
	Server  RuntimeServerPermissions `yaml:"server,omitempty" json:"server,omitempty"`
	Metrics RuntimeBoolPermission    `yaml:"metrics,omitempty" json:"metrics,omitempty"`
	Logs    RuntimeBoolPermission    `yaml:"logs,omitempty" json:"logs,omitempty"`
}

type RuntimeServerPermissions struct {
	OnStarted      bool `yaml:"on_started,omitempty" json:"on_started,omitempty"`
	RegisterRoutes bool `yaml:"register_routes,omitempty" json:"register_routes,omitempty"`
}

type RuntimeBoolPermission struct {
	Read bool `yaml:"read,omitempty" json:"read,omitempty"`
}

type SecretPermissions struct {
	Access SecretAccessPermissions `yaml:"access,omitempty" json:"access,omitempty"`
}

type SecretAccessPermissions struct {
	AdminTokens       bool `yaml:"admin_tokens,omitempty" json:"admin_tokens,omitempty"`
	SessionStore      bool `yaml:"session_store,omitempty" json:"session_store,omitempty"`
	PasswordHashes    bool `yaml:"password_hashes,omitempty" json:"password_hashes,omitempty"`
	TOTPSecrets       bool `yaml:"totp_secrets,omitempty" json:"totp_secrets,omitempty"`
	EnvSecrets        bool `yaml:"env_secrets,omitempty" json:"env_secrets,omitempty"`
	DeployKeys        bool `yaml:"deploy_keys,omitempty" json:"deploy_keys,omitempty"`
	UpdateCredentials bool `yaml:"update_credentials,omitempty" json:"update_credentials,omitempty"`
}

type CapabilityPermissions struct {
	Dangerous             bool `yaml:"dangerous,omitempty" json:"dangerous,omitempty"`
	RequiresAdminApproval bool `yaml:"requires_admin_approval,omitempty" json:"requires_admin_approval,omitempty"`
}

type RuntimeConfig struct {
	Mode            string            `yaml:"mode,omitempty" json:"mode,omitempty"`
	ProtocolVersion string            `yaml:"protocol_version,omitempty" json:"protocol_version,omitempty"`
	Command         []string          `yaml:"command,omitempty" json:"command,omitempty"`
	Socket          string            `yaml:"socket,omitempty" json:"socket,omitempty"`
	Env             map[string]string `yaml:"env,omitempty" json:"env,omitempty"`
	Sandbox         RuntimeSandbox    `yaml:"sandbox,omitempty" json:"sandbox,omitempty"`
}

type RuntimeSandbox struct {
	Profile              string `yaml:"profile,omitempty" json:"profile,omitempty"`
	AllowNetwork         bool   `yaml:"allow_network,omitempty" json:"allow_network,omitempty"`
	AllowFilesystemWrite bool   `yaml:"allow_filesystem_write,omitempty" json:"allow_filesystem_write,omitempty"`
	AllowProcessExec     bool   `yaml:"allow_process_exec,omitempty" json:"allow_process_exec,omitempty"`
}

// Dependency declares another plugin repo that should be present for this
// plugin to operate correctly.
type Dependency struct {
	Name     string `yaml:"name"`
	Version  string `yaml:"version,omitempty"`
	Optional bool   `yaml:"optional,omitempty"`
}

// AdminExtensions declares the admin-facing surfaces a plugin contributes.
//
// These declarations are consumed by the Admin SDK and admin themes so plugin
// UI can be mounted through stable contracts instead of internal shell details.
type AdminExtensions struct {
	Pages            []AdminPage            `yaml:"pages,omitempty"`
	Widgets          []AdminWidget          `yaml:"widgets,omitempty"`
	Slots            []AdminSlot            `yaml:"slots,omitempty"`
	SettingsSections []AdminSettingsSection `yaml:"settings_sections,omitempty"`
}

// AdminPage declares a plugin-provided admin page.
//
// Route is the logical admin route segment, while Module and Styles point to
// plugin-relative assets served by Foundry when the admin shell mounts the
// page. Capability gates whether the current admin user may access the page.
type AdminPage struct {
	Key         string   `yaml:"key"`
	Title       string   `yaml:"title"`
	Route       string   `yaml:"route"`
	NavGroup    string   `yaml:"nav_group,omitempty"`
	Capability  string   `yaml:"capability,omitempty"`
	Description string   `yaml:"description,omitempty"`
	Module      string   `yaml:"module,omitempty"`
	Styles      []string `yaml:"styles,omitempty"`
}

// AdminWidget declares a plugin-provided widget for a named admin theme slot.
//
// Slot values must match widget slots supported by the active admin theme.
type AdminWidget struct {
	Key         string   `yaml:"key"`
	Title       string   `yaml:"title"`
	Slot        string   `yaml:"slot"`
	Capability  string   `yaml:"capability,omitempty"`
	Description string   `yaml:"description,omitempty"`
	Module      string   `yaml:"module,omitempty"`
	Styles      []string `yaml:"styles,omitempty"`
}

// AdminSlot declares a logical admin slot that the plugin expects themes to
// expose or understand.
type AdminSlot struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description,omitempty"`
}

// AdminSettingsSection declares plugin-owned settings that Foundry can surface
// in the admin UI and validate against a schema.
type AdminSettingsSection struct {
	Key         string                   `yaml:"key"`
	Title       string                   `yaml:"title"`
	Capability  string                   `yaml:"capability,omitempty"`
	Description string                   `yaml:"description,omitempty"`
	Schema      []config.FieldDefinition `yaml:"schema,omitempty"`
}

// LoadMetadata reads plugin.yaml for a single plugin directory and applies
// Foundry defaults for omitted fields.
func LoadMetadata(pluginsDir, name string) (Metadata, error) {
	var err error
	name, err = validatePluginName(name)
	if err != nil {
		return Metadata{}, err
	}

	meta := Metadata{
		Name:      name,
		Title:     name,
		Version:   "0.0.0",
		Directory: filepath.Join(pluginsDir, name),
		Requires:  []string{},
	}

	path := filepath.Join(pluginsDir, name, "plugin.yaml")
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return meta, nil
		}
		return Metadata{}, fmt.Errorf("read plugin metadata %s: %w", path, err)
	}

	if err := yaml.Unmarshal(b, &meta); err != nil {
		return Metadata{}, fmt.Errorf("parse plugin metadata %s: %w", path, err)
	}

	meta.Name = strings.TrimSpace(meta.Name)
	meta.Title = strings.TrimSpace(meta.Title)
	meta.Version = strings.TrimSpace(meta.Version)
	meta.Description = strings.TrimSpace(meta.Description)
	meta.Author = strings.TrimSpace(meta.Author)
	meta.Homepage = strings.TrimSpace(meta.Homepage)
	meta.License = strings.TrimSpace(meta.License)
	meta.Repo = normalizeRepoRef(meta.Repo)
	meta.FoundryAPI = strings.TrimSpace(meta.FoundryAPI)
	meta.MinFoundryVersion = strings.TrimSpace(meta.MinFoundryVersion)
	meta.CompatibilityVersion = strings.TrimSpace(meta.CompatibilityVersion)
	meta.Directory = filepath.Join(pluginsDir, name)
	normalizePermissionSet(&meta.Permissions)
	normalizeRuntimeConfig(&meta.Runtime)

	if meta.Name == "" {
		meta.Name = name
	}
	if meta.Title == "" {
		meta.Title = meta.Name
	}
	if meta.Version == "" {
		meta.Version = "0.0.0"
	}

	reqs := make([]string, 0, len(meta.Requires))
	seen := make(map[string]struct{}, len(meta.Requires))
	for _, r := range meta.Requires {
		r = normalizeRepoRef(r)
		if r == "" {
			continue
		}
		if _, ok := seen[r]; ok {
			continue
		}
		seen[r] = struct{}{}
		reqs = append(reqs, r)
	}
	meta.Requires = reqs
	for i := range meta.Dependencies {
		meta.Dependencies[i].Name = normalizeRepoRef(meta.Dependencies[i].Name)
		meta.Dependencies[i].Version = strings.TrimSpace(meta.Dependencies[i].Version)
	}
	for i := range meta.AdminExtensions.Pages {
		meta.AdminExtensions.Pages[i].Key = strings.TrimSpace(meta.AdminExtensions.Pages[i].Key)
		meta.AdminExtensions.Pages[i].Title = strings.TrimSpace(meta.AdminExtensions.Pages[i].Title)
		meta.AdminExtensions.Pages[i].Route = strings.TrimSpace(meta.AdminExtensions.Pages[i].Route)
		meta.AdminExtensions.Pages[i].NavGroup = normalizeAdminNavGroup(meta.AdminExtensions.Pages[i].NavGroup)
		meta.AdminExtensions.Pages[i].Capability = strings.TrimSpace(meta.AdminExtensions.Pages[i].Capability)
		meta.AdminExtensions.Pages[i].Description = strings.TrimSpace(meta.AdminExtensions.Pages[i].Description)
		meta.AdminExtensions.Pages[i].Module = strings.TrimSpace(meta.AdminExtensions.Pages[i].Module)
		for j := range meta.AdminExtensions.Pages[i].Styles {
			meta.AdminExtensions.Pages[i].Styles[j] = strings.TrimSpace(meta.AdminExtensions.Pages[i].Styles[j])
		}
	}

	if err := validateMetadataCompatibility(meta); err != nil {
		return Metadata{}, fmt.Errorf("validate plugin metadata %s: %w", path, err)
	}

	return meta, nil
}

// LoadAllMetadata loads metadata for the enabled plugin list.
func LoadAllMetadata(pluginsDir string, enabled []string) (map[string]Metadata, error) {
	out := make(map[string]Metadata, len(enabled))

	for _, name := range enabled {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}

		meta, err := LoadMetadata(pluginsDir, name)
		if err != nil {
			return nil, err
		}
		out[name] = meta
	}

	return out, nil
}

// NormalizeAdminAssetPath validates a plugin-relative admin asset path.
//
// Plugin page and widget bundles are served from inside the plugin directory,
// so this helper rejects absolute paths and traversal segments before those
// paths are published to the admin shell.
func NormalizeAdminAssetPath(rel string) (string, error) {
	rel = strings.TrimSpace(rel)
	if rel == "" {
		return "", fmt.Errorf("admin asset path cannot be empty")
	}

	clean := filepath.ToSlash(filepath.Clean(rel))
	if strings.HasPrefix(clean, "/") || clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("admin asset path must stay inside the plugin directory")
	}

	return clean, nil
}

func normalizeRepoRef(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "https://")
	v = strings.TrimPrefix(v, "http://")
	v = strings.TrimPrefix(v, "git@")
	v = strings.TrimPrefix(v, "ssh://")
	v = strings.TrimPrefix(v, "github.com:")
	v = strings.TrimPrefix(v, "github.com/")
	v = strings.Trim(v, "/")
	v = strings.TrimSuffix(v, ".git")

	if v == "" {
		return ""
	}

	if strings.Count(v, "/") == 1 {
		return "github.com/" + v
	}

	return v
}

func normalizeAdminNavGroup(v string) string {
	return strings.ToLower(strings.TrimSpace(v))
}
