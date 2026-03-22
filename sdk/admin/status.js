export const createAdminStatusAPI = (http) => ({
  get() {
    return http.get('/api/status');
  },
});
