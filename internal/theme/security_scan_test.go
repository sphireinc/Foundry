package theme

import (
	"strings"
	"testing"
)

func TestScanThemeSecurityFileClassifiesAssetsAndIgnoresLinksAndComments(t *testing.T) {
	body := []byte(`<!-- <script src="https://ignored.example/script.js"></script> -->
<a href="https://ignored.example/docs">External documentation</a>
<script src="https://cdn.example.com/theme.js"></script>
<link rel="stylesheet" href="https://cdn.example.com/theme.css">
<img src="https://images.example.com/hero.png">
<video src="https://media.example.com/demo.mp4"></video>
<script>const markup = '<img src="https://ignored.example/script-string.png">';</script>
<style>
@import url("https://fonts.example.com/theme.css");
.hero { background-image: url("https://images.example.com/background.png"); }
</style>`)

	references := scanThemeSecurityFile("theme.html", ".html", body)
	if len(references) != 6 {
		t.Fatalf("expected six security references, got %#v", references)
	}

	gotKinds := make([]string, 0, len(references))
	for _, reference := range references {
		gotKinds = append(gotKinds, reference.Kind)
	}
	got := strings.Join(gotKinds, ",")
	if got != "script,style,image,media,style,image" {
		t.Fatalf("unexpected reference categories: %s", got)
	}
	if references[0].Line != 3 || references[5].Line != 10 {
		t.Fatalf("expected source line numbers, got %#v", references)
	}
}

func TestScanThemeSecurityFileDetectsRequestsAndIgnoresStringsAndComments(t *testing.T) {
	body := []byte(`// fetch("https://ignored.example/comment")
const documentationURL = "https://ignored.example/string"
const fakeRequest = 'fetch("https://ignored.example/fake")'
fetch("https://api.example.com/data")
const socket = new WebSocket("wss://socket.example.com/events")
request.open("GET", "https://api.example.com/open")`)

	references := scanThemeSecurityFile("theme.js", ".js", body)
	if len(references) != 3 {
		t.Fatalf("expected three frontend requests, got %#v", references)
	}
	for _, reference := range references {
		if reference.Kind != "request" {
			t.Fatalf("expected request category, got %#v", reference)
		}
	}
	if references[0].Line != 4 || references[1].Line != 5 || references[2].Line != 6 {
		t.Fatalf("unexpected request line numbers: %#v", references)
	}
}

func TestScanThemeSecurityFileClassifiesCSSFontsImagesAndMedia(t *testing.T) {
	body := []byte(`/* url("https://ignored.example/comment.png") */
@import url("https://fonts.example.com/theme.css");
@font-face { src: url("https://fonts.example.com/theme.woff2"); }
.hero { background-image: url("https://images.example.com/hero.png"); }
.video { background-image: url("https://media.example.com/demo.mp4"); }`)

	references := scanThemeSecurityFile("assets/theme.css", ".css", body)
	if len(references) != 4 {
		t.Fatalf("expected four CSS security references, got %#v", references)
	}
	gotKinds := make([]string, 0, len(references))
	for _, reference := range references {
		gotKinds = append(gotKinds, reference.Kind)
	}
	if got := strings.Join(gotKinds, ","); got != "style,font,image,media" {
		t.Fatalf("unexpected CSS reference categories: %s", got)
	}
}

func TestSecurityReferenceMatchingUsesTheCSPCategory(t *testing.T) {
	sec := ThemeSecurity{
		ExternalAssets: ThemeExternalAssets{
			Allowed: true,
			Scripts: []string{"https://cdn.example.com"},
		},
	}

	if !securityReferenceAllowed(themeSecurityReference{Kind: "script", URL: "https://cdn.example.com/theme.js"}, sec) {
		t.Fatal("expected declared script origin to be allowed")
	}
	if securityReferenceAllowed(themeSecurityReference{Kind: "image", URL: "https://cdn.example.com/hero.png"}, sec) {
		t.Fatal("expected script declaration not to authorize an image origin")
	}
	if got := securityReferenceField("image"); got != "security.external_assets.images" {
		t.Fatalf("unexpected image declaration field: %q", got)
	}
}
