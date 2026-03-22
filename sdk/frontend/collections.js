const applyQuery = (items, params = {}) => {
  let next = [...items];
  if (params.type) next = next.filter((item) => item.type === params.type);
  if (params.lang) next = next.filter((item) => item.lang === params.lang);
  if (params.taxonomy && params.term) {
    next = next.filter(
      (item) =>
        Array.isArray(item.taxonomies?.[params.taxonomy]) &&
        item.taxonomies[params.taxonomy].includes(params.term)
    );
  }
  if (params.q) {
    const q = String(params.q).trim().toLowerCase();
    next = next.filter((item) =>
      [item.title, item.slug, item.summary, item.url].join(' ').toLowerCase().includes(q)
    );
  }
  const pageSize = Number(params.page_size || 20);
  const page = Math.max(1, Number(params.page || 1));
  const total = next.length;
  const start = (page - 1) * pageSize;
  return {
    items: next.slice(start, start + pageSize),
    page,
    page_size: pageSize,
    total,
  };
};

export const createFrontendCollectionsAPI = (transport) => ({
  async list(type, params = {}) {
    if (transport.mode === 'static') {
      const payload = await transport.loadStaticJSON('/collections.json');
      return applyQuery(Array.isArray(payload?.items) ? payload.items : [], {
        ...params,
        type: type || params.type,
      });
    }
    return transport.http.get('/collections', { query: { ...params, type: type || params.type } });
  },
});
