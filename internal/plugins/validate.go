package plugins

import (
	"fmt"
	"sort"
	"strings"
)

type ValidationIssue struct {
	Name   string
	Status string
	Err    error
}

func (v ValidationIssue) String() string {
	if v.Err == nil {
		return fmt.Sprintf("%s: %s", v.Name, v.Status)
	}
	return fmt.Sprintf("%s: %s: %v", v.Name, v.Status, v.Err)
}

type PluginValidationReport struct {
	Passed []string
	Issues []ValidationIssue
}

func ValidateEnabledPlugins(pluginsDir string, enabled []string) PluginValidationReport {
	report := PluginValidationReport{
		Passed: make([]string, 0),
		Issues: make([]ValidationIssue, 0),
	}

	normalized := make([]string, 0, len(enabled))
	for _, name := range enabled {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		normalized = append(normalized, name)
	}

	sort.Strings(normalized)

	metadata, err := LoadAllMetadata(pluginsDir, normalized)
	if err != nil {
		report.Issues = append(report.Issues, ValidationIssue{
			Name:   "*",
			Status: "metadata load failed",
			Err:    err,
		})
		return report
	}

	for _, name := range normalized {
		if err := ValidateInstalledPlugin(pluginsDir, name); err != nil {
			report.Issues = append(report.Issues, ValidationIssue{
				Name:   name,
				Status: "invalid",
				Err:    err,
			})
			continue
		}

		report.Passed = append(report.Passed, name)
	}

	if err := validateDependencies(metadata); err != nil {
		report.Issues = append(report.Issues, ValidationIssue{
			Name:   "*",
			Status: "dependency validation failed",
			Err:    err,
		})
	}

	return report
}

func EnabledPluginStatus(pluginsDir string, enabled []string) map[string]string {
	statuses := make(map[string]string, len(enabled))

	for _, name := range enabled {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		statuses[name] = enabledPluginStatus(pluginsDir, name)
	}

	return statuses
}

func enabledPluginStatus(pluginsDir, name string) string {
	if strings.TrimSpace(name) == "" {
		return "invalid name"
	}

	meta, err := LoadMetadata(pluginsDir, name)
	if err != nil {
		msg := err.Error()

		switch {
		case strings.Contains(msg, "read ") && strings.Contains(msg, "plugin.yaml"):
			return "metadata missing"
		case strings.Contains(msg, "missing required field \"foundry_api\""):
			return "api missing"
		case strings.Contains(msg, "unsupported foundry_api"):
			return "api unsupported"
		case strings.Contains(msg, "missing required field \"min_foundry_version\""):
			return "version missing"
		default:
			return "metadata error"
		}
	}

	if err := ValidateInstalledPlugin(pluginsDir, name); err != nil {
		msg := err.Error()

		switch {
		case strings.Contains(msg, "does not exist"):
			return "not installed"
		case strings.Contains(msg, "metadata name"):
			return "metadata invalid"
		case strings.Contains(msg, "invalid repo"):
			return "metadata invalid"
		case strings.Contains(msg, "invalid requires"):
			return "metadata invalid"
		case strings.Contains(msg, "has no .go files"):
			return "code missing"
		case strings.Contains(msg, "security validation failed"):
			return "security mismatch"
		default:
			return "invalid"
		}
	}

	if strings.TrimSpace(meta.Repo) == "" {
		return "enabled"
	}

	return "enabled"
}
