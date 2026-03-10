package plugins

import (
	"fmt"
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

func ValidateEnabledPlugins(pluginsDir string, enabled []string) []ValidationIssue {
	issues := make([]ValidationIssue, 0)

	metadata, err := LoadAllMetadata(pluginsDir, enabled)
	if err != nil {
		issues = append(issues, ValidationIssue{
			Name:   "*",
			Status: "metadata load failed",
			Err:    err,
		})
		return issues
	}

	for _, name := range enabled {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}

		if err := validatePluginForSync(pluginsDir, name); err != nil {
			issues = append(issues, ValidationIssue{
				Name:   name,
				Status: "invalid",
				Err:    err,
			})
		}
	}

	if err := validateDependencies(metadata); err != nil {
		issues = append(issues, ValidationIssue{
			Name:   "*",
			Status: "dependency validation failed",
			Err:    err,
		})
	}

	return issues
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
		return "metadata error"
	}

	if err := validatePluginForSync(pluginsDir, name); err != nil {
		msg := err.Error()

		switch {
		case strings.Contains(msg, "does not exist"):
			return "not installed"
		case strings.Contains(msg, "read "+name) || strings.Contains(msg, "parse "):
			return "metadata error"
		case strings.Contains(msg, "metadata name"):
			return "metadata invalid"
		case strings.Contains(msg, "invalid repo"):
			return "metadata invalid"
		case strings.Contains(msg, "invalid requires"):
			return "metadata invalid"
		case strings.Contains(msg, "has no .go files"):
			return "code missing"
		default:
			return "invalid"
		}
	}

	if strings.TrimSpace(meta.Repo) == "" {
		return "enabled"
	}

	return "enabled"
}
