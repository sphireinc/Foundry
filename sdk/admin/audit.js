export const createAdminAuditAPI = (http) => ({
  list(params = {}) {
    return http.get('/api/audit', { query: params });
  },
});
