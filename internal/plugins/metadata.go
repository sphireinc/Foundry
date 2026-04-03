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
	AdminExtensions      AdminExtensions          `yaml:"admin,omitempty"`
	Screenshots          []string                 `yaml:"screenshots,omitempty"`
	Directory            string                   `yaml:"-"`
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
