import { createFrontendClient } from './client.js';

// A reusable progressive enhancement. Mark a search form with data-foundry-search,
// and include data-search-results and data-search-status containers nearby.
export const mountSearch = (root, { client = createFrontendClient(), debounce = 200 } = {}) => {
  const form = root.querySelector('form');
  const input = form?.querySelector('[name="q"]');
  const results = root.querySelector('[data-search-results]');
  const status = root.querySelector('[data-search-status]');
  if (!form || !input || !results || !status) return;
  let generation = 0;
  let timer;
  const params = new URLSearchParams(window.location.search);
  input.value = params.get('q') || input.value;
  const lang = root.dataset.searchLang || document.documentElement.lang;
  const type = form.querySelector('[name="type"]');
  // Keep URL filters even when a theme's control cannot represent them.
  let selectedType = params.get('type') ?? type?.value ?? '';
  if (type) type.value = selectedType;
  const message = (name, fallback) => root.dataset[name] || fallback;
  const run = async (id) => {
    const query = input.value.trim();
    const queryLabel = root.querySelector('[data-search-query]');
    if (queryLabel) queryLabel.textContent = query;
    const url = new URL(window.location.href);
    if (query) url.searchParams.set('q', query);
    else url.searchParams.delete('q');
    if (selectedType) url.searchParams.set('type', selectedType);
    else url.searchParams.delete('type');
    window.history.replaceState(null, '', url);
    if (!query) {
      results.replaceChildren();
      status.textContent = message('searchPrompt', 'Enter a search term.');
      results.removeAttribute('aria-busy');
      return;
    }
    status.textContent = message('searchLoading', 'Searching…');
    results.setAttribute('aria-busy', 'true');
    try {
      const response = await client.search.query(query, {
        lang,
        type: selectedType,
        limit: params.get('limit') || 20,
      });
      if (id !== generation) return;
      const nodes = response.items.map((item) => {
        const article = document.createElement('article');
        article.className = 'archive-item';
        const heading = document.createElement('h2');
        heading.className = 'archive-item-title';
        const link = document.createElement('a');
        const destination = new URL(item.url, window.location.href);
        // The index is public data, never an HTML or script injection surface.
        if (
          destination.origin !== window.location.origin ||
          !['http:', 'https:'].includes(destination.protocol)
        )
          return article;
        link.href = destination.href;
        link.textContent = item.title;
        heading.append(link);
        const snippet = document.createElement('p');
        snippet.textContent = item.snippet || item.summary || '';
        article.append(heading, snippet);
        return article;
      });
      results.replaceChildren(...nodes);
      status.textContent = response.total
        ? `${response.total} ${message('searchCount', 'results')}`
        : message('searchEmpty', 'No results matched your search.');
    } catch (_error) {
      if (id !== generation) return;
      results.replaceChildren();
      status.textContent = message('searchError', 'Search could not be loaded. Please try again.');
    } finally {
      if (id === generation) results.removeAttribute('aria-busy');
    }
  };
  const schedule = (immediate = false) => {
    const id = ++generation; // Invalidate in-flight results as soon as input changes.
    window.clearTimeout(timer);
    if (immediate) void run(id);
    else timer = window.setTimeout(() => void run(id), debounce);
  };
  form.addEventListener('submit', (event) => {
    event.preventDefault();
    schedule(true);
  });
  input.addEventListener('input', () => schedule());
  type?.addEventListener('change', () => {
    selectedType = type.value;
    schedule(true);
  });
  schedule(true);
};
