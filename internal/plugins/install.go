package plugins

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

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
func repoZipURL(repoURL string) (string, error) {
	u, err := url.Parse(repoURL)
	if err != nil {
		return "", err
	}

	if !strings.EqualFold(u.Host, "github.com") {
		return "", fmt.Errorf("zip fallback currently supports GitHub only")
	}

	path := strings.TrimSuffix(u.Path, ".git")
	path = strings.Trim(path, "/")

	return fmt.Sprintf("https://github.com/%s/archive/refs/heads/main.zip", path), nil
}

// downloadAndExtract downloads a repository archive and extracts it into
// targetDir with zip-slip and symlink protections.
func downloadAndExtract(repoURL, targetDir string) error {
	zipURL, err := repoZipURL(repoURL)
	if err != nil {
		return err
	}

	resp, err := pluginDownloadClient.Get(zipURL)
	if err != nil {
		return fmt.Errorf("download plugin zip: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: %s", resp.Status)
	}

	tmpFile, err := os.CreateTemp("", "foundry-plugin-*.zip")
	if err != nil {
		return err
	}
	defer os.Remove(tmpFile.Name())

	written, err := io.Copy(tmpFile, io.LimitReader(resp.Body, pluginZipMaxBytes+1))
	if err != nil {
		return err
	}
	if written > pluginZipMaxBytes {
		return fmt.Errorf("plugin zip exceeds %d bytes", pluginZipMaxBytes)
	}
	tmpFile.Close()

	zr, err := zip.OpenReader(tmpFile.Name())
	if err != nil {
		return err
	}
	defer zr.Close()

	tempDir, err := os.MkdirTemp("", "foundry-plugin")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)

	for _, f := range zr.File {
		if f.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("zip contains unsupported symlink entry: %s", f.Name)
		}

		fp, err := safeArchivePath(tempDir, f.Name)
		if err != nil {
			return err
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(fp, f.Mode()); err != nil {
				return err
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(fp), 0755); err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			return err
		}

		out, err := os.Create(fp)
		if err != nil {
			rc.Close()
			return err
		}

		if _, err := io.Copy(out, rc); err != nil {
			rc.Close()
			out.Close()
			return err
		}

		rc.Close()
		out.Close()
	}

	entries, err := os.ReadDir(tempDir)
	if err != nil || len(entries) == 0 {
		return fmt.Errorf("zip extraction failed")
	}

	root := filepath.Join(tempDir, entries[0].Name())
	rootInfo, err := os.Stat(root)
	if err != nil {
		return err
	}
	if !rootInfo.IsDir() {
		return fmt.Errorf("zip extraction failed: root entry is not a directory")
	}

	return os.Rename(root, targetDir)
}

func stripVCSMetadata(targetDir string) error {
	for _, rel := range []string{".git", ".gitmodules"} {
		path := filepath.Join(targetDir, rel)
		if err := os.RemoveAll(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove VCS metadata %q: %w", rel, err)
		}
	}
	return nil
}

// safeArchivePath confines archive extraction paths to the provided root.
func safeArchivePath(root, name string) (string, error) {
	if strings.TrimSpace(name) == "" {
		return "", fmt.Errorf("zip contains empty entry name")
	}
	if filepath.IsAbs(name) {
		return "", fmt.Errorf("zip entry escapes target dir: %s", name)
	}

	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}

	target := filepath.Join(rootAbs, filepath.Clean(filepath.FromSlash(name)))
	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return "", err
	}

	rootWithSep := rootAbs + string(filepath.Separator)
	if targetAbs != rootAbs && !strings.HasPrefix(targetAbs, rootWithSep) {
		return "", fmt.Errorf("zip entry escapes target dir: %s", name)
	}

	return targetAbs, nil
}

// normalizeInstallURL expands shorthand repository references into cloneable
// URLs where possible.
func normalizeInstallURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	if strings.HasPrefix(raw, "git@") {
		return raw
	}

	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		u, err := url.Parse(raw)
		if err != nil {
			return raw
		}

		if strings.EqualFold(u.Host, "github.com") {
			path := strings.TrimSuffix(u.Path, "/")
			if !strings.HasSuffix(path, ".git") {
				path += ".git"
			}
			u.Path = path
			return u.String()
		}

		return raw
	}

	parts := strings.Split(raw, "/")
	if len(parts) == 2 && parts[0] != "" && parts[1] != "" {
		return "https://github.com/" + raw + ".git"
	}

	return raw
}

// validateInstallURL constrains plugin install URLs to supported GitHub forms.
func validateInstallURL(raw string) (string, error) {
	normalized := normalizeInstallURL(raw)
	if strings.TrimSpace(normalized) == "" {
		return "", nil
	}

	if strings.HasPrefix(normalized, "git@github.com:") {
		name, err := inferPluginName(normalized)
		if err != nil {
			return "", err
		}
		if _, err := validatePluginName(name); err != nil {
			return "", fmt.Errorf("invalid GitHub repository path: %w", err)
		}
		return normalized, nil
	}

	u, err := url.Parse(normalized)
	if err != nil {
		return "", fmt.Errorf("parse plugin URL: %w", err)
	}
	if !strings.EqualFold(u.Scheme, "https") {
		return "", fmt.Errorf("plugin URL must use https or git@github.com")
	}
	if !strings.EqualFold(u.Host, "github.com") {
		return "", fmt.Errorf("plugin URL must target github.com")
	}

	path := strings.Trim(strings.TrimSuffix(u.Path, ".git"), "/")
	parts := strings.Split(path, "/")
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return "", fmt.Errorf("plugin URL must point to a GitHub owner/repository")
	}
	if _, err := validatePluginName(parts[1]); err != nil {
		return "", fmt.Errorf("invalid GitHub repository path: %w", err)
	}

	return normalized, nil
}

func validatePluginName(name string) (string, error) {
	return safepath.ValidatePathComponent("plugin name", name)
}

func inferPluginName(raw string) (string, error) {
	if strings.HasPrefix(raw, "git@") {
		idx := strings.Index(raw, ":")
		if idx >= 0 && idx+1 < len(raw) {
			path := strings.Trim(raw[idx+1:], "/")
			parts := strings.Split(path, "/")
			name := parts[len(parts)-1]
			name = strings.TrimSuffix(name, ".git")
			name = strings.TrimSpace(name)
			if name != "" {
				return name, nil
			}
		}
		return "", fmt.Errorf("could not infer plugin name from URL")
	}

	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("parse plugin URL: %w", err)
	}

	path := strings.TrimSpace(u.Path)
	path = strings.Trim(path, "/")
	if path == "" {
		return "", fmt.Errorf("could not infer plugin name from URL")
	}

	parts := strings.Split(path, "/")
	name := parts[len(parts)-1]
	name = strings.TrimSuffix(name, ".git")
	name = strings.TrimSpace(name)

	if name == "" {
		return "", fmt.Errorf("could not infer plugin name from URL")
	}

	return name, nil
}
