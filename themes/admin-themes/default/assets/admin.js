import { createAdminClient } from '/__foundry/sdk/admin/index.js';
import {
  createAdminRouter,
  createSectionForPath,
  adminPathForSection,
  normalizeAdminSection,
} from './admin/core/router.js';
import { createInitialState, sectionTitles } from './admin/core/state.js';
import { createUIStateHelpers } from './admin/core/ui-state.js';
import {
  clone,
  escapeHTML,
  formatDateTime,
  getValueAtPath,
  parseTagInput,
  slugify,
} from './admin/core/utils.js';
import {
  buildDefaultMarkdown,
  buildDocumentRaw,
  defaultValueForSchema,
  inferLangFromSourcePath,
  parseDocumentEditor,
  removeNestedFieldValue,
  updateNestedFieldValue,
} from './admin/editor/frontmatter.js';
import { createEditorSync } from './admin/editor/sync.js';
import { bindDashboardEvents } from './admin/events/dashboard.js';
import { createPlatformViews } from './admin/views/platform.js';
import {
  documentStatusLabel,
  mediaPreview,
  mediaThumb,
  panel,
  renderBreadcrumbs,
  renderDocumentHistoryRows,
  renderKeyboardHelp,
  renderMediaHistoryRows,
  renderOverview,
  renderTableControls,
  renderToasts,
  renderTrashSelectionRows,
  renderUpdateNotice,
  shellNav,
  summarizeLoadErrors,
} from './admin/views/shared.js';

(() => {
  const root = document.getElementById('app');
  if (!root) return;

  const adminBase = root.dataset.adminBase || '/__admin';
  const defaultLang = root.dataset.defaultLang || 'en';
  const sectionForPath = createSectionForPath(adminBase);
  const initialSection = sectionForPath(window.location.pathname);
  const state = createInitialState({
    section: initialSection === 'config' ? 'settings' : initialSection,
  });
  const DEBUG_FLAGS_STORAGE_KEY = 'foundry.admin.debug.flags';
  const DEBUG_HISTORY_STORAGE_KEY = 'foundry.admin.debug.history';
  const admin = createAdminClient({ baseURL: adminBase, getSession: () => state.session });
  admin.status = admin.status || {};
  if (typeof admin.status.get !== 'function') {
    admin.status.get = () => admin.raw.get('/api/status');
  }
  admin.documents = admin.documents || {};
  if (typeof admin.documents.list !== 'function') {
    admin.documents.list = (params = {}) => admin.raw.get('/api/documents', { query: params });
  }
  if (typeof admin.documents.trash !== 'function') {
    admin.documents.trash = () => admin.raw.get('/api/documents/trash');
  }
  admin.media = admin.media || {};
  if (typeof admin.media.list !== 'function') {
    admin.media.list = (params = {}) => admin.raw.get('/api/media', { query: params });
  }
  if (typeof admin.media.trash !== 'function') {
    admin.media.trash = () => admin.raw.get('/api/media/trash');
  }
  admin.users = admin.users || {};
  if (typeof admin.users.list !== 'function') {
    admin.users.list = () => admin.raw.get('/api/users');
  }
  admin.session = admin.session || {};
  if (typeof admin.session.list !== 'function') {
    admin.session.list = (params = {}) => admin.raw.get('/api/sessions', { query: params });
  }
  admin.plugins = admin.plugins || {};
  if (typeof admin.plugins.list !== 'function') {
    admin.plugins.list = () => admin.raw.get('/api/plugins');
  }
  admin.themes = admin.themes || {};
  if (typeof admin.themes.list !== 'function') {
    admin.themes.list = () => admin.raw.get('/api/themes');
  }
  admin.backups = admin.backups || {};
  if (typeof admin.backups.list !== 'function') {
    admin.backups.list = () => admin.raw.get('/api/backups');
  }
  if (typeof admin.backups.listGit !== 'function') {
    admin.backups.listGit = () => admin.raw.get('/api/backups/git');
  }
  admin.customFields = admin.customFields || {};
  if (typeof admin.customFields.get !== 'function') {
    admin.customFields.get = () => admin.raw.get('/api/custom-fields');
  }
  if (typeof admin.customFields.save !== 'function') {
    admin.customFields.save = (payload = {}) => admin.raw.post('/api/custom-fields/save', payload);
  }
  admin.operations = admin.operations || {};
  if (typeof admin.operations.get !== 'function') {
    admin.operations.get = () => admin.raw.get('/api/operations');
  }
  if (typeof admin.operations.logs !== 'function') {
    admin.operations.logs = () => admin.raw.get('/api/operations/logs');
  }
  admin.updates = admin.updates || {};
  if (typeof admin.updates.get !== 'function') {
    admin.updates.get = () => admin.raw.get('/api/update');
  }
  if (typeof admin.updates.apply !== 'function') {
    admin.updates.apply = () => admin.raw.post('/api/update/apply', {});
  }
  admin.audit = admin.audit || {};
  if (typeof admin.audit.list !== 'function') {
    admin.audit.list = (params = {}) => admin.raw.get('/api/audit', { query: params });
  }
  const settingsAPI = {
    getForm: () =>
      typeof admin.settings?.getForm === 'function'
        ? admin.settings.getForm()
        : admin.raw.get('/api/settings/form'),
    saveForm: (input) =>
      typeof admin.settings?.saveForm === 'function'
        ? admin.settings.saveForm(input)
        : admin.raw.post('/api/settings/form/save', input),
    getCustomCSS: () =>
      typeof admin.settings?.getCustomCSS === 'function'
        ? admin.settings.getCustomCSS()
        : admin.raw.get('/api/settings/custom-css'),
    getConfig: () =>
      typeof admin.settings?.getConfig === 'function'
        ? admin.settings.getConfig()
        : admin.raw.get('/api/config'),
    getSections: () =>
      typeof admin.settings?.getSections === 'function'
        ? admin.settings.getSections()
        : admin.raw.get('/api/settings/sections'),
  };
  let navigate = () => {};
  let debugAutoRefreshTimer = null;
  const extensionModuleCache = new Map();
  const extensionStyleCache = new Set();

  const safeJSON = (value) => {
    try {
      return JSON.stringify(value, null, 2);
    } catch (_error) {
      return String(value);
    }
  };
  const readLocalJSON = (key, fallback) => {
    try {
      const raw = window.localStorage.getItem(key);
      if (!raw) return fallback;
      return JSON.parse(raw);
    } catch (_error) {
      return fallback;
    }
  };
  state.debugTools.flags = {
    ...state.debugTools.flags,
    ...readLocalJSON(DEBUG_FLAGS_STORAGE_KEY, {}),
  };
  if (state.debugTools.flags.persistConsoleHistory) {
    state.debugTools.command.history = readLocalJSON(DEBUG_HISTORY_STORAGE_KEY, []);
  }
  const persistDebugFlags = () => {
    try {
      window.localStorage.setItem(DEBUG_FLAGS_STORAGE_KEY, JSON.stringify(state.debugTools.flags));
    } catch (_error) {}
  };
  const persistDebugHistory = () => {
    try {
      if (state.debugTools.flags.persistConsoleHistory) {
        window.localStorage.setItem(
          DEBUG_HISTORY_STORAGE_KEY,
          JSON.stringify(state.debugTools.command.history.slice(0, 12))
        );
      } else {
        window.localStorage.removeItem(DEBUG_HISTORY_STORAGE_KEY);
      }
    } catch (_error) {}
  };
  const recordDebugEvent = (kind, message, meta = null) => {
    const entry = {
      id: `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
      kind: String(kind || 'info'),
      message: String(message || '').trim() || 'event',
      at: new Date().toISOString(),
      meta,
    };
    state.debugTools.events = [entry, ...(state.debugTools.events || [])].slice(0, 60);
  };
  const recordRenderTrace = (label, meta = {}) => {
    const entry = {
      id: `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
      at: new Date().toISOString(),
      section: normalizeAdminSection(state.section),
      label,
      path: window.location.pathname,
      adminTheme: root.dataset.theme || 'default',
      frontendTheme: state.status?.theme?.current || 'unknown',
      ...meta,
    };
    state.debugTools.renderTrace = [entry, ...(state.debugTools.renderTrace || [])].slice(0, 24);
  };
  const setDebugFlag = (key, value) => {
    state.debugTools.flags[key] = !!value;
    if (key === 'persistConsoleHistory' && !value) {
      state.debugTools.command.history = [];
    }
    persistDebugFlags();
    persistDebugHistory();
  };
  const scheduleDebugAutoRefresh = () => {
    if (debugAutoRefreshTimer) {
      window.clearTimeout(debugAutoRefreshTimer);
      debugAutoRefreshTimer = null;
    }
    if (
      !state.session?.authenticated ||
      !state.debugTools.flags.autoRefreshRuntime ||
      !debugEnabled() ||
      !['debug', 'diagnostics'].includes(normalizeAdminSection(state.section))
    ) {
      return;
    }
    debugAutoRefreshTimer = window.setTimeout(async () => {
      try {
        state.runtimeStatus = await admin.raw.get('/api/debug/runtime');
        recordDebugEvent('runtime', 'Auto-refreshed runtime snapshot');
        render();
      } catch (error) {
        recordDebugEvent('runtime-error', 'Auto-refresh failed', {
          error: error?.message || String(error),
        });
      }
    }, 15000);
  };
  const executeDebugRequest = async ({ method, path, body }) => {
    const normalizedMethod = String(method || 'GET').toUpperCase();
    const normalizedPath = String(path || '').trim() || '/api/status';
    let payload = undefined;
    if (!['GET', 'HEAD'].includes(normalizedMethod)) {
      const trimmed = String(body || '').trim();
      payload = trimmed ? JSON.parse(trimmed) : {};
    }
    recordDebugEvent('request', `${normalizedMethod} ${normalizedPath}`, payload || null);
    const value = await admin.raw.request(normalizedPath, {
      method: normalizedMethod,
      body: payload,
    });
    state.debugTools.command = {
      ...state.debugTools.command,
      method: normalizedMethod,
      path: normalizedPath,
      body: String(body || ''),
      result: safeJSON(value),
      error: '',
      history: [
        {
          at: new Date().toISOString(),
          method: normalizedMethod,
          path: normalizedPath,
          ok: true,
        },
        ...(state.debugTools.command.history || []),
      ].slice(0, 12),
    };
    persistDebugHistory();
    return value;
  };

  window.addEventListener('error', (event) => {
    recordDebugEvent('window-error', event.message || 'Unhandled window error', {
      source: event.filename || '',
      line: event.lineno || 0,
      column: event.colno || 0,
    });
  });
  window.addEventListener('unhandledrejection', (event) => {
    recordDebugEvent('promise-error', 'Unhandled promise rejection', {
      reason: event.reason?.message || String(event.reason || ''),
    });
  });

  const {
    editorElements,
    setStructuredFields,
    syncStructuredEditorFromRaw,
    syncRawFromStructuredEditor,
    insertIntoMarkdown,
    mediaSnippet,
    renderPreviewFrame,
  } = createEditorSync({
    state,
    defaultLang,
    parseTagInput,
    inferLangFromSourcePath,
    parseDocumentEditor,
    buildDocumentRaw,
  });

  const capabilitySet = () => new Set(state.session?.capabilities || []);
  const hasCapability = (capability) => {
    if (!capability) return true;
    const set = capabilitySet();
    return set.has('*') || set.has(capability);
  };
  const capabilityInfoHas = (capability) => {
    if (!capability) return true;
    if (typeof state.capabilityInfo?.has === 'function') {
      return state.capabilityInfo.has(capability);
    }
    return hasCapability(capability);
  };
  const debugEnabled = () =>
    !!state.capabilityInfo?.features?.pprof && capabilityInfoHas('debug.read');
  const normalizeNavGroup = (group) => {
    switch (String(group || '').trim().toLowerCase()) {
      case 'dashboard':
      case 'content':
      case 'manage':
      case 'admin':
        return String(group).trim().toLowerCase();
      default:
        return 'admin';
    }
  };
  const builtinSectionCapability = (section) => {
    const normalized = normalizeAdminSection(section);
    switch (normalized) {
      case 'overview':
      case 'documents':
      case 'editor':
      case 'history':
      case 'trash':
      case 'extensions':
        return 'dashboard.read';
      case 'operations':
        return 'config.manage';
      case 'media':
        return 'media.read';
      case 'sessions':
        return 'users.manage';
      case 'custom-fields':
        return 'documents.read';
      case 'diagnostics':
      case 'debug':
        return debugEnabled() ? 'debug.read' : null;
      case 'audit':
        return 'audit.read';
      case 'users':
        return 'users.manage';
      case 'settings':
      case 'config':
        return 'config.manage';
      case 'plugins':
        return 'plugins.manage';
      case 'themes':
        return 'themes.manage';
      default:
        return null;
    }
  };
  const builtinSectionGroup = (section) => {
    switch (normalizeAdminSection(section)) {
      case 'overview':
        return 'dashboard';
      case 'documents':
      case 'editor':
      case 'history':
      case 'trash':
      case 'media':
        return 'content';
      case 'sessions':
      case 'users':
      case 'custom-fields':
      case 'audit':
      case 'settings':
      case 'config':
        return 'manage';
      case 'extensions':
      case 'plugins':
      case 'themes':
      case 'operations':
      case 'diagnostics':
      case 'debug':
      default:
        return 'admin';
    }
  };
  const canAccessBuiltinSection = (section) => {
    const normalized = normalizeAdminSection(section);
    const capability = builtinSectionCapability(normalized);
    if (!capability) return false;
    if ((normalized === 'debug' || normalized === 'diagnostics') && !debugEnabled()) return false;
    return capabilityInfoHas(capability);
  };
  const extensionPages = () =>
    (state.adminExtensions.pages || [])
      .filter((page) => page && page.key && hasCapability(page.capability))
      .map((page) => ({
        ...page,
        section: normalizeAdminSection(page.route || `plugins/${page.plugin}/${page.key}`),
        navGroup: normalizeNavGroup(page.nav_group),
      }));
  const extensionWidgetsForSlot = (slot) =>
    (state.adminExtensions.widgets || []).filter(
      (widget) => widget && widget.key && widget.slot === slot && hasCapability(widget.capability)
    );
  const extensionMountID = (kind, plugin, key, slot = '') =>
    `${kind}-${plugin}-${key}-${slot}`.replace(/[^a-zA-Z0-9_-]+/g, '-');
  const extensionPageBySection = (section) =>
    extensionPages().find((page) => page.section === normalizeAdminSection(section)) || null;
  const titleForSection = (section) => {
    const normalized = normalizeAdminSection(section);
    return (
      sectionTitles[normalized] ||
      extensionPageBySection(normalized)?.title ||
      normalized.charAt(0).toUpperCase() + normalized.slice(1)
    );
  };
  const canAccessSection = (section) => {
    const normalized = normalizeAdminSection(section);
    const extensionPage = extensionPageBySection(normalized);
    if (extensionPage) {
      return hasCapability(extensionPage.capability);
    }
    return canAccessBuiltinSection(normalized);
  };
  const firstAccessibleSection = () => {
    const candidates = ['overview', 'documents', 'editor', 'media', 'custom-fields', 'sessions', 'audit'];
    return candidates.find((section) => canAccessSection(section)) || extensionPages()[0]?.section || 'overview';
  };
  const canManageSharedFields = () => capabilityInfoHas('config.manage');
  const isSettingsSection = (section) => {
    const normalized = normalizeAdminSection(section);
    return normalized === 'settings' || normalized === 'config';
  };
  const renderWidgetPanels = (slot) =>
    extensionWidgetsForSlot(slot).map((widget) =>
      panel(
        widget.title,
        `
    <div class="panel-pad stack">
      ${widget.description ? `<div class="muted">${escapeHTML(widget.description)}</div>` : ''}
      <div id="${escapeHTML(extensionMountID('admin-widget', widget.plugin, widget.key, widget.slot))}"
           class="admin-extension-mount admin-extension-widget-mount"
           data-plugin="${escapeHTML(widget.plugin)}"
           data-extension-key="${escapeHTML(widget.key)}"
           data-extension-slot="${escapeHTML(widget.slot)}"
           data-extension-kind="widget">
        <div class="empty-state">This widget slot is ready for a plugin-provided admin widget mount.</div>
      </div>
    </div>`,
        widget.slot || 'Plugin-defined widget'
      )
    );
  const renderExtensionPage = () => {
    const page = extensionPageBySection(state.section);
    if (!page) {
      return panel(
        'Admin Extension',
        '<div class="panel-pad empty-state">This admin extension page is not registered.</div>'
      );
    }
    const relatedWidgets = (state.adminExtensions.widgets || []).filter(
      (widget) => widget.plugin === page.plugin && hasCapability(widget.capability)
    );
    const relatedSettings = (state.adminExtensions.settings || []).filter(
      (setting) => setting.plugin === page.plugin && hasCapability(setting.capability)
    );
    const relatedSlots = (state.adminExtensions.slots || []).filter(
      (slot) => slot.plugin === page.plugin
    );
    return panel(
      page.title,
      `
      <div class="panel-pad stack">
        <div class="note">
          <strong>${escapeHTML(page.plugin)}</strong>${page.description ? ` • ${escapeHTML(page.description)}` : ''}
        </div>
        <div class="cards">
          <article class="card"><span class="card-label">Route</span><strong>${escapeHTML(page.route || `/${page.section}`)}</strong><span class="card-copy">Mounted from plugin metadata.</span></article>
          <article class="card"><span class="card-label">Widgets</span><strong>${escapeHTML(relatedWidgets.length)}</strong><span class="card-copy">Plugin widgets registered for admin slots.</span></article>
          <article class="card"><span class="card-label">Settings</span><strong>${escapeHTML(relatedSettings.length)}</strong><span class="card-copy">Plugin settings sections exposed to admin.</span></article>
          <article class="card"><span class="card-label">Slots</span><strong>${escapeHTML(relatedSlots.length)}</strong><span class="card-copy">Declared admin shell slots.</span></article>
        </div>
        ${
          relatedSettings.length
            ? `<div class="stack">
          <h3>Settings Sections</h3>
          <div class="table table-three">
            <div class="table-head"><span>Section</span><span>Capability</span><span>Description</span></div>
            ${relatedSettings
              .map(
                (setting) => `
              <div class="table-row">
                <span><strong>${escapeHTML(setting.title)}</strong><div class="muted mono">${escapeHTML(setting.key)}</div></span>
                <span>${escapeHTML(setting.capability || '-')}</span>
                <span>${escapeHTML(setting.description || '-')}</span>
              </div>`
              )
              .join('')}
          </div>
        </div>`
            : ''
        }
        ${
          relatedWidgets.length
            ? `<div class="stack">
          <h3>Widgets</h3>
          <div class="table table-three">
            <div class="table-head"><span>Widget</span><span>Slot</span><span>Description</span></div>
            ${relatedWidgets
              .map(
                (widget) => `
              <div class="table-row">
                <span><strong>${escapeHTML(widget.title)}</strong><div class="muted mono">${escapeHTML(widget.key)}</div></span>
                <span>${escapeHTML(widget.slot)}</span>
                <span>${escapeHTML(widget.description || '-')}</span>
              </div>`
              )
              .join('')}
          </div>
        </div>`
            : ''
        }
        <div id="admin-extension-mount"
             class="admin-extension-mount"
             data-plugin="${escapeHTML(page.plugin)}"
             data-extension-key="${escapeHTML(page.key)}"
             data-extension-route="${escapeHTML(page.route || '')}"
             data-extension-section="${escapeHTML(page.section)}">
          <div class="empty-state">This page is ready for a plugin-provided admin UI mount. Listen for the <code>foundry:admin-extension-page</code> event or read <code>window.FoundryAdmin</code>.</div>
        </div>
      </div>`,
      page.description || 'Plugin-defined admin page'
    );
  };

  const publishAdminRuntime = () => {
    window.FoundryAdmin = {
      client: admin,
      adminBase,
      getSession: () => state.session,
      getCapabilities: () => [...capabilitySet()],
      getExtensions: () => clone(state.adminExtensions),
      getSettingsSections: () => clone(state.settingsSections),
    };
    const page = extensionPageBySection(state.section);
    if (page) {
      if (state.debugTools.flags.captureExtensionEvents) {
        recordDebugEvent('extension-page', `Dispatching extension page event for ${page.plugin}/${page.key}`, {
          section: page.section,
          route: page.route || '',
        });
      }
      document.dispatchEvent(
        new CustomEvent('foundry:admin-extension-page', {
          detail: {
            page,
            adminBase,
            session: state.session,
            capabilities: [...capabilitySet()],
            settingsSections: (state.settingsSections || []).filter(
              (section) => section.source === page.plugin
            ),
            extensions: clone(state.adminExtensions),
            mountId: 'admin-extension-mount',
          },
        })
      );
    }
  };

  const ensureExtensionStyles = (urls = []) => {
    urls.filter(Boolean).forEach((url) => {
      if (extensionStyleCache.has(url)) return;
      const link = document.createElement('link');
      link.rel = 'stylesheet';
      link.href = url;
      link.dataset.foundryAdminExtensionStyle = url;
      document.head.appendChild(link);
      extensionStyleCache.add(url);
    });
  };

  const loadExtensionModule = async (page) => {
    if (!page?.module_url) return null;
    ensureExtensionStyles(page.style_urls || []);
    if (!extensionModuleCache.has(page.module_url)) {
      extensionModuleCache.set(page.module_url, import(page.module_url));
    }
    return extensionModuleCache.get(page.module_url);
  };

  const mountActiveExtensionPage = async () => {
    const page = extensionPageBySection(state.section);
    if (!page) return;
    const mount = document.getElementById('admin-extension-mount');
    if (!mount) return;
    mount.dataset.extensionStatus = 'loading';
    try {
      const mod = await loadExtensionModule(page);
      state.extensionRuntimeErrors = state.extensionRuntimeErrors.filter(
        (entry) => !(entry.kind === 'page' && entry.plugin === page.plugin && entry.key === page.key)
      );
      const mountFn = mod?.mountAdminExtensionPage || mod?.default;
      if (typeof mountFn === 'function') {
        await mountFn({
          mount,
          page,
          adminBase,
          client: admin,
          session: state.session,
          capabilities: [...capabilitySet()],
          settingsSections: (state.settingsSections || []).filter(
            (section) => section.source === page.plugin
          ),
          extensions: clone(state.adminExtensions),
        });
        mount.dataset.extensionStatus = 'mounted';
        if (state.debugTools.flags.captureExtensionEvents) {
          recordDebugEvent('extension-page', `Mounted extension page ${page.plugin}/${page.key}`, {
            module: page.module_url || '',
          });
        }
      } else {
        mount.dataset.extensionStatus = 'ready';
      }
    } catch (error) {
      mount.dataset.extensionStatus = 'error';
      state.extensionRuntimeErrors = [
        ...state.extensionRuntimeErrors.filter(
          (entry) => !(entry.kind === 'page' && entry.plugin === page.plugin && entry.key === page.key)
        ),
        {
          kind: 'page',
          plugin: page.plugin,
          key: page.key,
          message: error?.message || String(error),
        },
      ];
      if (state.debugTools.flags.captureExtensionEvents) {
        recordDebugEvent('extension-error', `Failed to load extension page ${page.plugin}/${page.key}`, {
          error: error?.message || String(error),
        });
      }
      mount.innerHTML = `<div class="error">Failed to load plugin admin page bundle: ${escapeHTML(error?.message || String(error))}</div>`;
    }
  };

  const mountVisibleExtensionWidgets = async () => {
    const widgets = [
      ...extensionWidgetsForSlot('overview.after'),
      ...extensionWidgetsForSlot('documents.sidebar'),
      ...extensionWidgetsForSlot('media.sidebar'),
      ...extensionWidgetsForSlot('plugins.sidebar'),
    ];
    await Promise.all(
      widgets.map(async (widget) => {
        const mountId = extensionMountID('admin-widget', widget.plugin, widget.key, widget.slot);
        const mount = document.getElementById(mountId);
        if (!mount) return;
        mount.dataset.extensionStatus = 'loading';
        try {
          const mod = await loadExtensionModule(widget);
          state.extensionRuntimeErrors = state.extensionRuntimeErrors.filter(
            (entry) =>
              !(
                entry.kind === 'widget' &&
                entry.plugin === widget.plugin &&
                entry.key === widget.key &&
                entry.slot === widget.slot
              )
          );
          const mountFn = mod?.mountAdminExtensionWidget || mod?.default;
          document.dispatchEvent(
            new CustomEvent('foundry:admin-extension-widget', {
              detail: {
                widget,
                adminBase,
                session: state.session,
                capabilities: [...capabilitySet()],
                extensions: clone(state.adminExtensions),
                mountId,
              },
            })
          );
          if (state.debugTools.flags.captureExtensionEvents) {
            recordDebugEvent('extension-widget', `Dispatching widget event for ${widget.plugin}/${widget.key}`, {
              slot: widget.slot,
            });
          }
          if (typeof mountFn === 'function') {
            await mountFn({
              mount,
              widget,
              adminBase,
              client: admin,
              session: state.session,
              capabilities: [...capabilitySet()],
              extensions: clone(state.adminExtensions),
            });
            mount.dataset.extensionStatus = 'mounted';
            if (state.debugTools.flags.captureExtensionEvents) {
              recordDebugEvent('extension-widget', `Mounted widget ${widget.plugin}/${widget.key}`, {
                slot: widget.slot,
              });
            }
          } else {
            mount.dataset.extensionStatus = 'ready';
          }
        } catch (error) {
          mount.dataset.extensionStatus = 'error';
          state.extensionRuntimeErrors = [
            ...state.extensionRuntimeErrors.filter(
              (entry) =>
                !(
                  entry.kind === 'widget' &&
                  entry.plugin === widget.plugin &&
                  entry.key === widget.key &&
                  entry.slot === widget.slot
                )
            ),
            {
              kind: 'widget',
              plugin: widget.plugin,
              key: widget.key,
              slot: widget.slot,
              message: error?.message || String(error),
            },
          ];
          if (state.debugTools.flags.captureExtensionEvents) {
            recordDebugEvent('extension-error', `Failed to load widget ${widget.plugin}/${widget.key}`, {
              slot: widget.slot,
              error: error?.message || String(error),
            });
          }
          mount.innerHTML = `<div class="error">Failed to load plugin admin widget bundle: ${escapeHTML(error?.message || String(error))}</div>`;
        }
      })
    );
  };

  const updateDocumentFieldValue = (path, nextValue) => {
    updateNestedFieldValue(state, path, nextValue);
    compareSnapshot('document', {
      editor: state.documentEditor,
      fields: state.documentFieldValues,
      meta: state.documentMeta,
    });
  };

  let documentLockHeartbeatId = null;

  const stopDocumentLockHeartbeat = () => {
    if (documentLockHeartbeatId) {
      window.clearInterval(documentLockHeartbeatId);
      documentLockHeartbeatId = null;
    }
  };

  const releaseCurrentDocumentLock = async () => {
    if (!state.documentEditor.source_path || !state.documentEditor.lock_token) return;
    try {
      await admin.documents.unlock({
        source_path: state.documentEditor.source_path,
        lock_token: state.documentEditor.lock_token,
      });
    } catch (_error) {
    } finally {
      stopDocumentLockHeartbeat();
      state.documentEditor.lock_token = '';
      state.documentLock = null;
    }
  };

  const startDocumentLockHeartbeat = () => {
    stopDocumentLockHeartbeat();
    if (!state.documentEditor.source_path || !state.documentEditor.lock_token) return;
    documentLockHeartbeatId = window.setInterval(async () => {
      try {
        const response = await admin.documents.heartbeat({
          source_path: state.documentEditor.source_path,
          lock_token: state.documentEditor.lock_token,
        });
        state.documentLock = response.lock || null;
      } catch (error) {
        stopDocumentLockHeartbeat();
        state.error = error.message || String(error);
        render();
      }
    }, 45000);
  };

  const acquireDocumentLock = async (sourcePath, lockToken = '') => {
    if (!sourcePath) return null;
    const response = await admin.documents.lock({ source_path: sourcePath, lock_token: lockToken });
    state.documentLock = response.lock || null;
    state.documentEditor.lock_token = response.lock?.owned_by_me ? response.lock.token || '' : '';
    if (state.documentEditor.lock_token) {
      startDocumentLockHeartbeat();
    } else {
      stopDocumentLockHeartbeat();
    }
    return response.lock || null;
  };

  const loadDocumentIntoEditor = async (detail) => {
    if (!detail) return;
    if (
      state.documentEditor.source_path &&
      state.documentEditor.source_path !== detail.source_path
    ) {
      await releaseCurrentDocumentLock();
    }
    state.documentEditor = {
      source_path: detail.source_path,
      raw: detail.raw_body,
      version_comment: '',
      lock_token: '',
    };
    state.documentFieldSchema = detail.field_schema || [];
    state.documentFieldValues = clone(detail.fields || {});
    state.documentContractTitles = Array.isArray(detail.field_contract_titles) ? [...detail.field_contract_titles] : [];
    state.documentMeta = {
      status: detail.status || 'draft',
      author: detail.author || '',
      last_editor: detail.last_editor || '',
      created_at: detail.created_at || '',
      updated_at: detail.updated_at || '',
    };
    state.documentPreview = null;
    snapshotValue('document', {
      editor: state.documentEditor,
      fields: state.documentFieldValues,
      meta: state.documentMeta,
    });
    await acquireDocumentLock(detail.source_path, detail.lock?.token || '');
  };

  const {
    setUserForm,
    resetUserForm,
    resetUserSecurity,
    selectedUserRecord,
    resetDocumentEditor,
    setFlash: uiSetFlash,
    setError: uiSetError,
    pushToast,
    markDirty,
    snapshotValue,
    dirtyMessage,
    hasUnsavedChanges,
    clearDirtyState,
    confirmNavigation,
    compareSnapshot,
    updateTablePage,
    sortItems,
    paginateItems,
    toggleSelection,
    clearLoadErrors,
  } = createUIStateHelpers({
    state,
    render: () => render(),
    buildDefaultMarkdown,
  });
  const setFlash = (message) => {
    if (message) {
      recordDebugEvent('flash', message);
    }
    uiSetFlash(message);
  };
  const setError = (message) => {
    if (message) {
      recordDebugEvent('error', message);
    }
    uiSetError(message);
  };

  const parseJSONInput = (id, fallback) => {
    const node = document.getElementById(id);
    if (!node) return fallback;
    const raw = String(node.value || '').trim();
    if (!raw) return fallback;
    try {
      return JSON.parse(raw);
    } catch (error) {
      throw new Error(`${node.id}: ${error.message}`);
    }
  };

  const collectSettingsFormPayload = () => {
    const next = clone(state.settingsForm || {});
    next.Admin = next.Admin || {};
    next.Admin.Debug = next.Admin.Debug || {};
    next.Server = next.Server || {};
    next.Build = next.Build || {};
    next.Content = next.Content || {};
    next.Taxonomies = next.Taxonomies || {};
    next.Plugins = next.Plugins || {};
    next.SEO = next.SEO || {};
    next.Cache = next.Cache || {};
    next.Security = next.Security || {};
    next.Feed = next.Feed || {};
    next.Deploy = next.Deploy || {};
    const setPathValue = (path, value) => {
      const wrapper = { documentFieldValues: next };
      updateNestedFieldValue(wrapper, path.split('.'), value);
      return wrapper.documentFieldValues;
    };

    const setText = (path, id) => {
      const node = document.getElementById(id);
      if (!node) return;
      Object.assign(next, setPathValue(path, node.value));
    };
    const setBool = (path, id) => {
      const node = document.getElementById(id);
      if (!node) return;
      Object.assign(next, setPathValue(path, !!node.checked));
    };
    const setInt = (path, id) => {
      const node = document.getElementById(id);
      if (!node) return;
      const value = Number.parseInt(node.value || '0', 10);
      Object.assign(next, setPathValue(path, Number.isFinite(value) ? value : 0));
    };

    setText('Name', 'settings-name');
    setText('Title', 'settings-title');
    setText('BaseURL', 'settings-base-url');
    setText('Theme', 'settings-theme');
    setText('Environment', 'settings-environment');
    setText('DefaultLang', 'settings-default-lang');
    setText('ContentDir', 'settings-content-dir');
    setText('PublicDir', 'settings-public-dir');
    setText('ThemesDir', 'settings-themes-dir');
    setText('DataDir', 'settings-data-dir');
    setText('PluginsDir', 'settings-plugins-dir');

    setText('Server.Addr', 'settings-server-addr');
    setText('Server.LiveReloadMode', 'settings-server-live-reload-mode');
    setBool('Server.LiveReload', 'settings-server-live-reload');
    setBool('Server.AutoOpenBrowser', 'settings-server-auto-open-browser');
    setBool('Server.DebugRoutes', 'settings-server-debug-routes');

    setText('Content.PagesDir', 'settings-content-pages-dir');
    setText('Content.PostsDir', 'settings-content-posts-dir');
    setText('Content.ImagesDir', 'settings-content-images-dir');
    setText('Content.VideoDir', 'settings-content-video-dir');
    setText('Content.AudioDir', 'settings-content-audio-dir');
    setText('Content.DocumentsDir', 'settings-content-documents-dir');
    setText('Content.AssetsDir', 'settings-content-assets-dir');
    setText('Content.UploadsDir', 'settings-content-uploads-dir');
    setInt('Content.MaxVersionsPerFile', 'settings-content-max-versions');
    setText('Content.DefaultLayoutPage', 'settings-content-default-layout-page');
    setText('Content.DefaultLayoutPost', 'settings-content-default-layout-post');
    setText('Content.DefaultPageSlugIndex', 'settings-content-default-page-slug-index');

    setBool('Admin.Enabled', 'settings-admin-enabled');
    setBool('Admin.LocalOnly', 'settings-admin-local-only');
    setBool('Admin.Debug.Pprof', 'settings-admin-debug-pprof');
    setText('Admin.Addr', 'settings-admin-addr');
    setText('Admin.Path', 'settings-admin-path');
    setText('Admin.AccessToken', 'settings-admin-access-token');
    setText('Admin.Theme', 'settings-admin-theme');
    setText('Admin.UsersFile', 'settings-admin-users-file');
    setText('Admin.SessionStoreFile', 'settings-admin-session-store-file');
    setText('Admin.LockFile', 'settings-admin-lock-file');
    setInt('Admin.SessionTTLMinutes', 'settings-admin-session-ttl');
    setInt('Admin.PasswordMinLength', 'settings-admin-password-min-length');
    setInt('Admin.PasswordResetTTL', 'settings-admin-password-reset-ttl');
    setText('Admin.TOTPIssuer', 'settings-admin-totp-issuer');

    setBool('Build.CleanPublicDir', 'settings-build-clean-public-dir');
    setBool('Build.IncludeDrafts', 'settings-build-include-drafts');
    setBool('Build.MinifyHTML', 'settings-build-minify-html');
    setBool('Build.CopyAssets', 'settings-build-copy-assets');
    setBool('Build.CopyImages', 'settings-build-copy-images');
    setBool('Build.CopyUploads', 'settings-build-copy-uploads');

    setBool('Taxonomies.Enabled', 'settings-taxonomies-enabled');
    const defaultSetNode = document.getElementById('settings-taxonomies-default-set');
    if (defaultSetNode) {
      next.Taxonomies.DefaultSet = parseTagInput(defaultSetNode.value || '');
    }
    if (document.getElementById('settings-taxonomies-definitions')) {
      next.Taxonomies.Definitions = parseJSONInput(
        'settings-taxonomies-definitions',
        next.Taxonomies.Definitions || {}
      );
    }

    if (document.getElementById('settings-plugins-enabled')) {
      next.Plugins.Enabled = parseJSONInput('settings-plugins-enabled', next.Plugins.Enabled || []);
    }

    setBool('SEO.Enabled', 'settings-seo-enabled');
    setText('SEO.DefaultTitleSep', 'settings-seo-default-title-sep');
    setBool('Cache.Enabled', 'settings-cache-enabled');
    setBool('Security.AllowUnsafeHTML', 'settings-security-allow-unsafe-html');
    setText('Feed.RSSPath', 'settings-feed-rss-path');
    setText('Feed.SitemapPath', 'settings-feed-sitemap-path');
    setInt('Feed.RSSLimit', 'settings-feed-rss-limit');
    setText('Feed.RSSTitle', 'settings-feed-rss-title');
    setText('Feed.RSSDescription', 'settings-feed-rss-description');
    setText('Deploy.DefaultTarget', 'settings-deploy-default-target');
    if (document.getElementById('settings-deploy-targets')) {
      next.Deploy.Targets = parseJSONInput('settings-deploy-targets', next.Deploy.Targets || {});
    }

    if (document.getElementById('settings-permalinks')) {
      next.Permalinks = parseJSONInput('settings-permalinks', next.Permalinks || {});
    }
    if (document.getElementById('settings-menus')) {
      next.Menus = parseJSONInput('settings-menus', next.Menus || {});
    }
    if (document.getElementById('settings-params')) {
      next.Params = parseJSONInput('settings-params', next.Params || {});
    }

    return next;
  };

  const syncSettingsDraftFromDOM = () => {
    if (!document.getElementById('settings-structured-form')) return;
    try {
      state.settingsForm = collectSettingsFormPayload();
      state.settingsDraftError = '';
      compareSnapshot('settings', state.settingsForm);
    } catch (error) {
      state.settingsDraftError = error.message || String(error);
    }
  };

  const renderFieldSchemaControl = (schema, path = []) => {
    const fullPath = [...path, schema.name];
    const pathValue = fullPath.join('.');
    const value =
      getValueAtPath(state.documentFieldValues, fullPath) ?? defaultValueForSchema(schema);
    const label = schema.label || schema.name;
    const help = schema.help ? `<div class="muted">${escapeHTML(schema.help)}</div>` : '';
    switch (schema.type) {
      case 'bool':
        return `<label class="checkbox"><input type="checkbox" data-custom-field="${escapeHTML(pathValue)}" data-custom-type="bool" ${value ? 'checked' : ''}> ${escapeHTML(label)}</label>${help}`;
      case 'select':
        return `<label>${escapeHTML(label)}<select data-custom-field="${escapeHTML(pathValue)}" data-custom-type="select">
          ${(schema.enum || []).map((entry) => `<option value="${escapeHTML(entry)}" ${entry === value ? 'selected' : ''}>${escapeHTML(entry)}</option>`).join('')}
        </select></label>${help}`;
      case 'number':
        return `<label>${escapeHTML(label)}<input type="number" data-custom-field="${escapeHTML(pathValue)}" data-custom-type="number" value="${escapeHTML(value)}" placeholder="${escapeHTML(schema.placeholder || '')}"></label>${help}`;
      case 'object':
        return `<fieldset class="custom-field-group"><legend>${escapeHTML(label)}</legend>${help}${(schema.fields || []).map((field) => renderFieldSchemaControl(field, fullPath)).join('')}</fieldset>`;
      case 'repeater': {
        const items = Array.isArray(value) ? value : [];
        return `<fieldset class="custom-field-group"><legend>${escapeHTML(label)}</legend>${help}
          <div class="repeater-list">
            ${items
              .map((item, index) => {
                const itemPath = [...fullPath, index];
                const itemSchema = schema.item || { name: 'item', type: 'text', label: 'Item' };
                const body =
                  itemSchema.type === 'object'
                    ? `<div class="custom-object">${(itemSchema.fields || []).map((field) => renderFieldSchemaControl(field, itemPath)).join('')}</div>`
                    : renderFieldSchemaControl(
                        {
                          ...itemSchema,
                          name: String(index),
                          label: itemSchema.label || `Item ${index + 1}`,
                        },
                        fullPath
                      );
                return `<div class="repeater-item">${body}<button type="button" class="ghost small danger" data-repeater-remove="${escapeHTML(itemPath.join('.'))}">Remove</button></div>`;
              })
              .join('')}
          </div>
          <button type="button" class="ghost small" data-repeater-add="${escapeHTML(pathValue)}">Add Item</button>
        </fieldset>`;
      }
      case 'textarea':
        return `<label>${escapeHTML(label)}<textarea data-custom-field="${escapeHTML(pathValue)}" data-custom-type="textarea" rows="4" placeholder="${escapeHTML(schema.placeholder || '')}">${escapeHTML(value || '')}</textarea></label>${help}`;
      default:
        return `<label>${escapeHTML(label)}<input type="text" data-custom-field="${escapeHTML(pathValue)}" data-custom-type="text" value="${escapeHTML(value || '')}" placeholder="${escapeHTML(schema.placeholder || '')}"></label>${help}`;
    }
  };

  const updateSharedFieldValue = (path, nextValue) => {
    const wrapper = { documentFieldValues: state.customFields?.values || {} };
    updateNestedFieldValue(wrapper, path, nextValue);
    state.customFields = state.customFields || { path: 'content/custom-fields.yaml', raw: '', values: {} };
    state.customFields.values = wrapper.documentFieldValues;
    compareSnapshot('customFields', state.customFields.values || {});
  };

  const renderSharedFieldSchemaControl = (schema, contractKey, path = []) => {
    const fullPath = [contractKey, ...path, schema.name];
    const pathValue = fullPath.join('.');
    const value = getValueAtPath(state.customFields?.values || {}, fullPath) ?? defaultValueForSchema(schema);
    const label = schema.label || schema.name;
    const help = schema.help ? `<div class="muted">${escapeHTML(schema.help)}</div>` : '';
    const disabledAttr = canManageSharedFields() ? '' : ' disabled aria-disabled="true"';
    switch (schema.type) {
      case 'bool':
        return `<label class="checkbox"><input type="checkbox" data-shared-custom-field="${escapeHTML(pathValue)}" data-custom-type="bool" ${value ? 'checked' : ''}${disabledAttr}> ${escapeHTML(label)}</label>${help}`;
      case 'select':
        return `<label>${escapeHTML(label)}<select data-shared-custom-field="${escapeHTML(pathValue)}" data-custom-type="select"${disabledAttr}>
          ${(schema.enum || []).map((entry) => `<option value="${escapeHTML(entry)}" ${entry === value ? 'selected' : ''}>${escapeHTML(entry)}</option>`).join('')}
        </select></label>${help}`;
      case 'number':
        return `<label>${escapeHTML(label)}<input type="number" data-shared-custom-field="${escapeHTML(pathValue)}" data-custom-type="number" value="${escapeHTML(value)}" placeholder="${escapeHTML(schema.placeholder || '')}"${disabledAttr}></label>${help}`;
      case 'object':
        return `<fieldset class="custom-field-group"><legend>${escapeHTML(label)}</legend>${help}${(schema.fields || []).map((field) => renderSharedFieldSchemaControl(field, contractKey, [...path, schema.name])).join('')}</fieldset>`;
      case 'repeater': {
        const items = Array.isArray(value) ? value : [];
        return `<fieldset class="custom-field-group"><legend>${escapeHTML(label)}</legend>${help}
          <div class="repeater-list">
            ${items
              .map((_item, index) => {
                const itemPath = [...fullPath, index];
                const itemSchema = schema.item || { name: 'item', type: 'text', label: 'Item' };
                const body =
                  itemSchema.type === 'object'
                    ? `<div class="custom-object">${(itemSchema.fields || []).map((field) => renderSharedFieldSchemaControl(field, contractKey, [...path, schema.name, index])).join('')}</div>`
                    : renderSharedFieldSchemaControl(
                        {
                          ...itemSchema,
                          name: String(index),
                          label: itemSchema.label || `Item ${index + 1}`,
                        },
                        contractKey,
                        [...path, schema.name]
                      );
                return `<div class="repeater-item">${body}<button type="button" class="ghost small danger" data-shared-repeater-remove="${escapeHTML(itemPath.join('.'))}"${disabledAttr}>Remove</button></div>`;
              })
              .join('')}
          </div>
          <button type="button" class="ghost small" data-shared-repeater-add="${escapeHTML(pathValue)}"${disabledAttr}>Add Item</button>
        </fieldset>`;
      }
      case 'textarea':
        return `<label>${escapeHTML(label)}<textarea data-shared-custom-field="${escapeHTML(pathValue)}" data-custom-type="textarea" rows="4" placeholder="${escapeHTML(schema.placeholder || '')}"${disabledAttr}>${escapeHTML(value || '')}</textarea></label>${help}`;
      default:
        return `<label>${escapeHTML(label)}<input type="text" data-shared-custom-field="${escapeHTML(pathValue)}" data-custom-type="text" value="${escapeHTML(value || '')}" placeholder="${escapeHTML(schema.placeholder || '')}"${disabledAttr}></label>${help}`;
    }
  };

  const renderSplitDiffPane = (leftRaw, rightRaw) => {
    const leftLines = (leftRaw || '').replaceAll('\r\n', '\n').split('\n');
    const rightLines = (rightRaw || '').replaceAll('\r\n', '\n').split('\n');
    const total = Math.max(leftLines.length, rightLines.length);
    const leftRows = [];
    const rightRows = [];
    for (let index = 0; index < total; index += 1) {
      const left = leftLines[index] ?? '';
      const right = rightLines[index] ?? '';
      let leftClass = 'diff-line';
      let rightClass = 'diff-line';
      if (left !== right) {
        if (left && !right) {
          leftClass += ' removed';
          rightClass += ' empty';
        } else if (!left && right) {
          leftClass += ' empty';
          rightClass += ' added';
        } else {
          leftClass += ' changed';
          rightClass += ' changed';
        }
      }
      leftRows.push(
        `<div class="${leftClass}"><span class="diff-line-number">${index + 1}</span><code>${escapeHTML(left)}</code></div>`
      );
      rightRows.push(
        `<div class="${rightClass}"><span class="diff-line-number">${index + 1}</span><code>${escapeHTML(right)}</code></div>`
      );
    }
    return `<div class="diff-split">
      <section><h3>Previous</h3><div class="diff-pane">${leftRows.join('')}</div></section>
      <section><h3>Current</h3><div class="diff-pane">${rightRows.join('')}</div></section>
    </div>`;
  };

  const renderEditorPanel = () => {
    const editorDocument = parseDocumentEditor(
      state.documentEditor.raw,
      state.documentEditor.source_path,
      defaultLang
    );
    const mediaQuery = state.mediaPickerQuery.trim().toLowerCase();
    const mediaMatches = state.media
      .filter((item) => {
        if (!mediaQuery) return true;
        const haystack = [
          item.name,
          item.reference,
          item.metadata?.title,
          item.metadata?.alt,
          item.path,
        ]
          .join(' ')
          .toLowerCase();
        return haystack.includes(mediaQuery);
      })
      .slice(0, 10);
    const workflowStatus = editorDocument.fields.workflow || state.documentMeta.status || 'draft';
    return `
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
          <div class="editor-grid">
            <div class="stack">
              <label>Source Path<input id="document-source-path" type="text" value="${escapeHTML(state.documentEditor.source_path)}" placeholder="content/pages/about.md"></label>
              ${state.documentLock ? `<div class="status-line ${state.documentLock.owned_by_me ? '' : 'error'}">${state.documentLock.owned_by_me ? `Locked by you until ${escapeHTML(formatDateTime(state.documentLock.expires_at))}` : `Currently being edited by ${escapeHTML(state.documentLock.name || state.documentLock.username || 'another user')}`}</div>` : ''}
              <label>Version Comment<input id="document-version-comment" type="text" value="${escapeHTML(state.documentEditor.version_comment || '')}" placeholder="Explain what changed in this revision"></label>
              <div class="frontmatter-card">
                <div class="frontmatter-card-header">
                  <div>
                    <strong>Workflow</strong>
                    <div class="muted">Request review, schedule publication windows, and keep editorial notes with the draft.</div>
                  </div>
                </div>
                <div class="frontmatter-grid">
                  <label>Workflow
                    <select id="document-frontmatter-workflow" data-frontmatter-field="workflow">
                      ${['draft', 'in_review', 'scheduled', 'published', 'archived'].map((entry) => `<option value="${entry}" ${workflowStatus === entry ? 'selected' : ''}>${escapeHTML(entry)}</option>`).join('')}
                    </select>
                  </label>
                  <label>Scheduled Publish<input id="document-frontmatter-scheduled-publish-at" data-frontmatter-field="scheduled_publish_at" type="text" value="${escapeHTML(editorDocument.fields.scheduled_publish_at || '')}" placeholder="2026-03-21T14:00:00Z"></label>
                  <label>Scheduled Unpublish<input id="document-frontmatter-scheduled-unpublish-at" data-frontmatter-field="scheduled_unpublish_at" type="text" value="${escapeHTML(editorDocument.fields.scheduled_unpublish_at || '')}" placeholder="2026-03-28T14:00:00Z"></label>
                  <label class="frontmatter-span-3">Editorial Note<textarea id="document-frontmatter-editorial-note" data-frontmatter-field="editorial_note" rows="3">${escapeHTML(editorDocument.fields.editorial_note || '')}</textarea></label>
                </div>
                <div class="toolbar">
                  <button type="button" class="ghost small" data-apply-workflow="draft">Save as Draft</button>
                  <button type="button" class="ghost small" data-apply-workflow="in_review">Request Review</button>
                  <button type="button" class="ghost small" data-apply-workflow="published">Approve & Publish</button>
                  <button type="button" class="ghost small" data-apply-workflow="archived">Archive</button>
                </div>
              </div>
              <div class="frontmatter-card">
                <div class="frontmatter-card-header">
                  <div>
                    <strong>Structured Frontmatter</strong>
                    <div class="muted">Edit the common content fields without leaving raw Markdown.</div>
                  </div>
                </div>
                <div class="frontmatter-grid">
                  <label>Title<input id="document-frontmatter-title" data-frontmatter-field="title" type="text" value="${escapeHTML(editorDocument.fields.title)}"></label>
                  <label>Slug<input id="document-frontmatter-slug" data-frontmatter-field="slug" type="text" value="${escapeHTML(editorDocument.fields.slug)}"></label>
                  <label>Layout
                    <select id="document-frontmatter-layout" data-frontmatter-field="layout">
                      <option value="page" ${editorDocument.fields.layout === 'page' ? 'selected' : ''}>page</option>
                      <option value="post" ${editorDocument.fields.layout === 'post' ? 'selected' : ''}>post</option>
                      ${editorDocument.fields.layout && !['page', 'post'].includes(editorDocument.fields.layout) ? `<option value="${escapeHTML(editorDocument.fields.layout)}" selected>${escapeHTML(editorDocument.fields.layout)}</option>` : ''}
                    </select>
                  </label>
                  <label>Date<input id="document-frontmatter-date" data-frontmatter-field="date" type="text" value="${escapeHTML(editorDocument.fields.date || '')}" placeholder="2026-03-07"></label>
                  <label class="frontmatter-span-2">Summary<textarea id="document-frontmatter-summary" data-frontmatter-field="summary" rows="3">${escapeHTML(editorDocument.fields.summary || '')}</textarea></label>
                  <label>Tags<input id="document-frontmatter-tags" data-frontmatter-field="tags" type="text" value="${escapeHTML((editorDocument.fields.tags || []).join(', '))}" placeholder="go, cms, architecture"></label>
                  <label>Categories<input id="document-frontmatter-categories" data-frontmatter-field="categories" type="text" value="${escapeHTML((editorDocument.fields.categories || []).join(', '))}" placeholder="engineering"></label>
                  <label>Language<input id="document-frontmatter-lang" data-frontmatter-field="lang" type="text" value="${escapeHTML(editorDocument.fields.lang || defaultLang)}" placeholder="${escapeHTML(defaultLang)}"></label>
                  <label class="checkbox"><input id="document-frontmatter-draft" data-frontmatter-field="draft" type="checkbox" ${editorDocument.fields.draft ? 'checked' : ''}> Draft</label>
                  <label class="checkbox"><input id="document-frontmatter-archived" data-frontmatter-field="archived" type="checkbox" ${editorDocument.fields.archived ? 'checked' : ''}> Archived</label>
                </div>
              </div>
              <div class="frontmatter-card">
                <div class="frontmatter-card-header">
                  <div>
                    <strong>Attribution</strong>
                    <div class="muted">Tracked automatically in frontmatter and surfaced in history.</div>
                  </div>
                </div>
                <div class="frontmatter-grid">
                  <label>Author<input type="text" value="${escapeHTML(state.documentMeta.author || '')}" readonly></label>
                  <label>Last Editor<input type="text" value="${escapeHTML(state.documentMeta.last_editor || '')}" readonly></label>
                  <label>Created<input type="text" value="${escapeHTML(formatDateTime(state.documentMeta.created_at) || '')}" readonly></label>
                  <label>Updated<input type="text" value="${escapeHTML(formatDateTime(state.documentMeta.updated_at) || '')}" readonly></label>
                </div>
              </div>
              ${
                state.documentFieldSchema.length
                  ? `
                <div class="frontmatter-card">
                  <div class="frontmatter-card-header">
                    <div>
                      <strong>Custom Fields</strong>
                      <div class="muted">Theme-driven fields for ${escapeHTML(editorDocument.fields.layout || 'document')} content.${state.documentContractTitles?.length ? ` Active contracts: ${escapeHTML(state.documentContractTitles.join(', '))}.` : ''}</div>
                    </div>
                  </div>
                  <div class="frontmatter-grid custom-field-grid">
                    ${state.documentFieldSchema.map((schema) => renderFieldSchemaControl(schema)).join('')}
                  </div>
                </div>`
                  : ''
              }
            </div>
            <div class="stack">
              <label>Raw Markdown<textarea id="document-raw" rows="20" spellcheck="false">${escapeHTML(state.documentEditor.raw)}</textarea></label>
              <div class="media-picker">
                <div class="media-picker-header">
                  <div>
                    <strong>Media Picker</strong>
                    <div class="muted">Insert stable <code>media:</code> references at the cursor.</div>
                  </div>
                  <input id="document-media-picker-query" type="search" value="${escapeHTML(state.mediaPickerQuery)}" placeholder="Search media">
                </div>
                <div class="media-picker-list">
                  ${
                    mediaMatches.length
                      ? mediaMatches
                          .map(
                            (item) => `
                      <div class="media-picker-row">
                        <div class="media-picker-meta">
                          <strong>${escapeHTML(item.name)}</strong>
                          <div class="muted mono">${escapeHTML(item.reference)}</div>
                        </div>
                        <div class="row-actions">
                          <button type="button" class="ghost small" data-insert-media="${escapeHTML(item.reference)}" data-insert-mode="auto">Insert</button>
                          <button type="button" class="ghost small" data-insert-media="${escapeHTML(item.reference)}" data-insert-mode="link">Insert Link</button>
                        </div>
                      </div>`
                          )
                          .join('')
                      : '<div class="empty-state">No media matched your search.</div>'
                  }
                </div>
              </div>
            </div>
          </div>
          <div class="toolbar">
            <button type="submit">Save Document</button>
            <button type="button" class="ghost" id="document-preview-button">Preview</button>
            <button type="button" class="ghost" id="document-reset-button">New Draft</button>
          </div>
        </form>
      </div>`;
  };

  const renderCustomFields = () => {
    const contracts = Array.isArray(state.sharedFieldContracts) ? state.sharedFieldContracts : [];
    const raw = state.customFields?.raw || '';
    const saveEnabled = canManageSharedFields();
    return panel(
      'Custom Fields',
      `<div class="panel-pad stack">
        <div class="note">
          Shared custom fields live in <span class="mono">${escapeHTML(state.customFields?.path || 'content/custom-fields.yaml')}</span>. Themes declare the field contracts; this screen edits the shared values they expose.
        </div>
        ${
          saveEnabled
            ? ''
            : '<div class="note">You can view shared custom fields, but saving them requires the <code>config.manage</code> capability.</div>'
        }
        ${
          contracts.length
            ? contracts
                .map(
                  (contract) => `
              <div class="frontmatter-card">
                <div class="frontmatter-card-header">
                  <div>
                    <strong>${escapeHTML(contract.title || contract.key)}</strong>
                    <div class="muted">${escapeHTML(contract.description || `Shared field group: ${contract.key}`)}</div>
                  </div>
                </div>
                <div class="frontmatter-grid custom-field-grid">
                  ${(contract.fields || []).map((schema) => renderSharedFieldSchemaControl(schema, contract.key)).join('')}
                </div>
              </div>`
                )
                .join('')
            : '<div class="empty-state">The active theme does not declare any shared field contracts.</div>'
        }
        <form id="custom-fields-save-form" class="stack">
          <label>Raw YAML<textarea id="custom-fields-raw" rows="18" spellcheck="false" ${saveEnabled ? '' : 'readonly aria-readonly="true"'}>${escapeHTML(raw)}</textarea></label>
          <div class="toolbar"><button type="submit" ${saveEnabled ? '' : 'disabled aria-disabled="true" title="Saving shared custom fields requires config.manage"'}>Save Shared Custom Fields</button></div>
        </form>
      </div>`,
      contracts.length ? `${contracts.length} shared contract${contracts.length === 1 ? '' : 's'} from the active theme` : 'No shared contracts declared'
    );
  };

  const renderDocuments = () => {
    const filteredDocuments = (state.documents || []).filter((doc) => {
      const filters = state.documentFilters || {};
      if (filters.status && documentStatusLabel(doc) !== filters.status) return false;
      if (filters.type && doc.type !== filters.type) return false;
      if (filters.lang && doc.lang !== filters.lang) return false;
      if (
        filters.author &&
        !String(doc.author || '').toLowerCase().includes(String(filters.author).toLowerCase())
      ) {
        return false;
      }
      if (
        filters.tag &&
        !(doc.taxonomies?.tags || [])
          .map((entry) => String(entry).toLowerCase())
          .includes(String(filters.tag).toLowerCase())
      ) {
        return false;
      }
      if (
        filters.category &&
        !(doc.taxonomies?.categories || [])
          .map((entry) => String(entry).toLowerCase())
          .includes(String(filters.category).toLowerCase())
      ) {
        return false;
      }
      if (filters.dateFrom || filters.dateTo) {
        const docDate = doc.date ? new Date(doc.date) : null;
        if (!docDate || Number.isNaN(docDate.getTime())) return false;
        if (filters.dateFrom && docDate < new Date(filters.dateFrom)) return false;
        if (filters.dateTo) {
          const upper = new Date(filters.dateTo);
          upper.setHours(23, 59, 59, 999);
          if (docDate > upper) return false;
        }
      }
      return true;
    });
    const sortedDocuments = sortItems(filteredDocuments, 'documents', (doc, field) => {
      switch (field) {
        case 'type':
          return doc.type;
        case 'status':
          return documentStatusLabel(doc);
        case 'lang':
          return doc.lang;
        default:
          return doc.title || doc.slug || doc.id;
      }
    });
    const pagedDocuments = paginateItems(sortedDocuments, 'documents');
    const selectedDocCount = state.selectedDocuments.length;
    const rows = pagedDocuments.items.map(
      (doc) => `
      <div class="table-row table-row-actions">
        <span><label class="checkbox inline-checkbox"><input type="checkbox" data-select-document="${escapeHTML(doc.source_path)}" ${state.selectedDocuments.includes(doc.source_path) ? 'checked' : ''}><strong>${escapeHTML(doc.title || doc.slug || doc.id)}</strong></label><div class="muted mono">${escapeHTML(doc.source_path)}</div><div class="muted">Author ${escapeHTML(doc.author || '-')} • ${escapeHTML(doc.lang || '-')}</div>${!doc.summary ? '<div class="muted">Missing summary</div>' : ''}${!doc.author ? '<div class="muted">Missing author attribution</div>' : ''}</span>
        <span>${escapeHTML(doc.type)}</span>
        <span>${escapeHTML(documentStatusLabel(doc))}</span>
        <span class="row-actions">
          <button class="ghost small" data-edit-document="${escapeHTML(doc.id)}">Edit</button>
          <button class="ghost small" data-history-document="${escapeHTML(doc.source_path)}">History</button>
          <button class="ghost small" data-set-document-status="${escapeHTML(doc.source_path)}|in_review">Request Review</button>
          <button class="ghost small" data-set-document-status="${escapeHTML(doc.source_path)}|published">Publish</button>
          <button class="ghost small" data-set-document-status="${escapeHTML(doc.source_path)}|draft">Draft</button>
          <button class="ghost small" data-set-document-status="${escapeHTML(doc.source_path)}|archived">Archive</button>
          <button class="ghost small danger" data-delete-document="${escapeHTML(doc.source_path)}">Delete</button>
        </span>
      </div>`
    );

    const preview = state.documentPreview
      ? `<div class="panel-pad stack preview-body">
          ${state.documentPreview.field_errors?.length ? `<div class="warning-panel panel"><div class="panel-pad"><strong>Field Validation</strong><div class="mini-list">${state.documentPreview.field_errors.map((entry) => `<div class="mini-list-row"><span>${escapeHTML(entry)}</span></div>`).join('')}</div></div></div>` : ''}
          ${renderPreviewFrame(state.documentPreview.html)}
        </div>`
      : `<div class="panel-pad empty-state">Use Preview to render the current Markdown body.</div>`;
    const historyRows = renderDocumentHistoryRows(state.documentHistory);
    const trashRows = renderDocumentHistoryRows(state.documentTrash);
    const diffBody = state.documentDiff
      ? `<div class="panel-pad stack">
          <div class="toolbar">
            <button type="button" class="ghost small ${state.documentDiffMode === 'split' ? 'active-toggle' : ''}" data-diff-mode="split">Split View</button>
            <button type="button" class="ghost small ${state.documentDiffMode === 'unified' ? 'active-toggle' : ''}" data-diff-mode="unified">Unified Diff</button>
          </div>
          <div class="status-line mono">${escapeHTML(state.documentDiff.left_path)} -> ${escapeHTML(state.documentDiff.right_path)}</div>
          ${
            state.documentDiffMode === 'split'
              ? renderSplitDiffPane(state.documentDiff.left_raw, state.documentDiff.right_raw)
              : `<pre class="diff-viewer">${escapeHTML(state.documentDiff.diff)}</pre>`
          }
        </div>`
      : `<div class="panel-pad empty-state">Select a version or trashed document and choose Diff to compare it against the current file.</div>`;

    return `
      <div class="layout-grid">
        <div class="stack">
          ${panel(
            'Find Documents',
            `
            <form id="document-search-form" class="panel-pad stack">
              <label>Search Documents<input id="document-search-query" type="search" value="${escapeHTML(state.documentQuery)}" placeholder="Search title, slug, URL, summary, tags, or path"></label>
              <div class="frontmatter-grid">
                <label>Status<select id="document-filter-status"><option value="">Any</option>${['Draft','In Review','Scheduled','Published','Archived'].map((entry) => `<option value="${escapeHTML(entry)}" ${state.documentFilters.status === entry ? 'selected' : ''}>${escapeHTML(entry)}</option>`).join('')}</select></label>
                <label>Type<select id="document-filter-type"><option value="">Any</option>${Array.from(new Set((state.documents || []).map((doc) => doc.type).filter(Boolean))).map((entry) => `<option value="${escapeHTML(entry)}" ${state.documentFilters.type === entry ? 'selected' : ''}>${escapeHTML(entry)}</option>`).join('')}</select></label>
                <label>Language<select id="document-filter-lang"><option value="">Any</option>${Array.from(new Set((state.documents || []).map((doc) => doc.lang).filter(Boolean))).map((entry) => `<option value="${escapeHTML(entry)}" ${state.documentFilters.lang === entry ? 'selected' : ''}>${escapeHTML(entry)}</option>`).join('')}</select></label>
                <label>Author<input id="document-filter-author" type="text" value="${escapeHTML(state.documentFilters.author || '')}" placeholder="Filter by author"></label>
                <label>Tag<input id="document-filter-tag" type="text" value="${escapeHTML(state.documentFilters.tag || '')}" placeholder="Exact tag"></label>
                <label>Category<input id="document-filter-category" type="text" value="${escapeHTML(state.documentFilters.category || '')}" placeholder="Exact category"></label>
                <label>Date From<input id="document-filter-date-from" type="date" value="${escapeHTML(state.documentFilters.dateFrom || '')}"></label>
                <label>Date To<input id="document-filter-date-to" type="date" value="${escapeHTML(state.documentFilters.dateTo || '')}"></label>
              </div>
              <div class="toolbar">
                <button type="submit">Search</button>
                <button type="button" class="ghost" id="document-search-clear">Clear</button>
                <button type="button" class="ghost" data-open-editor>Open Editor</button>
              </div>
            </form>
          `,
            'Keep management here, then jump into Editor to write'
          )}
          ${panel('Bulk Editorial Actions', `
            <div class="panel-pad stack">
              <div class="note">${selectedDocCount ? `${escapeHTML(selectedDocCount)} documents selected.` : 'Select documents from the list to run bulk editorial actions.'}</div>
              <div class="frontmatter-grid">
                <label>Status<select id="document-bulk-status"><option value="">No change</option>${['draft','in_review','scheduled','published','archived'].map((entry) => `<option value="${entry}" ${state.documentBulk.status === entry ? 'selected' : ''}>${escapeHTML(entry)}</option>`).join('')}</select></label>
                <label>Author<input id="document-bulk-author" type="text" value="${escapeHTML(state.documentBulk.author || '')}" placeholder="Set author"></label>
                <label>Language<input id="document-bulk-lang" type="text" value="${escapeHTML(state.documentBulk.lang || '')}" placeholder="Set lang"></label>
                <label>Tags<input id="document-bulk-tags" type="text" value="${escapeHTML(state.documentBulk.tags || '')}" placeholder="append tags"></label>
                <label>Categories<input id="document-bulk-categories" type="text" value="${escapeHTML(state.documentBulk.categories || '')}" placeholder="append categories"></label>
              </div>
              <div class="toolbar">
                <button type="button" id="document-select-all-visible">Select Visible</button>
                <button type="button" class="ghost" id="document-clear-selection">Clear Selection</button>
                <button type="button" class="ghost" id="document-bulk-apply" ${selectedDocCount ? '' : 'disabled'}>Apply Bulk Changes</button>
              </div>
            </div>`,
            'Workflow, taxonomy, language, and author updates for selected documents')}
          ${panel('Documents', `${renderTableControls(state, 'documents', filteredDocuments.length, pagedDocuments.totalPages)}<div class="table table-four"><div class="table-head"><span>Document</span><span>Type</span><span>Status</span><span>Actions</span></div>${rows.length ? rows.join('') : '<div class="panel-pad empty-state">No documents matched the current search or filters. Try a broader query, clear filters, or create a new page/post.</div>'}</div>`, `${filteredDocuments.length} matching documents`)}
          ${panel('Trash', `<div class="table table-four"><div class="table-head"><span>Document</span><span>State</span><span>Captured</span><span>Actions</span></div>${trashRows || '<div class="panel-pad empty-state">No trashed documents.</div>'}</div>`, `${state.documentTrash.length} trashed`)}
        </div>
        <div class="stack">
          ${panel('Preview', preview, state.documentPreview ? state.documentPreview.title || state.documentPreview.slug || 'Rendered preview' : 'No preview yet')}
          ${panel('History', `<div class="table table-four"><div class="table-head"><span>Document</span><span>State</span><span>Captured</span><span>Actions</span></div>${historyRows || '<div class="panel-pad empty-state">Select a document to inspect version and trash history.</div>'}</div>`, state.documentHistoryPath || 'No document selected')}
          ${panel('Diff', diffBody, 'Line-based diff against the current version')}
          ${renderWidgetPanels('documents.sidebar').join('')}
        </div>
      </div>`;
  };

  const renderEditor = () => `
    <div class="layout-grid">
      <div class="stack">
        ${panel('Editor', renderEditorPanel(), 'Create, edit, publish, archive, or soft-delete Markdown content')}
      </div>
      <div class="stack">
        ${panel(
          'Inline Preview',
          state.documentPreview
            ? `<div class="panel-pad stack preview-body">
                ${state.documentPreview.field_errors?.length ? `<div class="warning-panel panel"><div class="panel-pad"><strong>Field Validation</strong><div class="mini-list">${state.documentPreview.field_errors.map((entry) => `<div class="mini-list-row"><span>${escapeHTML(entry)}</span></div>`).join('')}</div></div></div>` : ''}
                ${renderPreviewFrame(state.documentPreview.html)}
              </div>`
            : '<div class="panel-pad empty-state">Use Preview while editing to keep the authoring loop inside Editor.</div>',
          'Preview now lives in Editor as well as Documents'
        )}
        ${panel(
          'Editor Status',
          `
          <div class="panel-pad stack">
            <div class="status-line"><strong>Source Path:</strong> <span class="mono">${escapeHTML(state.documentEditor.source_path || 'Unsaved draft')}</span></div>
            <div class="status-line"><strong>Current Status:</strong> <span>${escapeHTML(documentStatusLabel({ status: state.documentMeta.status || 'draft' }))}</span></div>
            <div class="status-line"><strong>Author:</strong> <span>${escapeHTML(state.documentMeta.author || 'Unassigned')}</span></div>
            <div class="status-line"><strong>Last Editor:</strong> <span>${escapeHTML(state.documentMeta.last_editor || 'Unassigned')}</span></div>
            <div class="note"><strong>Publishing flow:</strong> draft -> in review -> scheduled/published -> archived. Publishing and scheduling actions now prompt with a summary before save.</div>
            <div class="toolbar">
              <button type="button" class="ghost" data-open-documents>Open Documents</button>
              <button type="button" class="ghost" id="editor-preview-documents">Show Preview on Documents</button>
            </div>
          </div>
        `,
          'Preview, diff, history, and trash stay on the Documents page'
        )}
      </div>
    </div>`;

  const renderMedia = () => {
    const filteredMedia = (state.media || []).filter((item) => {
      const filters = state.mediaFilters || {};
      if (filters.kind && item.kind !== filters.kind) return false;
      if (filters.collection && item.collection !== filters.collection) return false;
      if (filters.usage === 'used' && !(item.used_by_count > 0)) return false;
      if (filters.usage === 'unused' && item.used_by_count > 0) return false;
      return true;
    });
    const sortedMedia = sortItems(filteredMedia, 'media', (item, field) => {
      switch (field) {
        case 'kind':
          return item.kind;
        case 'reference':
          return item.reference;
        default:
          return item.name;
      }
    });
    const pagedMedia = paginateItems(sortedMedia, 'media');
    const duplicateHashes = Object.entries(
      (state.media || []).reduce((acc, item) => {
        const key = item.metadata?.content_hash || '';
        if (!key) return acc;
        acc[key] = acc[key] || [];
        acc[key].push(item.reference);
        return acc;
      }, {})
    ).filter(([, refs]) => refs.length > 1);
    const rows = pagedMedia.items.map(
      (item) => `
      <div class="table-row table-row-actions">
        <span class="media-library-cell">${mediaThumb(item)}<span><label class="checkbox inline-checkbox"><input type="checkbox" data-select-media-library="${escapeHTML(item.reference)}" ${state.selectedMediaLibrary.includes(item.reference) ? 'checked' : ''}><strong>${escapeHTML(item.name)}</strong></label><div class="muted mono">${escapeHTML(item.reference)}</div><div class="muted">Used by ${escapeHTML(String(item.used_by_count || 0))} document(s)</div></span></span>
        <span>${escapeHTML(item.kind)}</span>
        <span>${escapeHTML(item.metadata?.title || item.metadata?.alt || '')}</span>
        <span class="row-actions">
          <button class="ghost small" data-edit-media="${escapeHTML(item.reference)}">Details</button>
          <button class="ghost small" data-history-media-path="${escapeHTML(`content/${item.collection}/${item.path}`)}">History</button>
          <button class="ghost small" data-prepare-media-replace="${escapeHTML(item.reference)}">Replace</button>
          <a class="button-link ghost small" href="${escapeHTML(item.public_url)}" target="_blank" rel="noreferrer">View</a>
          <button class="ghost small danger" data-delete-media="${escapeHTML(item.reference)}">Delete</button>
        </span>
      </div>`
    );

    const detail = state.mediaDetail;
    const metadata = detail?.metadata || {};
    const historyRows = renderMediaHistoryRows(state.mediaHistory);
    const trashRows = renderMediaHistoryRows(state.mediaTrash);
    return `
      <div class="layout-grid">
        <div class="stack">
          ${panel(
            'Upload Media',
            `
            <form id="media-upload-form" class="panel-pad stack">
              <label>Search Library<input id="media-search-query" type="search" value="${escapeHTML(state.mediaQuery)}" placeholder="Search name, reference, metadata, or tags"></label>
              <div class="frontmatter-grid">
                <label>Kind<select id="media-filter-kind"><option value="">Any</option>${Array.from(new Set((state.media || []).map((item) => item.kind).filter(Boolean))).map((entry) => `<option value="${escapeHTML(entry)}" ${state.mediaFilters.kind === entry ? 'selected' : ''}>${escapeHTML(entry)}</option>`).join('')}</select></label>
                <label>Collection<select id="media-filter-collection"><option value="">Any</option>${Array.from(new Set((state.media || []).map((item) => item.collection).filter(Boolean))).map((entry) => `<option value="${escapeHTML(entry)}" ${state.mediaFilters.collection === entry ? 'selected' : ''}>${escapeHTML(entry)}</option>`).join('')}</select></label>
                <label>Usage<select id="media-filter-usage"><option value="">Any</option><option value="used" ${state.mediaFilters.usage === 'used' ? 'selected' : ''}>Used</option><option value="unused" ${state.mediaFilters.usage === 'unused' ? 'selected' : ''}>Unused</option></select></label>
              </div>
              <label>Collection<select id="media-collection"><option value="">Auto</option><option value="images">images</option><option value="videos">videos</option><option value="audio">audio</option><option value="documents">documents</option></select></label>
              <!-- Directory uploads remain supported by the backend, but the default admin theme intentionally hides this field to avoid path confusion in the UI. -->
              <label>File<input id="media-file" type="file"></label>
              <div class="toolbar">
                <button type="submit">Upload Media</button>
                <button type="button" class="ghost" id="media-search-apply">Search</button>
                <button type="button" class="ghost" id="media-search-clear">Clear</button>
              </div>
            </form>
          `,
          'Uploads return stable media: references that can be used in Markdown'
          )}
          ${panel('Bulk Media Actions', `
            <div class="panel-pad stack">
              <div class="note">${state.selectedMediaLibrary.length ? `${escapeHTML(state.selectedMediaLibrary.length)} media items selected.` : 'Select media from the library to apply bulk tags.'}</div>
              <div class="frontmatter-grid">
                <label>Append Tags<input id="media-bulk-tags" type="text" value="${escapeHTML(state.mediaBulkTags || '')}" placeholder="campaign, featured"></label>
              </div>
              <div class="toolbar">
                <button type="button" id="media-select-all-visible">Select Visible</button>
                <button type="button" class="ghost" id="media-clear-selection">Clear Selection</button>
                <button type="button" class="ghost" id="media-bulk-apply" ${state.selectedMediaLibrary.length ? '' : 'disabled'}>Apply Tags</button>
              </div>
            </div>`,
            'Bulk tag updates for selected media')}
          ${panel('Library', `${renderTableControls(state, 'media', filteredMedia.length, pagedMedia.totalPages)}<div class="table table-four"><div class="table-head"><span>Name</span><span>Kind</span><span>Metadata</span><span>Actions</span></div>${rows.length ? rows.join('') : '<div class="panel-pad empty-state">No media matched the current search or filters. Upload a file or clear the filters.</div>'}</div>`, `${filteredMedia.length} matching media items`)}
          ${panel('Trash', `<div class="table table-four"><div class="table-head"><span>Name</span><span>State</span><span>Captured</span><span>Actions</span></div>${trashRows || '<div class="panel-pad empty-state">No trashed media.</div>'}</div>`, `${state.mediaTrash.length} trashed`)}
        </div>
        <div class="stack">
          ${panel(
            'Selected Media',
            `
            <div class="panel-pad stack">
              ${mediaPreview(detail)}
              <div class="status-line mono">${escapeHTML(detail?.reference || '')}</div>
              <div class="stack subtle-meta">
                <div class="status-line"><strong>Original file:</strong> <span class="mono">${escapeHTML(metadata.original_filename || '')}</span></div>
                <div class="status-line"><strong>Stored file:</strong> <span class="mono">${escapeHTML(metadata.stored_filename || detail?.name || '')}</span></div>
                <div class="status-line"><strong>Type:</strong> <span class="mono">${escapeHTML(metadata.mime_type || detail?.kind || '')}</span></div>
                <div class="status-line"><strong>Hash:</strong> <span class="mono">${escapeHTML(metadata.content_hash || '')}</span></div>
                <div class="status-line"><strong>Size:</strong> <span class="mono">${escapeHTML(String(metadata.file_size || detail?.size || ''))}</span></div>
                <div class="status-line"><strong>Dimensions:</strong> <span class="mono">${escapeHTML(
                  metadata.width && metadata.height
                    ? `${metadata.width} x ${metadata.height}`
                    : 'n/a'
                )}</span></div>
                <div class="status-line"><strong>Used by:</strong> <span class="mono">${escapeHTML(String(detail?.used_by?.length || detail?.used_by_count || 0))}</span></div>
                <div class="status-line"><strong>Duplicate references:</strong> <span class="mono">${escapeHTML(String((duplicateHashes.find(([hash]) => hash === metadata.content_hash)?.[1] || []).length > 1 ? (duplicateHashes.find(([hash]) => hash === metadata.content_hash)?.[1] || []).join(', ') : 'none'))}</span></div>
                <div class="status-line"><strong>Uploaded:</strong> <span>${escapeHTML(metadata.uploaded_at ? formatDateTime(metadata.uploaded_at) : '')}</span></div>
                <div class="status-line"><strong>Uploaded by:</strong> <span>${escapeHTML(metadata.uploaded_by || '')}</span></div>
              </div>
              <form id="media-metadata-form" class="stack">
                <label>Title<input id="media-title" type="text" value="${escapeHTML(metadata.title || '')}"></label>
                <label>Alt Text<input id="media-alt" type="text" value="${escapeHTML(metadata.alt || '')}"></label>
                <label>Caption<input id="media-caption" type="text" value="${escapeHTML(metadata.caption || '')}"></label>
                <label>Description<textarea id="media-description" rows="5">${escapeHTML(metadata.description || '')}</textarea></label>
                <label>Credit<input id="media-credit" type="text" value="${escapeHTML(metadata.credit || '')}"></label>
                <label>Tags<input id="media-tags" type="text" value="${escapeHTML((metadata.tags || []).join(', '))}" placeholder="product, hero, launch"></label>
                <label>Focal Point X<input id="media-focal-x" type="text" value="${escapeHTML(String(metadata.focal_x || ''))}" placeholder="0.5"></label>
                <label>Focal Point Y<input id="media-focal-y" type="text" value="${escapeHTML(String(metadata.focal_y || ''))}" placeholder="0.5"></label>
                <label>Version Comment<input id="media-version-comment" type="text" value="${escapeHTML(state.mediaVersionComment || '')}" placeholder="Explain what changed in this metadata revision"></label>
                <label>Replace File<input id="media-replace-file" type="file" ${detail ? '' : 'disabled'}></label>
                <div class="toolbar">
                  <button type="submit" ${detail ? '' : 'disabled'}>Save Metadata</button>
                  <button type="button" class="ghost" id="media-replace-button" ${detail ? '' : 'disabled'}>Replace Media</button>
                </div>
              </form>
            </div>
          `,
            'Metadata is stored beside the file as .meta.yaml'
          )}
          ${panel('Duplicate Content Hashes', duplicateHashes.length ? `<div class="table table-two"><div class="table-head"><span>Hash</span><span>References</span></div>${duplicateHashes.map(([hash, refs]) => `<div class="table-row"><span class="mono">${escapeHTML(hash)}</span><span class="muted">${escapeHTML(refs.join(', '))}</span></div>`).join('')}</div>` : '<div class="panel-pad empty-state">No duplicate media hashes detected in the current library snapshot.</div>', 'Hash-level duplicate detection for DAM hygiene')}
          ${panel(
            'Used By',
            `
            <div class="table table-four">
              <div class="table-head"><span>Document</span><span>Type</span><span>Status</span><span>Path</span></div>
              ${
                detail?.used_by?.length
                  ? detail.used_by
                      .map(
                        (doc) => `
                  <div class="table-row">
                    <span><strong>${escapeHTML(doc.title || doc.slug || doc.id)}</strong></span>
                    <span>${escapeHTML(doc.type)}</span>
                    <span>${escapeHTML(documentStatusLabel(doc))}</span>
                    <span class="mono"><button class="ghost small" data-edit-document-path="${escapeHTML(doc.source_path)}">Open</button> ${escapeHTML(doc.source_path)}</span>
                  </div>`
                      )
                      .join('')
                  : '<div class="panel-pad empty-state">No documents reference this media yet.</div>'
              }
            </div>
          `,
            'Documents currently referencing this media: reference'
          )}
          ${panel('History', `<div class="table table-four"><div class="table-head"><span>Name</span><span>State</span><span>Captured</span><span>Actions</span></div>${historyRows || '<div class="panel-pad empty-state">Select a media item to inspect version and trash history.</div>'}</div>`, state.mediaHistoryReference || 'No media selected')}
          ${renderWidgetPanels('media.sidebar').join('')}
        </div>
      </div>`;
  };

  const renderHistory = () => `
    <div class="layout-grid">
      <div class="stack">
        ${panel('Document History', `<div class="table table-four"><div class="table-head"><span>Document</span><span>State</span><span>Captured</span><span>Actions</span></div>${renderDocumentHistoryRows(state.documentHistory) || '<div class="panel-pad empty-state">Choose History from a document to inspect revisions and restore points.</div>'}</div>`, state.documentHistoryPath || 'No document selected')}
        ${panel('Media History', `<div class="table table-four"><div class="table-head"><span>Name</span><span>State</span><span>Captured</span><span>Actions</span></div>${renderMediaHistoryRows(state.mediaHistory) || '<div class="panel-pad empty-state">Choose History from a media item to inspect revisions and restore points.</div>'}</div>`, state.mediaHistoryReference || 'No media selected')}
      </div>
      <div class="stack">
        ${panel(
          'Document Diff',
          state.documentDiff
            ? `<div class="panel-pad stack">
              <div class="toolbar">
                <button type="button" class="ghost small ${state.documentDiffMode === 'split' ? 'active-toggle' : ''}" data-diff-mode="split">Split View</button>
                <button type="button" class="ghost small ${state.documentDiffMode === 'unified' ? 'active-toggle' : ''}" data-diff-mode="unified">Unified Diff</button>
              </div>
              ${
                state.documentDiffMode === 'split'
                  ? renderSplitDiffPane(state.documentDiff.left_raw, state.documentDiff.right_raw)
                  : `<pre class="diff-viewer">${escapeHTML(state.documentDiff.diff)}</pre>`
              }
            </div>`
            : '<div class="panel-pad empty-state">Select a document version and choose Diff to review the changes.</div>',
          'Side-by-side and unified views are both available'
        )}
      </div>
    </div>`;

  const renderAudit = () =>
    panel(
      'Audit Log',
      `
    ${(() => {
      const filters = state.auditFilters || {};
      const filteredAudit = (state.audit || []).filter((entry) => {
        if (
          filters.actor &&
          !String(entry.actor || '').toLowerCase().includes(String(filters.actor).toLowerCase())
        )
          return false;
        if (
          filters.action &&
          !String(entry.action || '').toLowerCase().includes(String(filters.action).toLowerCase())
        )
          return false;
        if (
          filters.outcome &&
          String(entry.outcome || '').toLowerCase() !== String(filters.outcome).toLowerCase()
        )
          return false;
        return true;
      });
      const sortedAudit = sortItems(filteredAudit, 'audit', (entry, field) => {
        switch (field) {
          case 'action':
            return entry.action;
          case 'actor':
            return entry.actor;
          case 'outcome':
            return entry.outcome;
          default:
            return entry.timestamp;
        }
      });
      const pagedAudit = paginateItems(sortedAudit, 'audit');
      return `<div class="panel-pad stack">
        <div class="frontmatter-grid">
          <label>Actor<input id="audit-filter-actor" type="text" value="${escapeHTML(filters.actor || '')}" placeholder="actor"></label>
          <label>Action<input id="audit-filter-action" type="text" value="${escapeHTML(filters.action || '')}" placeholder="action"></label>
          <label>Outcome<select id="audit-filter-outcome"><option value="">Any</option><option value="success" ${filters.outcome === 'success' ? 'selected' : ''}>success</option><option value="fail" ${filters.outcome === 'fail' ? 'selected' : ''}>fail</option></select></label>
          <div class="toolbar"><button type="button" class="ghost" id="audit-filter-apply">Apply Filters</button><button type="button" class="ghost" id="audit-filter-clear">Clear</button></div>
        </div>
      </div>${renderTableControls(state, 'audit', filteredAudit.length, pagedAudit.totalPages)}
    <div class="table table-four">
      <div class="table-head"><span>Action</span><span>Actor</span><span>Outcome</span><span>Details</span></div>
      ${
        pagedAudit.items.length
          ? pagedAudit.items
              .map(
                (entry) => `
          <div class="table-row">
            <span><strong>${escapeHTML(entry.action)}</strong><div class="muted">${escapeHTML(formatDateTime(entry.timestamp))}</div></span>
            <span>${escapeHTML(entry.actor || '-')}</span>
            <span>${escapeHTML(entry.outcome || '-')}</span>
            <span><div>${escapeHTML(entry.target || '-')}</div>${entry.remote_addr ? `<div class="muted mono">${escapeHTML(entry.remote_addr)}</div>` : ''}</span>
          </div>`
              )
              .join('')
          : '<div class="panel-pad empty-state">No audit log entries yet. Activity will appear here after logins and admin actions.</div>'
      }
    </div>`;
    })()}`,
      `${(state.audit || []).length} recent events`
    );

  const formatBytes = (value) => {
    const bytes = Number(value || 0);
    if (!Number.isFinite(bytes) || bytes <= 0) return '0 B';
    const units = ['B', 'KB', 'MB', 'GB', 'TB'];
    const exp = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1);
    const scaled = bytes / 1024 ** exp;
    return `${scaled >= 10 || exp === 0 ? scaled.toFixed(0) : scaled.toFixed(1)} ${units[exp]}`;
  };

  const formatUptime = (seconds) => {
    const total = Number(seconds || 0);
    if (!Number.isFinite(total) || total <= 0) return '0m';
    const days = Math.floor(total / 86400);
    const hours = Math.floor((total % 86400) / 3600);
    const minutes = Math.floor((total % 3600) / 60);
    if (days > 0) return `${days}d ${hours}h`;
    if (hours > 0) return `${hours}h ${minutes}m`;
    return `${minutes}m`;
  };

  const formatMilliseconds = (value) => `${Number(value || 0)} ms`;

  const sortedEntries = (value) =>
    Object.entries(value || {}).sort((a, b) => a[0].localeCompare(b[0]));

  const renderMiniList = (value, formatter = (entryValue) => String(entryValue)) => {
    const entries = sortedEntries(value);
    if (!entries.length) return '<div class="empty-state">No data yet.</div>';
    return `<div class="mini-list">${entries
      .map(
        ([key, entryValue]) => `
      <div class="mini-list-row">
        <span>${escapeHTML(key)}</span>
        <strong>${escapeHTML(formatter(entryValue))}</strong>
      </div>`
      )
      .join('')}</div>`;
  };

  const renderLargestFiles = (files) => {
    if (!files?.length) return '<div class="empty-state">No file size data yet.</div>';
    return `<div class="mini-list">${files
      .map(
        (file) => `
      <div class="mini-list-row">
        <span class="mono">${escapeHTML(file.path)}</span>
        <strong>${escapeHTML(formatBytes(file.size_bytes))}</strong>
      </div>`
      )
      .join('')}</div>`;
  };

  const renderDiagnostics = () => {
    if (!debugEnabled()) {
      return panel(
        'Diagnostics',
        '<div class="panel-pad empty-state">pprof is disabled. Set <code>admin.debug.pprof: true</code> in site.yaml to enable runtime profiling in the admin.</div>'
      );
    }
    const pprofBase = `${adminBase}/debug/pprof`;
    const runtime = state.runtimeStatus;
    return `
      <div class="layout-grid">
        <div class="stack">
          ${panel(
            'Runtime Summary',
            runtime
              ? `<div class="panel-pad stack">
                <div class="cards">
                  <article class="card"><span class="card-label">Heap Alloc</span><strong>${escapeHTML(formatBytes(runtime.heap_alloc_bytes))}</strong><span class="card-copy">Live heap bytes allocated.</span></article>
                  <article class="card"><span class="card-label">Heap In Use</span><strong>${escapeHTML(formatBytes(runtime.heap_inuse_bytes))}</strong><span class="card-copy">Heap spans currently in use.</span></article>
                  <article class="card"><span class="card-label">Heap Objects</span><strong>${escapeHTML(String(runtime.heap_objects || 0))}</strong><span class="card-copy">Objects tracked by the runtime.</span></article>
                  <article class="card"><span class="card-label">Goroutines</span><strong>${escapeHTML(String(runtime.goroutines || 0))}</strong><span class="card-copy">Live goroutines right now.</span></article>
                  <article class="card"><span class="card-label">Process CPU</span><strong>${escapeHTML(String((runtime.process_user_cpu_ms || 0) + (runtime.process_system_cpu_ms || 0)))} ms</strong><span class="card-copy">Accumulated user + system CPU time.</span></article>
                  <article class="card"><span class="card-label">GC Runs</span><strong>${escapeHTML(String(runtime.num_gc || 0))}</strong><span class="card-copy">Completed garbage collection cycles.</span></article>
                  <article class="card"><span class="card-label">Uptime</span><strong>${escapeHTML(formatUptime(runtime.uptime_seconds))}</strong><span class="card-copy">Time since the current process started.</span></article>
                  <article class="card"><span class="card-label">CPU Cores</span><strong>${escapeHTML(String(runtime.num_cpu || 0))}</strong><span class="card-copy">Logical CPUs available to the process.</span></article>
                </div>
                <div class="subtle-meta">
                  <div><strong>Captured:</strong> ${escapeHTML(formatDateTime(runtime.captured_at) || '')}</div>
                  <div><strong>Go Version:</strong> ${escapeHTML(runtime.go_version || 'n/a')}</div>
                  <div><strong>Stack In Use:</strong> ${escapeHTML(formatBytes(runtime.stack_inuse_bytes))}</div>
                  <div><strong>Runtime Sys:</strong> ${escapeHTML(formatBytes(runtime.sys_bytes))}</div>
                  <div><strong>Next GC:</strong> ${escapeHTML(formatBytes(runtime.next_gc_bytes))}</div>
                  <div><strong>Last GC:</strong> ${escapeHTML(formatDateTime(runtime.last_gc_at) || 'n/a')}</div>
                  <div><strong>Live Reload Mode:</strong> ${escapeHTML(runtime.live_reload_mode || 'n/a')}</div>
                </div>
                <div class="toolbar">
                  <button type="button" class="ghost" id="debug-refresh-runtime">Refresh Runtime Snapshot</button>
                </div>
              </div>`
              : '<div class="panel-pad empty-state">No runtime snapshot loaded yet.</div>',
            'Heap, CPU, goroutines, and GC at a glance'
          )}
          ${panel(
            'Content Inventory',
            runtime
              ? `<div class="panel-pad stack">
                <div class="cards">
                  <article class="card"><span class="card-label">Documents</span><strong>${escapeHTML(String(runtime.content?.document_count || 0))}</strong><span class="card-copy">Total loaded documents.</span></article>
                  <article class="card"><span class="card-label">Routes</span><strong>${escapeHTML(String(runtime.content?.route_count || 0))}</strong><span class="card-copy">Resolved routes in the current graph.</span></article>
                  <article class="card"><span class="card-label">Taxonomies</span><strong>${escapeHTML(String(runtime.content?.taxonomy_count || 0))}</strong><span class="card-copy">Configured taxonomy groups in use.</span></article>
                  <article class="card"><span class="card-label">Terms</span><strong>${escapeHTML(String(runtime.content?.taxonomy_term_count || 0))}</strong><span class="card-copy">Known taxonomy terms across all groups.</span></article>
                </div>
                <div class="debug-grid-two">
                  <div>
                    <h3>Status</h3>
                    ${renderMiniList(runtime.content?.by_status)}
                  </div>
                  <div>
                    <h3>Languages</h3>
                    ${renderMiniList(runtime.content?.by_lang)}
                  </div>
                  <div>
                    <h3>Document Types</h3>
                    ${renderMiniList(runtime.content?.by_type)}
                  </div>
                  <div>
                    <h3>Media Collections</h3>
                    ${renderMiniList(runtime.content?.media_counts)}
                  </div>
                </div>
              </div>`
              : '<div class="panel-pad empty-state">No content inventory loaded yet.</div>',
            'Document, route, taxonomy, language, and media totals'
          )}
          ${panel(
            'Storage Footprint',
            runtime
              ? `<div class="panel-pad stack">
                <div class="cards">
                  <article class="card"><span class="card-label">Content Dir</span><strong>${escapeHTML(formatBytes(runtime.storage?.content_bytes))}</strong><span class="card-copy">Current size of the content tree.</span></article>
                  <article class="card"><span class="card-label">Public Dir</span><strong>${escapeHTML(formatBytes(runtime.storage?.public_bytes))}</strong><span class="card-copy">Current generated output footprint.</span></article>
                  <article class="card"><span class="card-label">Versions</span><strong>${escapeHTML(String(runtime.storage?.derived_version_count || 0))}</strong><span class="card-copy">Retained version snapshots on disk.</span></article>
                  <article class="card"><span class="card-label">Trash</span><strong>${escapeHTML(String(runtime.storage?.derived_trash_count || 0))}</strong><span class="card-copy">Soft-deleted files still retained.</span></article>
                </div>
                <div class="subtle-meta">
                  <div><strong>Derived Bytes:</strong> ${escapeHTML(formatBytes(runtime.storage?.derived_bytes))}</div>
                </div>
                <div class="debug-grid-two">
                  <div>
                    <h3>Media Count By Collection</h3>
                    ${renderMiniList(runtime.storage?.media_counts)}
                  </div>
                  <div>
                    <h3>Media Size By Collection</h3>
                    ${renderMiniList(runtime.storage?.media_bytes, formatBytes)}
                  </div>
                </div>
                <div>
                  <h3>Largest Files</h3>
                  ${renderLargestFiles(runtime.storage?.largest_files)}
                </div>
              </div>`
              : '<div class="panel-pad empty-state">No storage snapshot loaded yet.</div>',
            'Disk footprint across content, output, and retained lifecycle files'
          )}
          ${panel(
            'Last Build Report',
            runtime?.last_build
              ? `<div class="panel-pad stack">
                <div class="cards">
                  <article class="card"><span class="card-label">Generated</span><strong>${escapeHTML(formatDateTime(runtime.last_build.generated_at) || 'n/a')}</strong><span class="card-copy">Most recent persisted build report.</span></article>
                  <article class="card"><span class="card-label">Documents</span><strong>${escapeHTML(String(runtime.last_build.document_count || 0))}</strong><span class="card-copy">Documents included in that build.</span></article>
                  <article class="card"><span class="card-label">Routes</span><strong>${escapeHTML(String(runtime.last_build.route_count || 0))}</strong><span class="card-copy">Routes emitted in that build.</span></article>
                  <article class="card"><span class="card-label">Mode</span><strong>${escapeHTML(runtime.last_build.preview ? 'Preview' : 'Standard')}</strong><span class="card-copy">${escapeHTML(runtime.last_build.environment || 'default')}${runtime.last_build.target ? ` / ${escapeHTML(runtime.last_build.target)}` : ''}</span></article>
                </div>
                <div class="debug-grid-two">
                  <div>
                    <h3>Build Timings</h3>
                    ${renderMiniList(
                      {
                        prepare: runtime.last_build.prepare_ms || 0,
                        assets: runtime.last_build.assets_ms || 0,
                        documents: runtime.last_build.documents_ms || 0,
                        taxonomies: runtime.last_build.taxonomies_ms || 0,
                        search: runtime.last_build.search_ms || 0,
                      },
                      formatMilliseconds
                    )}
                  </div>
                </div>
              </div>`
              : '<div class="panel-pad empty-state">No persisted build report yet. Run <code>foundry build</code> or <code>foundry build --preview</code> to capture one.</div>',
            'Latest static build metrics written by the CLI'
          )}
          ${panel(
            'Integrity & Activity',
            runtime
              ? `<div class="panel-pad stack">
                <div class="cards">
                  <article class="card"><span class="card-label">Broken Media Refs</span><strong>${escapeHTML(String(runtime.integrity?.broken_media_refs || 0))}</strong><span class="card-copy">Current unresolved <code>media:</code> references.</span></article>
                  <article class="card"><span class="card-label">Broken Links</span><strong>${escapeHTML(String(runtime.integrity?.broken_internal_links || 0))}</strong><span class="card-copy">Internal links that do not resolve.</span></article>
                  <article class="card"><span class="card-label">Orphaned Media</span><strong>${escapeHTML(String(runtime.integrity?.orphaned_media || 0))}</strong><span class="card-copy">Media files with no current references.</span></article>
                  <article class="card"><span class="card-label">Missing Templates</span><strong>${escapeHTML(String(runtime.integrity?.missing_templates || 0))}</strong><span class="card-copy">Layouts missing from the active theme.</span></article>
                  <article class="card"><span class="card-label">Active Sessions</span><strong>${escapeHTML(String(runtime.activity?.active_sessions || 0))}</strong><span class="card-copy">Currently persisted admin sessions.</span></article>
                  <article class="card"><span class="card-label">Document Locks</span><strong>${escapeHTML(String(runtime.activity?.active_document_locks || 0))}</strong><span class="card-copy">Active editor locks right now.</span></article>
                  <article class="card"><span class="card-label">Audit Events</span><strong>${escapeHTML(String(runtime.activity?.recent_audit_events || 0))}</strong><span class="card-copy">Events in the last ${escapeHTML(String(runtime.activity?.audit_window_hours || 24))} hours.</span></article>
                  <article class="card"><span class="card-label">Failed Logins</span><strong>${escapeHTML(String(runtime.activity?.recent_failed_logins || 0))}</strong><span class="card-copy">Failed login attempts in the audit window.</span></article>
                </div>
                <div class="debug-grid-two">
                  <div>
                    <h3>Integrity Totals</h3>
                    ${renderMiniList({
                      duplicate_urls: runtime.integrity?.duplicate_urls || 0,
                      duplicate_slugs: runtime.integrity?.duplicate_slugs || 0,
                      taxonomy_inconsistency: runtime.integrity?.taxonomy_inconsistency || 0,
                    })}
                  </div>
                  <div>
                    <h3>Recent Audit Actions</h3>
                    ${renderMiniList(runtime.activity?.recent_audit_by_action)}
                  </div>
                </div>
              </div>`
              : '<div class="panel-pad empty-state">No integrity or activity snapshot loaded yet.</div>',
            'Reference health, route safety, and recent admin activity'
          )}
          ${panel(
            'Site Validation',
            state.siteValidation
              ? `<div class="panel-pad stack">
                <div class="toolbar">
                  <button type="button" class="ghost" id="debug-validate-site">Run Validation</button>
                </div>
                <div class="cards">
                  <article class="card"><span class="card-label">Findings</span><strong>${escapeHTML(String(state.siteValidation.message_count || 0))}</strong><span class="card-copy">Latest on-demand site validation result.</span></article>
                </div>
                <div class="debug-grid-two">
                  <div><h3>Broken Media</h3>${state.siteValidation.broken_media_refs?.length ? `<div class="mini-list">${state.siteValidation.broken_media_refs.slice(0, 8).map((entry) => `<div class="mini-list-row"><span>${escapeHTML(entry)}</span></div>`).join('')}</div>` : '<div class="empty-state">No broken media refs.</div>'}</div>
                  <div><h3>Broken Links</h3>${state.siteValidation.broken_internal_links?.length ? `<div class="mini-list">${state.siteValidation.broken_internal_links.slice(0, 8).map((entry) => `<div class="mini-list-row"><span>${escapeHTML(entry)}</span></div>`).join('')}</div>` : '<div class="empty-state">No broken internal links.</div>'}</div>
                  <div><h3>Templates & Routes</h3>${[...(state.siteValidation.missing_templates || []), ...(state.siteValidation.duplicate_urls || []), ...(state.siteValidation.duplicate_slugs || [])].length ? `<div class="mini-list">${[...(state.siteValidation.missing_templates || []), ...(state.siteValidation.duplicate_urls || []), ...(state.siteValidation.duplicate_slugs || [])].slice(0, 8).map((entry) => `<div class="mini-list-row"><span>${escapeHTML(entry)}</span></div>`).join('')}</div>` : '<div class="empty-state">No template or route issues.</div>'}</div>
                  <div><h3>Other</h3>${[...(state.siteValidation.orphaned_media || []), ...(state.siteValidation.taxonomy_inconsistency || [])].length ? `<div class="mini-list">${[...(state.siteValidation.orphaned_media || []), ...(state.siteValidation.taxonomy_inconsistency || [])].slice(0, 8).map((entry) => `<div class="mini-list-row"><span>${escapeHTML(entry)}</span></div>`).join('')}</div>` : '<div class="empty-state">No orphaned media or taxonomy issues.</div>'}</div>
                </div>
              </div>`
              : `<div class="panel-pad stack"><div class="note">Run a full validation pass from the admin to surface broken references, duplicate routes/slugs, missing templates, orphaned media, and taxonomy inconsistencies.</div><div class="toolbar"><button type="button" class="ghost" id="debug-validate-site">Run Validation</button></div></div>`,
            'On-demand validation without leaving the admin'
          )}
          ${panel(
            'pprof Profiles',
            `<div class="panel-pad stack">
            <p class="muted">Inspect live runtime state from the admin surface. These endpoints are served through Go\'s standard <code>net/http/pprof</code> handlers.</p>
            <div class="toolbar">
              <a class="button-link ghost" href="${escapeHTML(`${pprofBase}/`)}" target="_blank" rel="noreferrer">Index</a>
              <a class="button-link ghost" href="${escapeHTML(`${pprofBase}/heap`)}" target="_blank" rel="noreferrer">Heap</a>
              <a class="button-link ghost" href="${escapeHTML(`${pprofBase}/allocs`)}" target="_blank" rel="noreferrer">Allocs</a>
              <a class="button-link ghost" href="${escapeHTML(`${pprofBase}/goroutine?debug=1`)}" target="_blank" rel="noreferrer">Goroutines</a>
              <a class="button-link ghost" href="${escapeHTML(`${pprofBase}/mutex?debug=1`)}" target="_blank" rel="noreferrer">Mutex</a>
              <a class="button-link ghost" href="${escapeHTML(`${pprofBase}/block?debug=1`)}" target="_blank" rel="noreferrer">Block</a>
              <a class="button-link ghost" href="${escapeHTML(`${pprofBase}/profile?seconds=30`)}" target="_blank" rel="noreferrer">CPU 30s</a>
              <a class="button-link ghost" href="${escapeHTML(`${pprofBase}/trace?seconds=5`)}" target="_blank" rel="noreferrer">Trace 5s</a>
            </div>
          </div>`,
            'Admin-only runtime diagnostics'
          )}
        </div>
        <div class="stack">
          ${panel('Embedded pprof', `<div class="panel-pad"><iframe class="debug-frame" src="${escapeHTML(`${pprofBase}/`)}" title="Foundry pprof"></iframe></div>`, 'Open the index here or pop any profile out into a new tab')}
        </div>
      </div>`;
  };

  const commandPaletteCommands = () => {
    const commands = [
      { id: 'goto-overview', label: 'Go to Overview', section: 'overview', action: () => navigate('overview') },
      { id: 'goto-documents', label: 'Go to Documents', section: 'documents', action: () => navigate('documents') },
      { id: 'goto-editor', label: 'Go to Editor', section: 'editor', action: () => navigate('editor') },
      { id: 'goto-media', label: 'Go to Media', section: 'media', action: () => navigate('media') },
      { id: 'goto-sessions', label: 'Go to Sessions', section: 'sessions', action: () => navigate('sessions') },
      { id: 'goto-users', label: 'Go to Users', section: 'users', action: () => navigate('users') },
      { id: 'goto-audit', label: 'Go to Audit', section: 'audit', action: () => navigate('audit') },
      { id: 'goto-settings', label: 'Go to Settings', section: 'settings', action: () => navigate('settings') },
      { id: 'goto-custom-fields', label: 'Go to Custom Fields', section: 'custom-fields', action: () => navigate('custom-fields') },
      { id: 'goto-extensions', label: 'Go to Extensions', section: 'extensions', action: () => navigate('extensions') },
      { id: 'goto-plugins', label: 'Go to Plugins', section: 'plugins', action: () => navigate('plugins') },
      { id: 'goto-themes', label: 'Go to Themes', section: 'themes', action: () => navigate('themes') },
      { id: 'goto-operations', label: 'Go to Operations', section: 'operations', action: () => navigate('operations') },
      { id: 'goto-diagnostics', label: 'Go to Diagnostics', section: 'diagnostics', action: () => navigate('diagnostics') },
      {
        id: 'new-page',
        label: 'Create New Page Draft',
        section: 'editor',
        action: () => {
          navigate('editor');
          window.setTimeout(
            () =>
              document.getElementById('document-create-kind') &&
              (document.getElementById('document-create-kind').value = 'page'),
            0
          );
        },
      },
      {
        id: 'new-post',
        label: 'Create New Post Draft',
        section: 'editor',
        action: () => {
          navigate('editor');
          window.setTimeout(
            () =>
              document.getElementById('document-create-kind') &&
              (document.getElementById('document-create-kind').value = 'post'),
            0
          );
        },
      },
    ];
    if (debugEnabled()) {
      commands.push({
        id: 'goto-debug',
        label: 'Go to Debug Dashboard',
        section: 'debug',
        action: () => navigate('debug'),
      });
    }
    extensionPages().forEach((page) => {
      commands.push({
        id: `goto-extension-${page.key}`,
        label: `Go to ${page.title}`,
        action: () => navigate(page.section),
      });
    });
    return commands.filter((command) => {
      if (command.section) return canAccessSection(command.section);
      const match = command.id.match(/^goto-(.+)$/);
      if (!match || match[1].startsWith('extension-')) return true;
      return canAccessSection(match[1]);
    });
  };

  const filteredCommandPaletteCommands = () => {
    const query = String(state.commandPalette?.query || '')
      .trim()
      .toLowerCase();
    const commands = commandPaletteCommands();
    if (!query) return commands;
    return commands.filter((command) => command.label.toLowerCase().includes(query));
  };

  const renderCommandPalette = () => {
    if (!state.commandPalette?.open) return '';
    const commands = filteredCommandPaletteCommands();
    return `<div class="command-palette-backdrop" id="command-palette-close">
      <div class="command-palette" role="dialog" aria-modal="true" aria-label="Command palette" onclick="event.stopPropagation()">
        <div class="panel-pad stack">
          <input id="command-palette-query" type="search" placeholder="Jump to a section or action" value="${escapeHTML(state.commandPalette.query || '')}" autocomplete="off">
          <div class="mini-list">
            ${
              commands.length
                ? commands
                    .map(
                      (command) =>
                        `<button type="button" class="ghost command-palette-item" data-command-palette-action="${escapeHTML(command.id)}">${escapeHTML(command.label)}</button>`
                    )
                    .join('')
                : '<div class="empty-state">No commands matched that search.</div>'
            }
          </div>
        </div>
      </div>
    </div>`;
  };

  const renderTrash = () => `
    <div class="layout-grid">
      <div class="stack">
        ${panel('Document Trash', `<div class="table table-four"><div class="table-head"><span>Document</span><span>State</span><span>Captured</span><span>Actions</span></div>${renderTrashSelectionRows(state.documentTrash, state.selectedDocumentTrash, 'document') || '<div class="panel-pad empty-state">No trashed documents.</div>'}</div>`, `${state.documentTrash.length} trashed documents`, `<div class="toolbar"><button class="ghost small" type="button" id="document-trash-select-all">Select All</button><button class="ghost small" type="button" id="document-trash-restore-selected" ${state.selectedDocumentTrash.length ? '' : 'disabled'}>Restore Selected</button><button class="ghost small danger" type="button" id="document-trash-purge-selected" ${state.selectedDocumentTrash.length ? '' : 'disabled'}>Purge Selected</button></div>`)}
      </div>
      <div class="stack">
        ${panel('Media Trash', `<div class="table table-four"><div class="table-head"><span>Name</span><span>State</span><span>Captured</span><span>Actions</span></div>${renderTrashSelectionRows(state.mediaTrash, state.selectedMediaTrash, 'media') || '<div class="panel-pad empty-state">No trashed media.</div>'}</div>`, `${state.mediaTrash.length} trashed media items`, `<div class="toolbar"><button class="ghost small" type="button" id="media-trash-select-all">Select All</button><button class="ghost small" type="button" id="media-trash-restore-selected" ${state.selectedMediaTrash.length ? '' : 'disabled'}>Restore Selected</button><button class="ghost small danger" type="button" id="media-trash-purge-selected" ${state.selectedMediaTrash.length ? '' : 'disabled'}>Purge Selected</button></div>`)}
      </div>
    </div>`;

  const renderUsers = () => {
    const selectedUser = selectedUserRecord();
    const selectedSessions = selectedUser
      ? state.userSessions.filter((session) => session.username === selectedUser.username)
      : [];
    const sortedUsers = sortItems(state.users, 'users', (user, field) => {
      switch (field) {
        case 'name':
          return user.name;
        case 'email':
          return user.email;
        default:
          return user.username;
      }
    });
    const pagedUsers = paginateItems(sortedUsers, 'users');
    const rows = pagedUsers.items.map(
      (user) => `
      <div class="table-row table-row-actions">
        <span><strong>${escapeHTML(user.username)}</strong></span>
        <span>${escapeHTML(user.name || '')}</span>
        <span>${escapeHTML(user.email || '')}</span>
        <span class="row-actions">
          <button class="ghost small" data-edit-user="${escapeHTML(user.username)}">Edit</button>
          <button class="ghost small danger" data-delete-user="${escapeHTML(user.username)}">Delete</button>
        </span>
      </div>`
    );
    return `
      <div class="layout-grid">
        <div class="stack">
          ${panel('Users', `${renderTableControls(state, 'users', state.users.length, pagedUsers.totalPages)}<div class="table table-four"><div class="table-head"><span>Username</span><span>Name</span><span>Email</span><span>Actions</span></div>${rows.length ? rows.join('') : '<div class="panel-pad empty-state">No users found.</div>'}</div>`, `${state.users.length} users`)}
        </div>
        <div class="stack">
          ${panel(
            'User Editor',
            `
            <form id="user-save-form" class="panel-pad stack" autocomplete="new-password">
              <label>Username<input id="user-username" autocomplete="off" type="text" value="${escapeHTML(state.userForm.username)}" placeholder="editor"></label>
              <label>Name<input id="user-name" autocomplete="off" type="text" value="${escapeHTML(state.userForm.name)}" placeholder="Editor User"></label>
              <label>Email<input id="user-email" autocomplete="off" type="email" value="${escapeHTML(state.userForm.email)}" placeholder="editor@example.com"></label>
              <label>Role<input id="user-role" autocomplete="off" type="text" value="${escapeHTML(state.userForm.role)}" placeholder="editor"></label>
              <label>Password<input id="user-password" autocomplete="new-password" type="password" value="" placeholder="Leave blank to keep current password"></label>
              <div class="note">Password policy: minimum ${escapeHTML(String(state.settingsForm?.Admin?.PasswordMinLength || 12))} characters. TOTP can be managed below for higher-assurance accounts.</div>
              <label class="checkbox"><input id="user-disabled" autocomplete="off" type="checkbox" ${state.userForm.disabled ? 'checked' : ''}> Disabled</label>
              <div class="toolbar">
                <button type="submit">Save User</button>
                <button type="button" class="ghost" id="user-reset-button">New User</button>
              </div>
            </form>
          `,
            'Users are stored in content/config/admin-users.yaml'
          )}
          ${panel(
            'User Security',
            selectedUser
              ? `<div class="panel-pad stack">
                <div class="note">
                  <strong>${escapeHTML(selectedUser.username)}</strong>
                  <span class="muted">Role: ${escapeHTML(selectedUser.role || 'user')} · TOTP: ${selectedUser.totp_enabled ? 'enabled' : 'disabled'}</span>
                </div>
                <div class="stack">
                  <div class="toolbar">
                    <button type="button" class="ghost" id="user-revoke-sessions">Revoke ${escapeHTML(selectedUser.username)} Sessions</button>
                    <button type="button" class="ghost" id="user-open-sessions">Open Sessions View</button>
                    <button type="button" class="ghost danger" id="user-revoke-all-sessions">Revoke All Sessions</button>
                  </div>
                  ${
                    selectedSessions.length
                      ? `<div class="table table-five">
                          <div class="table-head"><span>Session</span><span>Address</span><span>Issued</span><span>Last Seen</span><span>Actions</span></div>
                          ${selectedSessions
                            .map(
                              (session) => `<div class="table-row table-row-actions">
                                <span>
                                  <strong>${escapeHTML(session.current ? 'Current Session' : session.user_agent || session.id)}</strong>
                                  <div class="muted">${escapeHTML(session.user_agent || 'Unknown client')}</div>
                                  <div class="muted">Expires ${escapeHTML(formatDateTime(session.expires_at) || session.expires_at || '-')}</div>
                                </span>
                                <span>${escapeHTML(session.remote_addr || '-')}</span>
                                <span>${escapeHTML(formatDateTime(session.issued_at) || session.issued_at || '-')}</span>
                                <span>${escapeHTML(formatDateTime(session.last_seen) || session.last_seen || '-')}</span>
                                <span class="row-actions">
                                  <button type="button" class="ghost small danger" data-revoke-session-id="${escapeHTML(session.id)}" ${session.current ? 'data-current-session="true"' : ''}>Revoke</button>
                                </span>
                              </div>`
                            )
                            .join('')}
                        </div>`
                      : '<div class="panel-pad empty-state">No active sessions for this user.</div>'
                  }
                </div>
                <div class="stack">
                  <h3>Password Reset</h3>
                  <div class="toolbar">
                    <button type="button" class="ghost" id="user-reset-start">Issue Reset Token</button>
                  </div>
                  ${
                    state.userSecurity.resetStart?.username === selectedUser.username
                      ? `<div class="note">
                        <div><strong>Reset token</strong></div>
                        <div class="mono break-all">${escapeHTML(state.userSecurity.resetStart.reset_token || '')}</div>
                        <div class="muted">Expires in ${escapeHTML(String(state.userSecurity.resetStart.expires_in_seconds || 0))} seconds.</div>
                      </div>
                      <form id="user-reset-complete-form" class="stack" autocomplete="off">
                        <label>Reset token<input id="user-reset-token" type="text" value="${escapeHTML(state.userSecurity.resetStart.reset_token || '')}"></label>
                        <label>New password<input id="user-reset-password" type="password" autocomplete="new-password" placeholder="Enter a new password"></label>
                        <label>TOTP code (optional)<input id="user-reset-totp" type="text" inputmode="numeric" autocomplete="one-time-code" placeholder="Required if the user has TOTP enabled"></label>
                        <button type="submit">Complete Password Reset</button>
                      </form>`
                      : '<div class="panel-pad empty-state">Issue a reset token here, then complete the reset directly from the admin.</div>'
                  }
                </div>
                <div class="stack">
                  <h3>Two-Factor Authentication</h3>
                  ${
                    selectedUser.totp_enabled
                      ? `<div class="note">TOTP is currently enabled for this user.</div>
                       <div class="toolbar"><button type="button" class="ghost danger" id="user-totp-disable">Disable TOTP</button></div>`
                      : `<div class="toolbar"><button type="button" class="ghost" id="user-totp-setup">Start TOTP Setup</button></div>`
                  }
                  ${
                    state.userSecurity.totpSetup?.username === selectedUser.username
                      ? `<div class="note">
                        <div><strong>Secret</strong></div>
                        <div class="mono break-all">${escapeHTML(state.userSecurity.totpSetup.secret || '')}</div>
                        <div><strong>Provisioning URI</strong></div>
                        <div class="mono break-all">${escapeHTML(state.userSecurity.totpSetup.provisioning_uri || '')}</div>
                      </div>
                      <form id="user-totp-enable-form" class="stack" autocomplete="off">
                        <label>Verification code<input id="user-totp-enable-code" type="text" inputmode="numeric" autocomplete="one-time-code" placeholder="Enter the 6-digit code from your authenticator"></label>
                        <div class="toolbar">
                          <button type="submit">Enable TOTP</button>
                          <button type="button" class="ghost" id="user-totp-cancel">Cancel</button>
                        </div>
                      </form>`
                      : !selectedUser.totp_enabled
                        ? '<div class="panel-pad empty-state">Start TOTP setup to generate a secret and provisioning URI for this user.</div>'
                        : ''
                  }
              </div>`
              : '<div class="panel-pad empty-state">Select a user from the list to manage password reset, sessions, and TOTP.</div>'
          )}
        </div>
      </div>`;
  };

  const renderSessions = () => {
    const normalizedFilter = String(state.sessionFilters?.username || '').trim().toLowerCase();
    const visibleSessions = (state.userSessions || []).filter((session) => {
      if (!normalizedFilter) return true;
      return String(session.username || '').trim().toLowerCase().includes(normalizedFilter);
    });
    const sortedSessions = sortItems(visibleSessions, 'sessions', (session, field) => {
      switch (field) {
        case 'username':
          return session.username;
        case 'issued_at':
          return session.issued_at;
        case 'expires_at':
          return session.expires_at;
        case 'last_seen':
        default:
          return session.last_seen;
      }
    });
    const pagedSessions = paginateItems(sortedSessions, 'sessions');
    const now = Date.now();
    const sessionAgeHours = (session) => {
      const issued = Date.parse(String(session.issued_at || ''));
      if (!Number.isFinite(issued)) return 0;
      return Math.max(0, (now - issued) / (1000 * 60 * 60));
    };
    const sessionIdleMinutes = (session) => {
      const lastSeen = Date.parse(String(session.last_seen || ''));
      if (!Number.isFinite(lastSeen)) return 0;
      return Math.max(0, (now - lastSeen) / (1000 * 60));
    };
    const usernameCounts = new Map();
    const addressSets = new Map();
    for (const session of visibleSessions) {
      const username = String(session.username || '').trim().toLowerCase();
      if (!username) continue;
      usernameCounts.set(username, (usernameCounts.get(username) || 0) + 1);
      if (!addressSets.has(username)) addressSets.set(username, new Set());
      if (String(session.remote_addr || '').trim()) {
        addressSets.get(username).add(String(session.remote_addr || '').trim());
      }
    }
    const longLivedCount = visibleSessions.filter((session) => sessionAgeHours(session) >= 12).length;
    const idleCount = visibleSessions.filter((session) => sessionIdleMinutes(session) >= 30).length;
    const sharedUsers = Array.from(usernameCounts.values()).filter((count) => count > 1).length;
    const spreadUsers = Array.from(addressSets.values()).filter((set) => set.size > 1).length;
    const clientLabel = (session) => {
      const agent = String(session.user_agent || '').trim();
      if (!agent) return session.current ? 'Current Browser Session' : 'Unknown Client';
      const lower = agent.toLowerCase();
      if (lower.includes('iphone') || lower.includes('ios')) return `iPhone / iOS${session.current ? ' (Current)' : ''}`;
      if (lower.includes('ipad')) return `iPad / iPadOS${session.current ? ' (Current)' : ''}`;
      if (lower.includes('android')) return `Android Device${session.current ? ' (Current)' : ''}`;
      if (lower.includes('mac os') || lower.includes('macintosh')) return `Mac Browser${session.current ? ' (Current)' : ''}`;
      if (lower.includes('windows')) return `Windows Browser${session.current ? ' (Current)' : ''}`;
      if (lower.includes('linux')) return `Linux Browser${session.current ? ' (Current)' : ''}`;
      if (lower.includes('curl')) return `CLI Client${session.current ? ' (Current)' : ''}`;
      return session.current ? `${agent} (Current)` : agent;
    };
    const addressFingerprint = (session) => {
      const raw = String(session.remote_addr || '').trim();
      if (!raw) return '-';
      const hash = Array.from(raw).reduce((acc, char) => {
        return (acc * 33 + char.charCodeAt(0)) >>> 0;
      }, 5381);
      if (raw.includes('.')) {
        const parts = raw.split('.');
        const suffix = parts.slice(-2).join('.');
        return `x.x.${suffix} #${hash.toString(16).slice(-6)}`;
      }
      if (raw.includes(':')) {
        const parts = raw.split(':').filter(Boolean);
        const suffix = parts.slice(-2).join(':') || raw.slice(-8);
        return `x:x:${suffix} #${hash.toString(16).slice(-6)}`;
      }
      const half = raw.slice(Math.max(0, Math.floor(raw.length / 2)));
      return `...${half} #${hash.toString(16).slice(-6)}`;
    };
    const sessionFlags = (session) => {
      const username = String(session.username || '').trim().toLowerCase();
      const flags = [];
      if (session.current) flags.push('Current');
      if (sessionAgeHours(session) >= 12) flags.push('Long-Lived');
      if (sessionIdleMinutes(session) >= 30) flags.push('Idle');
      if ((usernameCounts.get(username) || 0) > 1) flags.push('Concurrent');
      if ((addressSets.get(username)?.size || 0) > 1) flags.push('Address Spread');
      return flags;
    };
    const rows = pagedSessions.items.map(
      (session) => `
      <div class="table-row table-row-actions">
        <span>
          <label class="checkbox inline-checkbox">
            <input type="checkbox" data-select-session="${escapeHTML(session.id || '')}" ${state.selectedSessions.includes(session.id) ? 'checked' : ''}>
            <strong>${escapeHTML(clientLabel(session))}</strong>
          </label>
          <div class="muted">${escapeHTML(session.user_agent || 'Unknown client')}</div>
          <div class="muted mono">${escapeHTML(session.id || '')}</div>
          <div class="toolbar">
            ${sessionFlags(session)
              .map((flag) => `<span class="contract-badge ${flag === 'Current' ? 'ok' : 'warn'}">${escapeHTML(flag)}</span>`)
              .join('')}
          </div>
        </span>
        <span>
          <strong>${escapeHTML(session.username || '')}</strong>
          <div class="muted">${escapeHTML(session.role || '')}</div>
                                </span>
                                <span>
                                  <div>${escapeHTML(addressFingerprint(session))}</div>
                                  <div class="muted">${session.mfa_complete ? 'MFA complete' : 'Password only'}</div>
                                </span>
        <span>
          <div>${escapeHTML(formatDateTime(session.last_seen) || session.last_seen || '-')}</div>
          <div class="muted">Issued ${escapeHTML(formatDateTime(session.issued_at) || session.issued_at || '-')}</div>
          <div class="muted">Expires ${escapeHTML(formatDateTime(session.expires_at) || session.expires_at || '-')}</div>
        </span>
        <span class="row-actions">
          <button type="button" class="ghost small" data-session-user="${escapeHTML(session.username || '')}">Open User</button>
          <button type="button" class="ghost small danger" data-revoke-session-id="${escapeHTML(session.id || '')}" ${session.current ? 'data-current-session="true"' : ''}>Revoke</button>
        </span>
      </div>`
    );
    return `
      <div class="layout-grid">
        <div class="stack">
          ${panel(
            'Sessions',
            `
            <div class="panel-pad">
              <div class="cards">
                <article class="card"><span class="card-label">Concurrent Users</span><strong>${escapeHTML(String(sharedUsers))}</strong><span class="card-copy">Users with multiple active sessions.</span></article>
                <article class="card"><span class="card-label">Address Spread</span><strong>${escapeHTML(String(spreadUsers))}</strong><span class="card-copy">Users with sessions from multiple addresses.</span></article>
                <article class="card"><span class="card-label">Long-Lived</span><strong>${escapeHTML(String(longLivedCount))}</strong><span class="card-copy">Sessions older than 12 hours.</span></article>
                <article class="card"><span class="card-label">Idle</span><strong>${escapeHTML(String(idleCount))}</strong><span class="card-copy">Sessions idle for 30+ minutes.</span></article>
              </div>
            </div>
            <div class="panel-pad stack">
              <form id="session-filter-form" class="toolbar">
                <label>Username
                  <input id="session-filter-username" type="text" value="${escapeHTML(state.sessionFilters?.username || '')}" placeholder="Filter by username">
                </label>
                <button type="submit" class="ghost small">Apply Filter</button>
                <button type="button" class="ghost small" id="session-filter-clear">Clear</button>
                <button type="button" class="ghost small" id="session-select-all">${pagedSessions.items.length && pagedSessions.items.every((session) => state.selectedSessions.includes(session.id)) ? 'Deselect All' : 'Select All'}</button>
                <button type="button" class="ghost small danger" id="session-revoke-selected" ${state.selectedSessions.length ? '' : 'disabled'}>Revoke Selected</button>
                <button type="button" class="small danger" id="session-revoke-all">Emergency Revoke All</button>
              </form>
            </div>
            ${renderTableControls(state, 'sessions', visibleSessions.length, pagedSessions.totalPages)}
            <div class="table table-five"><div class="table-head"><span>Client</span><span>User</span><span>Network</span><span>Activity</span><span>Actions</span></div>${rows.length ? rows.join('') : '<div class="panel-pad empty-state">No active sessions found.</div>'}</div>`,
            `${visibleSessions.length} active sessions`
          )}
        </div>
        <div class="stack">
          ${panel(
            'Session Notes',
            `<div class="panel-pad stack">
              <div class="note">Sessions are stored with a hashed bearer token, a non-secret session id, coarse remote address, and truncated user-agent for operational review.</div>
              <div class="note">Revoking the current session will sign that browser out on its next authenticated action.</div>
              <div class="note">Address display is intentionally masked and fingerprinted so operators can distinguish sessions without exposing the full stored address in the UI.</div>
              <div class="note">Signals shown here are heuristic only: concurrent sessions, address spread, long-lived sessions, and idle sessions help operators spot suspicious patterns quickly.</div>
              <div class="note">Per-user session controls remain available from the Users screen.</div>
            </div>`,
            'Operational visibility without storing raw session tokens'
          )}
        </div>
      </div>`;
  };

  const renderDebug = () => {
    const debug = state.debugTools || {};
    const flags = debug.flags || {};
    const history = debug.command?.history || [];
    const sdkSnapshot = {
      session: state.session
        ? {
            username: state.session.username,
            role: state.session.role,
            capabilities: state.session.capabilities || [],
          }
        : null,
      capabilityInfo: state.capabilityInfo
        ? {
            modules: state.capabilityInfo.modules || {},
            features: state.capabilityInfo.features || {},
          }
        : null,
      extensions: {
        pages: (state.adminExtensions?.pages || []).length,
        widgets: (state.adminExtensions?.widgets || []).length,
        slots: (state.adminExtensions?.slots || []).length,
        settings: (state.adminExtensions?.settings || []).length,
      },
      foundryAdmin: {
        available: !!window.FoundryAdmin,
        api: window.FoundryAdmin
          ? ['client', 'adminBase', 'getSession', 'getCapabilities', 'getExtensions', 'getSettingsSections']
          : [],
      },
    };
    const renderTrace = debug.renderTrace || [];
    const eventRows = (debug.events || [])
      .map(
        (entry) => `<div class="debug-event-row">
          <div class="debug-event-meta">
            <span class="contract-badge ok">${escapeHTML(entry.kind || 'info')}</span>
            <span>${escapeHTML(formatDateTime(entry.at) || entry.at || '')}</span>
          </div>
          <strong>${escapeHTML(entry.message || '')}</strong>
          ${
            flags.verboseEventPayloads && entry.meta
              ? `<pre class="debug-code-block">${escapeHTML(safeJSON(entry.meta))}</pre>`
              : ''
          }
        </div>`
      )
      .join('');
    const traceRows = renderTrace
      .map(
        (entry) => `<div class="debug-trace-row">
          <div><strong>${escapeHTML(entry.label || entry.section || 'render')}</strong><div class="muted mono">${escapeHTML(entry.path || '')}</div></div>
          <div class="muted">${escapeHTML(formatDateTime(entry.at) || '')}</div>
          <div class="muted">${escapeHTML(entry.frontendTheme || 'unknown')} / ${escapeHTML(entry.adminTheme || 'default')}</div>
        </div>`
      )
      .join('');
    const graphSnapshot = {
      site: {
        name: state.status?.name || '',
        title: state.status?.title || '',
        base_url: state.status?.base_url || '',
        default_lang: state.status?.default_lang || '',
      },
      content: state.status?.content || {},
      theme: state.status?.theme || {},
      plugins: state.status?.plugins || [],
      taxonomies: state.status?.taxonomies || [],
      runtime: {
        content: state.runtimeStatus?.content || {},
        integrity: state.runtimeStatus?.integrity || {},
        storage: state.runtimeStatus?.storage || {},
      },
    };
    const currentRenderContext = {
      section: state.section,
      title: titleForSection(state.section),
      extensionPage: extensionPageBySection(state.section),
      widgetSlots: ['overview.after', 'documents.sidebar', 'media.sidebar', 'plugins.sidebar'].map((slot) => ({
        slot,
        widgets: extensionWidgetsForSlot(slot).map((widget) => `${widget.plugin}/${widget.key}`),
      })),
      documentEditor: state.documentEditor?.source_path
        ? {
            source_path: state.documentEditor.source_path,
            preview_loaded: !!state.documentPreview,
            contract_titles: state.documentContractTitles || [],
          }
        : null,
    };
    return `
      <div class="layout-grid">
        <div class="stack">
          ${panel(
            'Runtime Event Stream',
            `<div class="panel-pad stack">
              <div class="toolbar">
                <button type="button" class="ghost" id="debug-events-clear">Clear Events</button>
              </div>
              ${eventRows || '<div class="empty-state">No debug events captured yet.</div>'}
            </div>`,
            'Client-side admin events, extension lifecycle hooks, request activity, and runtime errors'
          )}
          ${panel(
            'Admin SDK Inspector',
            `<div class="panel-pad stack">
              <div class="cards">
                <article class="card"><span class="card-label">Role</span><strong>${escapeHTML(state.session?.role || 'unknown')}</strong><span class="card-copy">Current admin identity role.</span></article>
                <article class="card"><span class="card-label">Capabilities</span><strong>${escapeHTML(String((state.session?.capabilities || []).length))}</strong><span class="card-copy">Resolved capability set size.</span></article>
                <article class="card"><span class="card-label">Extension Pages</span><strong>${escapeHTML(String((state.adminExtensions?.pages || []).length))}</strong><span class="card-copy">Pages registered through plugin metadata.</span></article>
                <article class="card"><span class="card-label">Settings Sections</span><strong>${escapeHTML(String((state.settingsSections || []).length))}</strong><span class="card-copy">Core and plugin-owned sections available in admin.</span></article>
              </div>
              <pre class="debug-code-block">${escapeHTML(safeJSON(sdkSnapshot))}</pre>
            </div>`,
            'What the authenticated admin SDK and extension registry currently expose'
          )}
          ${panel(
            'Template / Render Trace',
            `<div class="panel-pad stack">
              <div class="note">This is an inferred shell/render trace for the current admin surface, not a full server-side template profiler.</div>
              <pre class="debug-code-block">${escapeHTML(safeJSON(currentRenderContext))}</pre>
              <div class="debug-trace-list">${traceRows || '<div class="empty-state">No render trace entries yet.</div>'}</div>
            </div>`,
            'Current section renderer, active themes, extension mounts, and recent admin render passes'
          )}
        </div>
        <div class="stack">
          ${panel(
            'Graph / Content Introspection',
            `<div class="panel-pad stack">
              <div class="cards">
                <article class="card"><span class="card-label">Documents</span><strong>${escapeHTML(String(state.status?.content?.document_count || 0))}</strong><span class="card-copy">Documents currently known to the graph.</span></article>
                <article class="card"><span class="card-label">Routes</span><strong>${escapeHTML(String(state.status?.content?.route_count || 0))}</strong><span class="card-copy">Resolved public routes.</span></article>
                <article class="card"><span class="card-label">Plugins</span><strong>${escapeHTML(String((state.status?.plugins || []).length))}</strong><span class="card-copy">Loaded plugin status records.</span></article>
                <article class="card"><span class="card-label">Taxonomies</span><strong>${escapeHTML(String((state.status?.taxonomies || []).length))}</strong><span class="card-copy">Configured taxonomy groups.</span></article>
              </div>
              <pre class="debug-code-block">${escapeHTML(safeJSON(graphSnapshot))}</pre>
            </div>`,
            'A raw, inspectable snapshot of site status, runtime graph, theme, plugin, and taxonomy state'
          )}
          ${panel(
            'Request / Command Console',
            `<div class="panel-pad stack">
              <div class="toolbar">
                <button type="button" class="ghost small" data-debug-preset="status">Status</button>
                <button type="button" class="ghost small" data-debug-preset="runtime">Runtime</button>
                <button type="button" class="ghost small" data-debug-preset="validate">Validate</button>
                <button type="button" class="ghost small" data-debug-preset="rebuild">Rebuild</button>
                <button type="button" class="ghost small" data-debug-preset="cache">Clear Cache</button>
              </div>
              <form id="debug-command-form" class="stack">
                <div class="frontmatter-grid">
                  <label>Method<select id="debug-command-method">
                    ${['GET', 'POST', 'PUT', 'DELETE'].map((method) => `<option value="${method}" ${debug.command?.method === method ? 'selected' : ''}>${method}</option>`).join('')}
                  </select></label>
                  <label>Path<input id="debug-command-path" type="text" value="${escapeHTML(debug.command?.path || '/api/status')}" placeholder="/api/status"></label>
                </div>
                <label>Body<textarea id="debug-command-body" rows="8" spellcheck="false" placeholder='{"key":"value"}'>${escapeHTML(debug.command?.body || '')}</textarea></label>
                <div class="toolbar"><button type="submit">Execute</button></div>
              </form>
              ${debug.command?.error ? `<div class="note error">${escapeHTML(debug.command.error)}</div>` : ''}
              <pre class="debug-code-block">${escapeHTML(debug.command?.result || 'Run a request to inspect the raw response here.')}</pre>
              <div class="debug-history-list">
                ${(history || [])
                  .map(
                    (entry) => `<div class="mini-list-row">
                      <span>${escapeHTML(entry.method)} ${escapeHTML(entry.path)}</span>
                      <strong>${escapeHTML(entry.ok ? formatDateTime(entry.at) || entry.at : 'failed')}</strong>
                    </div>`
                  )
                  .join('') || '<div class="empty-state">No command history yet.</div>'}
              </div>
            </div>`,
            'Run raw admin API requests and keep a short local command history'
          )}
          ${panel(
            'Feature Flags / Experiments',
            `<div class="panel-pad stack">
              <label class="checkbox"><input type="checkbox" id="debug-flag-auto-refresh-runtime" ${flags.autoRefreshRuntime ? 'checked' : ''}> Auto-refresh runtime snapshot while Diagnostics or Debug is open</label>
              <label class="checkbox"><input type="checkbox" id="debug-flag-capture-extension-events" ${flags.captureExtensionEvents ? 'checked' : ''}> Capture extension page and widget lifecycle events</label>
              <label class="checkbox"><input type="checkbox" id="debug-flag-persist-console-history" ${flags.persistConsoleHistory ? 'checked' : ''}> Persist request console history in local storage</label>
              <label class="checkbox"><input type="checkbox" id="debug-flag-show-state-overlay" ${flags.showStateOverlay ? 'checked' : ''}> Show compact admin state overlay</label>
              <label class="checkbox"><input type="checkbox" id="debug-flag-verbose-event-payloads" ${flags.verboseEventPayloads ? 'checked' : ''}> Show event payload JSON in the runtime event stream</label>
            </div>`,
            'Debug-only client-side flags for experiments and deeper tooling'
          )}
        </div>
      </div>`;
  };

  const settingsValue = () => state.settingsForm || {};
  const settingsJSON = (value, empty = '{}') => {
    if (value == null) return empty;
    if (Array.isArray(value)) return JSON.stringify(value, null, 2);
    if (typeof value === 'object') return JSON.stringify(value, null, 2);
    return empty;
  };
  const renderSettingsText = (id, label, value, options = {}) =>
    `<label>${escapeHTML(label)}<input id="${escapeHTML(id)}" data-settings-input type="${escapeHTML(options.type || 'text')}" value="${escapeHTML(value ?? '')}" ${options.placeholder ? `placeholder="${escapeHTML(options.placeholder)}"` : ''}></label>`;
  const renderSettingsNumber = (id, label, value) =>
    `<label>${escapeHTML(label)}<input id="${escapeHTML(id)}" data-settings-input type="number" value="${escapeHTML(value ?? 0)}"></label>`;
  const renderSettingsCheckbox = (id, label, checked) =>
    `<label class="checkbox"><input id="${escapeHTML(id)}" data-settings-input type="checkbox" ${checked ? 'checked' : ''}> ${escapeHTML(label)}</label>`;
  const renderSettingsTextarea = (id, label, value, rows = 10) =>
    `<label>${escapeHTML(label)}<textarea id="${escapeHTML(id)}" data-settings-json rows="${rows}" spellcheck="false">${escapeHTML(value ?? '')}</textarea></label>`;
  const renderSettingsThemeOptions = (kind, current) =>
    (state.themes || [])
      .filter((themeRecord) => themeRecord.kind === kind)
      .map(
        (themeRecord) =>
          `<option value="${escapeHTML(themeRecord.name)}" ${themeRecord.name === current ? 'selected' : ''}>${escapeHTML(themeRecord.title || themeRecord.name)}</option>`
      )
      .join('');
  const renderSettingsFormTab = (activeTab) => {
    const cfg = settingsValue();
    const adminCfg = cfg.Admin || {};
    const server = cfg.Server || {};
    const build = cfg.Build || {};
    const contentCfg = cfg.Content || {};
    const taxonomies = cfg.Taxonomies || {};
    const pluginsCfg = cfg.Plugins || {};
    const seo = cfg.SEO || {};
    const cache = cfg.Cache || {};
    const security = cfg.Security || {};
    const feed = cfg.Feed || {};
    const deploy = cfg.Deploy || {};
    const topLevelSubtitleMap = {
      general: 'Site identity, theme selection, runtime paths, and base language settings',
      server: 'Preview server behavior, live reload, and local serve options',
      content: 'Content roots, media directories, and default layout behavior',
      admin: 'Admin runtime, auth/session policy, and debug settings',
      build: 'Build output behavior and content copy settings',
      taxonomies: 'Taxonomy defaults and term/archive definitions',
      plugins: 'Plugin enablement as stored in site.yaml',
      publish: 'SEO, feed, cache, security, and deploy targets',
      navigation: 'Permalinks, menus, and arbitrary params',
    };
    switch (activeTab) {
      case 'general':
        return {
          subtitle: topLevelSubtitleMap[activeTab],
          body: `<form id="settings-structured-form" class="panel-pad stack">
            <div class="frontmatter-grid">
              ${renderSettingsText('settings-name', 'Name', cfg.Name)}
              ${renderSettingsText('settings-title', 'Title', cfg.Title)}
              ${renderSettingsText('settings-base-url', 'Base URL', cfg.BaseURL, { placeholder: 'https://example.com' })}
              <label>Frontend Theme<select id="settings-theme" data-settings-input>${renderSettingsThemeOptions('frontend', cfg.Theme)}</select></label>
              ${renderSettingsText('settings-environment', 'Environment', cfg.Environment)}
              ${renderSettingsText('settings-default-lang', 'Default Language', cfg.DefaultLang)}
              ${renderSettingsText('settings-content-dir', 'Content Dir', cfg.ContentDir)}
              ${renderSettingsText('settings-public-dir', 'Public Dir', cfg.PublicDir)}
              ${renderSettingsText('settings-themes-dir', 'Themes Dir', cfg.ThemesDir)}
              ${renderSettingsText('settings-data-dir', 'Data Dir', cfg.DataDir)}
              ${renderSettingsText('settings-plugins-dir', 'Plugins Dir', cfg.PluginsDir)}
            </div>
            <div class="toolbar"><button type="submit">Save Settings</button></div>
          </form>`,
        };
      case 'server':
        return {
          subtitle: topLevelSubtitleMap[activeTab],
          body: `<form id="settings-structured-form" class="panel-pad stack">
            <div class="frontmatter-grid">
              ${renderSettingsText('settings-server-addr', 'Server Addr', server.Addr, { placeholder: ':8080' })}
              ${renderSettingsText('settings-server-live-reload-mode', 'Live Reload Mode', server.LiveReloadMode)}
              ${renderSettingsCheckbox('settings-server-live-reload', 'Enable Live Reload', !!server.LiveReload)}
              ${renderSettingsCheckbox('settings-server-auto-open-browser', 'Auto Open Browser', !!server.AutoOpenBrowser)}
              ${renderSettingsCheckbox('settings-server-debug-routes', 'Debug Routes', !!server.DebugRoutes)}
            </div>
            <div class="toolbar"><button type="submit">Save Settings</button></div>
          </form>`,
        };
      case 'content':
        return {
          subtitle: topLevelSubtitleMap[activeTab],
          body: `<form id="settings-structured-form" class="panel-pad stack">
            <div class="frontmatter-grid">
              ${renderSettingsText('settings-content-pages-dir', 'Pages Dir', contentCfg.PagesDir)}
              ${renderSettingsText('settings-content-posts-dir', 'Posts Dir', contentCfg.PostsDir)}
              ${renderSettingsText('settings-content-images-dir', 'Images Dir', contentCfg.ImagesDir)}
              ${renderSettingsText('settings-content-video-dir', 'Videos Dir', contentCfg.VideoDir)}
              ${renderSettingsText('settings-content-audio-dir', 'Audio Dir', contentCfg.AudioDir)}
              ${renderSettingsText('settings-content-documents-dir', 'Documents Dir', contentCfg.DocumentsDir)}
              ${renderSettingsText('settings-content-assets-dir', 'Assets Dir', contentCfg.AssetsDir)}
              ${renderSettingsText('settings-content-uploads-dir', 'Uploads Dir', contentCfg.UploadsDir)}
              ${renderSettingsNumber('settings-content-max-versions', 'Max Versions Per File', contentCfg.MaxVersionsPerFile)}
              ${renderSettingsText('settings-content-default-layout-page', 'Default Layout Page', contentCfg.DefaultLayoutPage)}
              ${renderSettingsText('settings-content-default-layout-post', 'Default Layout Post', contentCfg.DefaultLayoutPost)}
              ${renderSettingsText('settings-content-default-page-slug-index', 'Default Page Slug Index', contentCfg.DefaultPageSlugIndex)}
            </div>
            <div class="toolbar"><button type="submit">Save Settings</button></div>
          </form>`,
        };
      case 'admin':
        return {
          subtitle: topLevelSubtitleMap[activeTab],
          body: `<form id="settings-structured-form" class="panel-pad stack">
            <div class="frontmatter-grid">
              ${renderSettingsCheckbox('settings-admin-enabled', 'Enable Admin', !!adminCfg.Enabled)}
              ${renderSettingsCheckbox('settings-admin-local-only', 'Local Only', !!adminCfg.LocalOnly)}
              ${renderSettingsCheckbox('settings-admin-debug-pprof', 'Enable pprof Debug', !!adminCfg.Debug?.Pprof)}
              ${renderSettingsText('settings-admin-addr', 'Admin Addr', adminCfg.Addr)}
              ${renderSettingsText('settings-admin-path', 'Admin Path', adminCfg.Path)}
              ${renderSettingsText('settings-admin-access-token', 'Access Token', adminCfg.AccessToken)}
              <label>Admin Theme<select id="settings-admin-theme" data-settings-input>${renderSettingsThemeOptions('admin', adminCfg.Theme)}</select></label>
              ${renderSettingsText('settings-admin-users-file', 'Users File', adminCfg.UsersFile)}
              ${renderSettingsText('settings-admin-session-store-file', 'Session Store File', adminCfg.SessionStoreFile)}
              ${renderSettingsText('settings-admin-lock-file', 'Lock File', adminCfg.LockFile)}
              ${renderSettingsNumber('settings-admin-session-ttl', 'Session TTL Minutes', adminCfg.SessionTTLMinutes)}
              ${renderSettingsNumber('settings-admin-password-min-length', 'Password Min Length', adminCfg.PasswordMinLength)}
              ${renderSettingsNumber('settings-admin-password-reset-ttl', 'Password Reset TTL Minutes', adminCfg.PasswordResetTTL)}
              ${renderSettingsText('settings-admin-totp-issuer', 'TOTP Issuer', adminCfg.TOTPIssuer)}
            </div>
            <div class="toolbar"><button type="submit">Save Settings</button></div>
          </form>`,
        };
      case 'build':
        return {
          subtitle: topLevelSubtitleMap[activeTab],
          body: `<form id="settings-structured-form" class="panel-pad stack">
            <div class="frontmatter-grid">
              ${renderSettingsCheckbox('settings-build-clean-public-dir', 'Clean Public Dir', !!build.CleanPublicDir)}
              ${renderSettingsCheckbox('settings-build-include-drafts', 'Include Drafts', !!build.IncludeDrafts)}
              ${renderSettingsCheckbox('settings-build-minify-html', 'Minify HTML', !!build.MinifyHTML)}
              ${renderSettingsCheckbox('settings-build-copy-assets', 'Copy Assets', !!build.CopyAssets)}
              ${renderSettingsCheckbox('settings-build-copy-images', 'Copy Images', !!build.CopyImages)}
              ${renderSettingsCheckbox('settings-build-copy-uploads', 'Copy Uploads', !!build.CopyUploads)}
            </div>
            <div class="toolbar"><button type="submit">Save Settings</button></div>
          </form>`,
        };
      case 'taxonomies':
        return {
          subtitle: topLevelSubtitleMap[activeTab],
          body: `<form id="settings-structured-form" class="panel-pad stack">
            <div class="frontmatter-grid">
              ${renderSettingsCheckbox('settings-taxonomies-enabled', 'Enable Taxonomies', !!taxonomies.Enabled)}
              ${renderSettingsText('settings-taxonomies-default-set', 'Default Set', (taxonomies.DefaultSet || []).join(', '), { placeholder: 'tags, categories' })}
            </div>
            <div class="note">Definitions JSON example: {"tags":{"label":"Tags","archive_layout":"list","order":"alpha"}}</div>
            ${renderSettingsTextarea('settings-taxonomies-definitions', 'Definitions JSON', settingsJSON(taxonomies.Definitions, '{}'), 18)}
            <div class="toolbar"><button type="submit">Save Settings</button></div>
          </form>`,
        };
      case 'plugins':
        return {
          subtitle: topLevelSubtitleMap[activeTab],
          body: `<form id="settings-structured-form" class="panel-pad stack">
            <div class="note">Enabled plugins are stored as a JSON array of plugin names, for example ["readingtime","relatedposts"].</div>
            ${renderSettingsTextarea('settings-plugins-enabled', 'Enabled Plugins JSON Array', settingsJSON(pluginsCfg.Enabled || [], '[]'), 10)}
            <div class="toolbar"><button type="submit">Save Settings</button></div>
          </form>`,
        };
      case 'publish':
        return {
          subtitle: topLevelSubtitleMap[activeTab],
          body: `<form id="settings-structured-form" class="panel-pad stack">
            <div class="frontmatter-grid">
              ${renderSettingsCheckbox('settings-seo-enabled', 'Enable SEO', !!seo.Enabled)}
              ${renderSettingsText('settings-seo-default-title-sep', 'Default Title Separator', seo.DefaultTitleSep)}
              ${renderSettingsCheckbox('settings-cache-enabled', 'Enable Cache', !!cache.Enabled)}
              ${renderSettingsCheckbox('settings-security-allow-unsafe-html', 'Allow Unsafe HTML', !!security.AllowUnsafeHTML)}
              ${renderSettingsText('settings-feed-rss-path', 'RSS Path', feed.RSSPath)}
              ${renderSettingsText('settings-feed-sitemap-path', 'Sitemap Path', feed.SitemapPath)}
              ${renderSettingsNumber('settings-feed-rss-limit', 'RSS Limit', feed.RSSLimit)}
              ${renderSettingsText('settings-feed-rss-title', 'RSS Title', feed.RSSTitle)}
              ${renderSettingsText('settings-feed-rss-description', 'RSS Description', feed.RSSDescription)}
              ${renderSettingsText('settings-deploy-default-target', 'Default Deploy Target', deploy.DefaultTarget)}
            </div>
            <div class="note">Deploy targets JSON example: {"production":{"kind":"local","path":"./public"}}.</div>
            ${renderSettingsTextarea('settings-deploy-targets', 'Deploy Targets JSON', settingsJSON(deploy.Targets, '{}'), 16)}
            <div class="toolbar"><button type="submit">Save Settings</button></div>
          </form>`,
        };
      case 'navigation':
        return {
          subtitle: topLevelSubtitleMap[activeTab],
          body: `<form id="settings-structured-form" class="panel-pad stack">
            <div class="note">Permalinks, menus, and params stay fully editable here. Use JSON objects such as {"main":[{"name":"Home","url":"/"}]} for menus and {"company_name":"Foundry"} for params.</div>
            ${renderSettingsTextarea('settings-permalinks', 'Permalinks JSON', settingsJSON(cfg.Permalinks, '{}'), 10)}
            ${renderSettingsTextarea('settings-menus', 'Menus JSON', settingsJSON(cfg.Menus, '{}'), 14)}
            ${renderSettingsTextarea('settings-params', 'Params JSON', settingsJSON(cfg.Params, '{}'), 14)}
            <div class="toolbar"><button type="submit">Save Settings</button></div>
          </form>`,
        };
      default:
        return { subtitle: '', body: '' };
    }
  };
  const renderSettings = () => {
    const activeTab = state.settingsTab || 'general';
    const tabs = [
      ['general', 'General'],
      ['server', 'Server'],
      ['content', 'Content'],
      ['admin', 'Admin'],
      ['build', 'Build'],
      ['taxonomies', 'Taxonomies'],
      ['plugins', 'Plugins'],
      ['publish', 'Publish'],
      ['navigation', 'Navigation'],
      ['config', 'Advanced YAML'],
      ['custom-css', 'Custom CSS'],
      ['sections', 'Sections'],
    ];
    let body = '';
    let subtitle = '';
    if (activeTab === 'custom-css') {
      subtitle = state.customCSS?.path || 'content/assets/css/custom.css';
      body = `
        <form id="custom-css-save-form" class="panel-pad stack">
          <label>Custom stylesheet<textarea id="custom-css-raw" rows="24" spellcheck="false">${escapeHTML(state.customCSS?.raw || '')}</textarea></label>
          <div class="note">
            <strong>Path:</strong> <span class="mono">${escapeHTML(state.customCSS?.path || 'content/assets/css/custom.css')}</span><br>
            Loaded with the site asset pipeline as the site-level override layer for the active frontend theme.
          </div>
          <div class="toolbar"><button type="submit">Save Custom CSS</button></div>
        </form>`;
    } else if (activeTab === 'sections') {
      subtitle = 'Core and plugin-defined settings groups';
      body = `
        <div class="panel-pad stack">
          ${state.settingsSections.length ? `<div class="note"><strong>Known sections:</strong> ${escapeHTML(state.settingsSections.map((section) => section.title).join(', '))}</div>` : ''}
          ${
            state.settingsSections.length
              ? `<div class="table table-three">
            <div class="table-head"><span>Section</span><span>Source</span><span>Writable</span></div>
            ${state.settingsSections
              .map(
                (section) => `
              <div class="table-row">
                <span><strong>${escapeHTML(section.title)}</strong><div class="muted mono">${escapeHTML(section.key)}</div></span>
                <span>${escapeHTML(section.source || 'core')}</span>
                <span>${section.writable ? 'yes' : 'no'}</span>
              </div>`
              )
              .join('')}
          </div>`
              : '<div class="empty-state">No settings sections are currently registered.</div>'
          }
        </div>`;
    } else if (activeTab === 'config') {
      subtitle = state.config?.path || 'content/config/site.yaml';
      body = `
        <form id="config-save-form" class="panel-pad stack">
          <label>Config file<textarea id="config-raw" rows="24" spellcheck="false">${escapeHTML(state.config?.raw || '')}</textarea></label>
          <div class="toolbar"><button type="submit">Save Configuration</button></div>
        </form>`;
    } else {
      const rendered = renderSettingsFormTab(activeTab);
      subtitle = rendered.subtitle;
      body = rendered.body;
    }
    return panel(
      'Settings',
      `
      <div class="panel-pad stack">
        <div class="toolbar settings-tabs">
          ${tabs
            .map(
              ([key, label]) =>
                `<button type="button" class="ghost small ${activeTab === key ? 'active-toggle' : ''}" data-settings-tab="${escapeHTML(key)}">${escapeHTML(label)}</button>`
            )
            .join('')}
        </div>
        ${
          state.settingsDraftError && activeTab !== 'config' && activeTab !== 'custom-css'
            ? `<div class="note error">${escapeHTML(state.settingsDraftError)}</div>`
            : ''
        }
      </div>
      ${body}`,
      subtitle
    );
  };

  const { renderExtensions, renderPlugins, renderThemes, renderOperations } = createPlatformViews({
    state,
    panel,
    escapeHTML,
    renderTableControls,
    adminBase,
    adminPathForSection,
    extensionPages,
    renderWidgetPanels,
    sortItems,
    paginateItems,
  });

  const renderSection = () => {
    switch (state.section) {
      case 'documents':
        return renderDocuments();
      case 'editor':
        return renderEditor();
      case 'history':
        return renderHistory();
      case 'trash':
        return renderTrash();
      case 'media':
        return renderMedia();
      case 'diagnostics':
        return renderDiagnostics();
      case 'debug':
        return renderDebug();
      case 'audit':
        return renderAudit();
      case 'sessions':
        return renderSessions();
      case 'users':
        return renderUsers();
      case 'settings':
      case 'config':
        return renderSettings();
      case 'custom-fields':
        return renderCustomFields();
      case 'extensions':
        return renderExtensions();
      case 'plugins':
        return renderPlugins();
      case 'themes':
        return renderThemes();
      case 'operations':
        return renderOperations();
      default: {
        const extensionPage = extensionPageBySection(state.section);
        if (extensionPage) return renderExtensionPage();
        return `${renderOverview(state)}${renderWidgetPanels('overview.after').join('')}`;
      }
    }
  };

  const renderLogin = () => {
    root.innerHTML = `
      <div class="login-shell">
        <div class="login-card">
          <div class="login-mark">F</div>
          <h1>Foundry Admin</h1>
          <p class="login-copy">Sign in to manage documents, media, users, settings, themes, and plugins.</p>
          <form id="login-form" class="login-form">
            <label>Username<input id="username" type="text" autocomplete="username" placeholder="admin"></label>
            <label>Password<input id="password" type="password" autocomplete="current-password" placeholder="Password"></label>
            <label>Two-Factor Code<input id="totp-code" type="text" inputmode="numeric" autocomplete="one-time-code" placeholder="Optional 6-digit code"></label>
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
      const totpCode = document.getElementById('totp-code').value;
      state.loading = true;
      render();
      try {
        await admin.session.login({ username, password, totp_code: totpCode });
        await fetchAll();
      } catch (error) {
        state.loading = false;
        state.error = error.message || String(error);
        render();
      }
    });
  };

  const loadDocumentHistory = async (sourcePath, rerender = true) => {
    try {
      const history = await admin.documents.history(sourcePath);
      state.documentHistoryPath = history.source_path || sourcePath;
      state.documentHistory = Array.isArray(history.entries) ? history.entries : [];
      if (rerender) {
        navigate('history');
      }
    } catch (error) {
      state.error = error.message || String(error);
      render();
    }
  };

  const loadDocumentDiff = async (leftPath, rightPath, rerender = true) => {
    try {
      state.documentDiff = await admin.documents.diff({
        left_path: leftPath,
        right_path: rightPath,
      });
      if (rerender) {
        navigate('history');
      }
    } catch (error) {
      state.error = error.message || String(error);
      render();
    }
  };

  const loadMediaHistory = async (path, rerender = true) => {
    try {
      const history = await admin.media.history(path);
      state.mediaHistoryReference = history.path || path;
      state.mediaHistory = Array.isArray(history.entries) ? history.entries : [];
      if (rerender) {
        navigate('history');
      }
    } catch (error) {
      state.error = error.message || String(error);
      render();
    }
  };

  const bindDashboardEventsLocal = () =>
    bindDashboardEvents({
      root,
      state,
      admin,
      render,
      navigate,
      updateTablePage,
      releaseCurrentDocumentLock,
      syncStructuredEditorFromRaw,
      compareSnapshot,
      syncRawFromStructuredEditor,
      slugify,
      updateDocumentFieldValue,
      updateSharedFieldValue,
      getValueAtPath,
      defaultValueForSchema,
      removeNestedFieldValue,
      insertIntoMarkdown,
      mediaSnippet,
      loadDocumentIntoEditor,
      setFlash,
      fetchAll,
      loadDocumentHistory,
      loadDocumentDiff,
      resetDocumentEditor,
      snapshotValue,
      loadMediaDetail,
      loadMediaHistory,
      toggleSelection,
      setUserForm,
      resetUserForm,
      resetUserSecurity,
      selectedUserRecord,
      parseTagInput,
      parseDocumentEditor,
      buildDocumentRaw,
      clone,
    });

  const renderDebugOverlay = () => {
    if (!state.debugTools.flags.showStateOverlay) return '';
    return `<div class="debug-overlay">
      <strong>${escapeHTML(titleForSection(state.section))}</strong>
      <span>section: ${escapeHTML(state.section)}</span>
      <span>docs: ${escapeHTML(String(state.documents?.length || 0))}</span>
      <span>routes: ${escapeHTML(String(state.status?.content?.route_count || 0))}</span>
      <span>load errors: ${escapeHTML(String(state.loadErrors?.length || 0))}</span>
      <span>dirty: ${escapeHTML(hasUnsavedChanges() ? dirtyMessage() : 'none')}</span>
    </div>`;
  };

  const renderDashboard = () => {
    if (!canAccessSection(state.section)) {
      state.section = firstAccessibleSection();
      window.history.replaceState({}, '', adminPathForSection(adminBase, state.section));
    }
    recordRenderTrace('dashboard render', {
      extensionPage: extensionPageBySection(state.section)?.key || '',
      previewLoaded: !!state.documentPreview,
      runtimeLoaded: !!state.runtimeStatus,
    });
    const topMessage =
      summarizeLoadErrors(state) ||
      'Manage content, media, users, settings, themes, and plugins.';
    root.innerHTML = `
      <div class="foundry-shell">
        ${renderToasts(state)}
        ${renderKeyboardHelp(state)}
        ${renderCommandPalette()}
        <aside class="foundry-sidebar">
          <div class="foundry-brand">Foundry</div>
          <nav class="foundry-nav">${shellNav(state, adminBase, {
            extensionPages: extensionPages(),
            builtinSectionGroup,
            debugEnabled: debugEnabled(),
            canAccessSection,
          })}</nav>
          <div class="foundry-sidebar-footer">Admin theme: ${escapeHTML(root.dataset.theme || 'default')}</div>
        </aside>
        <div class="foundry-main">
          <header class="foundry-topbar">
            <div>
              ${renderBreadcrumbs(state, titleForSection)}
              <h1>${escapeHTML(titleForSection(state.section))}</h1>
              <p>${escapeHTML(topMessage)}</p>
            </div>
            <div class="foundry-topbar-actions">
              ${hasUnsavedChanges() ? `<span class="dirty-pill">Unsaved: ${escapeHTML(dirtyMessage())}</span>` : ''}
              <button class="ghost" id="shortcut-help-toggle">Shortcuts</button>
              <div class="chrome-user"><strong>${escapeHTML(state.session?.name || state.session?.username || '')}</strong><span>${escapeHTML(state.session?.email || '')}</span></div>
              <button class="ghost" id="logout">Log Out</button>
            </div>
          </header>
          <main class="foundry-content">
            ${state.error ? `<div class="panel error-panel"><div class="panel-pad"><strong>Action Failed</strong><div class="error">${escapeHTML(state.error)}</div></div></div>` : ''}
            ${state.loadErrors.length ? `<div class="panel warning-panel"><div class="panel-pad"><strong>Partial Admin Load</strong><div class="muted">${escapeHTML(summarizeLoadErrors(state))}</div></div></div>` : ''}
            ${renderUpdateNotice(state)}
            ${renderSection()}
          </main>
        </div>
        ${renderDebugOverlay()}
      </div>`;
    document.getElementById('shortcut-help-toggle')?.addEventListener('click', () => {
      state.keyboardHelp = !state.keyboardHelp;
      render();
    });
    document.getElementById('command-palette-close')?.addEventListener('click', () => {
      state.commandPalette.open = false;
      render();
    });
    document.getElementById('command-palette-query')?.addEventListener('input', (event) => {
      state.commandPalette.query = event.target.value || '';
      render();
    });
    root.querySelectorAll('[data-command-palette-action]').forEach((button) => {
      button.addEventListener('click', () => {
        const command = commandPaletteCommands().find(
          (entry) => entry.id === button.dataset.commandPaletteAction
        );
        if (!command) return;
        state.commandPalette.open = false;
        state.commandPalette.query = '';
        command.action();
      });
    });
    if (state.commandPalette?.open) {
      window.setTimeout(() => document.getElementById('command-palette-query')?.focus(), 0);
    }
    bindDashboardEventsLocal();
    document.getElementById('debug-events-clear')?.addEventListener('click', () => {
      state.debugTools.events = [];
      render();
    });
    root.querySelectorAll('[data-debug-preset]').forEach((button) => {
      button.addEventListener('click', () => {
        const preset = button.dataset.debugPreset;
        const presets = {
          status: { method: 'GET', path: '/api/status', body: '' },
          runtime: { method: 'GET', path: '/api/debug/runtime', body: '' },
          validate: { method: 'POST', path: '/api/debug/validate', body: '{}' },
          rebuild: { method: 'POST', path: '/api/operations/rebuild', body: '{}' },
          cache: { method: 'POST', path: '/api/operations/cache/clear', body: '{}' },
        };
        const next = presets[preset];
        if (!next) return;
        state.debugTools.command = { ...state.debugTools.command, ...next, error: '' };
        render();
      });
    });
    document.getElementById('debug-command-form')?.addEventListener('submit', async (event) => {
      event.preventDefault();
      const method = document.getElementById('debug-command-method')?.value || 'GET';
      const path = document.getElementById('debug-command-path')?.value || '/api/status';
      const body = document.getElementById('debug-command-body')?.value || '';
      state.debugTools.command = { ...state.debugTools.command, method, path, body, error: '' };
      try {
        await executeDebugRequest({ method, path, body });
      } catch (error) {
        state.debugTools.command = {
          ...state.debugTools.command,
          result: '',
          error: error?.message || String(error),
          history: [
            {
              at: new Date().toISOString(),
              method,
              path,
              ok: false,
            },
            ...(state.debugTools.command.history || []),
          ].slice(0, 12),
        };
        persistDebugHistory();
        recordDebugEvent('request-error', `${method} ${path} failed`, {
          error: error?.message || String(error),
        });
      }
      render();
    });
    root.querySelectorAll('[id^="debug-flag-"]').forEach((input) => {
      input.addEventListener('change', () => {
        const key = input.id.replace('debug-flag-', '').replace(/-([a-z])/g, (_, c) => c.toUpperCase());
        setDebugFlag(key, !!input.checked);
        recordDebugEvent('flag', `Toggled ${key}`, { enabled: !!input.checked });
        scheduleDebugAutoRefresh();
        render();
      });
    });
    document.getElementById('settings-structured-form')?.addEventListener('submit', async (event) => {
      event.preventDefault();
      try {
        state.settingsForm = collectSettingsFormPayload();
        state.settingsDraftError = '';
        await settingsAPI.saveForm({ value: state.settingsForm });
        setFlash('Settings saved.');
        snapshotValue('settings', state.settingsForm);
        await fetchAll(false);
        navigate('settings');
      } catch (error) {
        if (String(error.message || error).includes('settings-')) {
          state.settingsDraftError = error.message || String(error);
        } else {
          state.error = error.message || String(error);
        }
        render();
      }
    });
    document.getElementById('custom-fields-save-form')?.addEventListener('submit', async (event) => {
      event.preventDefault();
      if (!canManageSharedFields()) {
        setFlash('Viewing only. Saving shared custom fields requires config.manage.');
        return;
      }
      try {
        const raw = document.getElementById('custom-fields-raw')?.value || '';
        const rawChanged = raw !== (state.customFields?.raw || '');
        const response = await admin.customFields.save({
          raw: rawChanged ? raw : '',
          values: rawChanged ? undefined : state.customFields?.values || {},
        });
        state.customFields = response || state.customFields;
        state.sharedFieldContracts = Array.isArray(response?.contracts) ? response.contracts : state.sharedFieldContracts;
        snapshotValue('customFields', state.customFields?.values || {});
        setFlash('Shared custom fields saved.');
        await fetchAll(false);
        navigate('custom-fields');
      } catch (error) {
        state.error = error.message || String(error);
        render();
      }
    });
    root
      .querySelectorAll('[data-settings-input], [data-settings-json]')
      .forEach((node) =>
        node.addEventListener(node.type === 'checkbox' ? 'change' : 'input', () => {
          syncSettingsDraftFromDOM();
        })
      );
    root.querySelectorAll('[data-settings-tab]').forEach((button) => {
      button.addEventListener('click', () => {
        syncSettingsDraftFromDOM();
        state.settingsTab = button.dataset.settingsTab || 'general';
        render();
      });
    });
    document.getElementById('custom-fields-raw')?.addEventListener('input', () => {
      const raw = document.getElementById('custom-fields-raw')?.value || '';
      state.dirty.customFields = raw !== (state.customFields?.raw || '');
    });
    publishAdminRuntime();
    void mountActiveExtensionPage();
    void mountVisibleExtensionWidgets();
    scheduleDebugAutoRefresh();
  };

  const render = () => {
    if (!state.session || !state.session.authenticated) {
      renderLogin();
      return;
    }
    if (!canAccessSection(state.section)) {
      state.section = firstAccessibleSection();
      window.history.replaceState({}, '', adminPathForSection(adminBase, state.section));
    }
    renderDashboard();
  };

  const router = createAdminRouter({
    adminBase,
    getState: () => state,
    confirmNavigation,
    render,
  });
  navigate = (section, options) => {
    recordDebugEvent('navigate', `Navigate to ${normalizeAdminSection(section)}`, {
      from: state.section,
      to: normalizeAdminSection(section),
    });
    const result = router.navigate(section, options);
    const nextSection = normalizeAdminSection(section);
    if (nextSection === 'users' || nextSection === 'sessions') {
      void loadSessionInventory({ rerender: true });
    }
    return result;
  };

  const loadMediaDetail = async (reference, rerender = true) => {
    try {
      state.mediaDetail = await admin.media.getDetail(reference);
      state.selectedMediaReference = reference;
      state.mediaVersionComment = '';
      snapshotValue('media', {
        reference,
        metadata: state.mediaDetail.metadata || {},
        versionComment: '',
      });
      setFlash('Media loaded.');
      if (rerender) {
        navigate('media');
      }
    } catch (error) {
      state.error = error.message || String(error);
      render();
    }
  };

  const loadSessionInventory = async ({ rerender = true, force = false } = {}) => {
    if (!capabilityInfoHas('users.manage')) {
      state.userSessions = [];
      state.userSessionsLoaded = true;
      return;
    }
    if (state.userSessionsLoaded && !force) {
      return;
    }
    try {
      const sessions = await admin.session.list();
      state.userSessions = Array.isArray(sessions) ? sessions : [];
      state.selectedSessions = state.selectedSessions.filter((sessionID) =>
        state.userSessions.some((session) => session.id === sessionID)
      );
      state.userSessionsLoaded = true;
    } catch (error) {
      state.userSessions = [];
      state.selectedSessions = [];
      const message = error?.message || String(error);
      const unavailable = String(message).includes('404');
      state.userSessionsLoaded = unavailable;
      if (rerender && !unavailable) {
        state.error = error.message || String(error);
      }
    } finally {
      if (rerender) {
        render();
      }
    }
  };

  const fetchAll = async (rerender = true) => {
    state.loading = true;
    state.error = '';
    clearLoadErrors();
    recordDebugEvent('fetch', 'Admin bootstrap fetch started');
    try {
      state.session = await admin.session.get();
      state.capabilityInfo = await admin.capabilities.get();
      state.runtimeStatus = debugEnabled() ? await admin.raw.get('/api/debug/runtime') : null;

      const results = await Promise.allSettled([
        admin.status.get(),
        admin.documents.list({ include_drafts: 1, q: state.documentQuery || undefined }),
        admin.documents.trash(),
        admin.media.list({ q: state.mediaQuery || undefined }),
        admin.media.trash(),
        capabilityInfoHas('users.manage') ? admin.users.list() : Promise.resolve([]),
        capabilityInfoHas('config.manage') ? settingsAPI.getForm() : Promise.resolve(null),
        capabilityInfoHas('config.manage') ? settingsAPI.getConfig() : Promise.resolve(null),
        capabilityInfoHas('config.manage') ? settingsAPI.getCustomCSS() : Promise.resolve(null),
        capabilityInfoHas('dashboard.read') ? admin.customFields.get() : Promise.resolve(null),
        capabilityInfoHas('dashboard.read') ? settingsAPI.getSections() : Promise.resolve([]),
        capabilityInfoHas('plugins.manage') ? admin.plugins.list() : Promise.resolve([]),
        capabilityInfoHas('dashboard.read')
          ? admin.extensions.getAdminExtensions()
          : Promise.resolve({ pages: [], widgets: [], slots: [], settings: [] }),
        capabilityInfoHas('themes.manage') ? admin.themes.list() : Promise.resolve([]),
        capabilityInfoHas('config.manage') ? admin.backups.list() : Promise.resolve([]),
        capabilityInfoHas('config.manage') ? admin.backups.listGit() : Promise.resolve([]),
        capabilityInfoHas('dashboard.read') ? admin.operations.get() : Promise.resolve(null),
        capabilityInfoHas('dashboard.read') ? admin.operations.logs() : Promise.resolve(null),
        capabilityInfoHas('dashboard.read') ? admin.updates.get() : Promise.resolve(null),
        capabilityInfoHas('audit.read') ? admin.audit.list() : Promise.resolve([]),
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

      assignResult(
        0,
        'status',
        (value) => {
          state.status = value;
        },
        () => {
          state.status = null;
        }
      );
      assignResult(
        1,
        'documents',
        (value) => {
          state.documents = Array.isArray(value) ? value : [];
        },
        () => {
          state.documents = [];
        }
      );
      assignResult(
        2,
        'document trash',
        (value) => {
          state.documentTrash = Array.isArray(value) ? value : [];
        },
        () => {
          state.documentTrash = [];
        }
      );
      assignResult(
        3,
        'media',
        (value) => {
          state.media = Array.isArray(value) ? value : [];
        },
        () => {
          state.media = [];
        }
      );
      assignResult(
        4,
        'media trash',
        (value) => {
          state.mediaTrash = Array.isArray(value) ? value : [];
        },
        () => {
          state.mediaTrash = [];
        }
      );
      assignResult(
        5,
        'users',
        (value) => {
          state.users = Array.isArray(value) ? value : [];
        },
        () => {
          state.users = [];
        }
      );
      assignResult(
        6,
        'settings form',
        (value) => {
          state.settingsForm = value?.value || null;
        },
        () => {
          state.settingsForm = null;
        }
      );
      assignResult(
        7,
        'config',
        (value) => {
          state.config = value;
        },
        () => {
          state.config = null;
        }
      );
      assignResult(
        8,
        'custom css',
        (value) => {
          state.customCSS = value;
        },
        () => {
          state.customCSS = null;
        }
      );
      assignResult(
        9,
        'custom fields',
        (value) => {
          state.customFields = value || null;
          state.sharedFieldContracts = Array.isArray(value?.contracts) ? value.contracts : [];
        },
        () => {
          state.customFields = null;
          state.sharedFieldContracts = [];
        }
      );
      assignResult(
        10,
        'settings sections',
        (value) => {
          state.settingsSections = Array.isArray(value) ? value : [];
        },
        () => {
          state.settingsSections = [];
        }
      );
      assignResult(
        11,
        'plugins',
        (value) => {
          state.plugins = Array.isArray(value) ? value : [];
        },
        () => {
          state.plugins = [];
        }
      );
      assignResult(
        12,
        'admin extensions',
        (value) => {
          state.adminExtensions = value || { pages: [], widgets: [], slots: [], settings: [] };
        },
        () => {
          state.adminExtensions = { pages: [], widgets: [], slots: [], settings: [] };
        }
      );
      assignResult(
        13,
        'themes',
        (value) => {
          state.themes = Array.isArray(value) ? value : [];
        },
        () => {
          state.themes = [];
        }
      );
      assignResult(
        14,
        'backups',
        (value) => {
          state.backups = Array.isArray(value) ? value : [];
        },
        () => {
          state.backups = [];
        }
      );
      assignResult(
        15,
        'git backups',
        (value) => {
          state.gitBackups = Array.isArray(value) ? value : [];
        },
        () => {
          state.gitBackups = [];
        }
      );
      assignResult(
        16,
        'operations status',
        (value) => {
          state.operationsStatus = value || null;
        },
        () => {
          state.operationsStatus = null;
        }
      );
      assignResult(
        17,
        'operations logs',
        (value) => {
          state.operationsLog = value || null;
        },
        () => {
          state.operationsLog = null;
        }
      );
      assignResult(
        18,
        'update status',
        (value) => {
          state.updateInfo = value || null;
        },
        () => {
          state.updateInfo = null;
        }
      );
      assignResult(
        19,
        'audit log',
        (value) => {
          state.audit = Array.isArray(value) ? value : [];
        },
        () => {
          state.audit = [];
        }
      );
      state.selectedDocumentTrash = state.selectedDocumentTrash.filter((path) =>
        state.documentTrash.some((entry) => entry.path === path)
      );
      state.selectedMediaTrash = state.selectedMediaTrash.filter((path) =>
        state.mediaTrash.some((entry) => entry.path === path)
      );

      if (state.selectedMediaReference) {
        const matching = state.media.find(
          (item) => item.reference === state.selectedMediaReference
        );
        if (matching) {
          state.mediaDetail = matching;
        } else {
          state.selectedMediaReference = '';
          state.mediaDetail = null;
          state.mediaVersionComment = '';
        }
      }
      if (state.documentHistoryPath) {
        const stillPresent =
          state.documents.some((doc) => doc.source_path === state.documentHistoryPath) ||
          state.documentTrash.some(
            (entry) =>
              entry.path === state.documentHistoryPath ||
              entry.original_path === state.documentHistoryPath
          );
        if (!stillPresent) {
          state.documentHistoryPath = '';
          state.documentHistory = [];
          state.documentDiff = null;
        }
      }
      if (state.mediaHistoryReference) {
        const stillPresent = state.media.some(
          (item) => `content/${item.collection}/${item.path}` === state.mediaHistoryReference
        );
        if (
          !stillPresent &&
          !state.mediaHistory.some(
            (entry) =>
              entry.path === state.mediaHistoryReference ||
              entry.original_path === state.mediaHistoryReference
          )
        ) {
          state.mediaHistoryReference = '';
          state.mediaHistory = [];
        }
      }
      snapshotValue('settings', state.settingsForm || {});
      snapshotValue('config', state.config?.raw || '');
      snapshotValue('customCss', state.customCSS?.raw || '');
      snapshotValue('customFields', state.customFields?.values || {});
      snapshotValue('user', state.userForm);
      state.userSessionsLoaded = false;
      state.selectedSessions = state.selectedSessions.filter((sessionID) =>
        state.userSessions.some((session) => session.id === sessionID)
      );
      if (!state.documentEditor.raw) {
        state.documentEditor.raw = buildDefaultMarkdown('post');
      }
      snapshotValue('document', {
        editor: state.documentEditor,
        fields: state.documentFieldValues,
        meta: state.documentMeta,
      });
      recordDebugEvent('fetch', 'Admin bootstrap fetch completed', {
        loadErrors: state.loadErrors.length,
      });
    } catch (error) {
      state.session = null;
      state.capabilityInfo = null;
      state.runtimeStatus = null;
      setError(error.message || String(error));
    } finally {
      state.loading = false;
      if (rerender) {
        render();
      }
      if (state.section === 'users' || state.section === 'sessions') {
        void loadSessionInventory({ rerender: true });
      }
    }
  };

  window.addEventListener('popstate', () => {
    if (!confirmNavigation()) {
      window.history.pushState({}, '', adminPathForSection(adminBase, state.section));
      return;
    }
    const nextSection = sectionForPath(window.location.pathname);
    const normalizedSection = nextSection === 'config' ? 'settings' : nextSection;
    state.section = canAccessSection(normalizedSection) ? normalizedSection : firstAccessibleSection();
    render();
    if (state.section === 'users' || state.section === 'sessions') {
      void loadSessionInventory({ rerender: true });
    }
  });

  window.addEventListener('beforeunload', (event) => {
    if (!hasUnsavedChanges()) return;
    event.preventDefault();
    event.returnValue = '';
  });

  const isEditableTarget = (target) => {
    if (!target) return false;
    if (target.closest?.('[contenteditable="true"]')) return true;
    const tagName = target.tagName?.toLowerCase();
    return tagName === 'input' || tagName === 'textarea' || tagName === 'select' || target.isContentEditable;
  };

  window.addEventListener('keydown', (event) => {
    if (!state.session?.authenticated) return;
    const isMac = /Mac|iPhone|iPad/.test(window.navigator.platform);
    const modifier = isMac ? event.metaKey : event.ctrlKey;
    const typing = isEditableTarget(event.target);
    if (modifier && event.key.toLowerCase() === 's') {
      event.preventDefault();
      if (state.section === 'editor')
        document.getElementById('document-save-form')?.requestSubmit();
      if (isSettingsSection(state.section)) {
        if (state.settingsTab === 'custom-css') {
          document.getElementById('custom-css-save-form')?.requestSubmit();
        } else if (state.settingsTab === 'config') {
          document.getElementById('config-save-form')?.requestSubmit();
        } else {
          document.getElementById('settings-structured-form')?.requestSubmit();
        }
      }
      if (state.section === 'users') document.getElementById('user-save-form')?.requestSubmit();
      if (state.section === 'media')
        document.getElementById('media-metadata-form')?.requestSubmit();
      if (state.section === 'custom-fields')
        document.getElementById('custom-fields-save-form')?.requestSubmit();
      return;
    }
    if (modifier && event.key === 'Enter' && state.section === 'editor') {
      event.preventDefault();
      document.getElementById('document-preview-button')?.click();
      return;
    }
    if (modifier && event.key.toLowerCase() === 'k') {
      event.preventDefault();
      state.commandPalette.open = !state.commandPalette.open;
      if (!state.commandPalette.open) {
        state.commandPalette.query = '';
      }
      render();
      return;
    }
    if (event.key === 'Escape' && state.commandPalette?.open) {
      event.preventDefault();
      state.commandPalette.open = false;
      state.commandPalette.query = '';
      render();
      return;
    }
    if (typing) return;
    if (event.shiftKey && event.key === '?') {
      event.preventDefault();
      state.keyboardHelp = !state.keyboardHelp;
      render();
      return;
    }
  });

  fetchAll();
})();
