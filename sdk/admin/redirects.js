// createAdminRedirectsAPI wraps first-class redirect management endpoints.
export const createAdminRedirectsAPI = (http) => ({
  list() {
    return http.get('/api/redirects');
  },
  save(input) {
    return http.post('/api/redirects/save', input);
  },
});
