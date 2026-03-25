// createAdminSettingsAPI wraps configuration and settings-section endpoints.
export const createAdminSettingsAPI = (http) => ({
  getSections() {
    return http.get('/api/settings/sections');
  },
  getConfig() {
    return http.get('/api/config');
  },
  saveConfig(input) {
    return http.post('/api/config/save', input);
  },
});
