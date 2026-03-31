export const createAdminOperationsAPI = (http) => ({
  get() {
    return http.get('/api/operations');
  },
  logs() {
    return http.get('/api/operations/logs');
  },
  validate() {
    return http.post('/api/operations/validate', {});
  },
  clearCache() {
    return http.post('/api/operations/cache/clear', {});
  },
  rebuild() {
    return http.post('/api/operations/rebuild', {});
  },
});
