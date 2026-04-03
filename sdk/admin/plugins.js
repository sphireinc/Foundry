// createAdminPluginsAPI wraps plugin management and validation endpoints.
export const createAdminPluginsAPI = (http) => ({
  list() {
    return http.get('/api/plugins');
  },
  validate(name) {
    return http.post('/api/plugins/validate', { name });
  },
  install(input) {
    return http.post('/api/plugins/install', input);
  },
  update(input) {
    return http.post('/api/plugins/update', input);
  },
  rollback(input) {
    return http.post('/api/plugins/rollback', input);
  },
  enable(input) {
    return http.post('/api/plugins/enable', typeof input === 'string' ? { name: input } : input);
  },
  disable(name) {
    return http.post('/api/plugins/disable', { name });
  },
});
