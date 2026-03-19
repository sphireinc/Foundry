(() => {
  const root = document.getElementById('app');
  if (!root) return;

  const adminBase = root.dataset.adminBase || '/__admin';
  const sectionForPath = (pathname) => {
    const path = pathname.replace(/\/+$/, '');
    if (path === '/__admin' || path === '') return 'overview';
    return path.replace(/^\/__admin\/?/, '') || 'overview';
  };

  const escapeHTML = (value) => String(value ?? '')
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&#39;');

  const state = {
    session: null,
    status: null,
    documents: [],
    media: [],
    users: [],
    config: null,
    plugins: [],
    themes: [],
    section: sectionForPath(window.location.pathname),
    documentEditor: { source_path: '', raw: '' },
    documentPreview: null,
    selectedMediaReference: '',
    mediaDetail: null,
    userForm: { username: '', name: '', email: '', role: '', password: '', disabled: false },
    loadErrors: [],
    error: '',
    flash: '',
    loading: false
  };

  const formatDate = (value) => value.toISOString().slice(0, 10);

  const buildDefaultMarkdown = (kind = 'post') => {
    const today = formatDate(new Date());
    if (kind === 'page') {
      return [
        '---',
        'title: ',
        'slug: ',
        'layout: page',
        'draft: false',
        'summary: ',
        '---',
        '',
        '# Title',
        ''
      ].join('\n');
    }
    return [
      '---',
      'title: ',
      'slug: ',
      'layout: post',
      'draft: true',
      'summary: ',
      `date: ${today}`,
      'tags: []',
      'categories: []',
      '---',
      '',
      '# Title',
      ''
    ].join('\n');
  };

  const request = async (path, options = {}) => {
    const baseHeaders = options.body instanceof FormData ? {} : { 'Content-Type': 'application/json' };
    const response = await fetch(adminBase + path, {
      credentials: 'same-origin',
      headers: { ...baseHeaders, ...(options.headers || {}) },
      ...options
    });
    const payload = await response.json().catch(() => ({}));
    if (!response.ok) {
      throw new Error(payload.error || `request failed for ${path}`);
    }
    return payload;
  };

  const navigate = (section) => {
    state.section = section;
    const nextPath = section === 'overview' ? '/__admin' : `/__admin/${section}`;
    if (window.location.pathname !== nextPath) {
      window.history.pushState({}, '', nextPath);
    }
    render();
  };

  const resetUserForm = () => {
    state.userForm = { username: '', name: '', email: '', role: '', password: '', disabled: false };
  };

  const resetDocumentEditor = () => {
    const createKind = document.getElementById('document-create-kind')?.value || 'post';
    state.documentEditor = { source_path: '', raw: buildDefaultMarkdown(createKind) };
    state.documentPreview = null;
  };

  const setFlash = (message) => {
    state.flash = message;
    state.error = '';
  };

  const clearLoadErrors = () => {
    state.loadErrors = [];
  };

  const summarizeLoadErrors = () => {
    if (!state.loadErrors.length) {
      return '';
    }
    return `Some admin data could not be loaded: ${state.loadErrors.join(', ')}`;
  };

  const mediaPreview = (item) => {
    if (!item) {
      return '<div class="empty-state">Select media to preview and edit metadata.</div>';
    }
    const url = escapeHTML(item.public_url);
    switch (item.kind) {
      case 'image':
        return `<img class="media-preview" src="${url}" alt="${escapeHTML(item.metadata?.alt || item.name)}">`;
      case 'video':
        return `<video class="media-preview" controls preload="metadata" src="${url}"></video>`;
      case 'audio':
        return `<audio class="media-audio" controls preload="metadata" src="${url}"></audio>`;
      default:
        return `<a class="file-link" href="${url}" target="_blank" rel="noreferrer">${escapeHTML(item.name)}</a>`;
    }
  };

  const shellNav = () => {
    const items = [
      ['overview', 'Overview'],
      ['documents', 'Documents'],
      ['media', 'Media'],
      ['users', 'Users'],
      ['config', 'Configuration'],
      ['plugins', 'Plugins'],
      ['themes', 'Themes']
    ];
    return items.map(([key, label]) => `<a class="wp-nav-item${state.section === key ? ' active' : ''}" href="/__admin/${key === 'overview' ? '' : key}" data-section="${key}">${label}</a>`).join('');
  };

  const panel = (title, body, subtitle = '', actions = '') => `
    <section class="panel">
      <div class="panel-header">
        <div>
          <h2>${escapeHTML(title)}</h2>
          ${subtitle ? `<div class="muted">${escapeHTML(subtitle)}</div>` : ''}
        </div>
        ${actions}
      </div>
      ${body}
    </section>`;

  const renderOverview = () => {
    const content = state.status?.content || {};
    const cards = `
      <div class="cards">
        <article class="card"><span class="card-label">Documents</span><strong>${escapeHTML(content.document_count ?? 0)}</strong><span class="card-copy">Loaded into the current graph.</span></article>
        <article class="card"><span class="card-label">Drafts</span><strong>${escapeHTML(content.draft_count ?? 0)}</strong><span class="card-copy">Draft and archived content.</span></article>
        <article class="card"><span class="card-label">Media</span><strong>${escapeHTML(state.media.length)}</strong><span class="card-copy">Images, uploads, and asset files.</span></article>
        <article class="card"><span class="card-label">Users</span><strong>${escapeHTML(state.users.length)}</strong><span class="card-copy">Filesystem-backed admin accounts.</span></article>
      </div>`;
    return cards + (state.loadErrors.length ? `<div class="panel"><div class="panel-pad"><div class="error">${escapeHTML(summarizeLoadErrors())}</div></div></div>` : '');
  };

  const documentStatusLabel = (doc) => {
    if (doc.archived) return 'Archived';
    if (doc.draft) return 'Draft';
    return 'Published';
  };

  const renderDocuments = () => {
    const rows = state.documents.map((doc) => `
      <div class="table-row table-row-actions">
        <span><strong>${escapeHTML(doc.title || doc.slug || doc.id)}</strong><div class="muted mono">${escapeHTML(doc.source_path)}</div></span>
        <span>${escapeHTML(doc.type)}</span>
        <span>${escapeHTML(documentStatusLabel(doc))}</span>
        <span class="row-actions">
          <button class="ghost small" data-edit-document="${escapeHTML(doc.id)}">Edit</button>
          <button class="ghost small" data-set-document-status="${escapeHTML(doc.source_path)}|published">Publish</button>
          <button class="ghost small" data-set-document-status="${escapeHTML(doc.source_path)}|draft">Draft</button>
          <button class="ghost small" data-set-document-status="${escapeHTML(doc.source_path)}|archived">Archive</button>
          <button class="ghost small danger" data-delete-document="${escapeHTML(doc.source_path)}">Delete</button>
        </span>
      </div>`);

    const editor = `
      <div class="panel-pad stack">
        <form id="document-create-form" class="inline-form">
          <label>Kind
            <select id="document-create-kind">
              <option value="page">page</option>
              <option value="post" selected>post</option>
            </select>
          </label>
          <label>Slug<input id="document-create-slug" type="text" placeholder="launch-notes"></label>
          <label>Lang<input id="document-create-lang" type="text" placeholder="en"></label>
          <label>Archetype<input id="document-create-archetype" type="text" placeholder="post"></label>
          <button type="submit">Create Document</button>
        </form>
        <form id="document-save-form" class="stack">
          <label>Source Path<input id="document-source-path" type="text" value="${escapeHTML(state.documentEditor.source_path)}" placeholder="content/pages/about.md"></label>
          <label>Raw Markdown<textarea id="document-raw" rows="18" spellcheck="false">${escapeHTML(state.documentEditor.raw)}</textarea></label>
          <div class="toolbar">
            <button type="submit">Save Document</button>
            <button type="button" class="ghost" id="document-preview-button">Preview</button>
            <button type="button" class="ghost" id="document-reset-button">New Draft</button>
          </div>
        </form>
      </div>`;

    const preview = state.documentPreview
      ? `<div class="panel-pad preview-body">${state.documentPreview.html}</div>`
      : `<div class="panel-pad empty-state">Use Preview to render the current Markdown body.</div>`;

    return `
      <div class="layout-grid">
        <div class="stack">
          ${panel('Documents', `<div class="table table-four"><div class="table-head"><span>Document</span><span>Type</span><span>Status</span><span>Actions</span></div>${rows.length ? rows.join('') : '<div class="panel-pad empty-state">No documents available.</div>'}</div>`, `${state.documents.length} documents`)}
        </div>
        <div class="stack">
          ${panel('Editor', editor, 'Create, edit, publish, archive, or soft-delete Markdown content')}
          ${panel('Preview', preview, state.documentPreview ? state.documentPreview.title || state.documentPreview.slug || 'Rendered preview' : 'No preview yet')}
        </div>
      </div>`;
  };

  const renderMedia = () => {
    const rows = state.media.map((item) => `
      <div class="table-row table-row-actions">
        <span><strong>${escapeHTML(item.name)}</strong><div class="muted mono">${escapeHTML(item.reference)}</div></span>
        <span>${escapeHTML(item.kind)}</span>
        <span>${escapeHTML(item.metadata?.title || item.metadata?.alt || '')}</span>
        <span class="row-actions">
          <button class="ghost small" data-edit-media="${escapeHTML(item.reference)}">Details</button>
          <a class="button-link ghost small" href="${escapeHTML(item.public_url)}" target="_blank" rel="noreferrer">View</a>
          <button class="ghost small danger" data-delete-media="${escapeHTML(item.reference)}">Delete</button>
        </span>
      </div>`);

    const detail = state.mediaDetail;
    const metadata = detail?.metadata || {};
    return `
      <div class="layout-grid">
        <div class="stack">
          ${panel('Upload Media', `
            <form id="media-upload-form" class="panel-pad stack">
              <label>Collection<select id="media-collection"><option value="">Auto</option><option value="images">images</option><option value="uploads">uploads</option></select></label>
              <label>Directory<input id="media-dir" type="text" placeholder="posts/launch"></label>
              <label>File<input id="media-file" type="file"></label>
              <button type="submit">Upload Media</button>
            </form>
          `, 'Uploads return stable media: references that can be used in Markdown')}
          ${panel('Library', `<div class="table table-four"><div class="table-head"><span>Name</span><span>Kind</span><span>Metadata</span><span>Actions</span></div>${rows.length ? rows.join('') : '<div class="panel-pad empty-state">No media found yet.</div>'}</div>`, `${state.media.length} media items`)}
        </div>
        <div class="stack">
          ${panel('Selected Media', `
            <div class="panel-pad stack">
              ${mediaPreview(detail)}
              <div class="status-line mono">${escapeHTML(detail?.reference || '')}</div>
              <form id="media-metadata-form" class="stack">
                <label>Title<input id="media-title" type="text" value="${escapeHTML(metadata.title || '')}"></label>
                <label>Alt Text<input id="media-alt" type="text" value="${escapeHTML(metadata.alt || '')}"></label>
                <label>Caption<input id="media-caption" type="text" value="${escapeHTML(metadata.caption || '')}"></label>
                <label>Description<textarea id="media-description" rows="5">${escapeHTML(metadata.description || '')}</textarea></label>
                <label>Credit<input id="media-credit" type="text" value="${escapeHTML(metadata.credit || '')}"></label>
                <label>Tags<input id="media-tags" type="text" value="${escapeHTML((metadata.tags || []).join(', '))}" placeholder="product, hero, launch"></label>
                <button type="submit" ${detail ? '' : 'disabled'}>Save Metadata</button>
              </form>
            </div>
          `, 'Metadata is stored beside the file as .meta.yaml')}
        </div>
      </div>`;
  };

  const renderUsers = () => {
    const rows = state.users.map((user) => `
      <div class="table-row table-row-actions">
        <span><strong>${escapeHTML(user.username)}</strong></span>
        <span>${escapeHTML(user.name || '')}</span>
        <span>${escapeHTML(user.email || '')}</span>
        <span class="row-actions">
          <button class="ghost small" data-edit-user="${escapeHTML(user.username)}">Edit</button>
          <button class="ghost small danger" data-delete-user="${escapeHTML(user.username)}">Delete</button>
        </span>
      </div>`);
    return `
      <div class="layout-grid">
        <div class="stack">
          ${panel('Users', `<div class="table table-four"><div class="table-head"><span>Username</span><span>Name</span><span>Email</span><span>Actions</span></div>${rows.join('')}</div>`, `${state.users.length} users`)}
        </div>
        <div class="stack">
          ${panel('User Editor', `
            <form id="user-save-form" class="panel-pad stack">
              <label>Username<input id="user-username" type="text" value="${escapeHTML(state.userForm.username)}" placeholder="editor"></label>
              <label>Name<input id="user-name" type="text" value="${escapeHTML(state.userForm.name)}" placeholder="Editor User"></label>
              <label>Email<input id="user-email" type="email" value="${escapeHTML(state.userForm.email)}" placeholder="editor@example.com"></label>
              <label>Role<input id="user-role" type="text" value="${escapeHTML(state.userForm.role)}" placeholder="editor"></label>
              <label>Password<input id="user-password" type="password" value="" placeholder="Leave blank to keep current password"></label>
              <label class="checkbox"><input id="user-disabled" type="checkbox" ${state.userForm.disabled ? 'checked' : ''}> Disabled</label>
              <div class="toolbar">
                <button type="submit">Save User</button>
                <button type="button" class="ghost" id="user-reset-button">New User</button>
              </div>
            </form>
          `, 'Users are stored in content/config/admin-users.yaml')}
        </div>
      </div>`;
  };

  const renderConfig = () => panel('Configuration', `
    <form id="config-save-form" class="panel-pad stack">
      <label>Config file<textarea id="config-raw" rows="24">${escapeHTML(state.config?.raw || '')}</textarea></label>
      <button type="submit">Save Configuration</button>
    </form>`, state.config?.path || 'content/config/site.yaml');

  const renderPlugins = () => {
    const rows = state.plugins.map((plugin) => `
      <div class="table-row table-row-actions">
        <span><strong>${escapeHTML(plugin.title || plugin.name)}</strong></span>
        <span>${escapeHTML(plugin.version || '-')}</span>
        <span>${escapeHTML(plugin.status)}</span>
        <span class="row-actions">
          ${plugin.enabled
            ? `<button class="ghost small" data-disable-plugin="${escapeHTML(plugin.name)}">Disable</button>`
            : `<button class="ghost small" data-enable-plugin="${escapeHTML(plugin.name)}">Enable</button>`}
        </span>
      </div>`);
    return panel('Plugins', `<div class="table table-four"><div class="table-head"><span>Plugin</span><span>Version</span><span>Status</span><span>Action</span></div>${rows.join('')}</div>`, `${state.plugins.length} plugins`);
  };

  const renderThemes = () => {
    const rows = state.themes.map((theme) => `
      <div class="table-row table-row-actions">
        <span><strong>${escapeHTML(theme.title || theme.name)}</strong></span>
        <span>${escapeHTML(theme.version || '-')}</span>
        <span>${theme.valid ? 'valid' : 'invalid'}</span>
        <span class="row-actions">
          ${theme.current ? '<span class="muted">Current</span>' : `<button class="ghost small" data-switch-theme="${escapeHTML(theme.name)}">Activate</button>`}
        </span>
      </div>`);
    return panel('Themes', `<div class="table table-four"><div class="table-head"><span>Theme</span><span>Version</span><span>Validation</span><span>Action</span></div>${rows.join('')}</div>`, `${state.themes.length} frontend themes`);
  };

  const renderSection = () => {
    switch (state.section) {
      case 'documents': return renderDocuments();
      case 'media': return renderMedia();
      case 'users': return renderUsers();
      case 'config': return renderConfig();
      case 'plugins': return renderPlugins();
      case 'themes': return renderThemes();
      default: return renderOverview();
    }
  };

  const renderLogin = () => {
    root.innerHTML = `
      <div class="login-shell">
        <div class="login-card">
          <div class="login-mark">F</div>
          <h1>Foundry Admin</h1>
          <p class="login-copy">Sign in to manage documents, media, users, configuration, themes, and plugins.</p>
          <form id="login-form" class="login-form">
            <label>Username<input id="username" type="text" autocomplete="username" placeholder="admin"></label>
            <label>Password<input id="password" type="password" autocomplete="current-password" placeholder="Password"></label>
            <button type="submit">Log In</button>
          </form>
          <p class="login-hint">Sessions expire after 30 minutes of inactivity and renew while you are active.</p>
          <div class="error">${escapeHTML(state.error)}</div>
        </div>
      </div>`;

    document.getElementById('login-form').addEventListener('submit', async (event) => {
      event.preventDefault();
      const username = document.getElementById('username').value;
      const password = document.getElementById('password').value;
      state.loading = true;
      render();
      try {
        await request('/api/login', {
          method: 'POST',
          body: JSON.stringify({
            username,
            password
          })
        });
        await fetchAll();
      } catch (error) {
        state.loading = false;
        state.error = error.message || String(error);
        render();
      }
    });
  };

  const bindDashboardEvents = () => {
    root.querySelectorAll('[data-section]').forEach((element) => {
      element.addEventListener('click', (event) => {
        event.preventDefault();
        navigate(element.dataset.section);
      });
    });

    document.getElementById('logout')?.addEventListener('click', async () => {
      try {
        await request('/api/logout', { method: 'POST' });
      } catch (_error) {
      }
      state.session = null;
      state.flash = '';
      state.error = '';
      render();
    });

    document.getElementById('document-create-form')?.addEventListener('submit', async (event) => {
      event.preventDefault();
      try {
        const created = await request('/api/documents/create', {
          method: 'POST',
          body: JSON.stringify({
            kind: document.getElementById('document-create-kind').value,
            slug: document.getElementById('document-create-slug').value,
            lang: document.getElementById('document-create-lang').value,
            archetype: document.getElementById('document-create-archetype').value
          })
        });
        state.documentEditor = { source_path: created.source_path, raw: created.raw || '' };
        setFlash('Document created.');
        await fetchAll(false);
        navigate('documents');
      } catch (error) {
        state.error = error.message || String(error);
        render();
      }
    });

    document.getElementById('document-save-form')?.addEventListener('submit', async (event) => {
      event.preventDefault();
      try {
        await request('/api/documents/save', {
          method: 'POST',
          body: JSON.stringify({
            source_path: document.getElementById('document-source-path').value,
            raw: document.getElementById('document-raw').value
          })
        });
        state.documentEditor = {
          source_path: document.getElementById('document-source-path').value,
          raw: document.getElementById('document-raw').value
        };
        setFlash('Document saved.');
        await fetchAll(false);
        navigate('documents');
      } catch (error) {
        state.error = error.message || String(error);
        render();
      }
    });

    document.getElementById('document-preview-button')?.addEventListener('click', async () => {
      try {
        state.documentPreview = await request('/api/documents/preview', {
          method: 'POST',
          body: JSON.stringify({
            source_path: document.getElementById('document-source-path').value,
            raw: document.getElementById('document-raw').value
          })
        });
        setFlash('Preview rendered.');
        render();
      } catch (error) {
        state.error = error.message || String(error);
        render();
      }
    });

    document.getElementById('document-reset-button')?.addEventListener('click', () => {
      resetDocumentEditor();
      setFlash('Editor reset.');
      render();
    });

    root.querySelectorAll('[data-edit-document]').forEach((button) => {
      button.addEventListener('click', async () => {
        try {
          const detail = await request(`/api/document?id=${encodeURIComponent(button.dataset.editDocument)}&include_drafts=1`);
          state.documentEditor = { source_path: detail.source_path, raw: detail.raw_body };
          state.documentPreview = null;
          setFlash('Document loaded.');
          navigate('documents');
        } catch (error) {
          state.error = error.message || String(error);
          render();
        }
      });
    });

    root.querySelectorAll('[data-set-document-status]').forEach((button) => {
      button.addEventListener('click', async () => {
        const [sourcePath, status] = button.dataset.setDocumentStatus.split('|');
        try {
          await request('/api/documents/status', {
            method: 'POST',
            body: JSON.stringify({ source_path: sourcePath, status })
          });
          setFlash(`Document moved to ${status}.`);
          await fetchAll(false);
          navigate('documents');
        } catch (error) {
          state.error = error.message || String(error);
          render();
        }
      });
    });

    root.querySelectorAll('[data-delete-document]').forEach((button) => {
      button.addEventListener('click', async () => {
        try {
          await request('/api/documents/delete', {
            method: 'POST',
            body: JSON.stringify({ source_path: button.dataset.deleteDocument })
          });
          resetDocumentEditor();
          setFlash('Document moved to trash.');
          await fetchAll(false);
          navigate('documents');
        } catch (error) {
          state.error = error.message || String(error);
          render();
        }
      });
    });

    document.getElementById('media-upload-form')?.addEventListener('submit', async (event) => {
      event.preventDefault();
      const file = document.getElementById('media-file').files[0];
      if (!file) {
        state.error = 'Choose a file to upload.';
        render();
        return;
      }
      try {
        const formData = new FormData();
        formData.append('file', file);
        formData.append('dir', document.getElementById('media-dir').value);
        formData.append('collection', document.getElementById('media-collection').value);
        const uploaded = await request('/api/media/upload', { method: 'POST', body: formData });
        state.selectedMediaReference = uploaded.reference;
        setFlash('Media uploaded.');
        await fetchAll(false);
        await loadMediaDetail(uploaded.reference, false);
        navigate('media');
      } catch (error) {
        state.error = error.message || String(error);
        render();
      }
    });

    root.querySelectorAll('[data-edit-media]').forEach((button) => {
      button.addEventListener('click', async () => {
        await loadMediaDetail(button.dataset.editMedia);
      });
    });

    document.getElementById('media-metadata-form')?.addEventListener('submit', async (event) => {
      event.preventDefault();
      if (!state.selectedMediaReference) return;
      try {
        state.mediaDetail = await request('/api/media/metadata', {
          method: 'POST',
          body: JSON.stringify({
            reference: state.selectedMediaReference,
            metadata: {
              title: document.getElementById('media-title').value,
              alt: document.getElementById('media-alt').value,
              caption: document.getElementById('media-caption').value,
              description: document.getElementById('media-description').value,
              credit: document.getElementById('media-credit').value,
              tags: document.getElementById('media-tags').value.split(',').map((tag) => tag.trim()).filter(Boolean)
            }
          })
        });
        setFlash('Media metadata saved.');
        await fetchAll(false);
        navigate('media');
      } catch (error) {
        state.error = error.message || String(error);
        render();
      }
    });

    root.querySelectorAll('[data-delete-media]').forEach((button) => {
      button.addEventListener('click', async () => {
        try {
          await request('/api/media/delete', {
            method: 'POST',
            body: JSON.stringify({ reference: button.dataset.deleteMedia })
          });
          if (state.selectedMediaReference === button.dataset.deleteMedia) {
            state.selectedMediaReference = '';
            state.mediaDetail = null;
          }
          setFlash('Media deleted.');
          await fetchAll(false);
          navigate('media');
        } catch (error) {
          state.error = error.message || String(error);
          render();
        }
      });
    });

    root.querySelectorAll('[data-edit-user]').forEach((button) => {
      button.addEventListener('click', () => {
        const user = state.users.find((item) => item.username === button.dataset.editUser);
        if (!user) return;
        state.userForm = {
          username: user.username,
          name: user.name || '',
          email: user.email || '',
          role: user.role || '',
          password: '',
          disabled: !!user.disabled
        };
        setFlash(`Editing ${user.username}.`);
        navigate('users');
      });
    });

    document.getElementById('user-save-form')?.addEventListener('submit', async (event) => {
      event.preventDefault();
      try {
        await request('/api/users/save', {
          method: 'POST',
          body: JSON.stringify({
            username: document.getElementById('user-username').value,
            name: document.getElementById('user-name').value,
            email: document.getElementById('user-email').value,
            role: document.getElementById('user-role').value,
            password: document.getElementById('user-password').value,
            disabled: document.getElementById('user-disabled').checked
          })
        });
        resetUserForm();
        setFlash('User saved.');
        await fetchAll(false);
        navigate('users');
      } catch (error) {
        state.error = error.message || String(error);
        render();
      }
    });

    document.getElementById('user-reset-button')?.addEventListener('click', () => {
      resetUserForm();
      setFlash('User form reset.');
      render();
    });

    root.querySelectorAll('[data-delete-user]').forEach((button) => {
      button.addEventListener('click', async () => {
        try {
          await request('/api/users/delete', {
            method: 'POST',
            body: JSON.stringify({ username: button.dataset.deleteUser })
          });
          resetUserForm();
          setFlash('User deleted.');
          await fetchAll(false);
          navigate('users');
        } catch (error) {
          state.error = error.message || String(error);
          render();
        }
      });
    });

    document.getElementById('config-save-form')?.addEventListener('submit', async (event) => {
      event.preventDefault();
      try {
        await request('/api/config/save', {
          method: 'POST',
          body: JSON.stringify({ raw: document.getElementById('config-raw').value })
        });
        setFlash('Configuration saved.');
        await fetchAll(false);
        navigate('config');
      } catch (error) {
        state.error = error.message || String(error);
        render();
      }
    });

    root.querySelectorAll('[data-enable-plugin]').forEach((button) => {
      button.addEventListener('click', async () => {
        try {
          await request('/api/plugins/enable', { method: 'POST', body: JSON.stringify({ name: button.dataset.enablePlugin }) });
          setFlash('Plugin enabled.');
          await fetchAll(false);
          navigate('plugins');
        } catch (error) {
          state.error = error.message || String(error);
          render();
        }
      });
    });

    root.querySelectorAll('[data-disable-plugin]').forEach((button) => {
      button.addEventListener('click', async () => {
        try {
          await request('/api/plugins/disable', { method: 'POST', body: JSON.stringify({ name: button.dataset.disablePlugin }) });
          setFlash('Plugin disabled.');
          await fetchAll(false);
          navigate('plugins');
        } catch (error) {
          state.error = error.message || String(error);
          render();
        }
      });
    });

    root.querySelectorAll('[data-switch-theme]').forEach((button) => {
      button.addEventListener('click', async () => {
        try {
          await request('/api/themes/switch', { method: 'POST', body: JSON.stringify({ name: button.dataset.switchTheme }) });
          setFlash('Theme switched.');
          await fetchAll(false);
          navigate('themes');
        } catch (error) {
          state.error = error.message || String(error);
          render();
        }
      });
    });
  };

  const renderDashboard = () => {
    const topMessage = state.error || summarizeLoadErrors() || state.flash || 'WordPress-style admin shell for managing content, media, users, configuration, themes, and plugins.';
    root.innerHTML = `
      <div class="wp-shell">
        <aside class="wp-sidebar">
          <div class="wp-brand">Foundry</div>
          <nav class="wp-nav">${shellNav()}</nav>
          <div class="wp-sidebar-footer">Admin theme: ${escapeHTML(root.dataset.theme || 'default')}</div>
        </aside>
        <div class="wp-main">
          <header class="wp-topbar">
            <div>
              <h1>${escapeHTML(state.section.charAt(0).toUpperCase() + state.section.slice(1))}</h1>
              <p>${escapeHTML(topMessage)}</p>
            </div>
            <div class="wp-topbar-actions">
              <div class="chrome-user"><strong>${escapeHTML(state.session?.name || state.session?.username || '')}</strong><span>${escapeHTML(state.session?.email || '')}</span></div>
              <button class="ghost" id="logout">Log Out</button>
            </div>
          </header>
          <main class="wp-content">
            ${state.error ? `<div class="panel error-panel"><div class="panel-pad"><div class="error">${escapeHTML(state.error)}</div></div></div>` : ''}
            ${renderSection()}
          </main>
        </div>
      </div>`;
    bindDashboardEvents();
  };

  const render = () => {
    if (!state.session || !state.session.authenticated) {
      renderLogin();
      return;
    }
    renderDashboard();
  };

  const loadMediaDetail = async (reference, rerender = true) => {
    try {
      state.mediaDetail = await request(`/api/media/detail?reference=${encodeURIComponent(reference)}`);
      state.selectedMediaReference = reference;
      setFlash('Media loaded.');
      if (rerender) {
        navigate('media');
      }
    } catch (error) {
      state.error = error.message || String(error);
      render();
    }
  };

  const fetchAll = async (rerender = true) => {
    state.loading = true;
    state.error = '';
    clearLoadErrors();
    try {
      state.session = await request('/api/session');

      const results = await Promise.allSettled([
        request('/api/status'),
        request('/api/documents?include_drafts=1'),
        request('/api/media'),
        request('/api/users'),
        request('/api/config'),
        request('/api/plugins'),
        request('/api/themes')
      ]);

      const assignResult = (index, label, onSuccess, fallback) => {
        const result = results[index];
        if (result.status === 'fulfilled') {
          onSuccess(result.value);
          return;
        }
        fallback();
        state.loadErrors.push(label);
      };

      assignResult(0, 'status', (value) => { state.status = value; }, () => { state.status = null; });
      assignResult(1, 'documents', (value) => { state.documents = Array.isArray(value) ? value : []; }, () => { state.documents = []; });
      assignResult(2, 'media', (value) => { state.media = Array.isArray(value) ? value : []; }, () => { state.media = []; });
      assignResult(3, 'users', (value) => { state.users = Array.isArray(value) ? value : []; }, () => { state.users = []; });
      assignResult(4, 'config', (value) => { state.config = value; }, () => { state.config = null; });
      assignResult(5, 'plugins', (value) => { state.plugins = Array.isArray(value) ? value : []; }, () => { state.plugins = []; });
      assignResult(6, 'themes', (value) => { state.themes = Array.isArray(value) ? value : []; }, () => { state.themes = []; });

      if (state.selectedMediaReference) {
        const matching = state.media.find((item) => item.reference === state.selectedMediaReference);
        if (matching) {
          state.mediaDetail = matching;
        } else {
          state.selectedMediaReference = '';
          state.mediaDetail = null;
        }
      }
    } catch (error) {
      state.session = null;
      state.error = error.message || String(error);
    } finally {
      state.loading = false;
      if (rerender) {
        render();
      }
    }
  };

  window.addEventListener('popstate', () => {
    state.section = sectionForPath(window.location.pathname);
    render();
  });

  fetchAll();
  if (!state.documentEditor.raw) {
    state.documentEditor.raw = buildDefaultMarkdown('post');
  }
})();
