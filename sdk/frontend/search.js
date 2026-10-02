// Search uses the same phrase matching, filters and ordering as server search.
// Go uses Unicode simple lowercase mappings (one rune per rune), including
// non-contextual sigma and dotted I. Apply the same mapping in JavaScript.
const lowercase = (text) =>
  Array.from(String(text), (ch) => Array.from(ch.toLowerCase())[0]).join('');
const ordinal = (a, b) => {
  const left = Array.from(a, (ch) => ch.codePointAt(0));
  const right = Array.from(b, (ch) => ch.codePointAt(0));
  for (let i = 0; i < Math.min(left.length, right.length); i++) {
    if (left[i] !== right[i]) return left[i] - right[i];
  }
  return left.length - right.length;
};
const snippet = (item, query) => {
  const summary = String(item.summary || '').trim();
  const body = String(item.content || '').trim();
  const text = summary || body || String(item.title || '').trim();
  const runes = Array.from(text);
  let start = 0;
  if (!summary && body) {
    const lower = lowercase(text);
    const index = lower.indexOf(query);
    if (index >= 0) start = Math.max(0, Array.from(lower.slice(0, index)).length - 60);
  }
  const end = start + 180;
  return (
    (start > 0 ? '...' : '') +
    runes.slice(start, end).join('').trim() +
    (end < runes.length ? '...' : '')
  );
};
export const searchEntries = (items, q, params = {}) => {
  const query = lowercase(String(q || '').trim());
  const requested = /^[-+]?\d+$/.test(String(params.limit)) ? Number(params.limit) : NaN;
  const limit =
    (Number.isInteger(requested) || requested === Infinity) && requested > 0
      ? Math.min(requested, 100)
      : 20;
  const matches =
    query && Array.isArray(items)
      ? items
          .filter(
            (item) =>
              (!params.lang || item.lang === params.lang) &&
              (!params.type || item.type === params.type)
          )
          .map((item) => {
            let score = 0;
            for (const [key, weight] of [
              ['title', 6],
              ['summary', 4],
              ['content', 2],
              ['url', 1],
            ]) {
              if (lowercase(String(item[key] || '')).includes(query)) score += weight;
            }
            return { item, score };
          })
          .filter(({ score }) => score > 0)
          .sort(
            (a, b) =>
              b.score - a.score ||
              ordinal(a.item.title || '', b.item.title || '') ||
              ordinal(a.item.url || '', b.item.url || '')
          )
          .map(({ item }) => ({ ...item, snippet: snippet(item, query) }))
      : [];
  return { query, items: matches.slice(0, limit), total: matches.length, limit };
};
export const createFrontendSearchAPI = (transport) => {
  let index;
  let defaultLanguage;
  const loadIndex = () => {
    if (!index)
      index = transport
        .loadStaticJSON('/search.json')
        .then((items) => {
          if (!Array.isArray(items)) throw new Error('Invalid search index');
          return items;
        })
        .catch((error) => {
          index = undefined;
          throw error;
        });
    return index;
  };
  return {
    async query(q, params = {}) {
      if (!String(q || '').trim()) return searchEntries([], q, params);
      if (transport.mode !== 'static') {
        try {
          return await transport.http.get('/search', { query: { q, ...params } });
        } catch (error) {
          if (!transport.autoFallback) throw error;
          transport.mode = 'static';
        }
      }
      const language =
        params.lang ||
        (await (defaultLanguage ||= transport
          .getStaticOrAPI('/site', '/site.json')
          .then((site) => site.default_lang)
          .catch((error) => {
            defaultLanguage = undefined;
            throw error;
          })));
      return searchEntries(await loadIndex(), q, { ...params, lang: language });
    },
  };
};
