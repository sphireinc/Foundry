import { adminPathForSection, normalizeAdminSection } from '../core/router.js';
import { _t } from '../core/i18n.js';
import { escapeHTML, formatDateTime, lifecycleLabel } from '../core/utils.js';

export const renderTableControls = (state, tableName, totalCount, totalPages) => {
  const table = state.tables[tableName];
  if (!table) return '';
  const choices = {
    documents: ['title', 'type', 'status', 'lang'],
    media: ['name', 'kind', 'reference'],
    sessions: ['last_seen', 'issued_at', 'expires_at', 'username'],
    users: ['username', 'name', 'email'],
    audit: ['timestamp', 'action', 'actor', 'outcome'],
    plugins: ['name', 'status', 'version'],
    themes: ['name', 'version', 'valid'],
    redirects: ['from', 'to', 'status', 'enabled'],
  }[tableName] || [table.sort];
  const options = Array.from(new Set([table.sort, ...choices]))
    .map(
      (choice) =>
        `<option value="${escapeHTML(choice)}" ${table.sort === choice ? 'selected' : ''}>${escapeHTML(_t(choice))}</option>`
    )
    .join('');
  return `<div class="table-controls">
    <label>${_t('Sort')}
      <select data-table-sort="${tableName}">
        ${options}
      </select>
    </label>
    <button type="button" class="ghost small" data-table-dir="${tableName}">${table.dir === 'asc' ? _t('Asc') : _t('Desc')}</button>
    <div class="table-paging">
      <button type="button" class="ghost small" data-table-page="${tableName}|prev" ${table.page <= 1 ? 'disabled' : ''}>${_t('Prev')}</button>
      <span class="muted">${_t('Page {page} / {pages} • {count} items', { page: table.page, pages: totalPages, count: totalCount })}</span>
      <button type="button" class="ghost small" data-table-page="${tableName}|next" ${table.page >= totalPages ? 'disabled' : ''}>${_t('Next')}</button>
    </div>
  </div>`;
};

export const renderBreadcrumbs = (state, sectionTitles) => {
  const titleForSection =
    typeof sectionTitles === 'function'
      ? sectionTitles
      : (section) => sectionTitles[section] || 'Overview';
  const trail = [_t('Admin'), titleForSection(state.section) || _t('Overview')];
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
  if (state.updateInfo?.install_mode === 'source' && state.updateInfo?.dirty) return '';
  return `<button type="button" class="panel warning-panel" data-section="operations" aria-label="${_t('Open Operations to review available Foundry update')}">
    <div class="panel-pad">
      <strong>${_t('Foundry {version} is available', { version: escapeHTML(state.updateInfo.latest_version || _t('update')) })}</strong>
      <div class="muted">${escapeHTML(state.updateInfo.instructions || _t('Open Operations to review the release and update options.'))}</div>
    </div>
  </button>`;
};

export const renderKeyboardHelp = (state) => {
  if (!state.keyboardHelp) return '';
  return `<div class="shortcut-help">
    <strong>${_t('Keyboard Shortcuts')}</strong>
    <div><code>Cmd/Ctrl+S</code> ${_t('Save current form')}</div>
    <div><code>Cmd/Ctrl+Enter</code> ${_t('Preview current document')}</div>
    <div><code>Cmd/Ctrl+K</code> ${_t('Open command palette')}</div>
    <div><code>Shift+/</code> ${_t('Toggle shortcut help')}</div>
    <div><code>${_t('Use the command palette')}</code> ${_t('Navigate sections and run quick actions')}</div>
  </div>`;
};

export const summarizeLoadErrors = (state) => {
  if (!state.loadErrors.length) {
    return '';
  }
  return _t('Some admin data could not be loaded: {items}', { items: state.loadErrors.join(', ') });
};

export const mediaPreview = (item) => {
  if (!item) {
    return `<div class="empty-state">${_t('Select media to preview and edit metadata.')}</div>`;
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
      return `<div class="media-thumb audio">${_t('AUDIO')}</div>`;
    default:
      return `<div class="media-thumb file">${_t('FILE')}</div>`;
  }
};

export const shellNav = (state, adminBase, options = {}) => {
  const currentSection = normalizeAdminSection(state.section);
  const canAccessSection =
    typeof options.canAccessSection === 'function' ? options.canAccessSection : () => true;
  const builtinSectionGroup =
    typeof options.builtinSectionGroup === 'function' ? options.builtinSectionGroup : () => 'admin';
  const navGroups = [
    { key: 'dashboard', label: _t('Dashboard') },
    { key: 'content', label: _t('Content') },
    { key: 'manage', label: _t('Manage') },
    { key: 'admin', label: _t('Admin') },
  ];
  const items = [
    ['overview', 'Overview'],
    ['documents', 'Documents'],
    ['editor', 'Editor'],
    ['history', 'History'],
    ['trash', 'Trash'],
    ['media', 'Media'],
    ['sessions', 'Sessions'],
    ['users', 'Users'],
    ['custom-fields', 'Custom Fields'],
    ['audit', 'Audit'],
    ['settings', 'Settings'],
    ['redirects', 'Redirects'],
    ['extensions', 'Extensions'],
    ['plugins', 'Plugins'],
    ['themes', 'Themes'],
    ['operations', 'Operations'],
    ...(options.debugEnabled
      ? [
          ['diagnostics', 'Diagnostics'],
          ['debug', 'Debug'],
        ]
      : []),
  ];
  const extensionPages = Array.isArray(options.extensionPages) ? options.extensionPages : [];
  const grouped = new Map(navGroups.map((group) => [group.key, []]));
  items
    .filter(([key]) => canAccessSection(key))
    .forEach(([key, label]) => {
      const groupKey = builtinSectionGroup(key);
      if (!grouped.has(groupKey)) grouped.set(groupKey, []);
      grouped
        .get(groupKey)
        .push(
          `<a class="foundry-nav-item${currentSection === key ? ' active' : ''}" href="${adminPathForSection(adminBase, key)}" data-section="${key}">${escapeHTML(_t(label))}</a>`
        );
    });
  extensionPages.forEach((page) => {
    const groupKey = String(page.navGroup || 'admin')
      .trim()
      .toLowerCase();
    if (!grouped.has(groupKey)) grouped.set(groupKey, []);
    grouped
      .get(groupKey)
      .push(
        `<a class="foundry-nav-item foundry-nav-item-extension${currentSection === normalizeAdminSection(page.section) ? ' active' : ''}" href="${adminPathForSection(adminBase, page.section)}" data-section="${page.section}" data-extension-page="${escapeHTML(page.key)}">${escapeHTML(page.title)}</a>`
      );
  });
  return navGroups
    .map((group) => {
      const links = grouped.get(group.key) || [];
      if (!links.length) return '';
      return `<section class="foundry-nav-group"><div class="foundry-nav-group-label">${escapeHTML(group.label)}</div>${links.join('')}</section>`;
    })
    .join('');
};

export const panel = (title, body, subtitle = '', actions = '', translateCopy = true) => `
  <section class="panel">
    <div class="panel-header">
      <div>
        <h2>${escapeHTML(translateCopy ? _t(title) : title)}</h2>
        ${subtitle ? `<div class="muted">${escapeHTML(translateCopy ? _t(subtitle) : subtitle)}</div>` : ''}
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
      ${entry.status ? `<div class="muted">${_t('Status')}: ${escapeHTML(_t(documentStatusLabel(entry)))}</div>` : ''}
      ${entry.version_comment ? `<div class="muted">${_t('Note')}: ${escapeHTML(entry.version_comment)}</div>` : ''}
      ${entry.actor ? `<div class="muted">${_t('By')} ${escapeHTML(entry.actor)}</div>` : ''}
      ${entry.author || entry.last_editor ? `<div class="muted">${_t('Author')} ${escapeHTML(entry.author || '-')} • ${_t('Last editor')} ${escapeHTML(entry.last_editor || '-')}</div>` : ''}
    </span>
    <span>${escapeHTML(_t(lifecycleLabel(entry.state)))}</span>
    <span>${escapeHTML(formatDateTime(entry.timestamp) || _t('Current'))}</span>
    <span class="row-actions">
      ${
        entry.state === 'current'
          ? `<span class="muted">${_t('Current')}</span>`
          : `
        <button class="ghost small" data-restore-document="${escapeHTML(entry.path)}">${_t('Restore')}</button>
        <button class="ghost small" data-preview-restore-document="${escapeHTML(entry.path)}">${_t('Preview Restore')}</button>
        <button class="ghost small danger" data-purge-document="${escapeHTML(entry.path)}">${_t('Purge')}</button>`
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
      ${entry.metadata_only ? `<div class="muted">${_t('Metadata revision')}</div>` : ''}
      ${entry.version_comment ? `<div class="muted">${_t('Note')}: ${escapeHTML(entry.version_comment)}</div>` : ''}
      ${entry.actor ? `<div class="muted">${_t('By')} ${escapeHTML(entry.actor)}</div>` : ''}
    </span>
    <span>${escapeHTML(_t(lifecycleLabel(entry.state)))}</span>
    <span>${escapeHTML(formatDateTime(entry.timestamp) || _t('Current'))}</span>
    <span class="row-actions">
      ${
        entry.state === 'current'
          ? entry.public_url
            ? `<a class="button-link ghost small" href="${escapeHTML(entry.public_url)}" target="_blank" rel="noreferrer">${_t('View')}</a>`
            : `<span class="muted">${_t('Current')}</span>`
          : `
          <button class="ghost small" data-restore-media-path="${escapeHTML(entry.path)}">${_t('Restore')}</button>
          <button class="ghost small danger" data-purge-media-path="${escapeHTML(entry.path)}">${_t('Purge')}</button>`
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
    <span>${escapeHTML(_t(lifecycleLabel(entry.state)))}</span>
    <span>${escapeHTML(formatDateTime(entry.timestamp) || _t('Current'))}</span>
    <span class="row-actions">
      ${
        kind === 'document'
          ? `<button class="ghost small" data-restore-document="${escapeHTML(entry.path)}">${_t('Restore')}</button>
           <button class="ghost small danger" data-purge-document="${escapeHTML(entry.path)}">${_t('Purge')}</button>`
          : `<button class="ghost small" data-restore-media-path="${escapeHTML(entry.path)}">${_t('Restore')}</button>
           <button class="ghost small danger" data-purge-media-path="${escapeHTML(entry.path)}">${_t('Purge')}</button>`
      }
    </span>
  </div>`
    )
    .join('');

export const renderOverview = (state) => {
  const content = state.status?.content || {};
  const runtime = state.runtimeStatus || {};
  const failingChecks = (state.status?.checks || []).filter(
    (check) => check?.status && check.status !== 'ok'
  );
  const sessionWarnings = [];
  if ((runtime.activity?.concurrent_users || 0) > 0) {
    sessionWarnings.push({
      name: 'session-concurrency',
      status: 'warn',
      message: _t('{count} user(s) have multiple active sessions', {
        count: runtime.activity.concurrent_users,
      }),
    });
  }
  if ((runtime.activity?.address_spread_users || 0) > 0) {
    sessionWarnings.push({
      name: 'session-address-spread',
      status: 'warn',
      message: _t('{count} user(s) have active sessions from multiple addresses', {
        count: runtime.activity.address_spread_users,
      }),
    });
  }
  if ((runtime.activity?.long_lived_sessions || 0) > 0) {
    sessionWarnings.push({
      name: 'session-long-lived',
      status: 'warn',
      message: _t('{count} session(s) are older than 12 hours', {
        count: runtime.activity.long_lived_sessions,
      }),
    });
  }
  if ((runtime.activity?.idle_sessions || 0) > 0) {
    sessionWarnings.push({
      name: 'session-idle',
      status: 'warn',
      message: _t('{count} session(s) have been idle for more than 30 minutes', {
        count: runtime.activity.idle_sessions,
      }),
    });
  }
  const combinedWarnings = [...failingChecks, ...sessionWarnings];
  const inReview = (state.documents || []).filter((doc) => doc.status === 'in_review');
  const scheduled = (state.documents || []).filter((doc) => doc.status === 'scheduled');
  const cards = `
    <div class="cards">
      <article class="card"><span class="card-label">${_t('Documents')}</span><strong>${escapeHTML(content.document_count ?? 0)}</strong><span class="card-copy">${_t('Loaded into the current graph.')}</span></article>
      <article class="card"><span class="card-label">${_t('Drafts')}</span><strong>${escapeHTML(content.draft_count ?? 0)}</strong><span class="card-copy">${_t('Draft and archived content.')}</span></article>
      <article class="card"><span class="card-label">${_t('In Review')}</span><strong>${escapeHTML(inReview.length)}</strong><span class="card-copy">${_t('Documents waiting on review.')}</span></article>
      <article class="card"><span class="card-label">${_t('Scheduled')}</span><strong>${escapeHTML(scheduled.length)}</strong><span class="card-copy">${_t('Documents with publish windows.')}</span></article>
      <article class="card"><span class="card-label">${_t('Media')}</span><strong>${escapeHTML(state.media.length)}</strong><span class="card-copy">${_t('Images, uploads, and asset files.')}</span></article>
      <article class="card"><span class="card-label">${_t('Users')}</span><strong>${escapeHTML(state.users.length)}</strong><span class="card-copy">${_t('Filesystem-backed admin accounts.')}</span></article>
      <article class="card"><span class="card-label">${_t('Settings Sections')}</span><strong>${escapeHTML(state.settingsSections.length)}</strong><span class="card-copy">${_t('Core and plugin-defined settings groups.')}</span></article>
      <article class="card"><span class="card-label">${_t('Admin Extensions')}</span><strong>${escapeHTML((state.adminExtensions.pages?.length || 0) + (state.adminExtensions.widgets?.length || 0) + (state.adminExtensions.settings?.length || 0))}</strong><span class="card-copy">${_t('Plugin-defined pages, widgets, and settings entries.')}</span></article>
      <article class="card"><span class="card-label">${_t('Broken Refs')}</span><strong>${escapeHTML((runtime.integrity?.broken_media_refs || 0) + (runtime.integrity?.broken_internal_links || 0))}</strong><span class="card-copy">${_t('Media and internal-link validation findings.')}</span></article>
      <article class="card"><span class="card-label">${_t('Active Sessions')}</span><strong>${escapeHTML(runtime.activity?.active_sessions || 0)}</strong><span class="card-copy">${_t('Persisted admin sessions.')}</span></article>
      <article class="card"><span class="card-label">${_t('Concurrent Users')}</span><strong>${escapeHTML(runtime.activity?.concurrent_users || 0)}</strong><span class="card-copy">${_t('Users with multiple active sessions.')}</span></article>
      <article class="card"><span class="card-label">${_t('Address Spread')}</span><strong>${escapeHTML(runtime.activity?.address_spread_users || 0)}</strong><span class="card-copy">${_t('Users active from multiple addresses.')}</span></article>
      <article class="card"><span class="card-label">${_t('Active Locks')}</span><strong>${escapeHTML(runtime.activity?.active_document_locks || 0)}</strong><span class="card-copy">${_t('Documents currently being edited.')}</span></article>
      <article class="card"><span class="card-label">${_t('Validate Site')}</span><strong>${escapeHTML(state.siteValidation?.message_count || 0)}</strong><span class="card-copy">${_t('Latest admin validation findings.')}</span></article>
      <article class="card"><span class="card-label">${_t('Release')}</span><strong>${escapeHTML(state.updateInfo?.has_update ? state.updateInfo.latest_version || _t('available') : state.updateInfo?.current_display_version || state.updateInfo?.current_version || _t('current'))}</strong><span class="card-copy">${escapeHTML(_t(state.updateInfo?.has_update ? 'New release available.' : 'Running the current Foundry build.'))}</span></article>
    </div>`;
  const warningSection = combinedWarnings.length
    ? `<section class="panel">
        <div class="panel-header"><div><h2>${_t('Warnings')}</h2><div class="muted">${_t('{count} item(s) need attention', { count: escapeHTML(String(combinedWarnings.length)) })}</div></div><div class="toolbar"><button type="button" class="ghost small" data-section="operations">${_t('Open Operations')}</button><button type="button" class="ghost small" data-section="sessions">${_t('Open Sessions')}</button></div></div>
        <div class="mini-list panel-pad">
          ${combinedWarnings
            .map(
              (check) =>
                `<div class="mini-list-row"><span>${escapeHTML(check.name || 'check')}</span><strong>${escapeHTML(check.message || check.status || 'attention required')}</strong></div>`
            )
            .join('')}
        </div>
      </section>`
    : '';
  const queueSection = `<div class="layout-grid">
    ${warningSection}
    <section class="panel">
      <div class="panel-header"><div><h2>${_t('Review Queue')}</h2><div class="muted">${_t('{count} documents in review', { count: escapeHTML(String(inReview.length)) })}</div></div><div class="toolbar"><button type="button" class="ghost small" data-section="documents">${_t('Open Documents')}</button></div></div>
      ${
        inReview.length
          ? `<div class="mini-list panel-pad">${inReview
              .slice(0, 5)
              .map(
                (doc) =>
                  `<div class="mini-list-row"><span>${escapeHTML(doc.title || doc.slug || doc.source_path)}</span><strong>${escapeHTML(doc.lang || 'default')}</strong></div>`
              )
              .join('')}</div>`
          : `<div class="panel-pad empty-state">${_t('No documents are currently waiting for review.')}</div>`
      }
    </section>
    <section class="panel">
      <div class="panel-header"><div><h2>${_t('Scheduled Queue')}</h2><div class="muted">${_t('{count} scheduled documents', { count: escapeHTML(String(scheduled.length)) })}</div></div><div class="toolbar"><button type="button" class="ghost small" data-section="documents">${_t('Open Documents')}</button></div></div>
      ${
        scheduled.length
          ? `<div class="mini-list panel-pad">${scheduled
              .slice(0, 5)
              .map(
                (doc) =>
                  `<div class="mini-list-row"><span>${escapeHTML(doc.title || doc.slug || doc.source_path)}</span><strong>${escapeHTML(doc.lang || 'default')}</strong></div>`
              )
              .join('')}</div>`
          : `<div class="panel-pad empty-state">${_t('No documents are currently scheduled.')}</div>`
      }
    </section>
  </div>`;
  return (
    cards +
    queueSection +
    `<div class="layout-grid">
      <section class="panel">
        <div class="panel-header"><div><h2>${_t('Integrity')}</h2><div class="muted">${_t('Current runtime validation snapshot')}</div></div><div class="toolbar"><button type="button" class="ghost small" id="overview-validate-site">${_t('Run Validation')}</button><button type="button" class="ghost small" data-section="debug">${_t('Open Debug')}</button></div></div>
        <div class="panel-pad mini-list">
          <div class="mini-list-row"><span>${_t('Broken media refs')}</span><strong>${escapeHTML(runtime.integrity?.broken_media_refs || 0)}</strong></div>
          <div class="mini-list-row"><span>${_t('Broken internal links')}</span><strong>${escapeHTML(runtime.integrity?.broken_internal_links || 0)}</strong></div>
          <div class="mini-list-row"><span>${_t('Missing templates')}</span><strong>${escapeHTML(runtime.integrity?.missing_templates || 0)}</strong></div>
          <div class="mini-list-row"><span>${_t('Orphaned media')}</span><strong>${escapeHTML(runtime.integrity?.orphaned_media || 0)}</strong></div>
          <div class="mini-list-row"><span>${_t('Duplicate URLs/slugs')}</span><strong>${escapeHTML((runtime.integrity?.duplicate_urls || 0) + (runtime.integrity?.duplicate_slugs || 0))}</strong></div>
        </div>
      </section>
      <section class="panel">
        <div class="panel-header"><div><h2>${_t('Recent Activity')}</h2><div class="muted">${_t('{count} audit events in window', { count: escapeHTML(String(runtime.activity?.recent_audit_events || 0)) })}</div></div></div>
        <div class="panel-pad mini-list">
          ${
            Object.entries(runtime.activity?.recent_audit_by_action || {})
              .slice(0, 6)
              .map(
                ([action, count]) =>
                  `<div class="mini-list-row"><span>${escapeHTML(action)}</span><strong>${escapeHTML(count)}</strong></div>`
              )
              .join('') || `<div class="empty-state">${_t('No recent audit activity yet.')}</div>`
          }
        </div>
      </section>
    </div>` +
    (state.loadErrors.length
      ? `<div class="panel"><div class="panel-pad"><div class="error">${escapeHTML(summarizeLoadErrors(state))}</div></div></div>`
      : '')
  );
};
