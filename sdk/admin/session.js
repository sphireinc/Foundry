export const createAdminSessionAPI = (http, sessionStore) => ({
  async get() {
    const payload = await http.get('/api/session');
    sessionStore.current = payload;
    return payload;
  },
  async login(input) {
    const payload = await http.post('/api/login', input);
    sessionStore.current = payload;
    return payload;
  },
  async logout() {
    const payload = await http.post('/api/logout', {});
    sessionStore.current = null;
    return payload;
  },
  async revoke(input) {
    return http.post('/api/sessions/revoke', input);
  },
  async startPasswordReset(input) {
    return http.post('/api/password-reset/start', input);
  },
  async completePasswordReset(input) {
    return http.post('/api/password-reset/complete', input);
  },
  async setupTOTP(input = {}) {
    return http.post('/api/totp/setup', input);
  },
  async enableTOTP(input) {
    return http.post('/api/totp/enable', input);
  },
  async disableTOTP(input = {}) {
    return http.post('/api/totp/disable', input);
  },
  getCached() {
    return sessionStore.current;
  },
});
