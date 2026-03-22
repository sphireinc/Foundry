import { createCapabilitySet } from '../core/capabilities.js';

export const createAdminCapabilitiesAPI = (http, sessionStore) => ({
  async get() {
    const payload = await http.get('/api/capabilities');
    return createCapabilitySet(payload);
  },
  fromSession() {
    return createCapabilitySet({
      capabilities: sessionStore.current?.capabilities || [],
    });
  },
});
