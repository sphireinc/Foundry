export const createAdminCustomFieldsAPI = (http) => ({
  get() {
    return http.get('/api/custom-fields');
  },
  save(payload) {
    return http.post('/api/custom-fields/save', payload || {});
  },
});
