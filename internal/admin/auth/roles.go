package auth

import "strings"

func normalizeRole(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "admin":
		return "admin"
	case "editor":
		return "editor"
	default:
		return ""
	}
}

func roleAllowed(actual, required string) bool {
	required = normalizeRole(required)
	actual = normalizeRole(actual)
	if required == "" {
		return true
	}
	switch required {
	case "editor":
		return actual == "editor" || actual == "admin"
	case "admin":
		return actual == "admin"
	default:
		return false
	}
}
