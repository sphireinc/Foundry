export const createFrontendSiteAPI = (transport) => ({
  getInfo() {
    return transport.getStaticOrAPI('/site', '/site.json');
  },
});
