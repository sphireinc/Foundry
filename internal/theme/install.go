package theme

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

	adminui "github.com/sphireinc/foundry/internal/admin/ui"
	"github.com/sphireinc/foundry/internal/safepath"
)

var themeDownloadClient = &http.Client{Timeout: 30 * time.Second}

const (
	themeCloneTimeout = 2 * time.Minute
	themeZipMaxBytes  = 128 << 20
)

type InstallKind string

const (
	InstallKindFrontend InstallKind = "frontend"
	InstallKindAdmin    InstallKind = "admin"
)

type InstallOptions struct {
	ThemesDir string
	URL       string
	Name      string
	Kind      InstallKind
}

func Install(opts InstallOptions) (any, error) {
	repoURL, err := validateInstallURL(opts.URL)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(repoURL) == "" {
		return nil, fmt.Errorf("theme URL cannot be empty")
	}

	themesDir := strings.TrimSpace(opts.ThemesDir)
	if themesDir == "" {
		return nil, fmt.Errorf("themes directory cannot be empty")
	}

	kind := opts.Kind
	if kind == "" {
		kind = InstallKindFrontend
	}
	if kind != InstallKindFrontend && kind != InstallKindAdmin {
		return nil, fmt.Errorf("unsupported theme kind %q", kind)
	}

	name := strings.TrimSpace(opts.Name)
	if name == "" {
		name, err = inferThemeName(repoURL)
		if err != nil {
			return nil, err
		}
	}
	name, err = validateThemeInstallName(name)
	if err != nil {
		return nil, err
	}

	targetRoot := themesDir
	if kind == InstallKindAdmin {
		targetRoot = filepath.Join(themesDir, "admin-themes")
	}
	targetDir := filepath.Join(targetRoot, name)
	if _, err := os.Stat(targetDir); err == nil {
		return nil, fmt.Errorf("theme directory already exists: %s", targetDir)
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	if err := os.MkdirAll(targetRoot, 0o755); err != nil {
		return nil, fmt.Errorf("create themes dir: %w", err)
	}

	cloneCtx, cancel := context.WithTimeout(context.Background(), themeCloneTimeout)
	defer cancel()
	cmd := exec.CommandContext(cloneCtx, "git", "clone", repoURL, targetDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Println("git clone failed because git not available, downloading repository archive instead")
		if err := downloadAndExtractTheme(repoURL, targetDir); err != nil {
			return nil, fmt.Errorf("git clone failed and zip fallback failed: %w", err)
		}
	}
	if err := stripThemeVCSMetadata(targetDir); err != nil {
		_ = os.RemoveAll(targetDir)
		return nil, err
	}

	if kind == InstallKindAdmin {
		meta, err := adminui.LoadManifest(themesDir, name)
		if err != nil {
			_ = os.RemoveAll(targetDir)
			return nil, err
		}
		if strings.TrimSpace(meta.Name) != "" && meta.Name != name {
			_ = os.RemoveAll(targetDir)
			return nil, fmt.Errorf("admin theme manifest name %q does not match install directory %q", meta.Name, name)
		}
		return meta, nil
	}

	meta, err := LoadManifest(themesDir, name)
	if err != nil {
		_ = os.RemoveAll(targetDir)
		return nil, err
	}
	if strings.TrimSpace(meta.Name) != "" && meta.Name != name {
		_ = os.RemoveAll(targetDir)
		return nil, fmt.Errorf("theme manifest name %q does not match install directory %q", meta.Name, name)
	}
	return meta, nil
}

func normalizeInstallURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "git@") || strings.HasPrefix(raw, "ssh://") {
		return raw
	}
	if strings.Contains(raw, "://") {
		u, err := url.Parse(raw)
		if err == nil && strings.EqualFold(u.Host, "github.com") {
			u.Path = strings.TrimSuffix(u.Path, "/")
			if !strings.HasSuffix(u.Path, ".git") {
				u.Path += ".git"
			}
			return u.String()
		}
		return raw
	}
	if strings.Count(raw, "/") == 1 {
		return "https://github.com/" + raw + ".git"
	}
	return raw
}

func validateInstallURL(raw string) (string, error) {
	normalized := normalizeInstallURL(raw)
	if normalized == "" {
		return "", nil
	}
	if strings.HasPrefix(normalized, "git@github.com:") {
		return normalized, nil
	}

	u, err := url.Parse(normalized)
	if err != nil {
		return "", fmt.Errorf("invalid theme install URL: %w", err)
	}
	if !strings.EqualFold(u.Scheme, "https") {
		return "", fmt.Errorf("theme install URL must use https")
	}
	if !strings.EqualFold(u.Host, "github.com") {
		return "", fmt.Errorf("theme install currently supports GitHub only")
	}
	path := strings.Trim(strings.TrimSuffix(u.Path, ".git"), "/")
	if strings.Count(path, "/") != 1 {
		return "", fmt.Errorf("theme install URL must point to a GitHub owner/repo")
	}
	return normalized, nil
}

func inferThemeName(repoURL string) (string, error) {
	raw := strings.TrimSpace(repoURL)
	if raw == "" {
		return "", fmt.Errorf("theme install URL cannot be empty")
	}
	if strings.HasPrefix(raw, "git@github.com:") {
		raw = strings.TrimPrefix(raw, "git@github.com:")
		raw = strings.TrimSuffix(raw, ".git")
		parts := strings.Split(strings.Trim(raw, "/"), "/")
		if len(parts) != 2 || strings.TrimSpace(parts[1]) == "" {
			return "", fmt.Errorf("could not infer theme name from repository")
		}
		return validateThemeInstallName(parts[1])
	}

	u, err := url.Parse(normalizeInstallURL(raw))
	if err != nil {
		return "", err
	}
	path := strings.Trim(strings.TrimSuffix(u.Path, ".git"), "/")
	parts := strings.Split(path, "/")
	if len(parts) != 2 || strings.TrimSpace(parts[1]) == "" {
		return "", fmt.Errorf("could not infer theme name from repository")
	}
	return validateThemeInstallName(parts[1])
}

func validateThemeInstallName(name string) (string, error) {
	return safepath.ValidatePathComponent("theme name", name)
}

func repoZipURL(repoURL string) (string, error) {
	if strings.HasPrefix(repoURL, "git@github.com:") {
		path := strings.TrimPrefix(repoURL, "git@github.com:")
		path = strings.Trim(strings.TrimSuffix(path, ".git"), "/")
		if strings.Count(path, "/") != 1 {
			return "", fmt.Errorf("zip fallback requires a GitHub owner/repo path")
		}
		return fmt.Sprintf("https://github.com/%s/archive/refs/heads/main.zip", path), nil
	}
	u, err := url.Parse(repoURL)
	if err != nil {
		return "", err
	}
	if !strings.EqualFold(u.Host, "github.com") {
		return "", fmt.Errorf("zip fallback currently supports GitHub only")
	}
	path := strings.Trim(strings.TrimSuffix(u.Path, ".git"), "/")
	return fmt.Sprintf("https://github.com/%s/archive/refs/heads/main.zip", path), nil
}

func downloadAndExtractTheme(repoURL, targetDir string) error {
	zipURL, err := repoZipURL(repoURL)
	if err != nil {
		return err
	}
	resp, err := themeDownloadClient.Get(zipURL)
	if err != nil {
		return fmt.Errorf("download theme zip: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: %s", resp.Status)
	}

	tmpFile, err := os.CreateTemp("", "foundry-theme-*.zip")
	if err != nil {
		return err
	}
	defer os.Remove(tmpFile.Name())

	written, err := io.Copy(tmpFile, io.LimitReader(resp.Body, themeZipMaxBytes+1))
	if err != nil {
		return err
	}
	if written > themeZipMaxBytes {
		return fmt.Errorf("theme zip exceeds %d bytes", themeZipMaxBytes)
	}
	tmpFile.Close()

	zr, err := zip.OpenReader(tmpFile.Name())
	if err != nil {
		return err
	}
	defer zr.Close()

	tempDir, err := os.MkdirTemp("", "foundry-theme")
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
		if err := os.MkdirAll(filepath.Dir(fp), 0o755); err != nil {
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

func stripThemeVCSMetadata(targetDir string) error {
	for _, rel := range []string{".git", ".gitmodules"} {
		path := filepath.Join(targetDir, rel)
		if err := os.RemoveAll(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove VCS metadata %q: %w", rel, err)
		}
	}
	return nil
}

func safeArchivePath(root, name string) (string, error) {
	if strings.TrimSpace(name) == "" {
		return "", fmt.Errorf("zip contains empty entry name")
	}
	clean := filepath.Clean(name)
	if clean == "." || clean == string(filepath.Separator) {
		return "", fmt.Errorf("zip contains invalid entry name: %s", name)
	}
	target := filepath.Join(root, clean)
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return "", err
	}
	if strings.HasPrefix(rel, "..") || rel == "." && clean != "." {
		return "", fmt.Errorf("zip entry escapes target directory: %s", name)
	}
	return target, nil
}
