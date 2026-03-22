export const createCapabilitySet = (payload = {}) => {
  const rawCapabilities = Array.isArray(payload.capabilities) ? payload.capabilities : [];
  const capabilitySet = new Set(
    rawCapabilities.map((entry) => String(entry).trim()).filter(Boolean)
  );
  const features = { ...(payload.features || {}) };
  const modules = { ...(payload.modules || {}) };

  return {
    raw: payload,
    list() {
      return Array.from(capabilitySet.values()).sort();
    },
    has(capability) {
      const value = String(capability || '').trim();
      return capabilitySet.has('*') || capabilitySet.has(value);
    },
    any(values = []) {
      return values.some((value) => this.has(value));
    },
    all(values = []) {
      return values.every((value) => this.has(value));
    },
    feature(name) {
      return Boolean(features[name]);
    },
    module(name) {
      return Boolean(modules[name]);
    },
    features,
    modules,
  };
};
