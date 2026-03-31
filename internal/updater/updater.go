package updater

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	versioncmd "github.com/sphireinc/foundry/internal/commands/version"
	"github.com/sphireinc/foundry/internal/logx"
	"github.com/sphireinc/foundry/internal/standalone"
)

const (
	defaultRepo    = "sphireinc/foundry"
	defaultAPIBase = "https://api.github.com"
)

type InstallMode string

const (
	ModeStandalone InstallMode = "standalone"
	ModeDocker     InstallMode = "docker"
	ModeSource     InstallMode = "source"
	ModeBinary     InstallMode = "binary"
	ModeUnknown    InstallMode = "unknown"
)

type ReleaseInfo struct {
	Repo             string      `json:"repo"`
	CurrentVersion   string      `json:"current_version"`
	LatestVersion    string      `json:"latest_version"`
	HasUpdate        bool        `json:"has_update"`
	InstallMode      InstallMode `json:"install_mode"`
	ApplySupported   bool        `json:"apply_supported"`
	ReleaseURL       string      `json:"release_url"`
	PublishedAt      time.Time   `json:"published_at"`
	Body             string      `json:"body,omitempty"`
	AssetName        string      `json:"asset_name,omitempty"`
	AssetURL         string      `json:"asset_url,omitempty"`
	ChecksumAssetURL string      `json:"checksum_asset_url,omitempty"`
	Instructions     string      `json:"instructions,omitempty"`
}

type githubRelease struct {
	TagName     string        `json:"tag_name"`
	HTMLURL     string        `json:"html_url"`
	Body        string        `json:"body"`
	PublishedAt string        `json:"published_at"`
	Assets      []githubAsset `json:"assets"`
	Draft       bool          `json:"draft"`
	Prerelease  bool          `json:"prerelease"`
}

type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

func Check(ctx context.Context, projectDir string) (*ReleaseInfo, error) {
	repo := strings.TrimSpace(os.Getenv("FOUNDRY_UPDATE_REPO"))
	if repo == "" {
		repo = defaultRepo
	}
	apiBase := strings.TrimSpace(os.Getenv("FOUNDRY_UPDATE_API_BASE"))
	if apiBase == "" {
		apiBase = defaultAPIBase
	}
	mode := DetectInstallMode(projectDir)
	current := normalizeVersion(versioncmd.Version)
	logx.Info("updater release check started", "project_dir", projectDir, "repo", repo, "install_mode", mode, "current_version", current)
	info := &ReleaseInfo{
		Repo:           repo,
		CurrentVersion: current,
		InstallMode:    mode,
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(apiBase, "/")+"/repos/"+repo+"/releases/latest", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "foundry-updater")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("release lookup failed: %s", strings.TrimSpace(string(body)))
	}
	var rel githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, err
	}
	latest := normalizeVersion(rel.TagName)
	info.LatestVersion = latest
	info.ReleaseURL = rel.HTMLURL
	info.Body = rel.Body
	if ts, err := time.Parse(time.RFC3339, rel.PublishedAt); err == nil {
		info.PublishedAt = ts
	}
	info.HasUpdate = compareVersions(latest, current) > 0
	asset, checksum := selectAssets(rel.Assets)
	if asset != nil {
		info.AssetName = asset.Name
		info.AssetURL = asset.BrowserDownloadURL
	}
	if checksum != nil {
		info.ChecksumAssetURL = checksum.BrowserDownloadURL
	}
	info.ApplySupported = info.HasUpdate && mode == ModeStandalone && info.AssetURL != ""
	info.Instructions = instructionsForMode(mode)
	logx.Info("updater release check completed", "project_dir", projectDir, "latest_version", latest, "has_update", info.HasUpdate, "install_mode", mode, "apply_supported", info.ApplySupported, "asset_name", info.AssetName)
	return info, nil
}

func DetectInstallMode(projectDir string) InstallMode {
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return ModeDocker
	}
	exe, err := os.Executable()
	if err != nil {
		return ModeUnknown
	}
	cleanExe := filepath.Clean(exe)
	tmp := filepath.Clean(os.TempDir())
	if strings.Contains(cleanExe, string(filepath.Separator)+"go-build"+string(filepath.Separator)) ||
		strings.HasPrefix(cleanExe, tmp+string(filepath.Separator)) {
		return ModeSource
	}
	if state, running, err := standalone.RunningState(projectDir); err == nil && state != nil && running {
		return ModeStandalone
	}
	return ModeBinary
}

func ScheduleApply(ctx context.Context, projectDir string) (*ReleaseInfo, error) {
	logx.Info("updater schedule apply started", "project_dir", projectDir)
	info, err := Check(ctx, projectDir)
	if err != nil {
		return nil, err
	}
	if !info.ApplySupported {
		logx.Info("updater schedule apply rejected", "project_dir", projectDir, "install_mode", info.InstallMode, "has_update", info.HasUpdate, "asset_name", info.AssetName)
		return nil, fmt.Errorf("self-update is not supported for install mode %q", info.InstallMode)
	}
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	state, running, err := standalone.RunningState(projectDir)
	if err != nil {
		return nil, err
	}
	if state == nil || !running {
		logx.Info("updater schedule apply rejected", "project_dir", projectDir, "reason", "standalone runtime is not running")
		return nil, fmt.Errorf("standalone runtime is not running")
	}
	newBinaryPath, err := downloadReleaseBinary(ctx, projectDir, info)
	if err != nil {
		return nil, err
	}
	logx.Info("updater binary downloaded", "project_dir", projectDir, "binary_path", newBinaryPath, "asset_name", info.AssetName)
	if err := StartHelper(projectDir, state.PID, exe, newBinaryPath); err != nil {
		return nil, err
	}
	logx.Info("updater helper started", "project_dir", projectDir, "target_pid", state.PID, "asset_name", info.AssetName)
	return info, nil
}

func StartHelper(projectDir string, pid int, targetExe, sourceBinary string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	logx.Info("updater starting helper process", "project_dir", projectDir, "target_pid", pid, "target_executable", targetExe, "source_binary", sourceBinary)
	cmd := execCommand(exe, "__update-helper",
		"--pid="+strconv.Itoa(pid),
		"--project-dir="+projectDir,
		"--target="+targetExe,
		"--source="+sourceBinary,
	)
	cmd.Dir = projectDir
	detachCommand(cmd)
	return cmd.Start()
}

func RunHelper(projectDir, targetExe, sourceBinary string, pid int) error {
	logx.Info("updater helper running", "project_dir", projectDir, "target_pid", pid, "target_executable", targetExe, "source_binary", sourceBinary)
	if pid > 0 {
		_ = terminatePID(pid)
		_ = waitForExit(pid, 10*time.Second)
	}
	info, err := os.Stat(sourceBinary)
	if err != nil {
		return err
	}
	backupPath := targetExe + ".bak"
	_ = os.Remove(backupPath)
	if _, err := os.Stat(targetExe); err == nil {
		if err := os.Rename(targetExe, backupPath); err != nil {
			return err
		}
	}
	if err := os.Rename(sourceBinary, targetExe); err != nil {
		_ = os.Rename(backupPath, targetExe)
		return err
	}
	if err := os.Chmod(targetExe, info.Mode()|0o111); err != nil {
		return err
	}
	logx.Info("updater helper swapped binary", "project_dir", projectDir, "target_executable", targetExe, "backup_path", backupPath)
	cmd := execCommand(targetExe, "restart")
	cmd.Dir = projectDir
	logx.Info("updater helper restarting target", "project_dir", projectDir, "target_executable", targetExe)
	return cmd.Start()
}

func instructionsForMode(mode InstallMode) string {
	switch mode {
	case ModeDocker:
		return "Docker install detected. Pull the new image and recreate the container instead of in-place self-update."
	case ModeSource:
		return "Source install detected. Pull the repo, rebuild Foundry, and restart the process."
	case ModeBinary:
		return "Binary install detected. Use a standalone managed runtime for in-place self-update support."
	case ModeStandalone:
		return "Standalone managed runtime detected. In-place self-update is available."
	default:
		return "Install mode could not be determined."
	}
}

func downloadReleaseBinary(ctx context.Context, projectDir string, info *ReleaseInfo) (string, error) {
	if info == nil || strings.TrimSpace(info.AssetURL) == "" {
		return "", fmt.Errorf("release asset URL is missing")
	}
	runDir, err := standalone.EnsureRunDir(projectDir)
	if err != nil {
		return "", err
	}
	tmpPath := filepath.Join(runDir.RunDir, "foundry-update-"+time.Now().UTC().Format("20060102-150405")+".bin")
	body, err := downloadBytes(ctx, info.AssetURL)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(info.ChecksumAssetURL) != "" {
		checksumBody, err := downloadBytes(ctx, info.ChecksumAssetURL)
		if err == nil {
			if err := verifyChecksum(body, info.AssetName, checksumBody); err != nil {
				return "", err
			}
		}
	}
	bin, err := extractExecutable(body, info.AssetName)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(tmpPath, bin, 0o755); err != nil {
		return "", err
	}
	return tmpPath, nil
}

func selectAssets(assets []githubAsset) (*githubAsset, *githubAsset) {
	var best *githubAsset
	for i := range assets {
		name := assets[i].Name
		lower := strings.ToLower(name)
		if strings.Contains(lower, runtime.GOOS) && strings.Contains(lower, runtime.GOARCH) {
			best = &assets[i]
			break
		}
	}
	if best == nil {
		for i := range assets {
			name := strings.ToLower(assets[i].Name)
			if name == "foundry" || name == "foundry.exe" {
				best = &assets[i]
				break
			}
		}
	}
	var checksum *githubAsset
	if best != nil {
		for i := range assets {
			if assets[i].Name == best.Name+".sha256" {
				checksum = &assets[i]
				break
			}
		}
	}
	if checksum == nil {
		for i := range assets {
			if strings.HasSuffix(strings.ToLower(assets[i].Name), ".sha256") {
				checksum = &assets[i]
				break
			}
		}
	}
	return best, checksum
}

func normalizeVersion(v string) string {
	return strings.TrimPrefix(strings.TrimSpace(v), "v")
}

func compareVersions(a, b string) int {
	parse := func(v string) []int {
		parts := strings.Split(normalizeVersion(v), ".")
		out := make([]int, 0, len(parts))
		for _, part := range parts {
			n, _ := strconv.Atoi(strings.TrimLeftFunc(part, func(r rune) bool { return r < '0' || r > '9' }))
			out = append(out, n)
		}
		return out
	}
	left := parse(a)
	right := parse(b)
	max := len(left)
	if len(right) > max {
		max = len(right)
	}
	for i := 0; i < max; i++ {
		var lv, rv int
		if i < len(left) {
			lv = left[i]
		}
		if i < len(right) {
			rv = right[i]
		}
		if lv < rv {
			return -1
		}
		if lv > rv {
			return 1
		}
	}
	return 0
}

func downloadBytes(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "foundry-updater")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("download failed: %s", strings.TrimSpace(string(body)))
	}
	return io.ReadAll(resp.Body)
}

func verifyChecksum(body []byte, assetName string, checksumBody []byte) error {
	sum := sha256.Sum256(body)
	actual := hex.EncodeToString(sum[:])
	lines := strings.Split(string(checksumBody), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if len(fields) == 1 || strings.TrimSpace(fields[len(fields)-1]) == assetName {
			if fields[0] == actual {
				return nil
			}
		}
	}
	return fmt.Errorf("checksum verification failed for %s", assetName)
}

func extractExecutable(body []byte, assetName string) ([]byte, error) {
	lower := strings.ToLower(assetName)
	switch {
	case strings.HasSuffix(lower, ".zip"):
		reader, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
		if err != nil {
			return nil, err
		}
		for _, file := range reader.File {
			if file.FileInfo().IsDir() {
				continue
			}
			base := filepath.Base(file.Name)
			if base == "foundry" || base == "foundry.exe" {
				rc, err := file.Open()
				if err != nil {
					return nil, err
				}
				defer rc.Close()
				return io.ReadAll(rc)
			}
		}
		return nil, fmt.Errorf("no foundry executable in zip asset")
	case strings.HasSuffix(lower, ".tar.gz"), strings.HasSuffix(lower, ".tgz"):
		gz, err := gzip.NewReader(bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		defer gz.Close()
		tr := tar.NewReader(gz)
		for {
			hdr, err := tr.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, err
			}
			base := filepath.Base(hdr.Name)
			if base == "foundry" || base == "foundry.exe" {
				return io.ReadAll(tr)
			}
		}
		return nil, fmt.Errorf("no foundry executable in tar.gz asset")
	default:
		return body, nil
	}
}
