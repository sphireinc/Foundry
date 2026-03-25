// normalizeMediaRef converts Foundry media: references into public URLs.
const normalizeMediaRef = (value) => {
  const raw = String(value || '').trim();
  if (!raw) return '';
  if (raw.startsWith('media:images/')) return `/images/${raw.slice('media:images/'.length)}`;
  if (raw.startsWith('media:uploads/')) return `/uploads/${raw.slice('media:uploads/'.length)}`;
  if (raw.startsWith('media:assets/')) return `/assets/${raw.slice('media:assets/'.length)}`;
  return raw;
};

// createFrontendMediaAPI provides media URL resolution helpers for themes.
export const createFrontendMediaAPI = () => ({
  url(asset) {
    if (!asset) return '';
    if (typeof asset === 'string') return normalizeMediaRef(asset);
    return normalizeMediaRef(asset.reference || asset.url || asset.path || '');
  },
});
