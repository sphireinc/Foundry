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

func TestScanThemeSecurityFileClassifiesPictureSourcesAndFetchedLinks(t *testing.T) {
	body := []byte(`<picture>
<source srcset="https://images.example.com/hero.avif" type="image/avif">
</picture>
<video><source src="https://media.example.com/demo.mp4"></video>
<link rel="preload" as="font" href="https://fonts.example.com/theme.woff2">
<link rel="modulepreload" href="https://cdn.example.com/module.js">
<link rel="preload" as="image" href="https://images.example.com/preloaded.png">
<link rel="icon" href="https://images.example.com/favicon.png">
<link rel="preload" as="fetch" href="https://api.example.com/data">
<link rel="prefetch" as="style" href="https://cdn.example.com/prefetched.css">
<link rel="apple-touch-icon" href="https://images.example.com/touch-icon.png">`)

	references := scanThemeSecurityFile("theme.html", ".html", body)
	if len(references) != 9 {
		t.Fatalf("expected nine security references, got %#v", references)
	}
	gotKinds := make([]string, 0, len(references))
	for _, reference := range references {
		gotKinds = append(gotKinds, reference.Kind)
	}
	if got := strings.Join(gotKinds, ","); got != "image,media,font,script,image,image,request,style,image" {
		t.Fatalf("unexpected reference categories: %s", got)
	}
}

func TestScanThemeSecurityFileDetectsInlineScriptRequests(t *testing.T) {
	body := []byte(`<script>
// fetch("https://ignored.example/comment")
fetch("https://api.example.com/data")
</script>`)

	references := scanThemeSecurityFile("theme.html", ".html", body)
	if len(references) != 1 || references[0].Kind != "request" || references[0].URL != "https://api.example.com/data" {
		t.Fatalf("expected inline script request, got %#v", references)
	}
	if references[0].Line != 3 {
		t.Fatalf("expected inline request line 3, got %#v", references[0])
	}
}

func TestScanThemeSecurityFileClassifiesDestinationlessPrefetch(t *testing.T) {
	body := []byte(`<link rel="prefetch" href="https://cdn.example.com/app.js">
<link rel="prefetch" href="https://cdn.example.com/page">`)

	references := scanThemeSecurityFile("theme.html", ".html", body)
	if len(references) != 2 {
		t.Fatalf("expected two destinationless prefetch references, got %#v", references)
	}
	if references[0].Kind != "script" || references[1].Kind != "request" {
		t.Fatalf("unexpected destinationless prefetch categories: %#v", references)
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

func TestSecurityReferenceStatusDistinguishesDisabledAndMissingDeclarations(t *testing.T) {
	sec := ThemeSecurity{
		ExternalAssets: ThemeExternalAssets{
			Images: []string{"https://images.example.com"},
		},
		FrontendRequests: ThemeFrontendRequests{
			Origins: []string{"https://api.example.com"},
		},
	}

	image := themeSecurityReference{Kind: "image", URL: "https://images.example.com/hero.png"}
	request := themeSecurityReference{Kind: "request", URL: "https://api.example.com/data"}
	missing := themeSecurityReference{Kind: "image", URL: "https://other.example.com/hero.png"}
	if got := securityReferenceStatus(image, sec); got != "disabled" {
		t.Fatalf("expected declared image with disabled policy to be disabled, got %q", got)
	}
	if got := securityReferenceStatus(request, sec); got != "disabled" {
		t.Fatalf("expected declared request with disabled policy to be disabled, got %q", got)
	}
	if got := securityReferenceStatus(missing, sec); got != "undeclared" {
		t.Fatalf("expected missing image declaration to be undeclared, got %q", got)
	}
	if got := securityReferenceRemediation(image, "disabled"); got != "Set security.external_assets.allowed: true in theme.yaml." {
		t.Fatalf("unexpected disabled asset remediation: %q", got)
	}
	if got := securityReferenceRemediation(request, "disabled"); got != "Set security.frontend_requests.allowed: true in theme.yaml." {
		t.Fatalf("unexpected disabled request remediation: %q", got)
	}
	if got := securityReferenceRemediation(image, "declared"); got != "" {
		t.Fatalf("expected no remediation for declared finding, got %q", got)
	}
}
