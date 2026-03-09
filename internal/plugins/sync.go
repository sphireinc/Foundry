package plugins

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sphireinc/foundry/internal/consts"
	"gopkg.in/yaml.v3"
)

const (
	DefaultSyncConfigPath = consts.ConfigFilePath
	DefaultSyncPluginsDir = "plugins"
	DefaultSyncOutputPath = "internal/generated/plugins_gen.go"
	DefaultSyncModulePath = "github.com/sphireinc/foundry"
)

type syncSiteConfig struct {
	Plugins struct {
		Enabled []string `yaml:"enabled"`
	} `yaml:"plugins"`
}

type syncPluginMetadata struct {
	Name        string   `yaml:"name"`
	Title       string   `yaml:"title"`
	Version     string   `yaml:"version"`
	Description string   `yaml:"description"`
	Author      string   `yaml:"author"`
	Homepage    string   `yaml:"homepage"`
	License     string   `yaml:"license"`
	Repo        string   `yaml:"repo"`
	Requires    []string `yaml:"requires"`
}

type SyncOptions struct {
	ConfigPath string
	PluginsDir string
	OutputPath string
	ModulePath string
}

func SyncFromConfig(opts SyncOptions) error {
	opts = normalizeSyncOptions(opts)

	cfg, err := loadSyncConfig(opts.ConfigPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	return SyncEnabledPlugins(opts, cfg.Plugins.Enabled)
}

func SyncEnabledPlugins(opts SyncOptions, enabled []string) error {
	opts = normalizeSyncOptions(opts)

	enabled = uniqueSorted(enabled)

	for _, name := range enabled {
		if err := validatePluginForSync(opts.PluginsDir, name); err != nil {
			return fmt.Errorf("validate plugin %s: %w", name, err)
		}
	}

	if err := os.MkdirAll(filepath.Dir(opts.OutputPath), 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	content := generateImportsFile(opts.ModulePath, enabled)

	existing, err := os.ReadFile(opts.OutputPath)
	if err == nil && bytes.Equal(existing, []byte(content)) {
		return nil
	}
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read existing generated imports: %w", err)
	}

	if err := os.WriteFile(opts.OutputPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write generated imports: %w", err)
	}

	return nil
}

func normalizeSyncOptions(opts SyncOptions) SyncOptions {
	if strings.TrimSpace(opts.ConfigPath) == "" {
		opts.ConfigPath = DefaultSyncConfigPath
	}
	if strings.TrimSpace(opts.PluginsDir) == "" {
		opts.PluginsDir = DefaultSyncPluginsDir
	}
	if strings.TrimSpace(opts.OutputPath) == "" {
		opts.OutputPath = DefaultSyncOutputPath
	}
	if strings.TrimSpace(opts.ModulePath) == "" {
		opts.ModulePath = DefaultSyncModulePath
	}
	return opts
}

func loadSyncConfig(path string) (*syncSiteConfig, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg syncSiteConfig
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func validatePluginForSync(pluginsDir, name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("plugin name cannot be empty")
	}
	if strings.Contains(name, "/") || strings.Contains(name, `\`) {
		return fmt.Errorf("plugin name %q must be a single directory name", name)
	}

	root := filepath.Join(pluginsDir, name)
	info, err := os.Stat(root)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("plugin %q is enabled but directory %q does not exist", name, root)
		}
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("plugin path %q is not a directory", root)
	}

	foundGo := false
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Ext(path) == ".go" {
			foundGo = true
		}
		return nil
	})
	if err != nil {
		return err
	}

	if !foundGo {
		return fmt.Errorf("plugin %q has no .go files under %q", name, root)
	}

	if err := validateMetadataForSync(pluginsDir, name); err != nil {
		return err
	}

	return nil
}

func validateMetadataForSync(pluginsDir, name string) error {
	path := filepath.Join(pluginsDir, name, "plugin.yaml")

	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read %s: %w", path, err)
	}

	var meta syncPluginMetadata
	if err := yaml.Unmarshal(b, &meta); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}

	meta.Name = strings.TrimSpace(meta.Name)
	if meta.Name != "" && meta.Name != name {
		return fmt.Errorf("%s: metadata name %q must match plugin directory %q", path, meta.Name, name)
	}

	meta.Repo = normalizeRepoRef(meta.Repo)
	if meta.Repo != "" && !isValidRepoRef(meta.Repo) {
		return fmt.Errorf("%s: invalid repo %q", path, meta.Repo)
	}

	for _, dep := range meta.Requires {
		dep = normalizeRepoRef(dep)
		if dep == "" {
			continue
		}
		if !isValidRepoRef(dep) {
			return fmt.Errorf("%s: invalid requires entry %q", path, dep)
		}
	}

	return nil
}

func uniqueSorted(in []string) []string {
	set := make(map[string]struct{}, len(in))
	for _, v := range in {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		set[v] = struct{}{}
	}

	out := make([]string, 0, len(set))
	for v := range set {
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

func generateImportsFile(modulePath string, enabled []string) string {
	var buf bytes.Buffer

	buf.WriteString("// Code generated by plugin sync; DO NOT EDIT.\n")
	buf.WriteString("package generated\n\n")

	if len(enabled) == 0 {
		return buf.String()
	}

	buf.WriteString("import (\n")
	for _, name := range enabled {
		_, _ = fmt.Fprintf(&buf, "\t_ %q\n", strings.TrimRight(modulePath, "/")+"/plugins/"+name)
	}
	buf.WriteString(")\n")

	return buf.String()
}

func isValidRepoRef(v string) bool {
	v = strings.TrimSpace(v)
	if v == "" {
		return false
	}

	parts := strings.Split(v, "/")
	return len(parts) >= 3 && parts[0] != "" && parts[1] != "" && parts[2] != ""
}
