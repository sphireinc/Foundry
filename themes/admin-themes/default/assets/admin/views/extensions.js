export const createExtensionViews = ({
  panel,
  escapeHTML,
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
    return panel(
      page.title,
      `
      <div class="panel-pad stack">
        <div class="note">
          <strong>${escapeHTML(page.plugin)}</strong>${page.description ? ` • ${escapeHTML(page.description)}` : ''}
        </div>
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
