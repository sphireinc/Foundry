package i18n

import (
	_ "embed"
	"encoding/json"
	"sort"
	"strings"
	"sync"
	"time"
)

//go:embed messages.json
var messagesJSON []byte

var (
	catalogOnce sync.Once
	catalog     map[string]map[string]string
	catalogErr  error
)

// Language describes a locale supported by Foundry's built-in interface.
type Language struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

var languageNames = map[string]string{
	"en": "English",
	"es": "Español",
}

func loadCatalog() (map[string]map[string]string, error) {
	catalogOnce.Do(func() {
		catalog = make(map[string]map[string]string)
		catalogErr = json.Unmarshal(messagesJSON, &catalog)
		if catalogErr != nil {
			catalog = nil
		}
	})
	return catalog, catalogErr
}

// Translate returns the message for lang, falling back to a base language,
// then English (the source copy, represented by key).
func Translate(lang, key string) string {
	translations, err := loadCatalog()
	if err != nil {
		return key
	}
	lang = NormalizeTag(lang)
	if translated := translations[lang][key]; translated != "" {
		return translated
	}
	if base, _, ok := strings.Cut(lang, "-"); ok {
		if translated := translations[base][key]; translated != "" {
			return translated
		}
	}
	return key
}

// CatalogJSON returns the bundled translations for safe embedding in the
// default admin page's HTML data attribute.
func CatalogJSON() string {
	if _, err := loadCatalog(); err != nil {
		return `{}`
	}
	return string(messagesJSON)
}

// SupportedLanguages returns the bundled interface languages in a stable
// order. English is the source/fallback language and needs no translated map.
func SupportedLanguages() []Language {
	translations, err := loadCatalog()
	if err != nil {
		return []Language{{Code: "en", Name: languageNames["en"]}}
	}
	codes := make([]string, 0, len(translations)+1)
	codes = append(codes, "en")
	for code := range translations {
		if code != "en" {
			codes = append(codes, code)
		}
	}
	sort.Strings(codes[1:])
	languages := make([]Language, 0, len(codes))
	for _, code := range codes {
		name := languageNames[code]
		if name == "" {
			name = code
		}
		languages = append(languages, Language{Code: code, Name: name})
	}
	return languages
}

// FormatDate formats the default theme's interface dates using the page
// language while keeping an English fallback for other locales.
func FormatDate(lang string, value *time.Time) string {
	if value == nil {
		return ""
	}
	date := *value
	if NormalizeTag(lang) == "es" || strings.HasPrefix(NormalizeTag(lang), "es-") {
		months := [...]string{"enero", "febrero", "marzo", "abril", "mayo", "junio", "julio", "agosto", "septiembre", "octubre", "noviembre", "diciembre"}
		return date.Format("2") + " de " + months[int(date.Month())-1] + " de " + date.Format("2006")
	}
	return date.Format("Jan 2, 2006")
}
