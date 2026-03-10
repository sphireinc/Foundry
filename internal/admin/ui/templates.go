package ui

import (
	"fmt"
	"html"
)

func IndexHTML(title string) string {
	escTitle := html.EscapeString(title)
	return fmt.Sprintf(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <title>%s Admin</title>
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <style>
    body { font-family: system-ui, sans-serif; max-width: 860px; margin: 40px auto; padding: 0 20px; line-height: 1.5; }
    h1 { margin-bottom: 0.25rem; }
    code { background: #f4f4f4; padding: 2px 6px; border-radius: 6px; }
    ul { padding-left: 20px; }
    .muted { color: #666; }
  </style>
</head>
<body>
  <h1>%s Admin</h1>
  <p class="muted">Filesystem-first admin boundary for Foundry.</p>

  <h2>Available endpoints</h2>
  <ul>
    <li><code>GET /__admin/api/status</code></li>
    <li><code>GET /__admin/api/documents</code></li>
    <li><code>GET /__admin/api/document?id=&lt;document-id-or-path&gt;</code></li>
    <li><code>POST /__admin/api/documents/save</code></li>
    <li><code>POST /__admin/api/documents/preview</code></li>
  </ul>
</body>
</html>`, escTitle, escTitle)
}
