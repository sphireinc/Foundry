package plugins

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/sphireinc/foundry/internal/installutil"
	"github.com/sphireinc/foundry/internal/safepath"
)

var pluginDownloadClient = &http.Client{Timeout: 30 * time.Second}

const (
	pluginCloneTimeout = 2 * time.Minute
	pluginZipMaxBytes  = 128 << 20
)

type InstallOptions struct {
	PluginsDir  string
	URL         string
	Name        string
	ApproveRisk bool
}

// Install clones or downloads a plugin repository into PluginsDir and returns
// its normalized metadata.
//
// Foundry prefers git clone and falls back to a constrained GitHub zip
// download. The install path is validated so plugin names remain filesystem-safe.
func Install(opts InstallOptions) (Metadata, error) {
	repoURL, err := validateInstallURL(opts.URL)
	if err != nil {
		return Metadata{}, err
	}
	if strings.TrimSpace(repoURL) == "" {
		return Metadata{}, fmt.Errorf("plugin URL cannot be empty")
	}

	pluginsDir := strings.TrimSpace(opts.PluginsDir)
	if pluginsDir == "" {
		return Metadata{}, fmt.Errorf("plugins directory cannot be empty")
	}

	name := strings.TrimSpace(opts.Name)
	if name == "" {
		name, err = inferPluginName(repoURL)
		if err != nil {
			return Metadata{}, err
		}
	}

	name, err = validatePluginName(name)
	if err != nil {
		return Metadata{}, err
	}

	targetDir, err := safepath.ResolveRelativeUnderRoot(pluginsDir, name)
	if err != nil {
		return Metadata{}, err
	}
	if _, err := os.Stat(targetDir); err == nil {
		return Metadata{}, fmt.Errorf("plugin directory already exists: %s", targetDir)
	} else if !os.IsNotExist(err) {
		return Metadata{}, err
	}

	if err := os.MkdirAll(pluginsDir, 0o755); err != nil {
		return Metadata{}, fmt.Errorf("create plugins dir: %w", err)
	}

	cloneCtx, cancel := context.WithTimeout(context.Background(), pluginCloneTimeout)
	defer cancel()
	cmd := exec.CommandContext(cloneCtx, "git", "clone", repoURL, targetDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Println("git clone failed because git not available, downloading repository archive instead")

		if err := downloadAndExtract(repoURL, targetDir); err != nil {
			return Metadata{}, fmt.Errorf("git clone failed and zip fallback failed: %w", err)
		}
	}
	if err := stripVCSMetadata(targetDir); err != nil {
		_ = removeInstalledPluginDir(pluginsDir, name)
		return Metadata{}, err
	}

	meta, err := LoadMetadata(pluginsDir, name)
	if err != nil {
		_ = removeInstalledPluginDir(pluginsDir, name)
		return Metadata{}, err
	}

	if strings.TrimSpace(meta.Name) != "" && meta.Name != name {
		_ = removeInstalledPluginDir(pluginsDir, name)
		return Metadata{}, fmt.Errorf("plugin metadata name %q does not match install directory %q", meta.Name, name)
	}
	report := AnalyzeInstalled(meta)
	if SecurityApprovalRequired(meta, report) && !opts.ApproveRisk {
		_ = removeInstalledPluginDir(pluginsDir, name)
		return Metadata{}, fmt.Errorf("plugin %q requires explicit approval due to declared or detected risky capabilities; rerun with approval", meta.Name)
	}

	return meta, nil
}

// Uninstall removes an installed plugin directory by name.
func Uninstall(pluginsDir, name string) error {
	pluginsDir = strings.TrimSpace(pluginsDir)

	if pluginsDir == "" {
		return fmt.Errorf("plugins directory cannot be empty")
	}
	var err error
	name, err = validatePluginName(name)
	if err != nil {
		return err
	}

	targetDir, err := safepath.ResolveRelativeUnderRoot(pluginsDir, name)
	if err != nil {
		return err
	}
	info, err := os.Stat(targetDir)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("plugin %q is not installed", name)
		}
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("plugin path %q is not a directory", targetDir)
	}

	if err := removeInstalledPluginDir(pluginsDir, name); err != nil {
		return fmt.Errorf("remove plugin directory: %w", err)
	}

	return nil
}

func removeInstalledPluginDir(pluginsDir, name string) error {
	validatedName, err := validatePluginName(name)
	if err != nil {
		return err
	}
	// This delete path is constrained to a single validated directory name under
	// the configured plugins root. ResolveRelativeUnderRoot rejects absolute
	// paths, separators, and traversal, and RemoveRelativeUnderRoot performs the
	// final root-bounded removal.
	return safepath.RemoveRelativeUnderRoot(strings.TrimSpace(pluginsDir), validatedName)
}

// repoZipURL returns the GitHub archive URL used by the zip fallback path.
func downloadAndExtract(repoURL, targetDir string) error {
	targetRoot := filepath.Dir(targetDir)
	targetName := filepath.Base(targetDir)
	return installutil.DownloadAndExtractRepoArchive(
		pluginDownloadClient,
		repoURL,
		targetRoot,
		targetName,
		"foundry-plugin",
		"plugin",
		pluginZipMaxBytes,
	)
}

func stripVCSMetadata(targetDir string) error {
	return installutil.StripVCSMetadata(filepath.Dir(targetDir), filepath.Base(targetDir))
}

// normalizeInstallURL expands shorthand repository references into cloneable
// URLs where possible.
func normalizeInstallURL(raw string) string {
	return installutil.NormalizeGitHubInstallURL(raw)
}

// validateInstallURL constrains plugin install URLs to supported GitHub forms.
func validateInstallURL(raw string) (string, error) {
	return installutil.ValidateGitHubInstallURL("plugin", raw, validatePluginName)
}

func validatePluginName(name string) (string, error) {
	return safepath.ValidatePathComponent("plugin name", name)
}

func inferPluginName(raw string) (string, error) {
	return installutil.InferRepoName(raw, "plugin", validatePluginName)
}
