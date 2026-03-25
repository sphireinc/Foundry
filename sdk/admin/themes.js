// createAdminThemesAPI wraps theme listing, validation, and activation flows.
export const createAdminThemesAPI = (http) => ({
  list() {
    return http.get('/api/themes');
  },
  validate(input) {
    return http.post('/api/themes/validate', input);
  },
  switchFrontend(name) {
    return http.post('/api/themes/switch', { name, kind: 'frontend' });
  },
  switchAdmin(name) {
    return http.post('/api/themes/switch', { name, kind: 'admin' });
  },
});
