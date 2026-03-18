package theme

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/sphireinc/foundry/internal/safepath"
)

type Manager struct {
	root        string
	activeTheme string
}

func NewManager(root, activeTheme string) *Manager {
	return &Manager{
		root:        root,
		activeTheme: activeTheme,
	}
}

func (m *Manager) LayoutPath(name string) string {
	themeName, err := safepath.ValidatePathComponent("theme name", m.activeTheme)
	if err != nil {
		return ""
	}
	return filepath.Join(m.root, themeName, "layouts", name+".html")
}

func (m *Manager) MustExist() error {
	themeName, err := safepath.ValidatePathComponent("theme name", m.activeTheme)
	if err != nil {
		return err
	}

	themeDir := filepath.Join(m.root, themeName)
	info, err := os.Stat(themeDir)
	if err != nil {
		return fmt.Errorf("active theme not found: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("theme path is not a directory: %s", themeDir)
	}
	return nil
}
