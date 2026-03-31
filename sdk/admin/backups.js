export const createAdminBackupsAPI = (http) => ({
  list() {
    return http.get('/api/backups');
  },
  create(input = {}) {
    return http.post('/api/backups/create', input);
  },
  restore(input) {
    return http.post('/api/backups/restore', input);
  },
  downloadURL(name) {
    return `${http.baseURL}/api/backups/download?name=${encodeURIComponent(name)}`;
  },
});
