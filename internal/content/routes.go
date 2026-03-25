package content

import (
	"fmt"
	"strings"
	"unicode"
)

// AuthorSlug normalizes a human author name into the canonical segment used by
// Foundry's built-in author archive routes.
func AuthorSlug(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return ""
	}

	var b strings.Builder
	lastDash := false
	for _, r := range name {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			lastDash = false
		case r == '-' || unicode.IsSpace(r) || r == '_' || r == '.':
			if !lastDash && b.Len() > 0 {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}

	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "author"
	}
	return out
}

// AuthorArchiveURL returns the canonical built-in author archive URL for a
// language-aware site.
func AuthorArchiveURL(defaultLang, lang, author string) string {
	slug := AuthorSlug(author)
	if slug == "" {
		return ""
	}
	if strings.TrimSpace(lang) == "" || strings.TrimSpace(lang) == strings.TrimSpace(defaultLang) {
		return fmt.Sprintf("/authors/%s/", slug)
	}
	return fmt.Sprintf("/%s/authors/%s/", strings.TrimSpace(lang), slug)
}

// SearchPageURL returns the canonical built-in search page URL for a
// language-aware site.
func SearchPageURL(defaultLang, lang string) string {
	if strings.TrimSpace(lang) == "" || strings.TrimSpace(lang) == strings.TrimSpace(defaultLang) {
		return "/search/"
	}
	return fmt.Sprintf("/%s/search/", strings.TrimSpace(lang))
}
