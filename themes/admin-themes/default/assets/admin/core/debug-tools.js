import { normalizeAdminSection } from './router.js';

export const createDebugTools = ({
  state,
  root,
  admin,
  safeJSON,
  debugEnabled,
  persistDebugFlags,
  persistDebugHistory,
  getRender,
}) => {
  let debugAutoRefreshTimer = null;

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
        getRender()();
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

  const bindDebugControls = () => {
    document.getElementById('debug-events-clear')?.addEventListener('click', () => {
      state.debugTools.events = [];
      getRender()();
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
        getRender()();
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
      getRender()();
    });
    root.querySelectorAll('[id^="debug-flag-"]').forEach((input) => {
      input.addEventListener('change', () => {
        const key = input.id.replace('debug-flag-', '').replace(/-([a-z])/g, (_, c) => c.toUpperCase());
        setDebugFlag(key, !!input.checked);
        recordDebugEvent('flag', `Toggled ${key}`, { enabled: !!input.checked });
        scheduleDebugAutoRefresh();
        getRender()();
      });
    });
  };

  const bindGlobalDebugEvents = () => {
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
  };

  return {
    bindDebugControls,
    bindGlobalDebugEvents,
    executeDebugRequest,
    recordDebugEvent,
    recordRenderTrace,
    scheduleDebugAutoRefresh,
    setDebugFlag,
  };
};
