package hostservice

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	metadataFile = "service.json"
	logFileName  = "service.log"
	binaryName   = "foundry-service"
)

type Metadata struct {
	Name         string    `json:"name"`
	Label        string    `json:"label"`
	Platform     string    `json:"platform"`
	ProjectDir   string    `json:"project_dir"`
	ServicePath  string    `json:"service_path"`
	Executable   string    `json:"executable"`
	LogPath      string    `json:"log_path"`
	InstalledAt  time.Time `json:"installed_at"`
	InstallScope string    `json:"install_scope"`
}

type Status struct {
	Installed bool
	Running   bool
	Enabled   bool
	Message   string
	Metadata  *Metadata
}

func ProjectRunDir(projectDir string) string {
	return filepath.Join(projectDir, ".foundry", "run")
}

func EnsureRunDir(projectDir string) (string, error) {
	runDir := ProjectRunDir(projectDir)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		return "", err
	}
	return runDir, nil
}

func metadataPath(projectDir string) string {
	return filepath.Join(ProjectRunDir(projectDir), metadataFile)
}

func logPath(projectDir string) string {
	return filepath.Join(ProjectRunDir(projectDir), logFileName)
}

func LoadMetadata(projectDir string) (*Metadata, error) {
	body, err := os.ReadFile(metadataPath(projectDir))
	if err != nil {
		return nil, err
	}
	var meta Metadata
	if err := json.Unmarshal(body, &meta); err != nil {
		return nil, err
	}
	return &meta, nil
}

func SaveMetadata(projectDir string, meta Metadata) error {
	if _, err := EnsureRunDir(projectDir); err != nil {
		return err
	}
	body, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(metadataPath(projectDir), body, 0o644)
}

func RemoveMetadata(projectDir string) error {
	if err := os.Remove(metadataPath(projectDir)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func ServiceName(projectDir string) string {
	base := strings.ToLower(filepath.Base(projectDir))
	base = sanitize(base)
	sum := sha1.Sum([]byte(filepath.Clean(projectDir)))
	return fmt.Sprintf("foundry-%s-%s", base, hex.EncodeToString(sum[:])[:8])
}

func ServiceLabel(projectDir string) string {
	return "io.getfoundry." + ServiceName(projectDir)
}

func sanitize(value string) string {
	var b strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "site"
	}
	return out
}

func shouldUseManagedBinary(executablePath, projectDir string) bool {
	exe := filepath.Clean(executablePath)
	tmp := filepath.Clean(os.TempDir())
	if strings.Contains(exe, string(filepath.Separator)+"go-build"+string(filepath.Separator)) {
		return fileExists(filepath.Join(projectDir, "cmd", "foundry", "main.go"))
	}
	if strings.HasPrefix(exe, tmp+string(filepath.Separator)) && fileExists(filepath.Join(projectDir, "cmd", "foundry", "main.go")) {
		return true
	}
	return false
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func EnsureExecutable(projectDir string) (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if !shouldUseManagedBinary(exe, projectDir) {
		return exe, nil
	}
	return ensureManagedBinary(projectDir)
}

func ensureManagedBinary(projectDir string) (string, error) {
	if _, err := exec.LookPath("go"); err != nil {
		return "", fmt.Errorf("foundry was launched via go run but go is not available in PATH")
	}
	runDir, err := EnsureRunDir(projectDir)
	if err != nil {
		return "", err
	}
	name := binaryName
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	target := filepath.Join(runDir, name)
	cmd := exec.Command("go", "build", "-o", target, "./cmd/foundry")
	cmd.Dir = projectDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Env = append(os.Environ(),
		"CGO_ENABLED=0",
		"GOOS="+runtime.GOOS,
		"GOARCH="+runtime.GOARCH,
	)
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("build managed service binary: %w", err)
	}
	return target, nil
}

func Install(projectDir string) (*Metadata, error) {
	executable, err := EnsureExecutable(projectDir)
	if err != nil {
		return nil, err
	}
	meta, err := install(projectDir, executable)
	if err != nil {
		return nil, err
	}
	if meta.InstalledAt.IsZero() {
		meta.InstalledAt = time.Now().UTC()
	}
	if err := SaveMetadata(projectDir, *meta); err != nil {
		return nil, err
	}
	return meta, nil
}

func Uninstall(projectDir string) error {
	meta, err := LoadMetadata(projectDir)
	if err != nil {
		if os.IsNotExist(err) {
			meta, err = metadataForProject(projectDir)
			if err != nil {
				return err
			}
			if err := uninstall(meta); err != nil {
				return err
			}
			return nil
		}
		return err
	}
	if err := uninstall(meta); err != nil {
		return err
	}
	return RemoveMetadata(projectDir)
}

func Start(projectDir string) error {
	meta, err := resolvedMetadata(projectDir)
	if err != nil {
		return err
	}
	return start(meta)
}

func Stop(projectDir string) error {
	meta, err := resolvedMetadata(projectDir)
	if err != nil {
		return err
	}
	return stop(meta)
}

func Restart(projectDir string) error {
	meta, err := resolvedMetadata(projectDir)
	if err != nil {
		return err
	}
	return restart(meta)
}

func CheckStatus(projectDir string) (*Status, error) {
	meta, err := resolvedMetadata(projectDir)
	if err != nil {
		if os.IsNotExist(err) {
			return status(projectDir, nil)
		}
		return nil, err
	}
	return status(projectDir, meta)
}

func resolvedMetadata(projectDir string) (*Metadata, error) {
	meta, err := LoadMetadata(projectDir)
	if err == nil {
		return meta, nil
	}
	if !os.IsNotExist(err) {
		return nil, err
	}
	return metadataForProject(projectDir)
}

func metadataForProject(projectDir string) (*Metadata, error) {
	servicePath, scope, platform, err := servicePath(projectDir)
	if err != nil {
		return nil, err
	}
	return &Metadata{
		Name:         ServiceName(projectDir),
		Label:        ServiceLabel(projectDir),
		Platform:     platform,
		ProjectDir:   projectDir,
		ServicePath:  servicePath,
		LogPath:      logPath(projectDir),
		InstallScope: scope,
	}, nil
}
