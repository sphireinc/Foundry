import { createFrontendClient } from '/__foundry/sdk/frontend/index.js';

const client = createFrontendClient({ mode: 'auto' });

const escapeHTML = (value) =>
  String(value ?? '')
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&#39;');

const searchResultMarkup = (entry) => {
  const summary = entry.snippet || entry.summary || '';
  const meta = [entry.type, entry.lang, entry.layout].filter(Boolean).join(' · ');
  return `
    <article class="search-result-card">
      <div class="search-result-meta">${escapeHTML(meta)}</div>
      <h3><a href="${escapeHTML(entry.url)}">${escapeHTML(entry.title)}</a></h3>
      ${summary ? `<p>${escapeHTML(summary)}</p>` : ''}
    </article>
  `;
};

const attachSearch = (state) => {
  const panel = document.getElementById('forge-search-panel');
  const toggle = document.querySelector('[data-foundry-search-toggle]');

  const setPanelOpen = (open) => {
    if (!panel || !toggle) return;
    panel.hidden = !open;
    toggle.setAttribute('aria-expanded', open ? 'true' : 'false');
  };

  toggle?.addEventListener('click', () => {
    const next = panel?.hidden !== false;
    setPanelOpen(next);
    if (next) {
      panel?.querySelector('input[type="search"]')?.focus();
    }
  });

  document.addEventListener('keydown', (event) => {
    if (event.key === '/' && !event.metaKey && !event.ctrlKey && !event.altKey) {
      const target = event.target;
      const tagName = target?.tagName?.toLowerCase();
      if (tagName !== 'input' && tagName !== 'textarea') {
        event.preventDefault();
        setPanelOpen(true);
        panel?.querySelector('input[type="search"]')?.focus();
      }
    }
    if (event.key === 'Escape') {
      setPanelOpen(false);
    }
  });

  document.querySelectorAll('[data-foundry-search-form]').forEach((form) => {
    form.addEventListener('submit', async (event) => {
      event.preventDefault();
      const input = form.querySelector('input[name="q"]');
      const query = String(input?.value || '').trim();
      const targetID = form.dataset.resultTarget;
      const resultsEl = targetID ? document.getElementById(targetID) : null;
      if (!resultsEl) return;

      if (!query) {
        resultsEl.innerHTML = `
          <div class="search-empty">
            <h3>Start with a keyword</h3>
            <p>Try a title, subject, taxonomy, or phrase.</p>
          </div>
        `;
        return;
      }

      resultsEl.innerHTML = `
        <div class="search-loading">
          <span class="loading-dot"></span>
          <span>Searching ${escapeHTML(state.site?.title || 'the site')}...</span>
        </div>
      `;

      try {
        const response = await client.search.query(query);
        const items = Array.isArray(response?.items) ? response.items : [];
        if (items.length === 0) {
          resultsEl.innerHTML = `
            <div class="search-empty">
              <h3>No results</h3>
              <p>Nothing matched “${escapeHTML(query)}”. Try a broader keyword.</p>
            </div>
          `;
          return;
        }
        resultsEl.innerHTML = `
          <div class="search-results-head">
            <strong>${items.length}</strong> result${items.length === 1 ? '' : 's'} for
            <span>“${escapeHTML(query)}”</span>
          </div>
          <div class="search-results-grid">
            ${items.slice(0, 8).map(searchResultMarkup).join('')}
          </div>
        `;
      } catch (error) {
        resultsEl.innerHTML = `
          <div class="search-empty">
            <h3>Search unavailable</h3>
            <p>${escapeHTML(error?.message || 'The search request failed.')}</p>
          </div>
        `;
      }
    });
  });
};

const bootstrap = async () => {
  const [capabilities, site, current, navigation, route] = await Promise.all([
    client.capabilities.get(),
    client.site.getInfo(),
    client.content.getCurrent().catch(() => null),
    client.navigation.get('main').catch(() => []),
    client.routes.current().catch(() => null),
  ]);

  const state = {
    capabilities: capabilities.raw,
    site,
    current,
    navigation,
    route,
  };

  window.FoundryFrontend = {
    client,
    state,
  };

  document.documentElement.dataset.foundrySdk = 'frontend-v1';
  document.documentElement.dataset.foundryTheme = 'forge-noir';
  attachSearch(state);
  document.dispatchEvent(new CustomEvent('foundry:ready', { detail: state }));
};

void bootstrap();
