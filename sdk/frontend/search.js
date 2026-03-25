// createFrontendSearchAPI wraps live and static search queries.
export const createFrontendSearchAPI = (transport) => ({
  async query(q, params = {}) {
    if (transport.mode === 'static') {
      const items = await transport.loadStaticJSON('/search.json');
      const query = String(q || '')
        .trim()
        .toLowerCase();
      const filtered = Array.isArray(items)
        ? items
            .map((item) => {
              const title = String(item.title || '').toLowerCase();
              const summary = String(item.summary || '').toLowerCase();
              const content = String(item.content || '').toLowerCase();
              const url = String(item.url || '').toLowerCase();
              let score = 0;
              if (title.includes(query)) score += 6;
              if (summary.includes(query)) score += 4;
              if (content.includes(query)) score += 2;
              if (url.includes(query)) score += 1;
              return { item, score };
            })
            .filter((entry) => query === '' || entry.score > 0)
            .sort(
              (left, right) =>
                right.score - left.score ||
                String(left.item.title || '').localeCompare(String(right.item.title || ''))
            )
            .map((entry) => entry.item)
        : [];
      return { query, items: filtered };
    }
    return transport.http.get('/search', { query: { q, ...params } });
  },
});
