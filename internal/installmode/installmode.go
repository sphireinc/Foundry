package installmode

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/sphireinc/foundry/internal/standalone"
)

type Mode string

const (
	Standalone Mode = "standalone"
	Docker     Mode = "docker"
	Source     Mode = "source"
	Binary     Mode = "binary"
	Unknown    Mode = "unknown"
)

func Detect(projectDir string) Mode {
	if strings.TrimSpace(os.Getenv("FOUNDRY_CONTAINER")) == "true" {
		return Docker
	}
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return Docker
	}
	exe, err := os.Executable()
	if err != nil {
		return Unknown
	}
	cleanExe := filepath.Clean(exe)
	// Go run/test executables live in go-build*/b*/; a release binary copied
	// under /tmp is still a binary installation.
	buildDir := filepath.Base(filepath.Dir(filepath.Dir(cleanExe)))
	if strings.HasPrefix(buildDir, "go-build") {
		return Source
	}
	if state, running, err := standalone.RunningState(projectDir); err == nil && state != nil && running {
		return Standalone
	}
	return Binary
}
