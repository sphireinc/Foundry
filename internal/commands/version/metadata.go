package version

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/sphireinc/foundry/internal/installmode"
)

type Metadata struct {
	Version        string `json:"version"`
	DisplayVersion string `json:"display_version,omitempty"`
	Commit         string `json:"commit"`
	BuiltAt        string `json:"built_at"`
	GoVersion      string `json:"go_version"`
	GOOS           string `json:"goos"`
	GOARCH         string `json:"goarch"`
	Executable     string `json:"executable"`
	InstallMode    string `json:"install_mode"`
	VCSRevision    string `json:"vcs_revision,omitempty"`
	VCSTime        string `json:"vcs_time,omitempty"`
	VCSModified    bool   `json:"vcs_modified"`
	ModuleVersion  string `json:"module_version,omitempty"`
	NearestTag     string `json:"nearest_tag,omitempty"`
	CommitCount    int    `json:"commit_count,omitempty"`
	Dirty          bool   `json:"dirty,omitempty"`
	ManagedRuntime bool   `json:"managed_runtime"`
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
		ManagedRuntime: envBool("FOUNDRY_MANAGED_RUNTIME") ||
			envBool("FOUNDRY_MANAGED_ENABLED") ||
			envBool("FOUNDRY_CLOUD_MANAGED"),
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

	if meta.NearestTag == "" {
		meta.NearestTag = gitNearestTag(projectDir)
	}
	if meta.Commit == "" {
		meta.Commit = gitCommit(projectDir)
	}
	if meta.BuiltAt == "" {
		meta.BuiltAt = gitCommitTime(projectDir)
	}
	if meta.CommitCount == 0 && meta.NearestTag != "" {
		meta.CommitCount = gitCommitsSinceTag(projectDir, meta.NearestTag)
	}
	if !meta.VCSModified {
		meta.Dirty = gitDirty(projectDir)
		meta.VCSModified = meta.Dirty
	} else {
		meta.Dirty = true
	}

	if meta.Version == "" {
		meta.Version = meta.NearestTag
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
	meta.DisplayVersion = meta.Version
	if meta.InstallMode == string(installmode.Source) {
		meta.DisplayVersion = sourceDisplayVersion(meta)
	}
	return meta
}

func (m Metadata) String() string {
	lines := []string{
		fmt.Sprintf("Foundry %s", firstNonEmpty(m.DisplayVersion, m.Version)),
		fmt.Sprintf("Commit: %s", m.Commit),
		fmt.Sprintf("Built: %s", m.BuiltAt),
		fmt.Sprintf("Go: %s", m.GoVersion),
		fmt.Sprintf("Target: %s/%s", m.GOOS, m.GOARCH),
		fmt.Sprintf("Install mode: %s", m.InstallMode),
		fmt.Sprintf("Managed runtime: %s", boolLabel(m.ManagedRuntime, "enabled", "disabled")),
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
	if m.InstallMode == string(installmode.Source) {
		lines = append(lines, fmt.Sprintf("Nearest tag: %s", firstNonEmpty(m.NearestTag, "unknown")))
		lines = append(lines, fmt.Sprintf("Current commit: %s", firstNonEmpty(m.Commit, "unknown")))
		lines = append(lines, fmt.Sprintf("Local changes: %s", boolLabel(m.Dirty, "dirty", "clean")))
	}
	return strings.Join(lines, "\n")
}

func envBool(name string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(name))) {
	case "1", "t", "true", "y", "yes", "on", "enabled":
		return true
	default:
		return false
	}
}

func (m Metadata) ShortString() string {
	return fmt.Sprintf("Foundry %s (%s)", firstNonEmpty(m.DisplayVersion, m.Version), m.Commit)
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

func gitNearestTag(projectDir string) string {
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

func gitCommitsSinceTag(projectDir, tag string) int {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return 0
	}
	value := gitOutput(projectDir, "rev-list", "--count", tag+"..HEAD")
	if value == "" {
		return 0
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	return n
}

func gitDirty(projectDir string) bool {
	return gitCommandSuccess(projectDir, "diff-index", "--quiet", "HEAD", "--")
}

func gitCommandSuccess(projectDir string, args ...string) bool {
	if strings.TrimSpace(projectDir) == "" {
		return false
	}
	cmd := exec.Command("git", args...)
	cmd.Dir = projectDir
	return cmd.Run() != nil
}

func sourceDisplayVersion(meta Metadata) string {
	base := firstNonEmpty(meta.NearestTag, meta.Version, "dev")
	commit := firstNonEmpty(meta.Commit, shortRevision(meta.VCSRevision))
	suffix := ""
	if meta.CommitCount > 0 && commit != "" {
		suffix = fmt.Sprintf("+%d.g%s", meta.CommitCount, commit)
	} else if commit != "" && (base == "dev" || base == "") {
		suffix = "+" + commit
	}
	if meta.Dirty {
		suffix += "-dirty"
	}
	return base + suffix
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func boolLabel(v bool, yes, no string) string {
	if v {
		return yes
	}
	return no
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
