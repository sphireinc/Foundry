// createAdminSettingsAPI wraps configuration and settings-section endpoints.
export const createAdminSettingsAPI = (http) => ({
  getSections() {
    return http.get('/api/settings/sections');
  },
  getForm() {
    return http.get('/api/settings/form');
  },
  saveForm(input) {
    return http.post('/api/settings/form/save', input);
  },
  getCustomCSS() {
    return http.get('/api/settings/custom-css');
  },
  saveCustomCSS(input) {
    return http.post('/api/settings/custom-css/save', input);
  },
  getConfig() {
    return http.get('/api/config');
  },
  saveConfig(input) {
    return http.post('/api/config/save', input);
  },
});
