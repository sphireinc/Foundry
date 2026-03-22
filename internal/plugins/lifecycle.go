package plugins

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

type ValidationDiagnostic struct {
	Severity string
	Path     string
	Message  string
}

type HealthReport struct {
	Healthy     bool
	Status      string
	Diagnostics []ValidationDiagnostic
}

func rollbackRoot(pluginsDir, name string) string {
	return filepath.Join(pluginsDir, ".rollback", name)
}

func listRollbacks(pluginsDir, name string) ([]string, error) {
	root := rollbackRoot(pluginsDir, name)
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	out := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			out = append(out, filepath.Join(root, entry.Name()))
		}
	}
	sort.Strings(out)
	return out, nil
}

func latestRollback(pluginsDir, name string) (string, bool, error) {
	items, err := listRollbacks(pluginsDir, name)
	if err != nil || len(items) == 0 {
		return "", false, err
	}
	return items[len(items)-1], true, nil
}

func HasRollback(pluginsDir, name string) (bool, error) {
	_, ok, err := latestRollback(pluginsDir, name)
	if err != nil {
		return false, err
	}
	return ok, nil
}

func backupInstalled(pluginsDir, name string) (string, error) {
	targetDir := filepath.Join(pluginsDir, name)
	if _, err := os.Stat(targetDir); err != nil {
		return "", err
	}
	root := rollbackRoot(pluginsDir, name)
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", err
	}
	timestamp := time.Now().UTC().Format("20060102T150405.000000000Z")
	backupDir := filepath.Join(root, timestamp)
	for attempt := 0; attempt < 5; attempt++ {
		if _, err := os.Stat(backupDir); os.IsNotExist(err) {
			break
		}
		backupDir = filepath.Join(root, fmt.Sprintf("%s-%d", timestamp, attempt+1))
	}
	if err := os.Rename(targetDir, backupDir); err != nil {
		return "", err
	}
	return backupDir, nil
}

func RollbackInstalled(pluginsDir, name string) (Metadata, error) {
	backupDir, ok, err := latestRollback(pluginsDir, name)
	if err != nil {
		return Metadata{}, err
	}
	if !ok {
		return Metadata{}, fmt.Errorf("plugin %q has no rollback snapshot", name)
	}

	targetDir := filepath.Join(pluginsDir, name)
	if _, err := os.Stat(targetDir); err == nil {
		if _, err := backupInstalled(pluginsDir, name); err != nil {
			return Metadata{}, fmt.Errorf("backup current plugin before rollback: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return Metadata{}, err
	}

	if err := os.Rename(backupDir, targetDir); err != nil {
		return Metadata{}, err
	}
	return LoadMetadata(pluginsDir, name)
}

func DiagnoseInstalled(pluginsDir string, meta Metadata, enabled bool) HealthReport {
	report := HealthReport{Healthy: true, Status: "ok", Diagnostics: make([]ValidationDiagnostic, 0)}
	add := func(severity, path, message string) {
		report.Diagnostics = append(report.Diagnostics, ValidationDiagnostic{
			Severity: severity,
			Path:     filepath.ToSlash(path),
			Message:  message,
		})
		if severity == "error" {
			report.Healthy = false
		}
	}

	if err := validateMetadataCompatibility(meta); err != nil {
		add("error", filepath.Join(meta.Directory, "plugin.yaml"), err.Error())
	}
	if err := validatePluginForSync(pluginsDir, meta.Name); err != nil {
		add("error", meta.Directory, err.Error())
	}
	if meta.CompatibilityVersion == "" {
		add("warn", filepath.Join(meta.Directory, "plugin.yaml"), "compatibility_version is not declared")
	}
	if len(meta.ConfigSchema) == 0 {
		add("warn", filepath.Join(meta.Directory, "plugin.yaml"), "config_schema is empty")
	}
	if len(meta.Screenshots) == 0 {
		add("warn", filepath.Join(meta.Directory, "plugin.yaml"), "screenshots are not declared")
	}
	if enabled && !report.Healthy {
		report.Status = "degraded"
	} else if enabled {
		report.Status = "enabled"
	} else if report.Healthy {
		report.Status = "installed"
	} else {
		report.Status = "invalid"
	}
	return report
}
