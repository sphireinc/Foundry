export const normalizeAdminSection = (section) => {
  const value = String(section || 'overview')
    .trim()
    .replace(/^\/+|\/+$/g, '');
  return value || 'overview';
};

export const adminPathForSection = (adminBase, section) => {
  const normalized = normalizeAdminSection(section);
  return normalized === 'overview' ? adminBase : `${adminBase}/${normalized}`;
};

export const createSectionForPath = (adminBase) => {
  const adminBaseEscaped = adminBase.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  const adminBasePattern = new RegExp(`^${adminBaseEscaped}/?`);
  return (pathname) => {
    const path = pathname.replace(/\/+$/, '');
    if (path === adminBase || path === '') return 'overview';
    return normalizeAdminSection(path.replace(adminBasePattern, '') || 'overview');
  };
};

export const createAdminRouter = ({ adminBase, getState, confirmNavigation, render }) => ({
  navigate(section) {
    const state = getState();
    const nextSection = normalizeAdminSection(section);
    if (nextSection !== state.section && !confirmNavigation()) {
      return;
    }
    state.section = nextSection;
    const nextPath = adminPathForSection(adminBase, nextSection);
    if (window.location.pathname !== nextPath) {
      window.history.pushState({}, '', nextPath);
    }
    render();
  },
});
