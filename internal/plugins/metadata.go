package plugins

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sphireinc/foundry/internal/config"
	"gopkg.in/yaml.v3"
)

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

type Dependency struct {
	Name     string `yaml:"name"`
	Version  string `yaml:"version,omitempty"`
	Optional bool   `yaml:"optional,omitempty"`
}

type AdminExtensions struct {
	Pages            []AdminPage            `yaml:"pages,omitempty"`
	Widgets          []AdminWidget          `yaml:"widgets,omitempty"`
	Slots            []AdminSlot            `yaml:"slots,omitempty"`
	SettingsSections []AdminSettingsSection `yaml:"settings_sections,omitempty"`
}

type AdminPage struct {
	Key         string   `yaml:"key"`
	Title       string   `yaml:"title"`
	Route       string   `yaml:"route"`
	Capability  string   `yaml:"capability,omitempty"`
	Description string   `yaml:"description,omitempty"`
	Module      string   `yaml:"module,omitempty"`
	Styles      []string `yaml:"styles,omitempty"`
}

type AdminWidget struct {
	Key         string   `yaml:"key"`
	Title       string   `yaml:"title"`
	Slot        string   `yaml:"slot"`
	Capability  string   `yaml:"capability,omitempty"`
	Description string   `yaml:"description,omitempty"`
	Module      string   `yaml:"module,omitempty"`
	Styles      []string `yaml:"styles,omitempty"`
}

type AdminSlot struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description,omitempty"`
}

type AdminSettingsSection struct {
	Key         string                   `yaml:"key"`
	Title       string                   `yaml:"title"`
	Capability  string                   `yaml:"capability,omitempty"`
	Description string                   `yaml:"description,omitempty"`
	Schema      []config.FieldDefinition `yaml:"schema,omitempty"`
}

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

	if err := validateMetadataCompatibility(meta); err != nil {
		return Metadata{}, fmt.Errorf("validate plugin metadata %s: %w", path, err)
	}

	return meta, nil
}

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
