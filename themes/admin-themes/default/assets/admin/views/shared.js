import { adminPathForSection, normalizeAdminSection } from '../core/router.js';
import { escapeHTML, formatDateTime, lifecycleLabel } from '../core/utils.js';

export const renderTableControls = (state, tableName, totalCount, totalPages) => {
  const table = state.tables[tableName];
  if (!table) return '';
  const choices = {
    documents: ['title', 'type', 'status', 'lang'],
    media: ['name', 'kind', 'reference'],
    users: ['username', 'name', 'email'],
    audit: ['timestamp', 'action', 'actor', 'outcome'],
    plugins: ['name', 'status', 'version'],
    themes: ['name', 'version', 'valid'],
  }[tableName] || [table.sort];
  const options = Array.from(new Set([table.sort, ...choices]))
    .map(
      (choice) =>
        `<option value="${escapeHTML(choice)}" ${table.sort === choice ? 'selected' : ''}>${escapeHTML(choice)}</option>`
    )
    .join('');
  return `<div class="table-controls">
    <label>Sort
      <select data-table-sort="${tableName}">
        ${options}
      </select>
    </label>
    <button type="button" class="ghost small" data-table-dir="${tableName}">${table.dir === 'asc' ? 'Asc' : 'Desc'}</button>
    <div class="table-paging">
      <button type="button" class="ghost small" data-table-page="${tableName}|prev" ${table.page <= 1 ? 'disabled' : ''}>Prev</button>
      <span class="muted">Page ${table.page} / ${totalPages} • ${totalCount} items</span>
      <button type="button" class="ghost small" data-table-page="${tableName}|next" ${table.page >= totalPages ? 'disabled' : ''}>Next</button>
    </div>
  </div>`;
};

export const renderBreadcrumbs = (state, sectionTitles) => {
  const titleForSection =
    typeof sectionTitles === 'function'
      ? sectionTitles
      : (section) => sectionTitles[section] || 'Overview';
  const trail = ['Admin', titleForSection(state.section) || 'Overview'];
  if (
    (state.section === 'documents' || state.section === 'editor') &&
    state.documentEditor.source_path
  ) {
    trail.push(state.documentEditor.source_path);
  } else if (state.section === 'media' && state.selectedMediaReference) {
    trail.push(state.selectedMediaReference);
  } else if (state.section === 'history' && state.documentHistoryPath) {
    trail.push(state.documentHistoryPath);
  } else if (state.section === 'users' && state.userForm.username) {
    trail.push(state.userForm.username);
  }
  return `<nav class="breadcrumbs">${trail.map((part, index) => `${index ? '<span>/</span>' : ''}<span>${escapeHTML(part)}</span>`).join(' ')}</nav>`;
};

export const renderToasts = (state) => {
  if (!state.toasts.length) return '';
  return `<div class="toast-stack">${state.toasts.map((toast) => `<div class="toast ${escapeHTML(toast.tone)}">${escapeHTML(toast.message)}</div>`).join('')}</div>`;
};

export const renderUpdateNotice = (state) => {
  if (!state.updateInfo?.has_update) return '';
  return `<button type="button" class="panel warning-panel" data-section="operations" aria-label="Open Operations to review available Foundry update">
    <div class="panel-pad">
      <strong>Foundry ${escapeHTML(state.updateInfo.latest_version || 'update')} is available</strong>
      <div class="muted">${escapeHTML(state.updateInfo.instructions || 'Open Operations to review the release and update options.')}</div>
    </div>
  </button>`;
};

export const renderKeyboardHelp = (state) => {
  if (!state.keyboardHelp) return '';
  return `<div class="shortcut-help">
    <strong>Keyboard Shortcuts</strong>
    <div><code>Cmd/Ctrl+S</code> Save current form</div>
    <div><code>Cmd/Ctrl+Enter</code> Preview current document</div>
    <div><code>Cmd/Ctrl+K</code> Open command palette</div>
    <div><code>Shift+/</code> Toggle shortcut help</div>
    <div><code>Use the command palette</code> Navigate sections and run quick actions</div>
  </div>`;
};

export const summarizeLoadErrors = (state) => {
  if (!state.loadErrors.length) {
    return '';
  }
  return `Some admin data could not be loaded: ${state.loadErrors.join(', ')}`;
};

export const mediaPreview = (item) => {
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

export const mediaThumb = (item) => {
  if (!item) return '<span class="media-thumb placeholder">-</span>';
  const url = escapeHTML(item.public_url);
  switch (item.kind) {
    case 'image':
      return `<img class="media-thumb" src="${url}" alt="${escapeHTML(item.metadata?.alt || item.name)}">`;
    case 'video':
      return `<video class="media-thumb" src="${url}" muted preload="metadata"></video>`;
    case 'audio':
      return '<div class="media-thumb audio">AUDIO</div>';
    default:
      return '<div class="media-thumb file">FILE</div>';
  }
};

export const shellNav = (state, adminBase, options = {}) => {
  const currentSection = normalizeAdminSection(state.section);
  const items = [
    ['overview', 'Overview'],
    ['documents', 'Documents'],
    ['editor', 'Editor'],
    ['history', 'History'],
    ['trash', 'Trash'],
    ['media', 'Media'],
    ...(options.debugEnabled ? [['debug', 'Debug']] : []),
    ['audit', 'Audit'],
    ['users', 'Users'],
    ['settings', 'Settings'],
    ['extensions', 'Extensions'],
    ['plugins', 'Plugins'],
    ['themes', 'Themes'],
    ['operations', 'Operations'],
  ];
  const extensionPages = Array.isArray(options.extensionPages) ? options.extensionPages : [];
  const builtins = items.map(
    ([key, label]) =>
      `<a class="foundry-nav-item${currentSection === key ? ' active' : ''}" href="${adminPathForSection(adminBase, key)}" data-section="${key}">${label}</a>`
  );
  const extensions = extensionPages.map(
    (page) =>
      `<a class="foundry-nav-item foundry-nav-item-extension${currentSection === normalizeAdminSection(page.section) ? ' active' : ''}" href="${adminPathForSection(adminBase, page.section)}" data-section="${page.section}" data-extension-page="${escapeHTML(page.key)}">${escapeHTML(page.title)}</a>`
  );
  return builtins.concat(extensions).join('');
};

export const panel = (title, body, subtitle = '', actions = '') => `
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

export const documentStatusLabel = (doc) => {
  const status = String(doc?.status || '').trim();
  switch (status) {
    case 'in_review':
      return 'In Review';
    case 'scheduled':
      return 'Scheduled';
    case 'archived':
      return 'Archived';
    case 'published':
      return 'Published';
    case 'draft':
    default:
      if (doc?.archived) return 'Archived';
      if (doc?.draft) return 'Draft';
      return 'Draft';
  }
};

export const renderDocumentHistoryRows = (entries) =>
  entries
    .map(
      (entry) => `
  <div class="table-row table-row-actions">
    <span>
      <strong>${escapeHTML(entry.title || entry.slug || entry.path)}</strong>
      <div class="muted mono">${escapeHTML(entry.path)}</div>
      ${entry.status ? `<div class="muted">Status: ${escapeHTML(documentStatusLabel(entry))}</div>` : ''}
      ${entry.version_comment ? `<div class="muted">Note: ${escapeHTML(entry.version_comment)}</div>` : ''}
      ${entry.actor ? `<div class="muted">By ${escapeHTML(entry.actor)}</div>` : ''}
      ${entry.author || entry.last_editor ? `<div class="muted">Author ${escapeHTML(entry.author || '-')} • Last editor ${escapeHTML(entry.last_editor || '-')}</div>` : ''}
    </span>
    <span>${escapeHTML(lifecycleLabel(entry.state))}</span>
    <span>${escapeHTML(formatDateTime(entry.timestamp) || 'Current')}</span>
    <span class="row-actions">
      ${
        entry.state === 'current'
          ? '<span class="muted">Current</span>'
          : `
        <button class="ghost small" data-restore-document="${escapeHTML(entry.path)}">Restore</button>
        <button class="ghost small" data-preview-restore-document="${escapeHTML(entry.path)}">Preview Restore</button>
        <button class="ghost small danger" data-purge-document="${escapeHTML(entry.path)}">Purge</button>`
      }
    </span>
  </div>`
    )
    .join('');

export const renderMediaHistoryRows = (entries) =>
  entries
    .map(
      (entry) => `
  <div class="table-row table-row-actions">
    <span>
      <strong>${escapeHTML(entry.name || entry.path)}</strong>
      <div class="muted mono">${escapeHTML(entry.path)}</div>
      ${entry.metadata_only ? '<div class="muted">Metadata revision</div>' : ''}
      ${entry.version_comment ? `<div class="muted">Note: ${escapeHTML(entry.version_comment)}</div>` : ''}
      ${entry.actor ? `<div class="muted">By ${escapeHTML(entry.actor)}</div>` : ''}
    </span>
    <span>${escapeHTML(lifecycleLabel(entry.state))}</span>
    <span>${escapeHTML(formatDateTime(entry.timestamp) || 'Current')}</span>
    <span class="row-actions">
      ${
        entry.state === 'current'
          ? entry.public_url
            ? `<a class="button-link ghost small" href="${escapeHTML(entry.public_url)}" target="_blank" rel="noreferrer">View</a>`
            : '<span class="muted">Current</span>'
          : `
          <button class="ghost small" data-restore-media-path="${escapeHTML(entry.path)}">Restore</button>
          <button class="ghost small danger" data-purge-media-path="${escapeHTML(entry.path)}">Purge</button>`
      }
    </span>
  </div>`
    )
    .join('');

export const renderTrashSelectionRows = (entries, selected, kind) =>
  entries
    .map(
      (entry) => `
  <div class="table-row table-row-actions">
    <span>
      <label class="checkbox inline-checkbox">
        <input type="checkbox" ${selected.includes(entry.path) ? 'checked' : ''} data-select-trash="${escapeHTML(entry.path)}" data-trash-kind="${kind}">
        <strong>${escapeHTML(entry.title || entry.name || entry.slug || entry.path)}</strong>
      </label>
      <div class="muted mono">${escapeHTML(entry.path)}</div>
      ${entry.version_comment ? `<div class="muted">${escapeHTML(entry.version_comment)}</div>` : ''}
    </span>
    <span>${escapeHTML(lifecycleLabel(entry.state))}</span>
    <span>${escapeHTML(formatDateTime(entry.timestamp) || 'Current')}</span>
    <span class="row-actions">
      ${
        kind === 'document'
          ? `<button class="ghost small" data-restore-document="${escapeHTML(entry.path)}">Restore</button>
           <button class="ghost small danger" data-purge-document="${escapeHTML(entry.path)}">Purge</button>`
          : `<button class="ghost small" data-restore-media-path="${escapeHTML(entry.path)}">Restore</button>
           <button class="ghost small danger" data-purge-media-path="${escapeHTML(entry.path)}">Purge</button>`
      }
    </span>
  </div>`
    )
    .join('');

export const renderOverview = (state) => {
  const content = state.status?.content || {};
  const runtime = state.runtimeStatus || {};
  const inReview = (state.documents || []).filter((doc) => doc.status === 'in_review');
  const scheduled = (state.documents || []).filter((doc) => doc.status === 'scheduled');
  const cards = `
    <div class="cards">
      <article class="card"><span class="card-label">Documents</span><strong>${escapeHTML(content.document_count ?? 0)}</strong><span class="card-copy">Loaded into the current graph.</span></article>
      <article class="card"><span class="card-label">Drafts</span><strong>${escapeHTML(content.draft_count ?? 0)}</strong><span class="card-copy">Draft and archived content.</span></article>
      <article class="card"><span class="card-label">In Review</span><strong>${escapeHTML(inReview.length)}</strong><span class="card-copy">Documents waiting on review.</span></article>
      <article class="card"><span class="card-label">Scheduled</span><strong>${escapeHTML(scheduled.length)}</strong><span class="card-copy">Documents with publish windows.</span></article>
      <article class="card"><span class="card-label">Media</span><strong>${escapeHTML(state.media.length)}</strong><span class="card-copy">Images, uploads, and asset files.</span></article>
      <article class="card"><span class="card-label">Users</span><strong>${escapeHTML(state.users.length)}</strong><span class="card-copy">Filesystem-backed admin accounts.</span></article>
      <article class="card"><span class="card-label">Settings Sections</span><strong>${escapeHTML(state.settingsSections.length)}</strong><span class="card-copy">Core and plugin-defined settings groups.</span></article>
      <article class="card"><span class="card-label">Admin Extensions</span><strong>${escapeHTML((state.adminExtensions.pages?.length || 0) + (state.adminExtensions.widgets?.length || 0) + (state.adminExtensions.settings?.length || 0))}</strong><span class="card-copy">Plugin-defined pages, widgets, and settings entries.</span></article>
      <article class="card"><span class="card-label">Broken Refs</span><strong>${escapeHTML((runtime.integrity?.broken_media_refs || 0) + (runtime.integrity?.broken_internal_links || 0))}</strong><span class="card-copy">Media and internal-link validation findings.</span></article>
      <article class="card"><span class="card-label">Active Sessions</span><strong>${escapeHTML(runtime.activity?.active_sessions || 0)}</strong><span class="card-copy">Persisted admin sessions.</span></article>
      <article class="card"><span class="card-label">Active Locks</span><strong>${escapeHTML(runtime.activity?.active_document_locks || 0)}</strong><span class="card-copy">Documents currently being edited.</span></article>
      <article class="card"><span class="card-label">Validate Site</span><strong>${escapeHTML(state.siteValidation?.message_count || 0)}</strong><span class="card-copy">Latest admin validation findings.</span></article>
      <article class="card"><span class="card-label">Release</span><strong>${escapeHTML(state.updateInfo?.has_update ? state.updateInfo.latest_version || 'available' : state.updateInfo?.current_version || 'current')}</strong><span class="card-copy">${escapeHTML(state.updateInfo?.has_update ? 'New release available.' : 'Running the latest known release.')}</span></article>
    </div>`;
  const queueSection = `<div class="layout-grid">
    <section class="panel">
      <div class="panel-header"><div><h2>Review Queue</h2><div class="muted">${escapeHTML(String(inReview.length))} documents in review</div></div><div class="toolbar"><button type="button" class="ghost small" data-section="documents">Open Documents</button></div></div>
      ${
        inReview.length
          ? `<div class="mini-list panel-pad">${inReview
              .slice(0, 5)
              .map(
                (doc) =>
                  `<div class="mini-list-row"><span>${escapeHTML(doc.title || doc.slug || doc.source_path)}</span><strong>${escapeHTML(doc.lang || 'default')}</strong></div>`
              )
              .join('')}</div>`
          : '<div class="panel-pad empty-state">No documents are currently waiting for review.</div>'
      }
    </section>
    <section class="panel">
      <div class="panel-header"><div><h2>Scheduled Queue</h2><div class="muted">${escapeHTML(String(scheduled.length))} scheduled documents</div></div><div class="toolbar"><button type="button" class="ghost small" data-section="documents">Open Documents</button></div></div>
      ${
        scheduled.length
          ? `<div class="mini-list panel-pad">${scheduled
              .slice(0, 5)
              .map(
                (doc) =>
                  `<div class="mini-list-row"><span>${escapeHTML(doc.title || doc.slug || doc.source_path)}</span><strong>${escapeHTML(doc.lang || 'default')}</strong></div>`
              )
              .join('')}</div>`
          : '<div class="panel-pad empty-state">No documents are currently scheduled.</div>'
      }
    </section>
  </div>`;
  return (
    cards +
    queueSection +
    `<div class="layout-grid">
      <section class="panel">
        <div class="panel-header"><div><h2>Integrity</h2><div class="muted">Current runtime validation snapshot</div></div><div class="toolbar"><button type="button" class="ghost small" id="overview-validate-site">Run Validation</button><button type="button" class="ghost small" data-section="debug">Open Debug</button></div></div>
        <div class="panel-pad mini-list">
          <div class="mini-list-row"><span>Broken media refs</span><strong>${escapeHTML(runtime.integrity?.broken_media_refs || 0)}</strong></div>
          <div class="mini-list-row"><span>Broken internal links</span><strong>${escapeHTML(runtime.integrity?.broken_internal_links || 0)}</strong></div>
          <div class="mini-list-row"><span>Missing templates</span><strong>${escapeHTML(runtime.integrity?.missing_templates || 0)}</strong></div>
          <div class="mini-list-row"><span>Orphaned media</span><strong>${escapeHTML(runtime.integrity?.orphaned_media || 0)}</strong></div>
          <div class="mini-list-row"><span>Duplicate URLs/slugs</span><strong>${escapeHTML((runtime.integrity?.duplicate_urls || 0) + (runtime.integrity?.duplicate_slugs || 0))}</strong></div>
        </div>
      </section>
      <section class="panel">
        <div class="panel-header"><div><h2>Recent Activity</h2><div class="muted">${escapeHTML(runtime.activity?.recent_audit_events || 0)} audit events in window</div></div></div>
        <div class="panel-pad mini-list">
          ${Object.entries(runtime.activity?.recent_audit_by_action || {})
            .slice(0, 6)
            .map(
              ([action, count]) =>
                `<div class="mini-list-row"><span>${escapeHTML(action)}</span><strong>${escapeHTML(count)}</strong></div>`
            )
            .join('') || '<div class="empty-state">No recent audit activity yet.</div>'}
        </div>
      </section>
    </div>` +
    (state.loadErrors.length
      ? `<div class="panel"><div class="panel-pad"><div class="error">${escapeHTML(summarizeLoadErrors(state))}</div></div></div>`
      : '')
  );
};
