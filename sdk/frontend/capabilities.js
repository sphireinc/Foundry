import { createCapabilitySet } from '../core/capabilities.js';

// createFrontendCapabilitiesAPI exposes platform capability discovery for
// frontend clients.
export const createFrontendCapabilitiesAPI = (transport) => ({
  async get() {
    const payload = await transport.getStaticOrAPI('/capabilities', '/capabilities.json');
    return createCapabilitySet(payload);
  },
});
