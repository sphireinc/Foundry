package i18n

import (
	"strings"
	"unicode"
)

func NormalizeTag(tag string) string {
	tag = strings.TrimSpace(tag)
	tag = strings.ReplaceAll(tag, "_", "-")
	tag = strings.ToLower(tag)
	return tag
}

func IsValidTag(tag string) bool {
	tag = NormalizeTag(tag)
	if tag == "" {
		return false
	}

	parts := strings.Split(tag, "-")
	if len(parts) == 0 {
		return false
	}

	// Primary language subtag: allow 2–3 ASCII letters.
	if !isAlpha(parts[0]) || len(parts[0]) < 2 || len(parts[0]) > 3 {
		return false
	}

	// Subsequent subtags: allow 2–8 ASCII alnum chars.
	for _, part := range parts[1:] {
		if len(part) < 2 || len(part) > 8 {
			return false
		}
		if !isAlnum(part) {
			return false
		}
	}

	return true
}

func SplitLeadingLang(rel string, defaultLang string) (lang, relDocPath string, isDefault bool) {
	rel = filepathToSlash(rel)
	defaultLang = NormalizeTag(defaultLang)

	parts := strings.Split(rel, "/")
	if len(parts) > 1 {
		candidate := NormalizeTag(parts[0])
		if IsValidTag(candidate) {
			return candidate, strings.Join(parts[1:], "/"), candidate == defaultLang
		}
	}

	return defaultLang, rel, true
}

func isAlpha(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r > unicode.MaxASCII || !unicode.IsLetter(r) {
			return false
		}
	}
	return true
}

func isAlnum(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r > unicode.MaxASCII || !(unicode.IsLetter(r) || unicode.IsDigit(r)) {
			return false
		}
	}
	return true
}

func filepathToSlash(s string) string {
	return strings.ReplaceAll(s, `\`, `/`)
}
