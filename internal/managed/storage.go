package managed

import (
	"fmt"
	"os"
	"strings"

	"github.com/sphireinc/foundry/internal/config"
)

type StorageLayoutEntry struct {
	Name      string `json:"name"`
	ConfigKey string `json:"config_key"`
	Writable  bool   `json:"writable"`
}

type StorageLayoutCheck struct {
	Name    string
	Status  string
	Message string
}

func RuntimeStorageLayout() []StorageLayoutEntry {
	return []StorageLayoutEntry{
		{Name: "content", ConfigKey: "content_dir", Writable: true},
		{Name: "data", ConfigKey: "data_dir", Writable: true},
		{Name: "public", ConfigKey: "public_dir", Writable: true},
		{Name: "themes", ConfigKey: "themes_dir", Writable: true},
		{Name: "plugins", ConfigKey: "plugins_dir", Writable: true},
	}
}

func CheckStorageLayout(cfg *config.Config) []StorageLayoutCheck {
	if cfg == nil {
		return []StorageLayoutCheck{{Name: "storage.config", Status: HealthCheckFail, Message: "configuration is unavailable"}}
	}
	targets := map[string]string{
		"content": cfg.ContentDir,
		"data":    cfg.DataDir,
		"public":  cfg.PublicDir,
		"themes":  cfg.ThemesDir,
		"plugins": cfg.PluginsDir,
	}
	checks := make([]StorageLayoutCheck, 0, len(targets))
	for _, entry := range RuntimeStorageLayout() {
		name := "storage." + entry.Name
		if err := checkStorageReadWrite(targets[entry.Name]); err != nil {
			checks = append(checks, StorageLayoutCheck{Name: name, Status: HealthCheckFail, Message: err.Error()})
			continue
		}
		checks = append(checks, StorageLayoutCheck{Name: name, Status: HealthCheckPass, Message: "readable and writable"})
	}
	return checks
}

func checkStorageReadWrite(dir string) error {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return fmt.Errorf("directory is not configured")
	}
	info, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("directory is not available")
	}
	if !info.IsDir() {
		return fmt.Errorf("target is not a directory")
	}
	if _, err := os.ReadDir(dir); err != nil {
		return fmt.Errorf("directory is not readable")
	}
	tmp, err := os.CreateTemp(dir, ".foundry-health-*")
	if err != nil {
		return fmt.Errorf("directory is not writable")
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write([]byte("ok")); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("directory is not writable")
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("directory is not writable")
	}
	if err := os.Remove(tmpName); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("temporary health file cleanup failed")
	}
	return nil
}
