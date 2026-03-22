export const createFrontendContentAPI = (transport, routesAPI) => ({
  async getCurrent() {
    return this.getByPath(transport.currentPath());
  },
  async getByPath(path) {
    const resolved = await routesAPI.resolve(path);
    if (!resolved || !resolved.content_id) return null;
    return this.getByID(resolved.content_id);
  },
  async getByID(id) {
    if (transport.mode === 'static') {
      return transport.loadStaticJSON(`/content/${encodeURIComponent(id)}.json`);
    }
    return transport.http.get('/content', { query: { id } });
  },
});
