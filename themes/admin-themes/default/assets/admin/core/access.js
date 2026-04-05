import { normalizeAdminSection } from './router.js';

export const createAccessHelpers = ({
  state,
  sectionTitles,
  hasCapability,
  capabilityInfoHas,
  debugEnabled,
}) => {
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
      case 'users':
        return 'users.manage';
      case 'custom-fields':
        return 'documents.read';
      case 'diagnostics':
      case 'debug':
        return debugEnabled() ? 'debug.read' : null;
      case 'audit':
        return 'audit.read';
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

  return {
    normalizeNavGroup,
    builtinSectionCapability,
    builtinSectionGroup,
    canAccessBuiltinSection,
    extensionPages,
    extensionWidgetsForSlot,
    extensionPageBySection,
    titleForSection,
    canAccessSection,
    firstAccessibleSection,
  };
};
