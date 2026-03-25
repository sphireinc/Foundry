// createAdminAuditAPI exposes read access to the admin audit log.
export const createAdminAuditAPI = (http) => ({
  list(params = {}) {
    return http.get('/api/audit', { query: params });
  },
});
