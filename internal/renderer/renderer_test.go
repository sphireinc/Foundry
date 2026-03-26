package renderer

import (
	"context"
	"html/template"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sphireinc/foundry/internal/config"
	"github.com/sphireinc/foundry/internal/content"
	"github.com/sphireinc/foundry/internal/theme"
)

type rendererHooks struct {
	failOn string
}

func (h rendererHooks) OnContext(*ViewData) error {
	if h.failOn == "context" {
		return os.ErrInvalid
	}
	return nil
}
func (h rendererHooks) OnAssets(_ *ViewData, a *AssetSet) error {
	if h.failOn == "assets" {
		return os.ErrInvalid
	}
	a.AddStyle("/app.css")
	a.AddScript("/app.js", ScriptPositionHead)
	a.AddScript("/body.js", ScriptPositionBodyEnd)
	return nil
}
func (h rendererHooks) OnBeforeRender(*ViewData) error {
	if h.failOn == "before" {
		return os.ErrInvalid
	}
	return nil
}
func (h rendererHooks) OnAfterRender(_ string, html []byte) ([]byte, error) {
	if h.failOn == "after" {
		return nil, os.ErrInvalid
	}
	return append(html, []byte("<!--after-->")...), nil
}
func (h rendererHooks) OnAssetsBuilding(*config.Config) error {
	if h.failOn == "assets-building" {
		return os.ErrInvalid
	}
	return nil
}
func (h rendererHooks) OnHTMLSlots(_ *ViewData, s *Slots) error {
	if h.failOn == "slots" {
		return os.ErrInvalid
	}
	s.Add("body.end", "<div>slot</div>")
	return nil
}

func TestBuildURLsRendersTaxonomyArchiveWithConfiguredLayout(t *testing.T) {
	cfg := testRendererConfig(t)
	writeRendererTheme(t, cfg)

	graph := content.NewSiteGraph(cfg)
	graph.Add(&content.Document{
		ID:         "post-1",
		Type:       "post",
		Lang:       cfg.DefaultLang,
		Title:      "Hello",
		Slug:       "hello",
		URL:        "/posts/hello/",
		Layout:     "post",
		SourcePath: filepath.ToSlash(filepath.Join(cfg.ContentDir, "posts", "hello.md")),
		HTMLBody:   template.HTML("<p>Hello</p>"),
		Taxonomies: map[string][]string{"tags": {"go"}},
	})

	r := New(cfg, theme.NewManager(cfg.ThemesDir, cfg.Theme), nil)
	if err := r.BuildURLs(context.Background(), graph, []string{"/tags/go/"}); err != nil {
		t.Fatalf("build urls failed: %v", err)
	}

	body, err := os.ReadFile(filepath.Join(cfg.PublicDir, "tags", "go", "index.html"))
	if err != nil {
		t.Fatalf("expected taxonomy archive output: %v", err)
	}
	if !strings.Contains(string(body), "taxonomy layout for tags/go") {
		t.Fatalf("expected taxonomy term layout to render, got %q", string(body))
	}
}

func TestRendererBuildsCoreSearchAuthorAndNotFoundRoutes(t *testing.T) {
	cfg := testRendererConfig(t)
	writeRendererTheme(t, cfg)

	graph := content.NewSiteGraph(cfg)
	graph.Add(&content.Document{
		ID:         "post-1",
		Type:       "post",
		Lang:       cfg.DefaultLang,
		Title:      "Hello",
		Slug:       "hello",
		URL:        "/posts/hello/",
		Layout:     "post",
		SourcePath: filepath.ToSlash(filepath.Join(cfg.ContentDir, "posts", "hello.md")),
		HTMLBody:   template.HTML("<p>Hello</p>"),
		Summary:    "Search me",
		Author:     "Jane Editor",
	})

	r := New(cfg, theme.NewManager(cfg.ThemesDir, cfg.Theme), nil)
	if err := r.Build(context.Background(), graph); err != nil {
		t.Fatalf("build renderer output: %v", err)
	}

	searchPage, err := os.ReadFile(filepath.Join(cfg.PublicDir, "search", "index.html"))
	if err != nil {
		t.Fatalf("expected search page output: %v", err)
	}
	if !strings.Contains(string(searchPage), "search Search") {
		t.Fatalf("expected search layout output, got %q", string(searchPage))
	}

	authorPage, err := os.ReadFile(filepath.Join(cfg.PublicDir, "authors", "jane-editor", "index.html"))
	if err != nil {
		t.Fatalf("expected author page output: %v", err)
	}
	if !strings.Contains(string(authorPage), "author Jane Editor") {
		t.Fatalf("expected author layout output, got %q", string(authorPage))
	}

	notFoundPage, err := os.ReadFile(filepath.Join(cfg.PublicDir, "404.html"))
	if err != nil {
		t.Fatalf("expected 404 output: %v", err)
	}
	if !strings.Contains(string(notFoundPage), "404 Page not found") {
		t.Fatalf("expected 404 layout output, got %q", string(notFoundPage))
	}
}

func TestRenderURLTracksThemeChangesOnSameRenderer(t *testing.T) {
	cfg := testRendererConfig(t)
	writeRendererTheme(t, cfg)
	if err := os.MkdirAll(filepath.Join(cfg.ThemesDir, "alt", "layouts", "partials"), 0o755); err != nil {
		t.Fatalf("mkdir alt theme: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(cfg.ThemesDir, "alt", "layouts", "base.html"),
		[]byte(`{{ define "base" }}<html><body><main class="alt-main">{{ template "page" . }}</main></body></html>{{ end }}`),
		0o644,
	); err != nil {
		t.Fatalf("write alt base: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(cfg.ThemesDir, "alt", "layouts", "page.html"),
		[]byte(`{{ define "page" }}<article>alt page</article>{{ end }}`),
		0o644,
	); err != nil {
		t.Fatalf("write alt page: %v", err)
	}

	graph := content.NewSiteGraph(cfg)
	graph.Add(&content.Document{
		ID:         "page-1",
		Type:       "page",
		Lang:       cfg.DefaultLang,
		Title:      "About",
		Slug:       "about",
		URL:        "/about/",
		Layout:     "page",
		SourcePath: filepath.ToSlash(filepath.Join(cfg.ContentDir, "pages", "about.md")),
		HTMLBody:   template.HTML("<p>About</p>"),
	})

	r := New(cfg, theme.NewManager(cfg.ThemesDir, cfg.Theme), nil)
	first, err := r.RenderURL(graph, "/about/", false)
	if err != nil {
		t.Fatalf("render default theme: %v", err)
	}
	if !strings.Contains(string(first), `page About`) {
		t.Fatalf("expected default theme layout, got %q", string(first))
	}

	cfg.Theme = "alt"
	second, err := r.RenderURL(graph, "/about/", false)
	if err != nil {
		t.Fatalf("render alt theme: %v", err)
	}
	if !strings.Contains(string(second), `class="alt-main"`) {
		t.Fatalf("expected alt theme layout after theme switch, got %q", string(second))
	}
}

func TestBuildURLsSkipsUnknownURLs(t *testing.T) {
	cfg := testRendererConfig(t)
	writeRendererTheme(t, cfg)

	r := New(cfg, theme.NewManager(cfg.ThemesDir, cfg.Theme), nil)
	graph := content.NewSiteGraph(cfg)

	if err := r.BuildURLs(context.Background(), graph, []string{"/missing/"}); err != nil {
		t.Fatalf("expected missing URL to be skipped, got %v", err)
	}

	if _, err := os.Stat(filepath.Join(cfg.PublicDir, "missing", "index.html")); !os.IsNotExist(err) {
		t.Fatalf("expected no output for missing URL, got err=%v", err)
	}
}

func TestRendererHelpersAndRenderTemplate(t *testing.T) {
	cfg := testRendererConfig(t)
	cfg.Menus = map[string][]config.MenuItem{
		"main": {
			{Name: "Home", URL: "/"},
			{Name: "Docs", URL: "docs"},
		},
	}
	writeRendererTheme(t, cfg)

	graph := content.NewSiteGraph(cfg)
	doc := &content.Document{
		ID:         "doc-1",
		Type:       "page",
		Lang:       cfg.DefaultLang,
		Title:      "About",
		Slug:       "about",
		URL:        "/about/",
		Layout:     "post",
		SourcePath: filepath.ToSlash(filepath.Join(cfg.ContentDir, "pages", "about.md")),
		HTMLBody:   template.HTML("<p>Hello</p>"),
		Fields:     map[string]any{"hero": "Hero"},
	}
	graph.Add(doc)

	r := New(cfg, theme.NewManager(cfg.ThemesDir, cfg.Theme), rendererHooks{})
	if got := r.taxonomyURL("en", "tags", "go"); got != "/tags/go/" {
		t.Fatalf("unexpected default taxonomy URL: %q", got)
	}
	if got := r.taxonomyURL("fr", "tags", "go"); got != "/fr/tags/go/" {
		t.Fatalf("unexpected translated taxonomy URL: %q", got)
	}
	if _, ok := r.findDocumentByID(graph, "missing"); ok {
		t.Fatal("expected missing document lookup to fail")
	}
	if docs := r.documentsForLang(graph, "en"); len(docs) != 1 || docs[0].ID != "doc-1" {
		t.Fatalf("unexpected documentsForLang: %#v", docs)
	}
	if !containsString([]string{"a", "b"}, "a") || containsString([]string{"a"}, "b") {
		t.Fatal("unexpected containsString behavior")
	}

	slots := NewSlots()
	slots.Add("body.end", "")
	slots.Add("body.end", "<div>x</div>")
	if !strings.Contains(string(slots.Render("body.end")), "x") || slots.Render("missing") != "" {
		t.Fatalf("unexpected slot rendering: %q", slots.Render("body.end"))
	}
	if (*Slots)(nil).Render("x") != "" {
		t.Fatal("expected nil slots render to be empty")
	}

	assets := NewAssetSet()
	assets.AddStyle("")
	assets.AddStyle("/app.css")
	assets.AddStyle("/app.css")
	assets.AddScript("", ScriptPositionHead)
	assets.AddScript("/head.js", ScriptPositionHead)
	assets.AddScript("/head.js", ScriptPositionHead)
	assets.AddScript("/body.js", ScriptPositionBodyEnd)
	assets.RenderInto(slots)
	if rendered := string(slots.Render("head.end")); !strings.Contains(rendered, "app.css") || !strings.Contains(rendered, "head.js") {
		t.Fatalf("unexpected head assets: %q", rendered)
	}

	if got := normalizeNavURL("docs"); got != "/docs/" {
		t.Fatalf("unexpected normalized nav URL: %q", got)
	}
	if got := normalizeNavURL("https://example.com/x"); got != "https://example.com/x" {
		t.Fatalf("unexpected external nav URL: %q", got)
	}
	if !navItemIsActive("/docs/", "/docs/page/") || !navItemIsActive("https://x", "https://x") || navItemIsActive("", "/") {
		t.Fatal("unexpected nav active behavior")
	}
	if nav := parseNavigationData(map[string]any{"main": []any{map[string]any{"name": "Docs", "url": "docs"}, "bad"}}); len(nav) != 1 || nav[0].URL != "/docs/" {
		t.Fatalf("unexpected parsed navigation: %#v", nav)
	}
	if nav := parseNavigationData("bad"); nav != nil {
		t.Fatalf("expected nil navigation from bad input, got %#v", nav)
	}

	html, err := r.renderTemplate("post", "/about/", ViewData{
		Site: cfg,
		Page: doc,
		Data: map[string]any{"navigation": map[string]any{"main": []any{map[string]any{"name": "Data", "url": "data"}}}},
		Lang: "en", Title: "About",
	})
	if err != nil || !strings.Contains(string(html), "post About") || !strings.Contains(string(html), "<!--after-->") {
		t.Fatalf("unexpected renderTemplate result: %v %q", err, string(html))
	}

	searchHTML, err := r.RenderURLWithQuery(graph, "/search/", "q=hello", false)
	if err != nil || !strings.Contains(string(searchHTML), "search Search: hello hello") {
		t.Fatalf("unexpected search render result: %v %q", err, string(searchHTML))
	}

	authorHTML, err := r.RenderURL(graph, "/authors/jane-editor/", false)
	if err == nil {
		t.Fatalf("expected missing author archive before author is added")
	}
	doc.Author = "Jane Editor"
	authorHTML, err = r.RenderURL(graph, "/authors/jane-editor/", false)
	if err != nil || !strings.Contains(string(authorHTML), "author Jane Editor") {
		t.Fatalf("unexpected author render result: %v %q", err, string(authorHTML))
	}

	notFoundHTML, err := r.RenderNotFoundPage(graph, "/missing/", false)
	if err != nil || !strings.Contains(string(notFoundHTML), "404 Page not found /missing/") {
		t.Fatalf("unexpected 404 render result: %v %q", err, string(notFoundHTML))
	}
}

func TestRendererHookFailuresAndBuild(t *testing.T) {
	cfg := testRendererConfig(t)
	cfg.Build.CleanPublicDir = true
	writeRendererTheme(t, cfg)

	graph := content.NewSiteGraph(cfg)
	graph.Add(&content.Document{
		ID:         "doc-1",
		Type:       "post",
		Lang:       cfg.DefaultLang,
		Title:      "Hello",
		Slug:       "hello",
		URL:        "/posts/hello/",
		Layout:     "post",
		SourcePath: filepath.ToSlash(filepath.Join(cfg.ContentDir, "posts", "hello.md")),
		HTMLBody:   template.HTML("<p>Hello</p>"),
		Taxonomies: map[string][]string{"tags": {"go"}},
	})

	failures := []string{"context", "assets", "slots", "before", "after"}
	for _, failOn := range failures {
		t.Run(failOn, func(t *testing.T) {
			r := New(cfg, theme.NewManager(cfg.ThemesDir, cfg.Theme), rendererHooks{failOn: failOn})
			if _, err := r.renderTemplate("post", "/posts/hello/", ViewData{Site: cfg, Page: graph.Documents[0], Lang: "en"}); err == nil {
				t.Fatalf("expected renderTemplate failure for %s", failOn)
			}
		})
	}

	r := New(cfg, theme.NewManager(cfg.ThemesDir, cfg.Theme), rendererHooks{})
	if err := r.Build(context.Background(), graph); err != nil {
		t.Fatalf("build renderer output: %v", err)
	}
	if _, err := os.Stat(filepath.Join(cfg.PublicDir, "posts", "hello", "index.html")); err != nil {
		t.Fatalf("expected built page output: %v", err)
	}
	if _, err := os.Stat(filepath.Join(cfg.PublicDir, "tags", "go", "index.html")); err != nil {
		t.Fatalf("expected built taxonomy output: %v", err)
	}
	if _, err := os.Stat(filepath.Join(cfg.PublicDir, "theme", "js", "foundry-theme.js")); err != nil {
		t.Fatalf("expected frontend theme sdk bootstrap asset to be copied: %v", err)
	}
	searchIndex, err := os.ReadFile(filepath.Join(cfg.PublicDir, "search.json"))
	if err != nil {
		t.Fatalf("expected search index output: %v", err)
	}
	if !strings.Contains(string(searchIndex), `"url": "/posts/hello/"`) {
		t.Fatalf("expected search index to include rendered document, got %q", string(searchIndex))
	}
	platformSite, err := os.ReadFile(filepath.Join(cfg.PublicDir, "__foundry", "site.json"))
	if err != nil {
		t.Fatalf("expected platform site metadata output: %v", err)
	}
	if !strings.Contains(string(platformSite), cfg.Title) {
		t.Fatalf("expected platform site metadata to include site title, got %q", string(platformSite))
	}
	if _, err := os.Stat(filepath.Join(cfg.PublicDir, "__foundry", "preview.json")); err != nil {
		t.Fatalf("expected preview manifest artifact: %v", err)
	}
	if _, err := os.Stat(filepath.Join(cfg.PublicDir, "__foundry", "sdk", "frontend", "index.js")); err != nil {
		t.Fatalf("expected frontend sdk asset to be copied: %v", err)
	}
}

func testRendererConfig(t *testing.T) *config.Config {
	t.Helper()

	root := t.TempDir()
	cfg := &config.Config{
		Title:       "Foundry",
		DefaultLang: "en",
		Theme:       "default",
		ContentDir:  filepath.Join(root, "content"),
		PublicDir:   filepath.Join(root, "public"),
		ThemesDir:   filepath.Join(root, "themes"),
		DataDir:     filepath.Join(root, "data"),
		Feed: config.FeedConfig{
			RSSPath:     "/rss.xml",
			SitemapPath: "/sitemap.xml",
		},
		Taxonomies: config.TaxonomyConfig{
			DefaultSet: []string{"tags"},
			Definitions: map[string]config.TaxonomyDefinition{
				"tags": {TermLayout: "taxonomy-term", Title: "Tags"},
			},
		},
	}
	cfg.ApplyDefaults()
	return cfg
}

func writeRendererTheme(t *testing.T, cfg *config.Config) {
	t.Helper()

	files := map[string]string{
		filepath.Join(cfg.ThemesDir, cfg.Theme, "layouts", "base.html"):               `{{ define "base" }}{{ template "content" . }}{{ end }}`,
		filepath.Join(cfg.ThemesDir, cfg.Theme, "layouts", "post.html"):               `{{ define "content" }}post {{ .Page.Title }} {{ field .Page "hero" }} {{ pluginSlot "body.end" }}{{ end }}`,
		filepath.Join(cfg.ThemesDir, cfg.Theme, "layouts", "page.html"):               `{{ define "content" }}page {{ .Title }}{{ end }}`,
		filepath.Join(cfg.ThemesDir, cfg.Theme, "layouts", "list.html"):               `{{ define "content" }}list {{ .Title }}{{ end }}`,
		filepath.Join(cfg.ThemesDir, cfg.Theme, "layouts", "search.html"):             `{{ define "content" }}search {{ .Title }} {{ .SearchQuery }} {{ range .Documents }}{{ .Slug }} {{ end }}{{ end }}`,
		filepath.Join(cfg.ThemesDir, cfg.Theme, "layouts", "author.html"):             `{{ define "content" }}author {{ .AuthorName }} {{ range .Documents }}{{ .Slug }} {{ end }}{{ end }}`,
		filepath.Join(cfg.ThemesDir, cfg.Theme, "layouts", "404.html"):                `{{ define "content" }}404 {{ .Title }} {{ .RequestPath }}{{ end }}`,
		filepath.Join(cfg.ThemesDir, cfg.Theme, "layouts", "index.html"):              `{{ define "content" }}index{{ end }}`,
		filepath.Join(cfg.ThemesDir, cfg.Theme, "layouts", "taxonomy-term.html"):      `{{ define "content" }}taxonomy layout for {{ .TaxonomyName }}/{{ .TaxonomyTerm }}{{ end }}`,
		filepath.Join(cfg.ThemesDir, cfg.Theme, "layouts", "partials", "head.html"):   `{{ define "head" }}{{ end }}`,
		filepath.Join(cfg.ThemesDir, cfg.Theme, "layouts", "partials", "header.html"): `{{ define "header" }}{{ end }}`,
		filepath.Join(cfg.ThemesDir, cfg.Theme, "layouts", "partials", "footer.html"): `{{ define "footer" }}{{ end }}`,
		filepath.Join(cfg.ThemesDir, cfg.Theme, "assets", "js", "foundry-theme.js"):   `console.log("frontend sdk bootstrap");`,
	}

	for path, body := range files {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", path, err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
}
