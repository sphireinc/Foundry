import { createFrontendClient } from '/__foundry/sdk/frontend/index.js';

const client = createFrontendClient({ mode: 'auto' });

const escapeHTML = (value) =>
  String(value ?? '')
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&#39;');

const slugify = (value) =>
  String(value || '')
    .toLowerCase()
    .replace(/[^a-z0-9\s-]/g, '')
    .trim()
    .replace(/\s+/g, '-');

const searchResultMarkup = (item) => `
  <article class="aq-search-result aq-surface">
    <div class="aq-card-meta">
      <span>${escapeHTML(item.type || '')}</span>
      <span>${escapeHTML(item.lang || '')}</span>
      ${item.layout ? `<span>${escapeHTML(item.layout)}</span>` : ''}
    </div>
    <h3><a href="${escapeHTML(item.url)}">${escapeHTML(item.title)}</a></h3>
    ${item.snippet || item.summary ? `<p>${escapeHTML(item.snippet || item.summary || '')}</p>` : ''}
  </article>
`;

const wireSearch = (state) => {
  const renderResults = async (query, target) => {
    if (!target) return;
    if (!query) {
      target.innerHTML = '';
      return;
    }

    target.innerHTML = `
      <div class="aq-search-loading">
        <span class="aq-loading-dot"></span>
        <span>Searching ${escapeHTML(state.site?.title || 'site content')}...</span>
      </div>
    `;

    try {
      const response = await client.search.query(query);
      const items = Array.isArray(response?.items) ? response.items : [];
      if (items.length === 0) {
        target.innerHTML = `
          <div class="aq-empty-state">
            <h3>No results</h3>
            <p>Nothing matched “${escapeHTML(query)}”.</p>
          </div>
        `;
        return;
      }

      target.innerHTML = `
        <div class="aq-search-result-head">
          <strong>${items.length}</strong> result${items.length === 1 ? '' : 's'} for
          <span>“${escapeHTML(query)}”</span>
        </div>
        <div class="aq-search-grid">
          ${items.slice(0, 8).map(searchResultMarkup).join('')}
        </div>
      `;
    } catch (error) {
      target.innerHTML = `
        <div class="aq-empty-state">
          <h3>Search unavailable</h3>
          <p>${escapeHTML(error?.message || 'Search request failed.')}</p>
        </div>
      `;
    }
  };

  document.querySelectorAll('[data-aq-search-form]').forEach((form) => {
    form.addEventListener('submit', async (event) => {
      event.preventDefault();
      const query = String(form.querySelector('input[name="q"]')?.value || '').trim();
      const targetID = form.dataset.resultTarget;
      const target = targetID ? document.getElementById(targetID) : null;
      await renderResults(query, target);
    });
  });

  document.querySelectorAll('[data-aq-search-open]').forEach((button) => {
    button.addEventListener('click', () => {
      document.querySelector('.aq-inline-search input[name="q"]')?.focus();
    });
  });
};

const buildTOC = () => {
  const prose = document.querySelector('[data-aq-article-prose]');
  const toc = document.querySelector('[data-aq-toc]');
  const panel = document.querySelector('[data-aq-toc-panel]');
  if (!prose || !toc || !panel) return;

  const headings = Array.from(prose.querySelectorAll('h2, h3')).filter(
    (heading) => heading.textContent?.trim()
  );
  if (!headings.length) {
    return;
  }

  headings.forEach((heading, index) => {
    if (!heading.id) {
      heading.id = `${slugify(heading.textContent || `section-${index + 1}`)}-${index + 1}`;
    }
  });

  toc.innerHTML = headings
    .map(
      (heading) => `
        <a class="aq-toc-link aq-toc-link-${heading.tagName.toLowerCase()}" href="#${escapeHTML(heading.id)}">
          ${escapeHTML(heading.textContent || '')}
        </a>
      `
    )
    .join('');

  panel.hidden = false;
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
  document.documentElement.dataset.foundryTheme = 'aqua-graph';
  wireSearch(state);
  buildTOC();
  document.dispatchEvent(new CustomEvent('foundry:ready', { detail: state }));
};

void bootstrap();
