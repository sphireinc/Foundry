import { _t } from '../core/i18n.js';

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
  const riskBadge = (risk) => {
    const value = (risk || 'low').toLowerCase();
    if (value === 'high') return `<span class="contract-badge missing">${_t('high risk')}</span>`;
    if (value === 'medium') return `<span class="contract-badge warn">${_t('medium risk')}</span>`;
    return `<span class="contract-badge ok">${_t('low risk')}</span>`;
  };

  const renderExtensions = () =>
    panel(
      'Extensions',
      `<div class="panel-pad stack">
        <div class="note">${_t('Inspect plugin-defined admin pages, widgets, settings registrations, slot usage, and bundle readiness in one place.')}</div>
        <div class="cards">
          <article class="card"><span class="card-label">${_t('Pages')}</span><strong>${escapeHTML(state.adminExtensions.pages?.length || 0)}</strong><span class="card-copy">${_t('Plugin-defined admin pages.')}</span></article>
          <article class="card"><span class="card-label">${_t('Widgets')}</span><strong>${escapeHTML(state.adminExtensions.widgets?.length || 0)}</strong><span class="card-copy">${_t('Plugin-defined widgets and slot mounts.')}</span></article>
          <article class="card"><span class="card-label">${_t('Settings')}</span><strong>${escapeHTML(state.adminExtensions.settings?.length || 0)}</strong><span class="card-copy">${_t('Plugin-defined settings sections.')}</span></article>
          <article class="card"><span class="card-label">${_t('Slots')}</span><strong>${escapeHTML(state.adminExtensions.slots?.length || 0)}</strong><span class="card-copy">${_t('Declared admin shell slots.')}</span></article>
        </div>
        ${
          state.adminExtensions.pages?.length
            ? `<div class="table table-four"><div class="table-head"><span>${_t('Page')}</span><span>${_t('Plugin')}</span><span>${_t('Route')}</span><span>${_t('Bundle')}</span></div>${state.adminExtensions.pages
                .map(
                  (page) => `<div class="table-row">
                    <span><strong>${escapeHTML(page.title)}</strong><div class="muted mono">${escapeHTML(page.key)}</div></span>
                    <span>${escapeHTML(page.plugin)}</span>
                    <span>${escapeHTML(page.route)}</span>
                    <span>${page.module_url ? `<span class="contract-badge ok">${escapeHTML(page.module_url)}</span>` : `<span class="contract-badge missing">${_t('no module')}</span>`}</span>
                  </div>`
                )
                .join('')}</div>`
            : `<div class="empty-state">${_t('No plugin admin pages are registered.')}</div>`
        }
        ${
          state.adminExtensions.widgets?.length
            ? `<div class="table table-four"><div class="table-head"><span>${_t('Widget')}</span><span>${_t('Plugin')}</span><span>${_t('Slot')}</span><span>${_t('Bundle')}</span></div>${state.adminExtensions.widgets
                .map(
                  (widget) => `<div class="table-row">
                    <span><strong>${escapeHTML(widget.title)}</strong><div class="muted mono">${escapeHTML(widget.key)}</div></span>
                    <span>${escapeHTML(widget.plugin)}</span>
                    <span>${escapeHTML(widget.slot)}</span>
                    <span>${widget.module_url ? `<span class="contract-badge ok">${escapeHTML(widget.module_url)}</span>` : `<span class="contract-badge missing">${_t('no module')}</span>`}</span>
                  </div>`
                )
                .join('')}</div>`
            : ''
        }
        ${
          state.extensionRuntimeErrors?.length
            ? `<div class="stack">
                <h3>${_t('Runtime Mount Failures')}</h3>
                <div class="table table-three">
                  <div class="table-head"><span>${_t('Extension')}</span><span>${_t('Kind')}</span><span>${_t('Error')}</span></div>
                  ${state.extensionRuntimeErrors
                    .map(
                      (entry) => `<div class="table-row">
                        <span><strong>${escapeHTML(entry.plugin)}</strong><div class="muted mono">${escapeHTML(entry.key)}</div></span>
                        <span>${escapeHTML(entry.kind)}${entry.slot ? ` • ${escapeHTML(entry.slot)}` : ''}</span>
                        <span>${escapeHTML(entry.message || _t('Unknown error'))}</span>
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
        <span><strong>${escapeHTML(plugin.title || plugin.name)}</strong><div class="muted mono">${escapeHTML(plugin.repo || plugin.name)}</div><div class="plugin-contract-meta">${riskBadge(plugin.risk_tier)}${plugin.requires_approval ? `<span class="contract-badge warn">${_t('approval required')}</span>` : ''}</div>${plugin.permission_summary?.length ? `<div class="muted">${escapeHTML(plugin.permission_summary.join(' • '))}</div>` : ''}${plugin.diagnostics?.length ? `<div class="muted">${escapeHTML(plugin.diagnostics[0].message)}</div>` : ''}</span>
        <span>${escapeHTML(plugin.version || '-')}</span>
        <span>${escapeHTML(plugin.health || plugin.status)}</span>
        <span class="row-actions">
          ${plugin.enabled ? `<button class="ghost small" data-disable-plugin="${escapeHTML(plugin.name)}">${_t('Disable')}</button>` : `<button class="ghost small" data-enable-plugin="${escapeHTML(plugin.name)}">${_t('Enable')}</button>`}
          <button class="ghost small" data-validate-plugin="${escapeHTML(plugin.name)}">${_t('Validate')}</button>
          <button class="ghost small" data-update-plugin="${escapeHTML(plugin.name)}">${_t('Update')}</button>
          ${plugin.can_rollback ? `<button class="ghost small" data-rollback-plugin="${escapeHTML(plugin.name)}">${_t('Rollback')}</button>` : ''}
        </span>
      </div>`
    );

    return panel(
      'Plugins',
      `<form id="plugin-install-form" class="inline-form compact-inline-form">
      <label class="frontmatter-span-2">${_t('Install Plugin URL')}<input id="plugin-install-url" type="text" placeholder="github.com/acme/search"></label>
      <label>${_t('Directory Name')}<input id="plugin-install-name" type="text" placeholder="search"></label>
      <button type="submit">${_t('Install')}</button>
    </form>${state.adminExtensions.pages?.length || state.adminExtensions.widgets?.length || state.adminExtensions.settings?.length ? `<div class="panel-pad note">${_t('Registered admin extensions: {count}', { count: (state.adminExtensions.pages?.length || 0) + (state.adminExtensions.widgets?.length || 0) + (state.adminExtensions.settings?.length || 0) })}</div>` : ''}${
      extensionPages().length
        ? `<div class="panel-pad stack">
      <h3>${_t('Extension Pages')}</h3>
      <div class="table table-three">
        <div class="table-head">
            <span>${_t('Menu Name')}</span>
            <span>${_t('Action')}</span>
            <span>${_t('Route')}</span>
        </div>
        ${extensionPages()
          .map(
            (page) => `
          <div class="table-row table-row-actions">
            <span>
                <strong>${escapeHTML(page.title)}</strong>
                <div class="muted mono">${_t('page')}: /plugins/${escapeHTML(page.key)}</div>
                <div class="muted mono">${_t('plugin')}: ${escapeHTML(page.plugin)}</div>
            </span>
            <span class="row-actions"><button class="ghost small" type="button" data-section="${escapeHTML(page.section)}">${_t('Open')}</button></span>
            
            <span>${escapeHTML(adminPathForSection(adminBase, page.section))}</span>
          </div>`
          )
          .join('')}
      </div>
    </div>`
        : ''
    }${renderTableControls(state, 'plugins', state.plugins.length, pagedPlugins.totalPages)}<div class="table table-four"><div class="table-head"><span>${_t('Plugin')}</span><span>${_t('Version')}</span><span>${_t('Health')}</span><span>${_t('Action')}</span></div>${rows.length ? rows.join('') : `<div class="panel-pad empty-state">${_t('No plugins found.')}</div>`}</div>${
      state.plugins.length
        ? `<div class="panel-pad stack"><h3>${_t('Plugin Security')}</h3><div class="plugin-contract-list">${state.plugins
            .map(
              (plugin) => `
              <article class="plugin-contract-card">
                <div class="plugin-contract-header">
                  <div>
                    <strong>${escapeHTML(plugin.title || plugin.name)}</strong>
                    <div class="muted mono">${escapeHTML(plugin.name)}</div>
                  </div>
                  <div class="plugin-contract-stats">
                    ${riskBadge(plugin.risk_tier)}
                    ${plugin.requires_approval ? `<span class="contract-stat bad">${_t('approval required')}</span>` : `<span class="contract-stat ok">${_t('declared')}</span>`}
                    ${plugin.security?.effective?.allowed === false ? `<span class="contract-stat bad">${_t('blocked by policy')}</span>` : `<span class="contract-stat ok">${_t('policy aligned')}</span>`}
                  </div>
                </div>
                <div class="plugin-contract-meta">${(plugin.permission_summary || []).map((item) => `<span>${escapeHTML(item)}</span>`).join('')}${(plugin.runtime_summary || []).map((item) => `<span>${escapeHTML(item)}</span>`).join('')}</div>
                ${plugin.security?.effective?.denied_reasons?.length ? `<section class="plugin-contract-section"><strong>${_t('Enforcement')}</strong><div class="contract-diagnostic-list">${plugin.security.effective.denied_reasons.map((reason) => `<div class="contract-diagnostic error"><span class="contract-diagnostic-severity">${_t('error')}</span><span>${escapeHTML(reason)}</span></div>`).join('')}</div></section>` : ''}
                ${plugin.security_mismatches?.length ? `<section class="plugin-contract-section"><strong>${_t('Security mismatches')}</strong><div class="contract-diagnostic-list">${plugin.security_mismatches.map((diag) => `<div class="contract-diagnostic error"><span class="contract-diagnostic-severity">${_t('error')}</span><span>${escapeHTML(diag.message || '')}</span></div>`).join('')}</div></section>` : ''}
                ${plugin.security_findings?.length ? `<section class="plugin-contract-section"><strong>${_t('Detected capabilities')}</strong><div class="plugin-contract-rows">${plugin.security_findings.map((finding) => `<div class="plugin-contract-row"><span><strong>${escapeHTML(finding.category)}</strong><div class="muted mono">${escapeHTML(finding.evidence || '')}</div></span><span>${escapeHTML(finding.path || '')}</span></div>`).join('')}</div></section>` : ''}
                <pre class="diff-viewer">${escapeHTML(JSON.stringify(plugin.permissions || {}, null, 2))}</pre>
                <pre class="diff-viewer">${escapeHTML(JSON.stringify(plugin.runtime || {}, null, 2))}</pre>
              </article>`
            )
            .join('')}</div></div>`
        : ''
    }${
      pluginExtensionCoverage.length
        ? `<div class="panel-pad stack"><h3>${_t('Extension Coverage')}</h3><div class="plugin-contract-list">${pluginExtensionCoverage
            .map(
              (entry) => `
      <article class="plugin-contract-card">
        <div class="plugin-contract-header">
          <div>
            <strong>${escapeHTML(entry.plugin.title || entry.plugin.name)}</strong>
            <div class="muted mono">${escapeHTML(entry.plugin.name)}</div>
          </div>
          <div class="plugin-contract-stats">
            <span class="contract-stat ${entry.pages.length ? 'ok' : 'bad'}">${_t('Pages {count}', { count: entry.pages.length })}</span>
            <span class="contract-stat ${entry.widgets.length ? 'ok' : 'bad'}">${_t('Widgets {count}', { count: entry.widgets.length })}</span>
            <span class="contract-stat ${entry.settings.length ? 'ok' : 'bad'}">${_t('Settings {count}', { count: entry.settings.length })}</span>
          </div>
        </div>
        <div class="plugin-contract-meta">
          ${entry.pages.length ? `<span><strong>${_t('Page bundles')}:</strong> ${escapeHTML(`${entry.bundledPages}/${entry.pages.length}`)}</span>` : ''}
          ${entry.widgets.length ? `<span><strong>${_t('Widget bundles')}:</strong> ${escapeHTML(`${entry.bundledWidgets}/${entry.widgets.length}`)}</span>` : ''}
          ${entry.eventOnlyPages || entry.eventOnlyWidgets ? `<span><strong>${_t('Event-only mounts')}:</strong> ${escapeHTML(String(entry.eventOnlyPages + entry.eventOnlyWidgets))}</span>` : ''}
        </div>
        ${
          entry.pages.length
            ? `<section class="plugin-contract-section">
          <strong>${_t('Admin Pages')}</strong>
          <div class="plugin-contract-rows">
            ${entry.pages
              .map(
                (page) => `<div class="plugin-contract-row">
              <span><strong>${escapeHTML(page.title)}</strong><div class="muted mono">${escapeHTML(page.route)}</div></span>
              <span>${page.module_url ? `<span class="contract-badge ok">${_t('bundle')}</span>` : `<span class="contract-badge missing">${_t('no bundle')}</span>`}</span>
              <span>${page.style_urls?.length ? `<span class="contract-badge ok">${_t('{count} styles', { count: escapeHTML(String(page.style_urls.length)) })}</span>` : `<span class="contract-badge missing">${_t('no styles')}</span>`}</span>
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
          <strong>${_t('Admin Widgets')}</strong>
          <div class="plugin-contract-rows">
            ${entry.widgets
              .map(
                (widget) => `<div class="plugin-contract-row">
              <span><strong>${escapeHTML(widget.title)}</strong><div class="muted mono">${escapeHTML(widget.slot)}</div></span>
              <span>${widget.module_url ? `<span class="contract-badge ok">${_t('bundle')}</span>` : `<span class="contract-badge missing">${_t('no bundle')}</span>`}</span>
              <span>${widget.style_urls?.length ? `<span class="contract-badge ok">${_t('{count} styles', { count: escapeHTML(String(widget.style_urls.length)) })}</span>` : `<span class="contract-badge missing">${_t('no styles')}</span>`}</span>
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
          ${entry.settings.length ? `<span><strong>${_t('Settings sections')}:</strong> ${escapeHTML(entry.settings.map((item) => item.title).join(', '))}</span>` : ''}
          ${entry.slots.length ? `<span><strong>${_t('Declared slots')}:</strong> ${escapeHTML(entry.slots.join(', '))}</span>` : ''}
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
        <span><strong>${escapeHTML(theme.title || theme.name)}</strong><div class="muted">${escapeHTML(theme.kind || 'frontend')} ${_t('theme')}</div>${theme.security_summary?.length ? `<div class="muted">${escapeHTML(theme.security_summary.join(' • '))}</div>` : ''}${theme.diagnostics?.length ? `<div class="muted">${escapeHTML(theme.diagnostics[0].message)}</div>` : ''}</span>
        <span>${escapeHTML(theme.version || '-')}</span>
        <span>${_t(theme.valid ? 'valid' : 'invalid')}</span>
        <span class="row-actions">
          <button class="ghost small" data-validate-theme="${escapeHTML(theme.name)}" data-theme-kind="${escapeHTML(theme.kind || 'frontend')}">${_t('Validate')}</button>
          ${theme.current ? `<span class="muted">${_t('Current')}</span>` : `<button class="ghost small" data-switch-theme="${escapeHTML(theme.name)}" data-theme-kind="${escapeHTML(theme.kind || 'frontend')}">${_t('Activate')}</button>`}
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
                <div class="muted">${escapeHTML(theme.name)} • ${escapeHTML(theme.compatibility_version || _t('compatibility version not set'))}</div>
              </div>
              <div class="theme-contract-stats">
                <span class="contract-stat ${theme.valid ? 'ok' : 'bad'}">${_t('Components {count} of {total}', { count: componentSet.size, total: requiredAdminComponents.length })}</span>
                <span class="contract-stat ${theme.valid ? 'ok' : 'bad'}">${_t('Widget slots {count} of {total}', { count: widgetSlotSet.size, total: requiredAdminWidgetSlots.length })}</span>
              </div>
            </div>
            <div class="theme-contract-grid">
              <section class="theme-contract-section">
                <strong>${_t('Components')}</strong>
                <div class="contract-badges">
                  ${requiredAdminComponents.map((component) => `<span class="contract-badge ${componentSet.has(component) ? 'ok' : 'missing'}">${escapeHTML(component)}</span>`).join('')}
                </div>
              </section>
              <section class="theme-contract-section">
                <strong>${_t('Widget Slots')}</strong>
                <div class="contract-badges">
                  ${requiredAdminWidgetSlots.map((slot) => `<span class="contract-badge ${widgetSlotSet.has(slot) ? 'ok' : 'missing'}">${escapeHTML(slot)}</span>`).join('')}
                </div>
              </section>
            </div>
            <div class="theme-contract-meta">
              <span><strong>${_t('Admin API')}:</strong> ${escapeHTML(theme.admin_api || 'v1')}</span>
              <span><strong>${_t('SDK')}:</strong> ${escapeHTML(theme.sdk_version || 'v1')}</span>
              <span><strong>${_t('Status')}:</strong> ${theme.valid ? _t('Valid') : _t('Needs attention')}</span>
              <span><strong>${_t('Current')}:</strong> ${theme.current ? _t('Yes') : _t('No')}</span>
            </div>
            <div class="theme-contract-diagnostics">
              <strong>${_t('Diagnostics')}</strong>
              ${diagnostics.length ? `<div class="contract-diagnostic-list">${diagnostics.map((diag) => `<div class="contract-diagnostic ${escapeHTML(diag.severity || 'info')}"><span class="contract-diagnostic-severity">${escapeHTML(diag.severity || 'info')}</span><span>${escapeHTML(diag.message || '')}</span></div>`).join('')}</div>` : `<div class="muted">${_t('No validation diagnostics.')}</div>`}
            </div>
          </article>`;
      })
      .join('');
    return panel(
      'Themes',
      `<form id="theme-install-form" class="inline-form compact-inline-form">
        <label class="frontmatter-span-2">${_t('Install Theme URL')}<input id="theme-install-url" type="text" placeholder="github.com/acme/aurora"></label>
        <label>${_t('Directory Name')}<input id="theme-install-name" type="text" placeholder="aurora"></label>
        <label class="frontmatter-span-2">${_t('Kind')}<select id="theme-install-kind"><option value="frontend">${_t('Frontend')}</option><option value="admin">${_t('Admin')}</option></select></label>
        <button type="submit">${_t('Install')}</button>
      </form>${renderTableControls(state, 'themes', state.themes.length, pagedThemes.totalPages)}<div class="table table-four"><div class="table-head"><span>${_t('Theme')}</span><span>${_t('Version')}</span><span>${_t('Validation')}</span><span>${_t('Action')}</span></div>${rows.length ? rows.join('') : `<div class="panel-pad empty-state">${_t('No themes found.')}</div>`}</div>${
        state.themes.filter((theme) => theme.kind === 'frontend').length
          ? `<div class="panel-pad stack"><h3>${_t('Theme Security')}</h3><div class="plugin-contract-list">${state.themes
              .filter((theme) => theme.kind === 'frontend')
              .map(
                (theme) => `
          <article class="plugin-contract-card">
            <div class="plugin-contract-header">
              <div>
                <strong>${escapeHTML(theme.title || theme.name)}</strong>
                <div class="muted mono">${escapeHTML(theme.name)}</div>
              </div>
              <div class="plugin-contract-stats">
                ${theme.valid ? `<span class="contract-stat ok">${_t('validated')}</span>` : `<span class="contract-stat bad">${_t('validation issues')}</span>`}
              </div>
            </div>
            <div class="plugin-contract-meta">${(theme.security_summary || []).map((item) => `<span>${escapeHTML(item)}</span>`).join('')}</div>
            ${theme.security_report?.detected_assets?.length ? `<section class="plugin-contract-section"><strong>${_t('Detected remote assets')}</strong><div class="plugin-contract-rows">${theme.security_report.detected_assets.map((item) => `<div class="plugin-contract-row"><span><strong>${escapeHTML(item.kind || 'asset')}</strong><div class="muted mono">${escapeHTML(item.path || '')}</div></span><span>${escapeHTML(item.url || '')}</span><span><span class="contract-badge ${item.status === 'declared' ? 'ok' : 'missing'}">${escapeHTML(item.status || 'unknown')}</span></span></div>`).join('')}</div></section>` : ''}
            ${theme.security_report?.detected_requests?.length ? `<section class="plugin-contract-section"><strong>${_t('Detected frontend requests')}</strong><div class="plugin-contract-rows">${theme.security_report.detected_requests.map((item) => `<div class="plugin-contract-row"><span><strong>${escapeHTML(item.kind || 'request')}</strong><div class="muted mono">${escapeHTML(item.path || '')}</div></span><span>${escapeHTML(item.url || '')}</span><span><span class="contract-badge ${item.status === 'declared' ? 'ok' : 'missing'}">${escapeHTML(item.status || 'unknown')}</span></span></div>`).join('')}</div></section>` : ''}
            ${theme.security_report?.mismatches?.length ? `<section class="plugin-contract-section"><strong>${_t('Security mismatches')}</strong><div class="contract-diagnostic-list">${theme.security_report.mismatches.map((diag) => `<div class="contract-diagnostic error"><span class="contract-diagnostic-severity">${_t('error')}</span><span>${escapeHTML(diag.message || '')}</span></div>`).join('')}</div></section>` : ''}
            ${theme.security_report?.csp_summary?.length ? `<section class="plugin-contract-section"><strong>${_t('Generated CSP summary')}</strong><div class="plugin-contract-meta">${theme.security_report.csp_summary.map((item) => `<span>${escapeHTML(item)}</span>`).join('')}</div></section>` : ''}
            ${theme.security_report?.generated_csp ? `<pre class="diff-viewer">${escapeHTML(theme.security_report.generated_csp)}</pre>` : ''}
            <pre class="diff-viewer">${escapeHTML(JSON.stringify(theme.security || {}, null, 2))}</pre>
          </article>`
              )
              .join('')}</div></div>`
          : ''
      }${adminThemeDetails ? `<div class="panel-pad stack"><h3>${_t('Admin Theme Contract')}</h3><div class="theme-contract-list">${adminThemeDetails}</div></div>` : ''}`,
      _t('{count} frontend and admin themes', { count: state.themes.length })
    );
  };

  const renderOperations = () => {
    const updateInfo = state.updateInfo || null;
    const operations = state.operationsStatus || {};
    const runtime = state.runtimeStatus || {};
    const backupConfig = state.settingsForm?.Backup || {};
    const updateReason = !updateInfo
      ? _t('Update status has not been loaded yet.')
      : updateInfo.has_update && updateInfo.apply_supported
        ? _t('A new release is available and this install can update in place.')
        : updateInfo.has_update
          ? updateInfo.instructions ||
            _t('A new release is available, but this install mode cannot self-update in place.')
          : _t('You are already on the latest release.');
    const currentReleaseDetail =
      updateInfo?.install_mode === 'source'
        ? `${_t('Source checkout at commit {commit}, based on {tag}.', {
            commit: escapeHTML(updateInfo?.current_commit || _t('unknown')),
            tag: escapeHTML(
              updateInfo?.nearest_tag || updateInfo?.current_version || _t('unknown')
            ),
          })}${updateInfo?.dirty ? ` ${_t('The checkout has local changes.')}` : ''}`
        : _t('This is the Foundry version currently running on this site.');
    const zipRows = (state.backups || [])
      .map(
        (item) => `<div class="table-row table-row-actions">
          <span><strong>${escapeHTML(item.name)}</strong><div class="muted mono">${escapeHTML(item.created_at || '')}</div></span>
          <span>${escapeHTML(String(item.size_bytes || 0))}</span>
          <span>${escapeHTML(item.created_at || '')}</span>
          <span class="row-actions">
            <a class="ghost small" href="${escapeHTML(`${adminBase}/api/backups/download?name=${encodeURIComponent(item.name)}`)}">${_t('Download')}</a>
            <button class="ghost small" type="button" data-restore-backup="${escapeHTML(item.name)}">${_t('Restore')}</button>
          </span>
        </div>`
      )
      .join('');
    const gitRows = (state.gitBackups || [])
      .map(
        (item) => `<div class="table-row">
          <span><strong>${escapeHTML((item.revision || '').slice(0, 12))}</strong><div class="muted mono">${escapeHTML(item.message || '')}</div></span>
          <span>${escapeHTML(item.branch || '-')}</span>
          <span>${escapeHTML(item.created_at || '')}</span>
          <span>${_t(item.pushed ? 'pushed' : item.changed ? 'local' : 'unchanged')}</span>
        </div>`
      )
      .join('');
    const checkRows = (operations.checks || [])
      .map(
        (item) =>
          `<div class="mini-list-row"><span>${escapeHTML(item.name)}</span><strong>${escapeHTML(item.status || 'unknown')}</strong></div>`
      )
      .join('');
    const logContent = state.operationsLog?.content
      ? `<pre class="diff-viewer">${escapeHTML(state.operationsLog.content)}</pre>`
      : `<div class="empty-state">${_t('No service or standalone log output available.')}</div>`;
    const validationSummary = state.siteValidation
      ? `<div class="cards">
          <article class="card"><span class="card-label">${_t('Findings')}</span><strong>${escapeHTML(String(state.siteValidation.message_count || 0))}</strong><span class="card-copy">${_t('Latest on-demand validation result.')}</span></article>
          <article class="card"><span class="card-label">${_t('Broken Media')}</span><strong>${escapeHTML(String((state.siteValidation.broken_media_refs || []).length))}</strong><span class="card-copy">${_t('Unresolved media references.')}</span></article>
          <article class="card"><span class="card-label">${_t('Broken Links')}</span><strong>${escapeHTML(String((state.siteValidation.broken_internal_links || []).length))}</strong><span class="card-copy">${_t('Internal links that do not resolve.')}</span></article>
          <article class="card"><span class="card-label">${_t('Templates')}</span><strong>${escapeHTML(String((state.siteValidation.missing_templates || []).length))}</strong><span class="card-copy">${_t('Missing layouts or duplicate routes.')}</span></article>
        </div>`
      : `<div class="empty-state">${_t('Run validation to capture the latest site integrity snapshot.')}</div>`;

    return panel(
      'Operations',
      `<div class="panel-pad stack">
        <div class="cards">
          <article class="card"><span class="card-label">${_t('Current Release')}</span><strong>${escapeHTML(updateInfo?.current_display_version || updateInfo?.current_version || _t('unknown'))}</strong><span class="card-copy">${escapeHTML(currentReleaseDetail)}</span></article>
          <article class="card"><span class="card-label">${_t('Latest Release')}</span><strong>${escapeHTML(updateInfo?.latest_version || _t('unknown'))}</strong><span class="card-copy">${_t('Latest GitHub release.')}</span></article>
          <article class="card"><span class="card-label">${_t('Install Mode')}</span><strong>${escapeHTML(updateInfo?.install_mode || _t('unknown'))}</strong><span class="card-copy">${_t('Update support depends on deployment mode.')}</span></article>
          <article class="card"><span class="card-label">${_t('Service')}</span><strong>${_t(operations.service_running ? 'running' : operations.standalone_active ? 'standalone' : 'inactive')}</strong><span class="card-copy">${escapeHTML(operations.service_message || _t('No managed service detected.'))}</span></article>
        </div>
        <div class="toolbar">
          <button class="ghost small" type="button" id="operations-refresh">${_t('Refresh Operations')}</button>
          <button class="ghost small" type="button" id="update-refresh">${_t('Refresh Update Status')}</button>
          <button class="ghost small" type="button" id="update-apply" ${updateInfo?.apply_supported && updateInfo?.has_update ? '' : `disabled aria-disabled="true" title="${_t('Self-update is not currently available')}"`}>${_t('Apply Update')}</button>
          <button class="ghost small" type="button" id="operations-clear-cache">${_t('Clear Cache')}</button>
          <button class="ghost small" type="button" id="operations-rebuild">${_t('Rebuild')}</button>
          <button class="ghost small" type="button" id="operations-validate">${_t('Run Validation')}</button>
          <button class="ghost small" type="button" id="operations-logs-refresh">${_t('Refresh Logs')}</button>
        </div>
        <div class="subtle-meta">
          ${updateInfo?.install_mode === 'source' ? `<div><strong>${_t('Nearest tag')}:</strong> ${escapeHTML(updateInfo.nearest_tag || '-')}</div><div><strong>${_t('Current commit')}:</strong> ${escapeHTML(updateInfo.current_commit || '-')}</div><div><strong>${_t('Local changes')}:</strong> ${_t(updateInfo.dirty ? 'dirty' : 'clean')}</div>` : ''}
          <div><strong>${_t('Service')}:</strong> ${escapeHTML(operations.service_name || _t('not installed'))}</div>
          <div><strong>${_t('Service file')}:</strong> ${escapeHTML(operations.service_file || '-')}</div>
          <div><strong>${_t('Standalone PID')}:</strong> ${escapeHTML(String(operations.standalone_pid || 0))}</div>
          <div><strong>${_t('Log')}:</strong> ${escapeHTML(operations.service_log || operations.standalone_log || '-')}</div>
        </div>
        <div class="note">${escapeHTML(updateReason)}</div>
      </div>
      <div class="panel-pad stack">
        <h3>${_t('Health Checks')}</h3>
        <div class="mini-list">${checkRows || `<div class="empty-state">${_t('No health checks loaded.')}</div>`}</div>
      </div>
      <div class="panel-pad stack">
        <h3>${_t('Backup Targets')}</h3>
        <form id="operations-backup-git-form" class="inline-form compact-inline-form">
          <label class="frontmatter-span-2">${_t('Git Remote URL')}<input id="operations-git-remote-url" type="text" value="${escapeHTML(backupConfig.GitRemoteURL || '')}" placeholder="git@github.com:you/foundry-backups.git"></label>
          <label>${_t('Branch')}<input id="operations-git-branch" type="text" value="${escapeHTML(backupConfig.GitBranch || 'main')}" placeholder="main"></label>
          <label class="checkbox frontmatter-span-2"><input id="operations-git-push-on-change" type="checkbox" ${backupConfig.GitPushOnChange ? 'checked' : ''}> ${_t('Push automatically after Git snapshots')}</label>
          <button type="submit">${_t('Save Git Backup Settings')}</button>
        </form>
        <div class="toolbar">
          <button class="ghost small" type="button" id="backup-create">${_t('Create Zip Backup')}</button>
          <button class="ghost small" type="button" id="backup-git-create">${_t('Create Git Snapshot')}</button>
          ${backupConfig.GitRemoteURL ? `<button class="ghost small" type="button" id="backup-git-push">${_t('Create & Push Git Snapshot')}</button>` : ''}
        </div>
      </div>
      <div class="panel-pad stack">
        <h3>${_t('Zip Backups')}</h3>
        <div class="table table-four"><div class="table-head"><span>${_t('Backup')}</span><span>${_t('Bytes')}</span><span>${_t('Created')}</span><span>${_t('Action')}</span></div>${zipRows || `<div class="panel-pad empty-state">${_t('No zip backups found.')}</div>`}</div>
      </div>
      <div class="panel-pad stack">
        <h3>${_t('Git Snapshots')}</h3>
        <div class="table table-four"><div class="table-head"><span>${_t('Revision')}</span><span>${_t('Branch')}</span><span>${_t('Created')}</span><span>${_t('Status')}</span></div>${gitRows || `<div class="panel-pad empty-state">${_t('No Git snapshots found.')}</div>`}</div>
      </div>
      <div class="panel-pad stack">
        <h3>${_t('Validation')}</h3>
        ${validationSummary}
      </div>
      <div class="panel-pad stack">
        <h3>${_t('Runtime')}</h3>
        <div class="cards">
          <article class="card"><span class="card-label">${_t('Uptime')}</span><strong>${escapeHTML(String(runtime.uptime_seconds || 0))}s</strong><span class="card-copy">${_t('Current process uptime.')}</span></article>
          <article class="card"><span class="card-label">${_t('Goroutines')}</span><strong>${escapeHTML(String(runtime.goroutines || 0))}</strong><span class="card-copy">${_t('Live goroutines.')}</span></article>
          <article class="card"><span class="card-label">${_t('Heap')}</span><strong>${escapeHTML(String(runtime.heap_alloc_bytes || 0))}</strong><span class="card-copy">${_t('Heap allocation bytes.')}</span></article>
          <article class="card"><span class="card-label">${_t('Last Build')}</span><strong>${escapeHTML(runtime.last_build?.generated_at || 'n/a')}</strong><span class="card-copy">${_t('Latest persisted build report.')}</span></article>
        </div>
      </div>
      <div class="panel-pad stack">
        <h3>${_t('Logs')}</h3>
        ${logContent}
      </div>`,
      _t('Updates, backups, logs, cache, rebuilds, and runtime health')
    );
  };

  return {
    renderExtensions,
    renderPlugins,
    renderThemes,
    renderOperations,
  };
};
