import { createFrontendClient } from '/__foundry/sdk/frontend/index.js';

const client = createFrontendClient({ mode: 'auto' });

const escapeHTML = (value) =>
  String(value ?? '')
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&#39;');

const searchResultMarkup = (entry) => `
  <article class="search-result-card surface">
    <div class="content-meta">
      <span>${escapeHTML(entry.type || '')}</span>
      <span>${escapeHTML(entry.lang || '')}</span>
      ${entry.layout ? `<span>${escapeHTML(entry.layout)}</span>` : ''}
    </div>
    <h3><a href="${escapeHTML(entry.url)}">${escapeHTML(entry.title)}</a></h3>
    ${
      entry.snippet || entry.summary
        ? `<p>${escapeHTML(entry.snippet || entry.summary || '')}</p>`
        : ''
    }
  </article>
`;

const wireSearch = (state) => {
  const drawer = document.getElementById('ledger-search-drawer');
  const openers = document.querySelectorAll('[data-ledger-search-open]');
  const closers = document.querySelectorAll('[data-ledger-search-close]');

  const setDrawer = (open) => {
    if (!drawer) return;
    drawer.hidden = !open;
    if (open) {
      drawer.querySelector('input[name="q"]')?.focus();
    }
  };

  openers.forEach((button) =>
    button.addEventListener('click', () => {
      setDrawer(true);
    })
  );

  closers.forEach((button) =>
    button.addEventListener('click', () => {
      setDrawer(false);
    })
  );

  document.addEventListener('keydown', (event) => {
    if (event.key === 'Escape') {
      setDrawer(false);
    }
    if (event.key === '/' && !event.metaKey && !event.ctrlKey && !event.altKey) {
      const tagName = event.target?.tagName?.toLowerCase();
      if (tagName !== 'input' && tagName !== 'textarea') {
        event.preventDefault();
        setDrawer(true);
      }
    }
  });

  document.querySelectorAll('[data-ledger-search-form]').forEach((form) => {
    form.addEventListener('submit', async (event) => {
      event.preventDefault();
      const targetID = form.dataset.resultTarget;
      const resultEl = targetID ? document.getElementById(targetID) : null;
      const query = String(form.querySelector('input[name="q"]')?.value || '').trim();
      if (!resultEl) return;

      if (!query) {
        resultEl.innerHTML = `
          <div class="empty-state surface">
            <h3>Start with a keyword</h3>
            <p>Search by title, tag, summary, or topic.</p>
          </div>
        `;
        return;
      }

      resultEl.innerHTML = `
        <div class="search-loading">
          <span class="loading-dot"></span>
          <span>Searching ${escapeHTML(state.site?.title || 'site content')}...</span>
        </div>
      `;

      try {
        const response = await client.search.query(query);
        const items = Array.isArray(response?.items) ? response.items : [];
        if (items.length === 0) {
          resultEl.innerHTML = `
            <div class="empty-state surface">
              <h3>No results</h3>
              <p>Nothing matched “${escapeHTML(query)}”. Try a broader search.</p>
            </div>
          `;
          return;
        }

        resultEl.innerHTML = `
          <div class="search-results-head">
            <strong>${items.length}</strong> result${items.length === 1 ? '' : 's'} for
            <span>“${escapeHTML(query)}”</span>
          </div>
          <div class="search-results-grid">
            ${items.slice(0, 8).map(searchResultMarkup).join('')}
          </div>
        `;
      } catch (error) {
        resultEl.innerHTML = `
          <div class="empty-state surface">
            <h3>Search unavailable</h3>
            <p>${escapeHTML(error?.message || 'Search request failed.')}</p>
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
  document.documentElement.dataset.foundryTheme = 'carbon-ledger';
  wireSearch(state);
  document.dispatchEvent(new CustomEvent('foundry:ready', { detail: state }));
};

void bootstrap();
