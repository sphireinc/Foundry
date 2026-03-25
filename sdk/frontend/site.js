// createFrontendSiteAPI wraps site metadata access for frontend themes.
export const createFrontendSiteAPI = (transport) => ({
  getInfo() {
    return transport.getStaticOrAPI('/site', '/site.json');
  },
});
