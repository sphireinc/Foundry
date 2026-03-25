// createFrontendNavigationAPI wraps frontend navigation/menu lookups.
export const createFrontendNavigationAPI = (transport) => ({
  async get(name = 'main') {
    const payload = await transport.getStaticOrAPI('/navigation', '/navigation.json');
    if (Array.isArray(payload)) return payload;
    return payload?.[name] || [];
  },
  async listAll() {
    return transport.getStaticOrAPI('/navigation', '/navigation.json');
  },
});
