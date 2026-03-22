import { createCapabilitySet } from '../core/capabilities.js';

export const createFrontendCapabilitiesAPI = (transport) => ({
  async get() {
    const payload = await transport.getStaticOrAPI('/capabilities', '/capabilities.json');
    return createCapabilitySet(payload);
  },
});
