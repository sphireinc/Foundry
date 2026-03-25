// createFrontendPreviewAPI wraps preview-manifest access for frontend clients.
export const createFrontendPreviewAPI = (transport, capabilitiesAPI) => ({
  async getManifest() {
    const capabilities = await capabilitiesAPI.get();
    if (!capabilities.feature('preview_manifest')) {
      return { generated_at: null, environment: '', target: '', links: [] };
    }
    return transport.getStaticOrAPI('/preview', '/preview.json');
  },
  async listLinks() {
    const manifest = await this.getManifest();
    return Array.isArray(manifest?.links) ? manifest.links : [];
  },
  async getByPath(path) {
    const links = await this.listLinks();
    return links.find((entry) => entry.url === transport.normalizePath(path)) || null;
  },
});
