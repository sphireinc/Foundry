package theme

import (
	"fmt"
	"os"
	"path/filepath"
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
	return filepath.Join(m.root, m.activeTheme, "layouts", name+".html")
}

func (m *Manager) MustExist() error {
	themeDir := filepath.Join(m.root, m.activeTheme)
	info, err := os.Stat(themeDir)
	if err != nil {
		return fmt.Errorf("active theme not found: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("theme path is not a directory: %s", themeDir)
	}
	return nil
}
