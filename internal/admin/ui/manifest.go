package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sphireinc/foundry/internal/consts"
	"github.com/sphireinc/foundry/internal/safepath"
	"gopkg.in/yaml.v3"
)

// Manifest is the contract Foundry reads from an admin theme's
// admin-theme.yaml.
//
// Admin themes should declare the admin API version, SDK version, shell
// components, and widget slots they support so alternate admin frontends remain
// compatible with Foundry's extension system.
type Manifest struct {
	Name                 string   `yaml:"name"`
	Title                string   `yaml:"title"`
	Version              string   `yaml:"version"`
	Description          string   `yaml:"description"`
	Author               string   `yaml:"author"`
	License              string   `yaml:"license"`
	AdminAPI             string   `yaml:"admin_api"`
	SDKVersion           string   `yaml:"sdk_version,omitempty"`
	CompatibilityVersion string   `yaml:"compatibility_version,omitempty"`
	Components           []string `yaml:"components"`
	WidgetSlots          []string `yaml:"widget_slots,omitempty"`
	Screenshots          []string `yaml:"screenshots,omitempty"`
}

// Diagnostic is a single validation finding for an admin theme
type Diagnostic struct {
	Severity string
	Path     string
	Message  string
}

// ValidationResult summarizes admin theme validation
type ValidationResult struct {
	Valid       bool
	Diagnostics []Diagnostic
}

// ThemeInfo identifies an installed admin theme directory
type ThemeInfo struct {
	Name string
	Path string
}

var requiredComponents = []string{
	"shell",
	"login",
	"navigation",
	"documents",
	"media",
	"users",
	"config",
	"plugins",
	"themes",
	"audit",
}

var requiredWidgetSlots = []string{
	"overview.after",
	"documents.sidebar",
	"media.sidebar",
	"plugins.sidebar",
}

// ListInstalled returns all admin themes under themesDir/admin-themes.
func ListInstalled(themesDir string) ([]ThemeInfo, error) {
	root := filepath.Join(themesDir, "admin-themes")
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return []ThemeInfo{}, nil
		}
		return nil, err
	}
	out := make([]ThemeInfo, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		out = append(out, ThemeInfo{Name: entry.Name(), Path: filepath.Join(root, entry.Name())})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// LoadManifest reads and normalizes admin-theme.yaml for an admin theme.
//
// When the manifest is missing, Foundry synthesizes a default contract so the
// built-in admin theme can still work
func LoadManifest(themesDir, name string) (*Manifest, error) {
	name, err := safepath.ValidatePathComponent("admin theme name", name)
	if err != nil {
		return nil, err
	}
	path := filepath.Join(themesDir, "admin-themes", name, "admin-theme.yaml")
	body, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Manifest{
				Name:                 name,
				Title:                name,
				Version:              "0.0.0",
				AdminAPI:             consts.AdminAPIContractVersion,
				SDKVersion:           consts.AdminSDKVersion,
				CompatibilityVersion: consts.AdminThemeCompatibility,
				Components:           append([]string(nil), requiredComponents...),
				WidgetSlots:          append([]string(nil), requiredWidgetSlots...),
			}, nil
		}
		return nil, err
	}
	var manifest Manifest
	if err := yaml.Unmarshal(body, &manifest); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if strings.TrimSpace(manifest.Name) == "" {
		manifest.Name = name
	}
	if strings.TrimSpace(manifest.Title) == "" {
		manifest.Title = manifest.Name
	}
	if strings.TrimSpace(manifest.Version) == "" {
		manifest.Version = "0.0.0"
	}
	if strings.TrimSpace(manifest.AdminAPI) == "" {
		manifest.AdminAPI = consts.AdminAPIContractVersion
	}
	if strings.TrimSpace(manifest.SDKVersion) == "" {
		manifest.SDKVersion = consts.AdminSDKVersion
	}
	if strings.TrimSpace(manifest.CompatibilityVersion) == "" {
		manifest.CompatibilityVersion = consts.AdminThemeCompatibility
	}
	if len(manifest.Components) == 0 {
		manifest.Components = append([]string(nil), requiredComponents...)
	}
	if len(manifest.WidgetSlots) == 0 {
		manifest.WidgetSlots = append([]string(nil), requiredWidgetSlots...)
	}
	return &manifest, nil
}

// ValidateTheme validates an installed admin theme against Foundry's required
// contract
func ValidateTheme(themesDir, name string) (*ValidationResult, error) {
	name, err := safepath.ValidatePathComponent("admin theme name", name)
	if err != nil {
		return nil, err
	}
	root := filepath.Join(themesDir, "admin-themes", name)
	if _, err := os.Stat(root); err != nil {
		return nil, err
	}
	manifest, err := LoadManifest(themesDir, name)
	if err != nil {
		return nil, err
	}
	result := &ValidationResult{Valid: true, Diagnostics: make([]Diagnostic, 0)}
	add := func(severity, path, message string) {
		result.Diagnostics = append(result.Diagnostics, Diagnostic{
			Severity: severity,
			Path:     filepath.ToSlash(path),
			Message:  message,
		})
		if severity == "error" {
			result.Valid = false
		}
	}
	if manifest.Name != name {
		add("error", filepath.Join(root, "admin-theme.yaml"), fmt.Sprintf("admin theme manifest name %q must match directory %q", manifest.Name, name))
	}
	if manifest.AdminAPI != consts.AdminAPIContractVersion {
		add("error", filepath.Join(root, "admin-theme.yaml"), fmt.Sprintf("unsupported admin_api %q", manifest.AdminAPI))
	}
	if manifest.SDKVersion != consts.AdminSDKVersion {
		add("error", filepath.Join(root, "admin-theme.yaml"), fmt.Sprintf("unsupported sdk_version %q", manifest.SDKVersion))
	}
	if manifest.CompatibilityVersion != consts.AdminThemeCompatibility {
		add("error", filepath.Join(root, "admin-theme.yaml"), fmt.Sprintf("unsupported compatibility_version %q", manifest.CompatibilityVersion))
	}
	for _, rel := range []string{"index.html", filepath.Join("assets", "admin.css"), filepath.Join("assets", "admin.js")} {
		path := filepath.Join(root, rel)
		if _, err := os.Stat(path); err != nil {
			if os.IsNotExist(err) {
				add("error", path, "missing required admin theme file")
				continue
			}
			return nil, err
		}
	}
	declared := make(map[string]struct{}, len(manifest.Components))
	for _, component := range manifest.Components {
		declared[strings.TrimSpace(component)] = struct{}{}
	}
	for _, component := range requiredComponents {
		if _, ok := declared[component]; !ok {
			add("error", filepath.Join(root, "admin-theme.yaml"), fmt.Sprintf("missing required admin component %q", component))
		}
	}
	declaredSlots := make(map[string]struct{}, len(manifest.WidgetSlots))
	for _, slot := range manifest.WidgetSlots {
		declaredSlots[strings.TrimSpace(slot)] = struct{}{}
	}
	for _, slot := range requiredWidgetSlots {
		if _, ok := declaredSlots[slot]; !ok {
			add("error", filepath.Join(root, "admin-theme.yaml"), fmt.Sprintf("missing required admin widget slot %q", slot))
		}
	}
	return result, nil
}
