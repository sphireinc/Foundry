export const createExtensionsRuntime = ({
  admin,
  adminBase,
  state,
  documentRef = document,
  windowRef = window,
  clone,
  escapeHTML,
  capabilitySet,
  extensionPageBySection,
  extensionWidgetsForSlot,
  extensionMountID,
  recordDebugEvent,
}) => {
  const extensionModuleCache = new Map();
  const extensionStyleCache = new Set();

  const publishAdminRuntime = () => {
    windowRef.FoundryAdmin = {
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
      documentRef.dispatchEvent(
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
      const link = documentRef.createElement('link');
      link.rel = 'stylesheet';
      link.href = url;
      link.dataset.foundryAdminExtensionStyle = url;
      documentRef.head.appendChild(link);
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
    const mount = documentRef.getElementById('admin-extension-mount');
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
        const mount = documentRef.getElementById(mountId);
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
          documentRef.dispatchEvent(
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

  return {
    publishAdminRuntime,
    mountActiveExtensionPage,
    mountVisibleExtensionWidgets,
  };
};
