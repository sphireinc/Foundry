export const createAdminDocumentsAPI = (http) => ({
  list(params = {}) {
    return http.get('/api/documents', { query: params });
  },
  get(id, params = {}) {
    return http.get('/api/document', { query: { id, ...params } });
  },
  create(input) {
    return http.post('/api/documents/create', input);
  },
  save(input) {
    return http.post('/api/documents/save', input);
  },
  preview(input) {
    return http.post('/api/documents/preview', input);
  },
  setStatus(input) {
    return http.post('/api/documents/status', input);
  },
  delete(input) {
    return http.post('/api/documents/delete', input);
  },
  history(sourcePath) {
    return http.get('/api/documents/history', { query: { source_path: sourcePath } });
  },
  trash() {
    return http.get('/api/documents/trash');
  },
  restore(input) {
    return http.post('/api/documents/restore', input);
  },
  purge(input) {
    return http.post('/api/documents/purge', input);
  },
  diff(input) {
    return http.post('/api/documents/diff', input);
  },
  lock(input) {
    return http.post('/api/documents/lock', input);
  },
  heartbeat(input) {
    return http.post('/api/documents/lock/heartbeat', input);
  },
  unlock(input) {
    return http.post('/api/documents/unlock', input);
  },
});
