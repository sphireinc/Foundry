// createAdminUsersAPI wraps user-management endpoints for admin clients.
export const createAdminUsersAPI = (http) => ({
  list() {
    return http.get('/api/users');
  },
  save(input) {
    return http.post('/api/users/save', input);
  },
  delete(input) {
    return http.post('/api/users/delete', input);
  },
});
