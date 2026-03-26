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

const renderSearchResult = (item) => `
  <article class="lf-search-result">
    <div class="lf-meta-row">
      <span>${escapeHTML(item.type || '')}</span>
      <span>${escapeHTML(item.lang || '')}</span>
    </div>
    <h3><a href="${escapeHTML(item.url)}">${escapeHTML(item.title)}</a></h3>
    ${item.snippet || item.summary ? `<p>${escapeHTML(item.snippet || item.summary || '')}</p>` : ''}
  </article>
`;

const wireSearch = () => {
  document.querySelectorAll('[data-lf-search-form]').forEach((form) => {
    form.addEventListener('submit', async (event) => {
      event.preventDefault();
      const query = String(form.querySelector('input[name="q"]')?.value || '').trim();
      const target = document.getElementById(form.dataset.resultTarget || '');
      if (!target) return;

      if (!query) {
        target.innerHTML = '';
        return;
      }

      target.innerHTML = `<div class="lf-search-result"><p>Searching for “${escapeHTML(query)}”…</p></div>`;

      try {
        const response = await client.search.query(query);
        const items = Array.isArray(response?.items) ? response.items : [];
        if (!items.length) {
          target.innerHTML = `<div class="lf-search-result"><p>No results matched “${escapeHTML(query)}”.</p></div>`;
          return;
        }

        target.innerHTML = items.slice(0, 6).map(renderSearchResult).join('');
      } catch (error) {
        target.innerHTML = `<div class="lf-search-result"><p>${escapeHTML(error?.message || 'Search request failed.')}</p></div>`;
      }
    });
  });
};

const buildTOC = () => {
  const prose = document.querySelector('[data-lf-article-prose]');
  const toc = document.querySelector('[data-lf-toc]');
  const panel = document.querySelector('[data-lf-toc-panel]');
  if (!prose || !toc || !panel) return;

  const headings = Array.from(prose.querySelectorAll('h2, h3')).filter((heading) =>
    heading.textContent?.trim()
  );
  if (!headings.length) return;

  headings.forEach((heading, index) => {
    if (!heading.id) {
      heading.id = `${slugify(heading.textContent || `section-${index + 1}`)}-${index + 1}`;
    }
  });

  toc.innerHTML = headings
    .map(
      (heading) => `
        <a class="lf-toc-link lf-toc-link-${heading.tagName.toLowerCase()}" href="#${escapeHTML(heading.id)}">
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
  document.documentElement.dataset.foundryTheme = 'letterform';
  wireSearch();
  buildTOC();
  document.dispatchEvent(new CustomEvent('foundry:ready', { detail: state }));
};

void bootstrap();
