export const createFrontendRoutesAPI = (transport) => ({
  async current() {
    return this.resolve(transport.currentPath());
  },
  async resolve(path) {
    if (transport.mode === 'static') {
      const routes = await transport.loadStaticJSON('/routes.json');
      const normalized = transport.normalizePath(path);
      return Array.isArray(routes)
        ? routes.find((entry) => entry.url === normalized) || null
        : null;
    }
    return transport.http.get('/routes/resolve', { query: { path } });
  },
});
