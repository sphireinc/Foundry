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
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return Docker
	}
	exe, err := os.Executable()
	if err != nil {
		return Unknown
	}
	cleanExe := filepath.Clean(exe)
	tmp := filepath.Clean(os.TempDir())
	if strings.Contains(cleanExe, string(filepath.Separator)+"go-build"+string(filepath.Separator)) ||
		strings.HasPrefix(cleanExe, tmp+string(filepath.Separator)) {
		return Source
	}
	if state, running, err := standalone.RunningState(projectDir); err == nil && state != nil && running {
		return Standalone
	}
	return Binary
}
