package version

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"runtime/debug"
	"strings"
	"time"

	"github.com/sphireinc/foundry/internal/installmode"
)

type Metadata struct {
	Version       string `json:"version"`
	Commit        string `json:"commit"`
	BuiltAt       string `json:"built_at"`
	GoVersion     string `json:"go_version"`
	GOOS          string `json:"goos"`
	GOARCH        string `json:"goarch"`
	Executable    string `json:"executable"`
	InstallMode   string `json:"install_mode"`
	VCSRevision   string `json:"vcs_revision,omitempty"`
	VCSTime       string `json:"vcs_time,omitempty"`
	VCSModified   bool   `json:"vcs_modified"`
	ModuleVersion string `json:"module_version,omitempty"`
}

func Current(projectDir string) Metadata {
	meta := Metadata{
		Version:     normalizeValue(Version, "dev"),
		Commit:      normalizeValue(Commit, "none"),
		BuiltAt:     normalizeValue(Date, "unknown"),
		GoVersion:   runtime.Version(),
		GOOS:        runtime.GOOS,
		GOARCH:      runtime.GOARCH,
		InstallMode: string(installmode.Detect(projectDir)),
	}
	if exe, err := os.Executable(); err == nil {
		meta.Executable = exe
	}

	if info, ok := debug.ReadBuildInfo(); ok && info != nil {
		if info.GoVersion != "" {
			meta.GoVersion = info.GoVersion
		}
		if info.Main.Version != "" && info.Main.Version != "(devel)" {
			meta.ModuleVersion = info.Main.Version
			if meta.Version == "" {
				meta.Version = info.Main.Version
			}
		}
		for _, setting := range info.Settings {
			switch setting.Key {
			case "vcs.revision":
				meta.VCSRevision = setting.Value
				if meta.Commit == "" {
					meta.Commit = shortRevision(setting.Value)
				}
			case "vcs.time":
				meta.VCSTime = setting.Value
				if meta.BuiltAt == "" {
					meta.BuiltAt = setting.Value
				}
			case "vcs.modified":
				meta.VCSModified = setting.Value == "true"
			}
		}
	}

	if meta.Version == "" {
		meta.Version = gitVersion(projectDir)
	}
	if meta.Commit == "" {
		meta.Commit = gitCommit(projectDir)
	}
	if meta.BuiltAt == "" {
		meta.BuiltAt = gitCommitTime(projectDir)
	}

	if meta.Version == "" {
		meta.Version = "dev"
	}
	if meta.Commit == "" {
		meta.Commit = "unknown"
	}
	if meta.BuiltAt == "" {
		meta.BuiltAt = "unknown"
	}
	return meta
}

func (m Metadata) String() string {
	lines := []string{
		fmt.Sprintf("Foundry %s", m.Version),
		fmt.Sprintf("Commit: %s", m.Commit),
		fmt.Sprintf("Built: %s", m.BuiltAt),
		fmt.Sprintf("Go: %s", m.GoVersion),
		fmt.Sprintf("Target: %s/%s", m.GOOS, m.GOARCH),
		fmt.Sprintf("Install mode: %s", m.InstallMode),
	}
	if m.Executable != "" {
		lines = append(lines, fmt.Sprintf("Executable: %s", m.Executable))
	}
	if m.VCSRevision != "" && shortRevision(m.VCSRevision) != m.Commit {
		lines = append(lines, fmt.Sprintf("VCS revision: %s", m.VCSRevision))
	}
	if m.VCSTime != "" && m.VCSTime != m.BuiltAt {
		lines = append(lines, fmt.Sprintf("VCS time: %s", m.VCSTime))
	}
	if m.VCSModified {
		lines = append(lines, "VCS modified: true")
	}
	if m.ModuleVersion != "" && m.ModuleVersion != m.Version {
		lines = append(lines, fmt.Sprintf("Module version: %s", m.ModuleVersion))
	}
	return strings.Join(lines, "\n")
}

func (m Metadata) ShortString() string {
	return fmt.Sprintf("Foundry %s (%s)", m.Version, m.Commit)
}

func (m Metadata) JSON() string {
	body, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return "{}"
	}
	return string(body)
}

func normalizeValue(v, placeholder string) string {
	v = strings.TrimSpace(v)
	if v == "" || v == placeholder {
		return ""
	}
	return v
}

func shortRevision(v string) string {
	v = strings.TrimSpace(v)
	if len(v) > 12 {
		return v[:12]
	}
	return v
}

func gitVersion(projectDir string) string {
	return gitOutput(projectDir, "describe", "--tags", "--abbrev=0")
}

func gitCommit(projectDir string) string {
	return gitOutput(projectDir, "rev-parse", "--short", "HEAD")
}

func gitCommitTime(projectDir string) string {
	value := gitOutput(projectDir, "show", "-s", "--format=%cI", "HEAD")
	if value == "" {
		return ""
	}
	if _, err := time.Parse(time.RFC3339, value); err != nil {
		return ""
	}
	return value
}

func gitOutput(projectDir string, args ...string) string {
	if strings.TrimSpace(projectDir) == "" {
		return ""
	}
	cmd := exec.Command("git", args...)
	cmd.Dir = projectDir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
