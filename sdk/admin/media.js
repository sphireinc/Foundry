export const createAdminMediaAPI = (http) => ({
  list(params = {}) {
    return http.get('/api/media', { query: params });
  },
  getDetail(reference) {
    return http.get('/api/media/detail', { query: { reference } });
  },
  history(path) {
    return http.get('/api/media/history', { query: { path } });
  },
  trash() {
    return http.get('/api/media/trash');
  },
  upload(formData) {
    return http.post('/api/media/upload', formData);
  },
  replace(formData) {
    return http.post('/api/media/replace', formData);
  },
  updateMetadata(input) {
    return http.post('/api/media/metadata', input);
  },
  delete(input) {
    return http.post('/api/media/delete', input);
  },
  restore(input) {
    return http.post('/api/media/restore', input);
  },
  purge(input) {
    return http.post('/api/media/purge', input);
  },
});
