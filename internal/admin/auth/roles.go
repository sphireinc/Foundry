package auth

import "strings"

const capabilityAll = "*"

var roleCapabilities = map[string][]string{
	"admin": {
		capabilityAll,
	},
	"editor": {
		"dashboard.read",
		"documents.read",
		"documents.create",
		"documents.write",
		"documents.review",
		"documents.history",
		"documents.lifecycle",
		"documents.diff",
		"media.read",
		"media.write",
		"media.lifecycle",
		"audit.read",
	},
	"author": {
		"dashboard.read",
		"documents.read.own",
		"documents.create",
		"documents.write.own",
		"documents.history.own",
		"documents.lifecycle.own",
		"documents.diff.own",
		"media.read",
		"media.write",
	},
	"reviewer": {
		"dashboard.read",
		"documents.read",
		"documents.review",
		"documents.history",
		"documents.diff",
		"media.read",
		"audit.read",
	},
}

func normalizeRole(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "admin":
		return "admin"
	case "editor":
		return "editor"
	case "author":
		return "author"
	case "reviewer":
		return "reviewer"
	default:
		return ""
	}
}

func normalizeCapabilities(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(strings.ToLower(value))
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func capabilitiesFor(role string, custom []string) []string {
	role = normalizeRole(role)
	base := append([]string(nil), roleCapabilities[role]...)
	base = append(base, normalizeCapabilities(custom)...)
	return normalizeCapabilities(base)
}

func capabilityAllowed(actual []string, required string) bool {
	required = strings.TrimSpace(strings.ToLower(required))
	if required == "" {
		return true
	}
	normalized := normalizeCapabilities(actual)
	for _, cap := range normalized {
		if cap == capabilityAll || cap == required {
			return true
		}
		if strings.HasSuffix(required, ".own") {
			if cap == strings.TrimSuffix(required, ".own") {
				return true
			}
			continue
		}
		if cap == required+".own" {
			return true
		}
	}
	return false
}
