package theme

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	adminui "github.com/sphireinc/foundry/internal/admin/ui"
	"github.com/sphireinc/foundry/internal/installutil"
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
	return installutil.NormalizeGitHubInstallURL(raw)
}

func validateInstallURL(raw string) (string, error) {
	return installutil.ValidateGitHubInstallURL("theme", raw, validateThemeInstallName)
}

func inferThemeName(repoURL string) (string, error) {
	return installutil.InferRepoName(repoURL, "theme", validateThemeInstallName)
}

func validateThemeInstallName(name string) (string, error) {
	return safepath.ValidatePathComponent("theme name", name)
}

func downloadAndExtractTheme(repoURL, targetDir string) error {
	return installutil.DownloadAndExtractRepoArchive(
		themeDownloadClient,
		repoURL,
		targetDir,
		"foundry-theme",
		"theme",
		themeZipMaxBytes,
	)
}

func stripThemeVCSMetadata(targetDir string) error {
	return installutil.StripVCSMetadata(targetDir)
}
