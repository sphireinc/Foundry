package i18n

import (
	"encoding/json"
	"testing"
	"time"
)

func TestNormalizeTag(t *testing.T) {
	if got := NormalizeTag(" EN_us "); got != "en-us" {
		t.Fatalf("expected en-us, got %q", got)
	}
}

func TestIsValidTag(t *testing.T) {
	valid := []string{"en", "en-us", "pt-br", "zh-hant"}
	for _, tag := range valid {
		if !IsValidTag(tag) {
			t.Fatalf("expected %q to be valid", tag)
		}
	}

	invalid := []string{"", "e", "english", "en/", "en-!"}
	for _, tag := range invalid {
		if IsValidTag(tag) {
			t.Fatalf("expected %q to be invalid", tag)
		}
	}
}

func TestSplitLeadingLang(t *testing.T) {
	lang, rel, isDefault := SplitLeadingLang(`es\posts\hello.md`, "en")
	if lang != "es" || rel != "posts/hello.md" || isDefault {
		t.Fatalf("unexpected split result: %q %q %v", lang, rel, isDefault)
	}

	lang, rel, isDefault = SplitLeadingLang("posts/hello.md", "en")
	if lang != "en" || rel != "posts/hello.md" || !isDefault {
		t.Fatalf("unexpected default split result: %q %q %v", lang, rel, isDefault)
	}
}

func TestTranslateFallsBackAcrossLocales(t *testing.T) {
	tests := []struct {
		name string
		lang string
		key  string
		want string
	}{
		{name: "Spanish translation", lang: "es", key: "Post", want: "Artículo"},
		{name: "regional Spanish", lang: "es-MX", key: "Post", want: "Artículo"},
		{name: "English source fallback", lang: "en", key: "Post", want: "Post"},
		{name: "unsupported language source fallback", lang: "fr-CA", key: "Post", want: "Post"},
		{name: "missing message source fallback", lang: "es", key: "A new source message", want: "A new source message"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Translate(tt.lang, tt.key); got != tt.want {
				t.Fatalf("Translate(%q, %q) = %q, want %q", tt.lang, tt.key, got, tt.want)
			}
		})
	}
}

func TestCatalogJSONAndSupportedLanguages(t *testing.T) {
	var catalog map[string]map[string]string
	if err := json.Unmarshal([]byte(CatalogJSON()), &catalog); err != nil {
		t.Fatalf("CatalogJSON returned invalid JSON: %v", err)
	}
	if got := catalog["es"]["Post"]; got != "Artículo" {
		t.Fatalf("Spanish Post translation = %q, want Artículo", got)
	}

	want := []Language{{Code: "en", Name: "English"}, {Code: "es", Name: "Español"}}
	got := SupportedLanguages()
	if len(got) != len(want) {
		t.Fatalf("SupportedLanguages() = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("SupportedLanguages()[%d] = %#v, want %#v", i, got[i], want[i])
		}
	}
}

func TestFormatDateUsesInterfaceLocale(t *testing.T) {
	date := time.Date(2025, time.June, 4, 15, 30, 0, 0, time.UTC)
	tests := []struct {
		name string
		lang string
		want string
	}{
		{name: "Spanish", lang: "es", want: "4 de junio de 2025"},
		{name: "regional Spanish", lang: "es-MX", want: "4 de junio de 2025"},
		{name: "English", lang: "en", want: "Jun 4, 2025"},
		{name: "unsupported locale falls back to English", lang: "fr", want: "Jun 4, 2025"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FormatDate(tt.lang, &date); got != tt.want {
				t.Fatalf("FormatDate(%q) = %q, want %q", tt.lang, got, tt.want)
			}
		})
	}
	if got := FormatDate("es", nil); got != "" {
		t.Fatalf("FormatDate(nil) = %q, want empty string", got)
	}
}
