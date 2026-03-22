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
  enable(name) {
    return http.post('/api/plugins/enable', { name });
  },
  disable(name) {
    return http.post('/api/plugins/disable', { name });
  },
});
