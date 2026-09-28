package theme

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestListLoadValidateAndScaffoldTheme(t *testing.T) {
	root := t.TempDir()
	scaffolded, err := Scaffold(root, "default-theme")
	if err != nil {
		t.Fatalf("scaffold theme: %v", err)
	}
	if _, err := os.Stat(filepath.Join(scaffolded, "layouts", "base.html")); err != nil {
		t.Fatalf("expected scaffolded layout: %v", err)
	}

	list, err := ListInstalled(root)
	if err != nil || len(list) != 1 || list[0].Name != "default-theme" {
		t.Fatalf("unexpected installed list: %#v %v", list, err)
	}

	manifest, err := LoadManifest(root, "default-theme")
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}
	if manifest.Name != "default-theme" || manifest.Version == "" {
		t.Fatalf("unexpected manifest: %#v", manifest)
	}

	if err := ValidateInstalled(root, "default-theme"); err != nil {
		t.Fatalf("validate installed: %v", err)
	}
}

func TestThemeManagementErrorsAndHelpers(t *testing.T) {
	root := t.TempDir()
	if _, err := ListInstalled(filepath.Join(root, "missing")); err != nil {
		t.Fatalf("expected missing themes dir to return empty list, got %v", err)
	}
	if _, err := LoadManifest(root, ""); err == nil {
		t.Fatal("expected empty theme name error")
	}
	if err := ValidateInstalled(root, ""); err == nil {
		t.Fatal("expected empty theme name error")
	}
	if err := ValidateInstalled(root, ".."); err == nil {
		t.Fatal("expected path traversal theme name error")
	}
	if _, err := Scaffold(root, "bad/name"); err == nil {
		t.Fatal("expected invalid scaffold name error")
	}
	if _, err := Scaffold(root, ".."); err == nil {
		t.Fatal("expected traversal scaffold name error")
	}
	if err := SwitchInConfig(filepath.Join(t.TempDir(), "site.yaml"), ".."); err == nil {
		t.Fatal("expected invalid switch theme name error")
	}

	name := humanizeName("my_theme-name")
	if name != "My Theme Name" {
		t.Fatalf("unexpected humanized name: %q", name)
	}
	if !strings.Contains(scaffoldManifest("hello"), "name: hello") {
		t.Fatal("expected manifest scaffold content")
	}
	for _, slot := range requiredLaunchSlots {
		if !strings.Contains(scaffoldManifest("hello"), slot) {
			t.Fatalf("expected scaffold manifest to include slot %q", slot)
		}
	}
	if !strings.Contains(scaffoldBase(), `define "base"`) ||
		!strings.Contains(scaffoldHead(), `define "head"`) ||
		!strings.Contains(scaffoldHeader(), `define "header"`) ||
		!strings.Contains(scaffoldFooter(), `define "footer"`) ||
		!strings.Contains(scaffoldIndex(), `define "content"`) ||
		!strings.Contains(scaffoldPage(), `safeHTML`) ||
		!strings.Contains(scaffoldPost(), `safeHTML`) ||
		!strings.Contains(scaffoldList(), `No entries found.`) ||
		!strings.Contains(scaffoldCSS(), "font-family") {
		t.Fatal("expected scaffold helper content")
	}
	if !strings.Contains(scaffoldBase(), "window.__foundryReloadSource") ||
		!strings.Contains(scaffoldBase(), "window.__foundryReloadPollTimer") ||
		!strings.Contains(scaffoldBase(), "/__reload/poll") ||
		!strings.Contains(scaffoldBase(), "mode === 'poll'") ||
		!strings.Contains(scaffoldBase(), "pagehide") ||
		!strings.Contains(scaffoldBase(), "beforeunload") {
		t.Fatal("expected scaffold base to close live reload connections")
	}
}

func TestValidateInstalledRequiresLaunchSlotsInManifestAndTemplates(t *testing.T) {
	root := t.TempDir()
	scaffolded, err := Scaffold(root, "launch-theme")
	if err != nil {
		t.Fatalf("scaffold theme: %v", err)
	}

	manifestPath := filepath.Join(scaffolded, "theme.yaml")
	body, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	body = []byte(strings.Replace(string(body), "  - post.sidebar.bottom\n", "", 1))
	if err := os.WriteFile(manifestPath, body, 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	if err := ValidateInstalled(root, "launch-theme"); err == nil {
		t.Fatal("expected validation failure for missing required slot declaration")
	}

	scaffolded, err = Scaffold(root, "template-theme")
	if err != nil {
		t.Fatalf("scaffold theme: %v", err)
	}
	postPath := filepath.Join(scaffolded, "layouts", "post.html")
	postBody, err := os.ReadFile(postPath)
	if err != nil {
		t.Fatalf("read post layout: %v", err)
	}
	postBody = []byte(strings.Replace(string(postBody), `{{ pluginSlot "post.sidebar.overview" }}`, "", 1))
	if err := os.WriteFile(postPath, postBody, 0o644); err != nil {
		t.Fatalf("write post layout: %v", err)
	}
	if err := ValidateInstalled(root, "template-theme"); err == nil {
		t.Fatal("expected validation failure for missing required slot rendering")
	}
}

func TestValidateInstalledDetailedReportsTemplateProblems(t *testing.T) {
	root := t.TempDir()
	scaffolded, err := Scaffold(root, "broken-theme")
	if err != nil {
		t.Fatalf("scaffold theme: %v", err)
	}
	basePath := filepath.Join(scaffolded, "layouts", "base.html")
	if err := os.WriteFile(basePath, []byte(`{{ define "base" }}{{ template "missing-partial" . }}{{ end }}`), 0o644); err != nil {
		t.Fatalf("write broken base: %v", err)
	}

	result, err := ValidateInstalledDetailed(root, "broken-theme")
	if err != nil {
		t.Fatalf("validate detailed: %v", err)
	}
	if result.Valid {
		t.Fatal("expected invalid validation result")
	}
	var foundReference bool
	for _, diagnostic := range result.Diagnostics {
		if strings.Contains(diagnostic.Message, "unknown partial") {
			foundReference = true
			break
		}
	}
	if !foundReference {
		t.Fatalf("expected unknown partial diagnostic, got %#v", result.Diagnostics)
	}
}

func TestValidateInstalledDetailedRejectsUnsupportedSDKVersion(t *testing.T) {
	root := t.TempDir()
	scaffolded, err := Scaffold(root, "sdk-theme")
	if err != nil {
		t.Fatalf("scaffold theme: %v", err)
	}
	manifestPath := filepath.Join(scaffolded, "theme.yaml")
	body, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	body = []byte(strings.Replace(string(body), "sdk_version: v1\n", "sdk_version: v2\n", 1))
	if err := os.WriteFile(manifestPath, body, 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	result, err := ValidateInstalledDetailed(root, "sdk-theme")
	if err != nil {
		t.Fatalf("validate detailed: %v", err)
	}
	if result.Valid {
		t.Fatal("expected unsupported sdk version to invalidate theme")
	}
}

func TestValidateInstalledDetailedChecksThemeSecurity(t *testing.T) {
	root := t.TempDir()
	scaffolded, err := Scaffold(root, "security-theme")
	if err != nil {
		t.Fatalf("scaffold theme: %v", err)
	}
	headPath := filepath.Join(scaffolded, "layouts", "partials", "head.html")
	headBody, err := os.ReadFile(headPath)
	if err != nil {
		t.Fatalf("read head: %v", err)
	}
	updatedHead := strings.Replace(string(headBody), `{{ pluginSlot "head.end" }}`, `{{ pluginSlot "head.end" }}<script src="https://cdn.example.com/theme.js"></script>`, 1)
	if err := os.WriteFile(headPath, []byte(updatedHead), 0o644); err != nil {
		t.Fatalf("write head: %v", err)
	}

	result, err := ValidateInstalledDetailed(root, "security-theme")
	if err != nil {
		t.Fatalf("validate detailed: %v", err)
	}
	if result.Valid {
		t.Fatal("expected undeclared remote asset to invalidate theme")
	}
	var securityDiagnostic ValidationDiagnostic
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Code == "theme.security.remote_asset_undeclared" {
			securityDiagnostic = diagnostic
			break
		}
	}
	if securityDiagnostic.Path != filepath.ToSlash(headPath) || securityDiagnostic.Line == 0 {
		t.Fatalf("expected source path and line in security diagnostic, got %#v", securityDiagnostic)
	}
	if securityDiagnostic.Field != "security.external_assets.scripts" || !strings.Contains(securityDiagnostic.Hint, "security.external_assets.allowed: true") {
		t.Fatalf("expected actionable script remediation, got %#v", securityDiagnostic)
	}

	manifestPath := filepath.Join(scaffolded, "theme.yaml")
	body, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	updated := strings.Replace(string(body), "  external_assets:\n    allowed: false\n", "  external_assets:\n    allowed: true\n    scripts:\n      - https://cdn.example.com\n", 1)
	if err := os.WriteFile(manifestPath, []byte(updated), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	result, err = ValidateInstalledDetailed(root, "security-theme")
	if err != nil {
		t.Fatalf("validate detailed after allowlist: %v", err)
	}
	if !result.Valid {
		t.Fatalf("expected security declaration to validate, got %#v", result.Diagnostics)
	}
	report, err := AnalyzeInstalledSecurity(root, "security-theme")
	if err != nil {
		t.Fatalf("analyze security: %v", err)
	}
	if len(report.DetectedAssets) != 1 || report.DetectedAssets[0].Kind != "script" || report.DetectedAssets[0].Status != "declared" {
		t.Fatalf("expected declared script finding, got %#v", report.DetectedAssets)
	}
	manifest, err := LoadManifest(root, "security-theme")
	if err != nil {
		t.Fatalf("load declared security manifest: %v", err)
	}
	if csp := ContentSecurityPolicy(manifest); !strings.Contains(csp, "script-src 'self' 'unsafe-inline' https://cdn.example.com") {
		t.Fatalf("expected declared script in script-src CSP, got %q", csp)
	}
}

func TestValidateInstalledDetailedUsesAssetCategoriesAndRequestOrigins(t *testing.T) {
	root := t.TempDir()
	scaffolded, err := Scaffold(root, "category-theme")
	if err != nil {
		t.Fatalf("scaffold theme: %v", err)
	}

	headPath := filepath.Join(scaffolded, "layouts", "partials", "head.html")
	headBody, err := os.ReadFile(headPath)
	if err != nil {
		t.Fatalf("read head: %v", err)
	}
	updatedHead := strings.Replace(string(headBody), `{{ pluginSlot "head.end" }}`, `{{ pluginSlot "head.end" }}<img src="https://cdn.example.com/hero.png">`, 1)
	if err := os.WriteFile(headPath, []byte(updatedHead), 0o644); err != nil {
		t.Fatalf("write head: %v", err)
	}
	requestPath := filepath.Join(scaffolded, "assets", "js", "requests.js")
	if err := os.MkdirAll(filepath.Dir(requestPath), 0o755); err != nil {
		t.Fatalf("mkdir request assets: %v", err)
	}
	if err := os.WriteFile(requestPath, []byte(`fetch("https://api.example.com/data")`), 0o644); err != nil {
		t.Fatalf("write request asset: %v", err)
	}

	manifestPath := filepath.Join(scaffolded, "theme.yaml")
	manifestBody, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	manifest := strings.Replace(string(manifestBody), "  external_assets:\n    allowed: false\n", "  external_assets:\n    allowed: true\n    scripts:\n      - https://cdn.example.com\n", 1)
	manifest = strings.Replace(manifest, "  frontend_requests:\n    allowed: false\n", "  frontend_requests:\n    allowed: true\n    origins:\n      - https://declared-api.example.com\n", 1)
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	result, err := ValidateInstalledDetailed(root, "category-theme")
	if err != nil {
		t.Fatalf("validate detailed: %v", err)
	}
	if result.Valid {
		t.Fatal("expected image declared under scripts and request to be invalid")
	}
	var imageDiagnostic, requestDiagnostic ValidationDiagnostic
	for _, diagnostic := range result.Diagnostics {
		switch diagnostic.Code {
		case "theme.security.remote_asset_undeclared":
			if diagnostic.Category == "images" {
				imageDiagnostic = diagnostic
			}
		case "theme.security.frontend_request_undeclared":
			requestDiagnostic = diagnostic
		}
	}
	if imageDiagnostic.Field != "security.external_assets.images" || !strings.Contains(imageDiagnostic.Hint, "security.external_assets.images") {
		t.Fatalf("expected image category remediation, got %#v", imageDiagnostic)
	}
	if requestDiagnostic.Field != "security.frontend_requests.origins" || !strings.Contains(requestDiagnostic.Hint, "security.frontend_requests.origins") {
		t.Fatalf("expected request origin remediation, got %#v", requestDiagnostic)
	}
}

func TestValidateInstalledDetailedAcceptsEachDeclaredAssetCategory(t *testing.T) {
	root := t.TempDir()
	scaffolded, err := Scaffold(root, "all-assets-theme")
	if err != nil {
		t.Fatalf("scaffold theme: %v", err)
	}

	headPath := filepath.Join(scaffolded, "layouts", "partials", "head.html")
	headBody, err := os.ReadFile(headPath)
	if err != nil {
		t.Fatalf("read head: %v", err)
	}
	updatedHead := strings.Replace(string(headBody), `{{ pluginSlot "head.end" }}`, `{{ pluginSlot "head.end" }}<script src="https://scripts.example.com/theme.js"></script><link rel="stylesheet" href="https://styles.example.com/theme.css"><img src="https://images.example.com/hero.png"><video src="https://media.example.com/demo.mp4"></video>`, 1)
	if err := os.WriteFile(headPath, []byte(updatedHead), 0o644); err != nil {
		t.Fatalf("write head: %v", err)
	}
	cssPath := filepath.Join(scaffolded, "assets", "css", "theme.css")
	if err := os.MkdirAll(filepath.Dir(cssPath), 0o755); err != nil {
		t.Fatalf("mkdir css dir: %v", err)
	}
	if err := os.WriteFile(cssPath, []byte(`@import url("https://styles.example.com/import.css"); @font-face { src: url("https://fonts.example.com/theme.woff2"); } .hero { background: url("https://images.example.com/background.png"); } .video { background: url("https://media.example.com/background.mp4"); }`), 0o644); err != nil {
		t.Fatalf("write css: %v", err)
	}

	manifestPath := filepath.Join(scaffolded, "theme.yaml")
	manifestBody, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	manifest := strings.Replace(string(manifestBody), "  external_assets:\n    allowed: false\n", "  external_assets:\n    allowed: true\n    scripts:\n      - https://scripts.example.com\n    styles:\n      - https://styles.example.com\n    fonts:\n      - https://fonts.example.com\n    images:\n      - https://images.example.com\n    media:\n      - https://media.example.com\n", 1)
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	result, err := ValidateInstalledDetailed(root, "all-assets-theme")
	if err != nil {
		t.Fatalf("validate detailed: %v", err)
	}
	if !result.Valid {
		t.Fatalf("expected each declared asset category to pass, got %#v", result.Diagnostics)
	}
	report, err := AnalyzeInstalledSecurity(root, "all-assets-theme")
	if err != nil {
		t.Fatalf("analyze declared assets: %v", err)
	}
	seen := map[string]bool{}
	for _, finding := range report.DetectedAssets {
		seen[finding.Kind] = finding.Status == "declared"
	}
	for _, kind := range []string{"script", "style", "font", "image", "media"} {
		if !seen[kind] {
			t.Fatalf("expected declared %s finding, got %#v", kind, report.DetectedAssets)
		}
	}
}

func TestValidateInstalledDetailedReportsDisabledSecurityPolicies(t *testing.T) {
	root := t.TempDir()
	scaffolded, err := Scaffold(root, "disabled-policy-theme")
	if err != nil {
		t.Fatalf("scaffold theme: %v", err)
	}
	manifestPath := filepath.Join(scaffolded, "theme.yaml")
	body, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	manifest := strings.Replace(string(body), "  external_assets:\n    allowed: false\n", "  external_assets:\n    allowed: false\n    images:\n      - https://images.example.com\n", 1)
	manifest = strings.Replace(manifest, "  frontend_requests:\n    allowed: false\n", "  frontend_requests:\n    allowed: false\n    origins:\n      - https://api.example.com\n", 1)
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	result, err := ValidateInstalledDetailed(root, "disabled-policy-theme")
	if err != nil {
		t.Fatalf("validate detailed: %v", err)
	}
	if result.Valid {
		t.Fatal("expected disabled policies with declarations to be invalid")
	}
	seen := map[string]ValidationDiagnostic{}
	for _, diagnostic := range result.Diagnostics {
		seen[diagnostic.Code] = diagnostic
	}
	if diagnostic := seen["theme.security.external_assets.disabled"]; diagnostic.Field != "security.external_assets.allowed" || diagnostic.Hint == "" {
		t.Fatalf("expected external assets disabled diagnostic, got %#v", diagnostic)
	}
	if diagnostic := seen["theme.security.frontend_requests.disabled"]; diagnostic.Field != "security.frontend_requests.allowed" || diagnostic.Hint == "" {
		t.Fatalf("expected frontend requests disabled diagnostic, got %#v", diagnostic)
	}
	loadedManifest, err := LoadManifest(root, "disabled-policy-theme")
	if err != nil {
		t.Fatalf("load disabled policy manifest: %v", err)
	}
	if csp := ContentSecurityPolicy(loadedManifest); strings.Contains(csp, "https://images.example.com") {
		t.Fatalf("disabled external asset policy should not widen CSP, got %q", csp)
	}
}

func TestLoadManifestRejectsSymlinkedManifest(t *testing.T) {
	root := t.TempDir()
	themeRoot := filepath.Join(root, "symlink-theme")
	if err := os.MkdirAll(themeRoot, 0o755); err != nil {
		t.Fatalf("mkdir theme root: %v", err)
	}
	outside := filepath.Join(t.TempDir(), "theme.yaml")
	if err := os.WriteFile(outside, []byte("name: symlink-theme\n"), 0o644); err != nil {
		t.Fatalf("write outside manifest: %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(themeRoot, "theme.yaml")); err != nil {
		t.Skipf("symlink not supported on %s: %v", runtime.GOOS, err)
	}

	if _, err := LoadManifest(root, "symlink-theme"); err == nil {
		t.Fatal("expected symlinked manifest to be rejected")
	}
}

func TestLoadManifestRejectsSymlinkRoot(t *testing.T) {
	root := t.TempDir()
	realTheme := filepath.Join(root, "real-theme")
	if err := os.MkdirAll(realTheme, 0o755); err != nil {
		t.Fatalf("mkdir theme root: %v", err)
	}
	if err := os.WriteFile(filepath.Join(realTheme, "theme.yaml"), []byte("name: linked-theme\n"), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	if err := os.Symlink(realTheme, filepath.Join(root, "linked-theme")); err != nil {
		t.Skipf("symlink not supported on %s: %v", runtime.GOOS, err)
	}

	if _, err := LoadManifest(root, "linked-theme"); err == nil {
		t.Fatal("expected symlinked theme root to be rejected")
	}
}

func TestValidateInstalledDetailedRejectsSymlinkedLayout(t *testing.T) {
	root := t.TempDir()
	scaffolded, err := Scaffold(root, "layout-theme")
	if err != nil {
		t.Fatalf("scaffold theme: %v", err)
	}
	outside := filepath.Join(t.TempDir(), "page.html")
	if err := os.WriteFile(outside, []byte(`{{ define "content" }}outside{{ end }}`), 0o644); err != nil {
		t.Fatalf("write outside layout: %v", err)
	}
	layoutPath := filepath.Join(scaffolded, "layouts", "page.html")
	if err := os.Remove(layoutPath); err != nil {
		t.Fatalf("remove layout: %v", err)
	}
	if err := os.Symlink(outside, layoutPath); err != nil {
		t.Skipf("symlink not supported on %s: %v", runtime.GOOS, err)
	}

	result, err := ValidateInstalledDetailed(root, "layout-theme")
	if err != nil {
		t.Fatalf("validate detailed: %v", err)
	}
	if result.Valid {
		t.Fatal("expected symlinked layout to invalidate theme")
	}
}

func TestValidateInstalledDetailedRejectsSymlinkRoot(t *testing.T) {
	root := t.TempDir()
	realTheme := filepath.Join(root, "real-theme")
	if err := os.MkdirAll(realTheme, 0o755); err != nil {
		t.Fatalf("mkdir theme root: %v", err)
	}
	if err := os.WriteFile(filepath.Join(realTheme, "theme.yaml"), []byte(`name: linked-theme
title: Linked Theme
version: 0.1.0
min_foundry_version: 0.1.0
sdk_version: v1
compatibility_version: v1
layouts: [base, index, page, post, list]
slots: [head.end, body.start, body.end, page.before_main, page.after_main, page.before_content, page.after_content, post.before_header, post.after_header, post.before_content, post.after_content, post.sidebar.top, post.sidebar.overview, post.sidebar.bottom]
`), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	if err := os.Symlink(realTheme, filepath.Join(root, "linked-theme")); err != nil {
		t.Skipf("symlink not supported on %s: %v", runtime.GOOS, err)
	}

	result, err := ValidateInstalledDetailed(root, "linked-theme")
	if err != nil {
		t.Fatalf("validate detailed: %v", err)
	}
	if result.Valid {
		t.Fatal("expected symlinked root to invalidate theme")
	}
	var found bool
	for _, diagnostic := range result.Diagnostics {
		if strings.Contains(strings.ToLower(diagnostic.Message), "symlink") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected symlink diagnostic, got %#v", result.Diagnostics)
	}
}

func TestManagerMustExistRejectsSymlinkRoot(t *testing.T) {
	root := t.TempDir()
	themeRoot := filepath.Join(root, "linked-theme")
	if err := os.MkdirAll(filepath.Join(root, "real-theme"), 0o755); err != nil {
		t.Fatalf("mkdir theme root: %v", err)
	}
	if err := os.Symlink(filepath.Join(root, "real-theme"), themeRoot); err != nil {
		t.Skipf("symlink not supported on %s: %v", runtime.GOOS, err)
	}

	mgr := NewManager(root, "linked-theme")
	if err := mgr.MustExist(); err == nil {
		t.Fatal("expected symlinked theme root to be rejected")
	}
	if err := ValidateInstalled(root, "linked-theme"); err == nil {
		t.Fatal("expected symlinked theme root validation to fail")
	}
}

func TestDocumentFieldDefinitionsMatchesThemeContracts(t *testing.T) {
	root := t.TempDir()
	themeRoot := filepath.Join(root, "contract-theme")
	if err := os.MkdirAll(themeRoot, 0o755); err != nil {
		t.Fatalf("mkdir theme root: %v", err)
	}
	manifest := `name: contract-theme
title: Contract Theme
version: 0.1.0
min_foundry_version: 0.1.0
sdk_version: v1
compatibility_version: v1
layouts: [base, index, page, post, list]
slots: [head.end, body.start, body.end, page.before_main, page.after_main, page.before_content, page.after_content, post.before_header, post.after_header, post.before_content, post.after_content, post.sidebar.top, post.sidebar.overview, post.sidebar.bottom]
field_contracts:
  - key: marketing-page
    target:
      scope: document
      types: [page]
      layouts: [page]
      slugs: [about]
    fields:
      - name: hero_title
        type: text
  - key: blog-post
    target:
      scope: document
      types: [post]
      layouts: [post]
    fields:
      - name: hero_eyebrow
        type: text
`
	if err := os.WriteFile(filepath.Join(themeRoot, "theme.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	pageDefs := DocumentFieldDefinitions(root, "contract-theme", "page", "page", "about")
	if len(pageDefs) != 1 || pageDefs[0].Name != "hero_title" {
		t.Fatalf("expected page contract fields, got %#v", pageDefs)
	}

	postDefs := DocumentFieldDefinitions(root, "contract-theme", "post", "post", "hello-world")
	if len(postDefs) != 1 || postDefs[0].Name != "hero_eyebrow" {
		t.Fatalf("expected post contract fields, got %#v", postDefs)
	}

	missingDefs := DocumentFieldDefinitions(root, "contract-theme", "page", "page", "pricing")
	if len(missingDefs) != 0 {
		t.Fatalf("expected unmatched slug to return no fields, got %#v", missingDefs)
	}
}
