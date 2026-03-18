package i18n

import "testing"

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
