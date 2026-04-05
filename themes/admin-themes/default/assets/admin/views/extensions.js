export const createExtensionViews = ({
  state,
  panel,
  escapeHTML,
  hasCapability,
  normalizeAdminSection,
  extensionPageBySection,
  extensionWidgetsForSlot,
}) => {
  const extensionMountID = (kind, plugin, key, slot = '') =>
    `${kind}-${plugin}-${key}-${slot}`.replace(/[^a-zA-Z0-9_-]+/g, '-');

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

  const renderExtensionPage = (section) => {
    const page = extensionPageBySection(section);
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
             data-extension-section="${escapeHTML(normalizeAdminSection(section))}">
          <div class="empty-state">This page is ready for a plugin-provided admin UI mount. Listen for the <code>foundry:admin-extension-page</code> event or read <code>window.FoundryAdmin</code>.</div>
        </div>
      </div>`,
      page.description || 'Plugin-defined admin page'
    );
  };

  return {
    extensionMountID,
    renderExtensionPage,
    renderWidgetPanels,
  };
};
