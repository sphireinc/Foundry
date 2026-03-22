package plugins

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	foundryconfig "github.com/sphireinc/foundry/internal/config"
)

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
	var err error
	name, err = validatePluginName(name)
	if err != nil {
		return err
	}

	return foundryconfig.EnsureStringListValue(configPath, []string{"plugins", "enabled"}, name)
}

func DisableInConfig(configPath, name string) error {
	var err error
	name, err = validatePluginName(name)
	if err != nil {
		return err
	}

	return foundryconfig.RemoveStringListValue(configPath, []string{"plugins", "enabled"}, name)
}

func UpdateInstalled(pluginsDir, name string) (Metadata, error) {
	var err error
	name, err = validatePluginName(name)
	if err != nil {
		return Metadata{}, err
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

	if _, err := backupInstalled(pluginsDir, name); err != nil {
		_ = os.RemoveAll(tmpDir)
		return Metadata{}, fmt.Errorf("backup current plugin: %w", err)
	}

	if err := os.Rename(filepath.Join(pluginsDir, tmpName), targetDir); err != nil {
		if backupDir, ok, latestErr := latestRollback(pluginsDir, name); latestErr == nil && ok {
			_ = os.Rename(backupDir, targetDir)
		}
		return Metadata{}, fmt.Errorf("replace plugin with updated version: %w", err)
	}

	installMeta.Name = name
	installMeta.Directory = filepath.Join(pluginsDir, name)
	return installMeta, nil
}
