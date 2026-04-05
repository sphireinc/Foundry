package ui

import (
	"bytes"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/sphireinc/foundry/internal/config"
	"github.com/sphireinc/foundry/internal/safepath"
)

type Manager struct {
	cfg *config.Config
}

func NewManager(cfg *config.Config) *Manager {
	return &Manager{cfg: cfg}
}

func (m *Manager) RenderIndex() ([]byte, error) {
	tmplBody, err := m.loadIndexTemplate()
	if err != nil {
		return nil, err
	}

	tmpl, err := template.New("admin-index").Parse(tmplBody)
	if err != nil {
		return nil, err
	}

	data := struct {
		Title       string
		AdminPath   string
		DefaultLang string
		ThemeName   string
		ThemeBase   string
	}{
		Title:       m.cfg.Title,
		AdminPath:   m.cfg.AdminPath(),
		DefaultLang: m.cfg.DefaultLang,
		ThemeName:   m.themeName(),
		ThemeBase:   m.cfg.AdminPath() + "/theme",
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (m *Manager) AssetHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/"))
		name = filepath.ToSlash(filepath.Clean(name))
		name = strings.TrimPrefix(name, "/")
		if name == "." || name == "" {
			http.NotFound(w, r)
			return
		}

		assetsRoot := filepath.Join(m.themeRoot(), "assets")
		path, err := safepath.ResolveRelativeUnderRoot(assetsRoot, filepath.FromSlash(name))
		if err == nil && fileExists(path) {
			w.Header().Set("X-Content-Type-Options", "nosniff")
			http.ServeFile(w, r, path)
			return
		}

		body, contentType, ok := fallbackAsset(name)
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		_, _ = w.Write([]byte(body))
	})
}

func (m *Manager) loadIndexTemplate() (string, error) {
	path := filepath.Join(m.themeRoot(), "index.html")
	if fileExists(path) {
		b, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
	return defaultIndexTemplate, nil
}

func (m *Manager) themeRoot() string {
	return filepath.Join(m.cfg.ThemesDir, "admin-themes", m.themeName())
}

func (m *Manager) themeName() string {
	if m == nil || m.cfg == nil || strings.TrimSpace(m.cfg.Admin.Theme) == "" {
		return "default"
	}
	return strings.TrimSpace(m.cfg.Admin.Theme)
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func fallbackAsset(name string) (string, string, bool) {
	switch name {
	case "admin.css":
		return defaultCSS, "text/css; charset=utf-8", true
	case "admin.js":
		return defaultJS, "application/javascript; charset=utf-8", true
	default:
		return "", "", false
	}
}

const defaultIndexTemplate = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>{{ .Title }} Admin</title>
  <link rel="stylesheet" href="{{ .ThemeBase }}/admin.css">
</head>
<body>
  <div id="app" data-admin-base="{{ .AdminPath }}" data-default-lang="{{ .DefaultLang }}" data-theme="{{ .ThemeName }}">
    <noscript>Foundry admin requires JavaScript.</noscript>
  </div>
  <script type="module" src="{{ .ThemeBase }}/admin.js"></script>
</body>
</html>
`

const defaultCSS = `:root {
  --bg: #f4efe6;
  --panel: rgba(255,255,255,0.85);
  --line: rgba(24,31,41,0.12);
  --text: #182029;
  --muted: #5d6773;
  --accent: #0c7c59;
  --accent-strong: #0a6448;
  --danger: #9d2a2a;
  --shadow: 0 18px 60px rgba(16,24,32,0.08);
  --radius: 22px;
}

* { box-sizing: border-box; }
body {
  margin: 0;
  font-family: "IBM Plex Sans", "Segoe UI", sans-serif;
  color: var(--text);
  background:
    radial-gradient(circle at top left, rgba(12,124,89,0.14), transparent 26rem),
    linear-gradient(180deg, #fbf8f2, #ece5d7);
}

.admin-shell {
  max-width: 1200px;
  margin: 0 auto;
  padding: 32px 20px 48px;
}

.admin-header {
  display: flex;
  justify-content: space-between;
  gap: 16px;
  align-items: flex-end;
  margin-bottom: 24px;
}

.admin-title {
  margin: 0;
  font-size: clamp(2rem, 3vw, 3.1rem);
  letter-spacing: -0.04em;
}

.admin-subtitle {
  margin: 8px 0 0;
  color: var(--muted);
  max-width: 40rem;
}

.admin-badge {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 10px 14px;
  border-radius: 999px;
  background: rgba(255,255,255,0.72);
  border: 1px solid var(--line);
  color: var(--muted);
}

.grid {
  display: grid;
  grid-template-columns: 320px minmax(0, 1fr);
  gap: 20px;
}

.panel {
  background: var(--panel);
  border: 1px solid var(--line);
  border-radius: var(--radius);
  box-shadow: var(--shadow);
  backdrop-filter: blur(18px);
}

.panel-body {
  padding: 20px;
}

.panel-title {
  margin: 0 0 14px;
  font-size: 1rem;
  text-transform: uppercase;
  letter-spacing: 0.08em;
  color: var(--muted);
}

.field, .actions, .stack { display: grid; gap: 12px; }

label {
  display: grid;
  gap: 6px;
  font-size: 0.94rem;
}

input, button, select, textarea {
  font: inherit;
}

input, select, textarea {
  width: 100%;
  padding: 12px 14px;
  border-radius: 14px;
  border: 1px solid var(--line);
  background: rgba(255,255,255,0.92);
  color: var(--text);
}

button {
  border: 0;
  border-radius: 14px;
  padding: 12px 16px;
  background: var(--accent);
  color: white;
  font-weight: 600;
  cursor: pointer;
}

button.secondary {
  background: rgba(24,32,41,0.08);
  color: var(--text);
}

button:hover { background: var(--accent-strong); }
button.secondary:hover { background: rgba(24,32,41,0.14); }

.note, .status-line {
  color: var(--muted);
  font-size: 0.92rem;
}

.error {
  color: var(--danger);
  font-weight: 600;
}

.cards {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 16px;
  margin-bottom: 16px;
}

.stat {
  padding: 18px;
  border-radius: 18px;
  border: 1px solid var(--line);
  background: rgba(255,255,255,0.72);
}

.stat strong {
  display: block;
  font-size: 1.65rem;
  letter-spacing: -0.04em;
}

.stat span {
  color: var(--muted);
  font-size: 0.92rem;
}

.list {
  display: grid;
  gap: 12px;
}

.item {
  padding: 14px 16px;
  border-radius: 16px;
  border: 1px solid var(--line);
  background: rgba(255,255,255,0.72);
}

.item-header {
  display: flex;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 6px;
}

.item-title {
  font-weight: 700;
}

.item-meta, .item-path {
  color: var(--muted);
  font-size: 0.9rem;
}

.media-ref {
  display: inline-block;
  margin-top: 8px;
  padding: 4px 8px;
  border-radius: 999px;
  background: rgba(12,124,89,0.1);
  color: var(--accent-strong);
  font-family: "IBM Plex Mono", monospace;
  font-size: 0.84rem;
}

@media (max-width: 960px) {
  .grid, .cards { grid-template-columns: 1fr; }
}
`

const defaultJS = `(() => {
  const root = document.getElementById('app');
  if (!root) return;

  const adminBase = root.dataset.adminBase || '/__admin';
  const tokenKey = 'foundry.admin.token';

  const state = {
    token: window.localStorage.getItem(tokenKey) || '',
    status: null,
    documents: [],
    media: [],
    error: '',
    loading: false
  };

  const headers = () => {
    const token = state.token.trim();
    return token ? { 'X-Foundry-Admin-Token': token } : {};
  };

  const setToken = (value) => {
    state.token = value.trim();
    if (state.token) {
      window.localStorage.setItem(tokenKey, state.token);
    } else {
      window.localStorage.removeItem(tokenKey);
    }
  };

  const escapeHTML = (value) => String(value ?? '')
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&#39;');

  const renderStatusCards = () => {
    if (!state.status) {
      return '<div class="status-line">Enter a token and load data to inspect the admin API.</div>';
    }
    const content = state.status.content || {};
    const checks = Array.isArray(state.status.checks) ? state.status.checks : [];
    return '<div class="cards">' +
      '<div class="stat"><strong>' + escapeHTML(content.document_count ?? 0) + '</strong><span>documents</span></div>' +
      '<div class="stat"><strong>' + escapeHTML(content.draft_count ?? 0) + '</strong><span>drafts</span></div>' +
      '<div class="stat"><strong>' + escapeHTML(checks.length) + '</strong><span>health checks</span></div>' +
      '</div>';
  };

  const renderDocuments = () => {
    if (!state.documents.length) {
      return '<div class="status-line">No documents loaded yet.</div>';
    }
    return '<div class="list">' + state.documents.map((doc) => (
      '<div class="item">' +
        '<div class="item-header">' +
          '<div class="item-title">' + escapeHTML(doc.title || doc.slug || doc.id) + '</div>' +
          '<div class="item-meta">' + escapeHTML(doc.type) + ' · ' + escapeHTML(doc.lang) + '</div>' +
        '</div>' +
        '<div class="item-path">' + escapeHTML(doc.url || doc.source_path) + '</div>' +
      '</div>'
    )).join('') + '</div>';
  };

  const renderMedia = () => {
    if (!state.media.length) {
      return '<div class="status-line">No uploaded media found yet.</div>';
    }
    return '<div class="list">' + state.media.map((item) => (
      '<div class="item">' +
        '<div class="item-header">' +
          '<div class="item-title">' + escapeHTML(item.name) + '</div>' +
          '<div class="item-meta">' + escapeHTML(item.kind) + ' · ' + escapeHTML(item.collection) + '</div>' +
        '</div>' +
        '<div class="item-path">' + escapeHTML(item.public_url) + '</div>' +
        '<span class="media-ref">' + escapeHTML(item.reference) + '</span>' +
      '</div>'
    )).join('') + '</div>';
  };

  const render = () => {
    root.innerHTML = '' +
      '<div class="admin-shell">' +
        '<header class="admin-header">' +
          '<div>' +
            '<h1 class="admin-title">Foundry Admin</h1>' +
            '<p class="admin-subtitle">Themeable admin shell with token-based API access. Tokens stay in local storage in this browser only.</p>' +
          '</div>' +
          '<div class="admin-badge">API base: ' + escapeHTML(adminBase) + '/api</div>' +
        '</header>' +
        '<div class="grid">' +
          '<section class="panel"><div class="panel-body">' +
            '<h2 class="panel-title">Session</h2>' +
            '<form id="token-form" class="stack">' +
              '<label>Access token<input id="token-input" type="password" autocomplete="off" placeholder="Paste admin token" value="' + escapeHTML(state.token) + '"></label>' +
              '<div class="actions">' +
                '<button type="submit">Load admin data</button>' +
                '<button class="secondary" type="button" id="clear-token">Clear token</button>' +
              '</div>' +
            '</form>' +
            '<p class="note">Accepted header: <code>X-Foundry-Admin-Token</code> or bearer token.</p>' +
            '<form id="upload-form" class="stack">' +
              '<h2 class="panel-title">Upload Media</h2>' +
              '<label>Collection<select id="media-collection"><option value="">Auto</option><option value="images">images</option><option value="videos">videos</option><option value="audio">audio</option><option value="documents">documents</option></select></label>' +
              '<label>File<input id="media-file" type="file"></label>' +
              '<button type="submit">Upload</button>' +
            '</form>' +
            '<div id="session-status" class="status-line"></div>' +
            '<div id="session-error" class="error"></div>' +
          '</div></section>' +
          '<section class="stack">' +
            '<section class="panel"><div class="panel-body"><h2 class="panel-title">Status</h2>' + renderStatusCards() + '</div></section>' +
            '<section class="panel"><div class="panel-body"><h2 class="panel-title">Documents</h2>' + renderDocuments() + '</div></section>' +
            '<section class="panel"><div class="panel-body"><h2 class="panel-title">Media</h2>' + renderMedia() + '</div></section>' +
          '</section>' +
        '</div>' +
      '</div>';

    const tokenForm = document.getElementById('token-form');
    const clearToken = document.getElementById('clear-token');
    const uploadForm = document.getElementById('upload-form');
    const tokenInput = document.getElementById('token-input');
    const sessionStatus = document.getElementById('session-status');
    const sessionError = document.getElementById('session-error');

    sessionStatus.textContent = state.loading ? 'Loading…' : (state.token ? 'Token stored locally in this browser.' : 'Enter a token to load admin data.');
    sessionError.textContent = state.error || '';

    tokenForm.addEventListener('submit', async (event) => {
      event.preventDefault();
      setToken(tokenInput.value);
      await loadAll();
    });

    clearToken.addEventListener('click', () => {
      setToken('');
      state.status = null;
      state.documents = [];
      state.media = [];
      state.error = '';
      render();
    });

    uploadForm.addEventListener('submit', async (event) => {
      event.preventDefault();
      if (!state.token) {
        state.error = 'Enter an admin token before uploading media.';
        render();
        return;
      }
      const fileInput = document.getElementById('media-file');
      const collectionInput = document.getElementById('media-collection');
      const file = fileInput.files && fileInput.files[0];
      if (!file) {
        state.error = 'Choose a file to upload.';
        render();
        return;
      }
      state.loading = true;
      state.error = '';
      render();
      try {
        const form = new FormData();
        form.append('file', file);
        form.append('collection', collectionInput.value);
        const response = await fetch(adminBase + '/api/media/upload', {
          method: 'POST',
          headers: headers(),
          body: form
        });
        if (!response.ok) {
          const payload = await response.json().catch(() => ({}));
          throw new Error(payload.error || 'media upload failed');
        }
        await loadAll();
      } catch (error) {
        state.loading = false;
        state.error = error.message || String(error);
        render();
      }
    });
  };

  const loadJSON = async (path) => {
    const response = await fetch(adminBase + path, { headers: headers() });
    if (!response.ok) {
      const payload = await response.json().catch(() => ({}));
      throw new Error(payload.error || ('request failed for ' + path));
    }
    return response.json();
  };

  const loadAll = async () => {
    if (!state.token) {
      render();
      return;
    }
    state.loading = true;
    state.error = '';
    render();
    try {
      const [status, documents, media] = await Promise.all([
        loadJSON('/api/status'),
        loadJSON('/api/documents?include_drafts=1'),
        loadJSON('/api/media')
      ]);
      state.status = status;
      state.documents = Array.isArray(documents) ? documents : [];
      state.media = Array.isArray(media) ? media : [];
    } catch (error) {
      state.error = error.message || String(error);
    } finally {
      state.loading = false;
      render();
    }
  };

  render();
  if (state.token) {
    void loadAll();
  }
})();
`
