export const createPlatformViews = ({
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
}) => {
  const renderExtensions = () =>
    panel(
      'Extensions',
      `<div class="panel-pad stack">
        <div class="note">Inspect plugin-defined admin pages, widgets, settings registrations, slot usage, and bundle readiness in one place.</div>
        <div class="cards">
          <article class="card"><span class="card-label">Pages</span><strong>${escapeHTML(state.adminExtensions.pages?.length || 0)}</strong><span class="card-copy">Plugin-defined admin pages.</span></article>
          <article class="card"><span class="card-label">Widgets</span><strong>${escapeHTML(state.adminExtensions.widgets?.length || 0)}</strong><span class="card-copy">Plugin-defined widgets and slot mounts.</span></article>
          <article class="card"><span class="card-label">Settings</span><strong>${escapeHTML(state.adminExtensions.settings?.length || 0)}</strong><span class="card-copy">Plugin-defined settings sections.</span></article>
          <article class="card"><span class="card-label">Slots</span><strong>${escapeHTML(state.adminExtensions.slots?.length || 0)}</strong><span class="card-copy">Declared admin shell slots.</span></article>
        </div>
        ${
          state.adminExtensions.pages?.length
            ? `<div class="table table-four"><div class="table-head"><span>Page</span><span>Plugin</span><span>Route</span><span>Bundle</span></div>${state.adminExtensions.pages
                .map(
                  (page) => `<div class="table-row">
                    <span><strong>${escapeHTML(page.title)}</strong><div class="muted mono">${escapeHTML(page.key)}</div></span>
                    <span>${escapeHTML(page.plugin)}</span>
                    <span>${escapeHTML(page.route)}</span>
                    <span>${page.module_url ? `<span class="contract-badge ok">${escapeHTML(page.module_url)}</span>` : '<span class="contract-badge missing">no module</span>'}</span>
                  </div>`
                )
                .join('')}</div>`
            : '<div class="empty-state">No plugin admin pages are registered.</div>'
        }
        ${
          state.adminExtensions.widgets?.length
            ? `<div class="table table-four"><div class="table-head"><span>Widget</span><span>Plugin</span><span>Slot</span><span>Bundle</span></div>${state.adminExtensions.widgets
                .map(
                  (widget) => `<div class="table-row">
                    <span><strong>${escapeHTML(widget.title)}</strong><div class="muted mono">${escapeHTML(widget.key)}</div></span>
                    <span>${escapeHTML(widget.plugin)}</span>
                    <span>${escapeHTML(widget.slot)}</span>
                    <span>${widget.module_url ? `<span class="contract-badge ok">${escapeHTML(widget.module_url)}</span>` : '<span class="contract-badge missing">no module</span>'}</span>
                  </div>`
                )
                .join('')}</div>`
            : ''
        }
        ${
          state.extensionRuntimeErrors?.length
            ? `<div class="stack">
                <h3>Runtime Mount Failures</h3>
                <div class="table table-three">
                  <div class="table-head"><span>Extension</span><span>Kind</span><span>Error</span></div>
                  ${state.extensionRuntimeErrors
                    .map(
                      (entry) => `<div class="table-row">
                        <span><strong>${escapeHTML(entry.plugin)}</strong><div class="muted mono">${escapeHTML(entry.key)}</div></span>
                        <span>${escapeHTML(entry.kind)}${entry.slot ? ` • ${escapeHTML(entry.slot)}` : ''}</span>
                        <span>${escapeHTML(entry.message || 'Unknown error')}</span>
                      </div>`
                    )
                    .join('')}
                </div>
              </div>`
            : ''
        }
      </div>`,
      'Bundle readiness and extension diagnostics'
    );

  const renderPlugins = () => {
    const sortedPlugins = sortItems(state.plugins, 'plugins', (plugin, field) =>
      field === 'version' ? plugin.version : field === 'status' ? plugin.status : plugin.name
    );
    const pagedPlugins = paginateItems(sortedPlugins, 'plugins');
    const pluginExtensionCoverage = state.plugins
      .map((plugin) => {
        const pages = (state.adminExtensions.pages || []).filter(
          (page) => page.plugin === plugin.name
        );
        const widgets = (state.adminExtensions.widgets || []).filter(
          (widget) => widget.plugin === plugin.name
        );
        const settings = (state.adminExtensions.settings || []).filter(
          (setting) => setting.plugin === plugin.name
        );
        const slots = Array.from(
          new Set(
            (state.adminExtensions.slots || [])
              .filter((slot) => slot.plugin === plugin.name)
              .map((slot) => slot.name)
          )
        );
        const bundledPages = pages.filter((page) => page.module_url).length;
        const bundledWidgets = widgets.filter((widget) => widget.module_url).length;
        const eventOnlyPages = pages.length - bundledPages;
        const eventOnlyWidgets = widgets.length - bundledWidgets;
        return {
          plugin,
          pages,
          widgets,
          settings,
          slots,
          bundledPages,
          bundledWidgets,
          eventOnlyPages,
          eventOnlyWidgets,
        };
      })
      .filter(
        (entry) =>
          entry.pages.length || entry.widgets.length || entry.settings.length || entry.slots.length
      );

    const rows = pagedPlugins.items.map(
      (plugin) => `
      <div class="table-row table-row-actions">
        <span><strong>${escapeHTML(plugin.title || plugin.name)}</strong><div class="muted mono">${escapeHTML(plugin.repo || plugin.name)}</div>${plugin.diagnostics?.length ? `<div class="muted">${escapeHTML(plugin.diagnostics[0].message)}</div>` : ''}</span>
        <span>${escapeHTML(plugin.version || '-')}</span>
        <span>${escapeHTML(plugin.health || plugin.status)}</span>
        <span class="row-actions">
          ${plugin.enabled ? `<button class="ghost small" data-disable-plugin="${escapeHTML(plugin.name)}">Disable</button>` : `<button class="ghost small" data-enable-plugin="${escapeHTML(plugin.name)}">Enable</button>`}
          <button class="ghost small" data-validate-plugin="${escapeHTML(plugin.name)}">Validate</button>
          <button class="ghost small" data-update-plugin="${escapeHTML(plugin.name)}">Update</button>
          ${plugin.can_rollback ? `<button class="ghost small" data-rollback-plugin="${escapeHTML(plugin.name)}">Rollback</button>` : ''}
        </span>
      </div>`
    );

    return panel(
      'Plugins',
      `<form id="plugin-install-form" class="inline-form compact-inline-form">
      <label class="frontmatter-span-2">Install Plugin URL<input id="plugin-install-url" type="text" placeholder="github.com/acme/search"></label>
      <label>Directory Name<input id="plugin-install-name" type="text" placeholder="search"></label>
      <button type="submit">Install</button>
    </form>${state.adminExtensions.pages?.length || state.adminExtensions.widgets?.length || state.adminExtensions.settings?.length ? `<div class="panel-pad note">Registered admin extensions: ${escapeHTML(String((state.adminExtensions.pages?.length || 0) + (state.adminExtensions.widgets?.length || 0) + (state.adminExtensions.settings?.length || 0)))}</div>` : ''}${
      extensionPages().length
        ? `<div class="panel-pad stack">
      <h3>Extension Pages</h3>
      <div class="table table-three">
        <div class="table-head"><span>Page</span><span>Route</span><span>Plugin</span></div>
        ${extensionPages()
          .map(
            (page) => `
          <div class="table-row table-row-actions">
            <span><strong>${escapeHTML(page.title)}</strong><div class="muted mono">${escapeHTML(page.key)}</div></span>
            <span>${escapeHTML(adminPathForSection(adminBase, page.section))}</span>
            <span class="row-actions"><span>${escapeHTML(page.plugin)}</span><button class="ghost small" type="button" data-section="${escapeHTML(page.section)}">Open</button></span>
          </div>`
          )
          .join('')}
      </div>
    </div>`
        : ''
    }${renderTableControls(state, 'plugins', state.plugins.length, pagedPlugins.totalPages)}<div class="table table-four"><div class="table-head"><span>Plugin</span><span>Version</span><span>Health</span><span>Action</span></div>${rows.length ? rows.join('') : '<div class="panel-pad empty-state">No plugins found.</div>'}</div>${
      pluginExtensionCoverage.length
        ? `<div class="panel-pad stack"><h3>Extension Coverage</h3><div class="plugin-contract-list">${pluginExtensionCoverage
            .map(
              (entry) => `
      <article class="plugin-contract-card">
        <div class="plugin-contract-header">
          <div>
            <strong>${escapeHTML(entry.plugin.title || entry.plugin.name)}</strong>
            <div class="muted mono">${escapeHTML(entry.plugin.name)}</div>
          </div>
          <div class="plugin-contract-stats">
            <span class="contract-stat ${entry.pages.length ? 'ok' : 'bad'}">Pages ${entry.pages.length}</span>
            <span class="contract-stat ${entry.widgets.length ? 'ok' : 'bad'}">Widgets ${entry.widgets.length}</span>
            <span class="contract-stat ${entry.settings.length ? 'ok' : 'bad'}">Settings ${entry.settings.length}</span>
          </div>
        </div>
        <div class="plugin-contract-meta">
          ${entry.pages.length ? `<span><strong>Page bundles:</strong> ${escapeHTML(`${entry.bundledPages}/${entry.pages.length}`)}</span>` : ''}
          ${entry.widgets.length ? `<span><strong>Widget bundles:</strong> ${escapeHTML(`${entry.bundledWidgets}/${entry.widgets.length}`)}</span>` : ''}
          ${entry.eventOnlyPages || entry.eventOnlyWidgets ? `<span><strong>Event-only mounts:</strong> ${escapeHTML(String(entry.eventOnlyPages + entry.eventOnlyWidgets))}</span>` : ''}
        </div>
        ${
          entry.pages.length
            ? `<section class="plugin-contract-section">
          <strong>Admin Pages</strong>
          <div class="plugin-contract-rows">
            ${entry.pages
              .map(
                (page) => `<div class="plugin-contract-row">
              <span><strong>${escapeHTML(page.title)}</strong><div class="muted mono">${escapeHTML(page.route)}</div></span>
              <span>${page.module_url ? '<span class="contract-badge ok">bundle</span>' : '<span class="contract-badge missing">no bundle</span>'}</span>
              <span>${page.style_urls?.length ? `<span class="contract-badge ok">${escapeHTML(String(page.style_urls.length))} styles</span>` : '<span class="contract-badge missing">no styles</span>'}</span>
            </div>`
              )
              .join('')}
          </div>
        </section>`
            : ''
        }
        ${
          entry.widgets.length
            ? `<section class="plugin-contract-section">
          <strong>Admin Widgets</strong>
          <div class="plugin-contract-rows">
            ${entry.widgets
              .map(
                (widget) => `<div class="plugin-contract-row">
              <span><strong>${escapeHTML(widget.title)}</strong><div class="muted mono">${escapeHTML(widget.slot)}</div></span>
              <span>${widget.module_url ? '<span class="contract-badge ok">bundle</span>' : '<span class="contract-badge missing">no bundle</span>'}</span>
              <span>${widget.style_urls?.length ? `<span class="contract-badge ok">${escapeHTML(String(widget.style_urls.length))} styles</span>` : '<span class="contract-badge missing">no styles</span>'}</span>
            </div>`
              )
              .join('')}
          </div>
        </section>`
            : ''
        }
        ${
          entry.settings.length || entry.slots.length
            ? `<div class="plugin-contract-meta">
          ${entry.settings.length ? `<span><strong>Settings sections:</strong> ${escapeHTML(entry.settings.map((item) => item.title).join(', '))}</span>` : ''}
          ${entry.slots.length ? `<span><strong>Declared slots:</strong> ${escapeHTML(entry.slots.join(', '))}</span>` : ''}
        </div>`
            : ''
        }
      </article>`
            )
            .join('')}</div></div>`
        : ''
    }${renderWidgetPanels('plugins.sidebar').join('')}`,
      `${state.plugins.length} plugins`
    );
  };

  const renderThemes = () => {
    const requiredAdminComponents = [
      'shell',
      'login',
      'navigation',
      'documents',
      'media',
      'users',
      'config',
      'plugins',
      'themes',
      'audit',
    ];
    const requiredAdminWidgetSlots = [
      'overview.after',
      'documents.sidebar',
      'media.sidebar',
      'plugins.sidebar',
    ];
    const sortedThemes = sortItems(state.themes, 'themes', (theme, field) =>
      field === 'version' ? theme.version : field === 'valid' ? String(theme.valid) : theme.name
    );
    const pagedThemes = paginateItems(sortedThemes, 'themes');
    const rows = pagedThemes.items.map(
      (theme) => `
      <div class="table-row table-row-actions">
        <span><strong>${escapeHTML(theme.title || theme.name)}</strong><div class="muted">${escapeHTML(theme.kind || 'frontend')} theme</div>${theme.diagnostics?.length ? `<div class="muted">${escapeHTML(theme.diagnostics[0].message)}</div>` : ''}</span>
        <span>${escapeHTML(theme.version || '-')}</span>
        <span>${theme.valid ? 'valid' : 'invalid'}</span>
        <span class="row-actions">
          <button class="ghost small" data-validate-theme="${escapeHTML(theme.name)}" data-theme-kind="${escapeHTML(theme.kind || 'frontend')}">Validate</button>
          ${theme.current ? '<span class="muted">Current</span>' : `<button class="ghost small" data-switch-theme="${escapeHTML(theme.name)}" data-theme-kind="${escapeHTML(theme.kind || 'frontend')}">Activate</button>`}
        </span>
      </div>`
    );
    const adminThemeDetails = state.themes
      .filter((theme) => theme.kind === 'admin')
      .map((theme) => {
        const componentSet = new Set(theme.components || []);
        const widgetSlotSet = new Set(theme.widget_slots || []);
        const diagnostics = Array.isArray(theme.diagnostics) ? theme.diagnostics : [];
        return `
          <article class="theme-contract-card">
            <div class="theme-contract-header">
              <div>
                <h3>${escapeHTML(theme.title || theme.name)}</h3>
                <div class="muted">${escapeHTML(theme.name)} • ${escapeHTML(theme.compatibility_version || 'compatibility version not set')}</div>
              </div>
              <div class="theme-contract-stats">
                <span class="contract-stat ${theme.valid ? 'ok' : 'bad'}">Components ${componentSet.size}/${requiredAdminComponents.length}</span>
                <span class="contract-stat ${theme.valid ? 'ok' : 'bad'}">Widget slots ${widgetSlotSet.size}/${requiredAdminWidgetSlots.length}</span>
              </div>
            </div>
            <div class="theme-contract-grid">
              <section class="theme-contract-section">
                <strong>Components</strong>
                <div class="contract-badges">
                  ${requiredAdminComponents.map((component) => `<span class="contract-badge ${componentSet.has(component) ? 'ok' : 'missing'}">${escapeHTML(component)}</span>`).join('')}
                </div>
              </section>
              <section class="theme-contract-section">
                <strong>Widget Slots</strong>
                <div class="contract-badges">
                  ${requiredAdminWidgetSlots.map((slot) => `<span class="contract-badge ${widgetSlotSet.has(slot) ? 'ok' : 'missing'}">${escapeHTML(slot)}</span>`).join('')}
                </div>
              </section>
            </div>
            <div class="theme-contract-meta">
              <span><strong>Admin API:</strong> ${escapeHTML(theme.admin_api || 'v1')}</span>
              <span><strong>SDK:</strong> ${escapeHTML(theme.sdk_version || 'v1')}</span>
              <span><strong>Status:</strong> ${theme.valid ? 'Valid' : 'Needs attention'}</span>
              <span><strong>Current:</strong> ${theme.current ? 'Yes' : 'No'}</span>
            </div>
            <div class="theme-contract-diagnostics">
              <strong>Diagnostics</strong>
              ${diagnostics.length ? `<div class="contract-diagnostic-list">${diagnostics.map((diag) => `<div class="contract-diagnostic ${escapeHTML(diag.severity || 'info')}"><span class="contract-diagnostic-severity">${escapeHTML(diag.severity || 'info')}</span><span>${escapeHTML(diag.message || '')}</span></div>`).join('')}</div>` : '<div class="muted">No validation diagnostics.</div>'}
            </div>
          </article>`;
      })
      .join('');
    return panel(
      'Themes',
      `<form id="theme-install-form" class="inline-form compact-inline-form">
        <label class="frontmatter-span-2">Install Theme URL<input id="theme-install-url" type="text" placeholder="github.com/acme/aurora"></label>
        <label>Directory Name<input id="theme-install-name" type="text" placeholder="aurora"></label>
        <label class="frontmatter-span-2">Kind<select id="theme-install-kind"><option value="frontend">Frontend</option><option value="admin">Admin</option></select></label>
        <button type="submit">Install</button>
      </form>${renderTableControls(state, 'themes', state.themes.length, pagedThemes.totalPages)}<div class="table table-four"><div class="table-head"><span>Theme</span><span>Version</span><span>Validation</span><span>Action</span></div>${rows.length ? rows.join('') : '<div class="panel-pad empty-state">No themes found.</div>'}</div>${adminThemeDetails ? `<div class="panel-pad stack"><h3>Admin Theme Contract</h3><div class="theme-contract-list">${adminThemeDetails}</div></div>` : ''}`,
      `${state.themes.length} frontend and admin themes`
    );
  };

  return {
    renderExtensions,
    renderPlugins,
    renderThemes,
  };
};
