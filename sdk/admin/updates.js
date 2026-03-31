export const createAdminUpdatesAPI = (http) => ({
  get() {
    return http.get('/api/update');
  },
  apply() {
    return http.post('/api/update/apply', {});
  },
});
