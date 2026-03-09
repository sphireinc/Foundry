package theme

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	foundryconfig "github.com/sphireinc/foundry/internal/config"
)

type Info struct {
	Name string
	Path string
}

func ListInstalled(themesDir string) ([]Info, error) {
	entries, err := os.ReadDir(themesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []Info{}, nil
		}
		return nil, err
	}

	out := make([]Info, 0)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		out = append(out, Info{
			Name: name,
			Path: filepath.Join(themesDir, name),
		})
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})

	return out, nil
}

func ValidateInstalled(themesDir, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("theme name cannot be empty")
	}

	root := filepath.Join(themesDir, name)
	info, err := os.Stat(root)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("theme %q does not exist", name)
		}
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("theme path %q is not a directory", root)
	}

	required := []string{
		filepath.Join(root, "layouts", "base.html"),
		filepath.Join(root, "layouts", "index.html"),
		filepath.Join(root, "layouts", "page.html"),
		filepath.Join(root, "layouts", "post.html"),
		filepath.Join(root, "layouts", "list.html"),
		filepath.Join(root, "layouts", "partials", "head.html"),
		filepath.Join(root, "layouts", "partials", "header.html"),
		filepath.Join(root, "layouts", "partials", "footer.html"),
	}

	for _, path := range required {
		if _, err := os.Stat(path); err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("missing required theme file: %s", path)
			}
			return err
		}
	}

	return nil
}

func Scaffold(themesDir, name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("theme name cannot be empty")
	}
	if strings.Contains(name, "/") || strings.Contains(name, `\`) {
		return "", fmt.Errorf("theme name must be a single directory name")
	}

	root := filepath.Join(themesDir, name)
	if _, err := os.Stat(root); err == nil {
		return "", fmt.Errorf("theme already exists: %s", root)
	} else if !os.IsNotExist(err) {
		return "", err
	}

	dirs := []string{
		filepath.Join(root, "assets", "css"),
		filepath.Join(root, "layouts"),
		filepath.Join(root, "layouts", "partials"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", err
		}
	}

	files := map[string]string{
		filepath.Join(root, "assets", "css", "base.css"):          scaffoldCSS(),
		filepath.Join(root, "layouts", "base.html"):               scaffoldBase(),
		filepath.Join(root, "layouts", "index.html"):              scaffoldIndex(),
		filepath.Join(root, "layouts", "page.html"):               scaffoldPage(),
		filepath.Join(root, "layouts", "post.html"):               scaffoldPost(),
		filepath.Join(root, "layouts", "list.html"):               scaffoldList(),
		filepath.Join(root, "layouts", "partials", "head.html"):   scaffoldHead(),
		filepath.Join(root, "layouts", "partials", "header.html"): scaffoldHeader(),
		filepath.Join(root, "layouts", "partials", "footer.html"): scaffoldFooter(),
	}

	for path, body := range files {
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			return "", err
		}
	}

	return root, nil
}

func SwitchInConfig(configPath, themeName string) error {
	themeName = strings.TrimSpace(themeName)
	if themeName == "" {
		return fmt.Errorf("theme name cannot be empty")
	}

	return foundryconfig.UpsertTopLevelScalar(configPath, "theme", themeName)
}

func scaffoldCSS() string {
	return `html, body {
  margin: 0;
  padding: 0;
  font-family: system-ui, sans-serif;
  color: #111;
  background: #fff;
}

a {
  color: inherit;
}

.container {
  max-width: 960px;
  margin: 0 auto;
  padding: 2rem;
}

.site-header,
.site-footer {
  border-bottom: 1px solid #ddd;
}

.site-footer {
  border-top: 1px solid #ddd;
  border-bottom: 0;
  margin-top: 3rem;
}

.site-header .container,
.site-footer .container {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.prose {
  line-height: 1.7;
}

.prose img {
  max-width: 100%;
  height: auto;
}
`
}

func scaffoldBase() string {
	return `{{ define "base" }}
<!doctype html>
<html lang="{{ .Lang }}">
  {{ template "head" . }}
  <body>
    {{ pluginSlot "body.start" }}

    {{ template "header" . }}

    {{ pluginSlot "page.before_main" }}

    <main class="site-main">
      <div class="container">
        {{ template "content" . }}
      </div>
    </main>

    {{ pluginSlot "page.after_main" }}

    {{ template "footer" . }}

    {{ pluginSlot "body.end" }}

    {{ if .LiveReload }}
    <script>
      (() => {
        const es = new EventSource('/__reload');
        es.onmessage = () => window.location.reload();
      })();
    </script>
    {{ end }}
  </body>
</html>
{{ end }}
`
}

func scaffoldHead() string {
	return `{{ define "head" }}
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width,initial-scale=1">
  <title>{{ .Title }}</title>
  <link rel="stylesheet" href="/assets/css/foundry.bundle.css">
  {{ pluginSlot "head.end" }}
</head>
{{ end }}
`
}

func scaffoldHeader() string {
	return `{{ define "header" }}
<header class="site-header">
  <div class="container">
    <strong>{{ .Site.Title }}</strong>
    <nav>
      {{ range .Nav }}
        <a href="{{ .URL }}">{{ .Name }}</a>
      {{ end }}
    </nav>
  </div>
</header>
{{ end }}
`
}

func scaffoldFooter() string {
	return `{{ define "footer" }}
<footer class="site-footer">
  <div class="container">
    <small>{{ .Site.Title }}</small>
  </div>
</footer>
{{ end }}
`
}

func scaffoldIndex() string {
	return `{{ define "content" }}
<h1>{{ .Site.Title }}</h1>

{{ if .Documents }}
  <ul>
    {{ range .Documents }}
      <li><a href="{{ .URL }}">{{ .Title }}</a></li>
    {{ end }}
  </ul>
{{ else }}
  <p>No content found.</p>
{{ end }}
{{ end }}
`
}

func scaffoldPage() string {
	return `{{ define "content" }}
<article class="prose">
  <h1>{{ .Page.Title }}</h1>
  {{ pluginSlot "page.before_content" }}
  {{ safeHTML .Page.HTMLBody }}
  {{ pluginSlot "page.after_content" }}
</article>
{{ end }}
`
}

func scaffoldPost() string {
	return `{{ define "content" }}
<article class="prose">
  {{ pluginSlot "post.before_header" }}
  <h1>{{ .Page.Title }}</h1>
  {{ pluginSlot "post.after_header" }}
  {{ pluginSlot "post.before_content" }}
  {{ safeHTML .Page.HTMLBody }}
  {{ pluginSlot "post.after_content" }}
</article>
{{ end }}
`
}

func scaffoldList() string {
	return `{{ define "content" }}
<h1>{{ .Title }}</h1>

{{ if .Documents }}
  <ul>
    {{ range .Documents }}
      <li><a href="{{ .URL }}">{{ .Title }}</a></li>
    {{ end }}
  </ul>
{{ else }}
  <p>No entries found.</p>
{{ end }}
{{ end }}
`
}
