export const createDebugViews = ({ state, panel, escapeHTML, formatDateTime, debugEnabled }) => {
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

  const renderDebug = () => {
    if (!debugEnabled()) {
      return panel(
        'Runtime Profiling',
        '<div class="panel-pad empty-state">pprof is disabled. Set <code>admin.debug.pprof: true</code> in site.yaml to enable runtime profiling in the admin.</div>'
      );
    }
    const runtime = state.runtimeStatus;
    return `
      <div class="stack">
        ${panel(
          'Runtime Summary',
          runtime
            ? `<div class="panel-pad stack">
              <div class="cards">
                <article class="card"><span class="card-label">Heap Alloc</span><strong>${escapeHTML(formatBytes(runtime.heap_alloc_bytes))}</strong><span class="card-copy">Live bytes currently allocated.</span></article>
                <article class="card"><span class="card-label">Heap In Use</span><strong>${escapeHTML(formatBytes(runtime.heap_inuse_bytes))}</strong><span class="card-copy">Heap pages currently in use.</span></article>
                <article class="card"><span class="card-label">Heap Objects</span><strong>${escapeHTML(String(runtime.heap_objects || 0))}</strong><span class="card-copy">Objects tracked in the heap.</span></article>
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
                <div><h3>Status</h3>${renderMiniList(runtime.content?.by_status)}</div>
                <div><h3>Languages</h3>${renderMiniList(runtime.content?.by_lang)}</div>
                <div><h3>Document Types</h3>${renderMiniList(runtime.content?.by_type)}</div>
                <div><h3>Media Collections</h3>${renderMiniList(runtime.content?.media_counts)}</div>
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
              <div class="subtle-meta"><div><strong>Derived Bytes:</strong> ${escapeHTML(formatBytes(runtime.storage?.derived_bytes))}</div></div>
              <div class="debug-grid-two">
                <div><h3>Media Count By Collection</h3>${renderMiniList(runtime.storage?.media_counts)}</div>
                <div><h3>Media Size By Collection</h3>${renderMiniList(runtime.storage?.media_bytes, formatBytes)}</div>
              </div>
              <div><h3>Largest Files</h3>${renderLargestFiles(runtime.storage?.largest_files)}</div>
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
                <div><h3>Integrity Totals</h3>${renderMiniList({
                  duplicate_urls: runtime.integrity?.duplicate_urls || 0,
                  duplicate_slugs: runtime.integrity?.duplicate_slugs || 0,
                  taxonomy_inconsistency: runtime.integrity?.taxonomy_inconsistency || 0,
                })}</div>
                <div><h3>Recent Audit Actions</h3>${renderMiniList(runtime.activity?.recent_audit_by_action)}</div>
              </div>
            </div>`
            : '<div class="panel-pad empty-state">No integrity or activity snapshot loaded yet.</div>',
          'Reference health, route safety, and recent admin activity'
        )}
        ${panel(
          'pprof Profiles',
          `<div class="panel-pad stack">
          <p class="muted">Inspect live runtime state from the admin surface. These endpoints are served through Go's standard <code>net/http/pprof</code> handlers.</p>
          <div class="toolbar">
            <a class="file-link" href="debug/pprof/" target="_blank" rel="noreferrer">Index</a>
            <a class="file-link" href="debug/pprof/heap" target="_blank" rel="noreferrer">Heap</a>
            <a class="file-link" href="debug/pprof/allocs" target="_blank" rel="noreferrer">Allocs</a>
            <a class="file-link" href="debug/pprof/goroutine" target="_blank" rel="noreferrer">Goroutines</a>
            <a class="file-link" href="debug/pprof/mutex" target="_blank" rel="noreferrer">Mutex</a>
            <a class="file-link" href="debug/pprof/block" target="_blank" rel="noreferrer">Block</a>
            <a class="file-link" href="debug/pprof/profile?seconds=10" target="_blank" rel="noreferrer">CPU (10s)</a>
            <a class="file-link" href="debug/pprof/trace?seconds=5" target="_blank" rel="noreferrer">Trace (5s)</a>
          </div>
          <iframe class="debug-frame" src="debug/pprof/" loading="lazy"></iframe>
        </div>`,
          'Authenticated pprof access for live profiling'
        )}
      </div>`;
  };

  return {
    formatBytes,
    formatUptime,
    formatMilliseconds,
    renderMiniList,
    renderLargestFiles,
    renderDebug,
  };
};
