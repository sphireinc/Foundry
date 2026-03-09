package plugins

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type configFile struct {
	Plugins struct {
		Enabled []string `yaml:"enabled"`
	} `yaml:"plugins"`
}

func ListInstalled(pluginsDir string) ([]Metadata, error) {
	entries, err := os.ReadDir(pluginsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []Metadata{}, nil
		}
		return nil, err
	}

	out := make([]Metadata, 0)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		name := entry.Name()
		meta, err := LoadMetadata(pluginsDir, name)
		if err != nil {
			return nil, err
		}
		out = append(out, meta)
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})

	return out, nil
}

func ValidateInstalledPlugin(pluginsDir, name string) error {
	return validatePluginForSync(pluginsDir, name)
}

func EnableInConfig(configPath, name string) error {
	cfg, err := loadPluginConfigFile(configPath)
	if err != nil {
		return err
	}

	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("plugin name cannot be empty")
	}

	for _, existing := range cfg.Plugins.Enabled {
		if existing == name {
			return nil
		}
	}

	cfg.Plugins.Enabled = append(cfg.Plugins.Enabled, name)
	sort.Strings(cfg.Plugins.Enabled)

	return writePluginConfigFile(configPath, cfg)
}

func DisableInConfig(configPath, name string) error {
	cfg, err := loadPluginConfigFile(configPath)
	if err != nil {
		return err
	}

	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("plugin name cannot be empty")
	}

	out := make([]string, 0, len(cfg.Plugins.Enabled))
	for _, existing := range cfg.Plugins.Enabled {
		if existing != name {
			out = append(out, existing)
		}
	}
	cfg.Plugins.Enabled = out

	return writePluginConfigFile(configPath, cfg)
}

func UpdateInstalled(pluginsDir, name string) (Metadata, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Metadata{}, fmt.Errorf("plugin name cannot be empty")
	}

	targetDir := filepath.Join(pluginsDir, name)
	if _, err := os.Stat(targetDir); err != nil {
		if os.IsNotExist(err) {
			return Metadata{}, fmt.Errorf("plugin %q is not installed", name)
		}
		return Metadata{}, err
	}

	gitDir := filepath.Join(targetDir, ".git")
	if info, err := os.Stat(gitDir); err == nil && info.IsDir() {
		cmd := exec.Command("git", "-C", targetDir, "pull", "--ff-only")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return Metadata{}, fmt.Errorf("git pull failed: %w", err)
		}
		return LoadMetadata(pluginsDir, name)
	}

	meta, err := LoadMetadata(pluginsDir, name)
	if err != nil {
		return Metadata{}, err
	}
	if strings.TrimSpace(meta.Repo) == "" {
		return Metadata{}, fmt.Errorf("plugin %q cannot be updated: no .git directory and no repo metadata", name)
	}

	tmpName := name + "-update-tmp"
	tmpDir := filepath.Join(pluginsDir, tmpName)
	_ = os.RemoveAll(tmpDir)

	installMeta, err := Install(InstallOptions{
		PluginsDir: pluginsDir,
		URL:        meta.Repo,
		Name:       tmpName,
	})
	if err != nil {
		_ = os.RemoveAll(tmpDir)
		return Metadata{}, fmt.Errorf("update fallback install failed: %w", err)
	}

	backupDir := filepath.Join(pluginsDir, name+"-backup-old")
	_ = os.RemoveAll(backupDir)

	if err := os.Rename(targetDir, backupDir); err != nil {
		_ = os.RemoveAll(tmpDir)
		return Metadata{}, fmt.Errorf("backup current plugin: %w", err)
	}

	if err := os.Rename(filepath.Join(pluginsDir, tmpName), targetDir); err != nil {
		_ = os.Rename(backupDir, targetDir)
		return Metadata{}, fmt.Errorf("replace plugin with updated version: %w", err)
	}

	_ = os.RemoveAll(backupDir)

	installMeta.Name = name
	installMeta.Directory = filepath.Join(pluginsDir, name)
	return installMeta, nil
}

func loadPluginConfigFile(path string) (*configFile, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg configFile
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return nil, err
	}
	if cfg.Plugins.Enabled == nil {
		cfg.Plugins.Enabled = []string{}
	}

	return &cfg, nil
}

func writePluginConfigFile(path string, cfg *configFile) error {
	b, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}
