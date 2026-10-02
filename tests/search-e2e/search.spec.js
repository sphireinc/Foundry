const { test, expect } = require('@playwright/test');
const fixture = require('../fixtures/search/contract.json');

test('search preserves an unrepresented type until the visitor changes it', async ({ page }) => {
  await page.goto('/search/?q=alpha&type=product');
  await expect(page.locator('[data-search-status]')).toContainText('No results');
  await expect(page).toHaveURL(/type=product/);
  await page.getByRole('searchbox').fill('guide');
  await page.getByRole('searchbox').press('Enter');
  await expect(page.locator('[data-search-status]')).toContainText('No results');
  await expect(page).toHaveURL(/type=product/);
  await page.locator('[name="type"]').selectOption('page');
  await expect(page.locator('[data-search-results] h2 a')).toHaveText('Alpha guide');
  await expect(page).toHaveURL(/type=page/);
  await page.locator('[name="type"]').selectOption('');
  await expect(page).not.toHaveURL(/type=/);
});

test('search preserves the URL type when the theme has no type control', async ({ page }) => {
  await page.goto('/?q=alpha&type=product');
  await page.evaluate(async () => {
    const { mountSearch } = await import('/__foundry/sdk/frontend/search-ui.js');
    const root = document.createElement('section');
    root.id = 'filter-search';
    root.dataset.searchLang = 'en';
    root.innerHTML =
      '<form><input name="q" type="search"><button>Find</button></form><p data-search-status></p><div data-search-results></div>';
    document.body.append(root);
    mountSearch(root);
  });
  const root = page.locator('#filter-search');
  await expect(root.locator('[data-search-status]')).toContainText('No results');
  await expect(page).toHaveURL(/type=product/);
  await root.getByRole('searchbox').fill('guide');
  await root.getByRole('searchbox').press('Enter');
  await expect(root.locator('[data-search-status]')).toContainText('No results');
  await expect(page).toHaveURL(/type=product/);
});

test('discoverable search supports keyboard submission, ranking and language', async ({ page }) => {
  await page.goto('/');
  await page.getByRole('link', { name: 'Search', exact: true }).click();
  const input = page.getByRole('searchbox');
  await input.fill('alpha');
  await input.press('Enter');
  const links = page.locator('[data-search-results] h2 a');
  await expect(links).toHaveCount(2);
  await expect(links.first()).toHaveText('Alpha guide');
  await expect(page.locator('[data-search-status]')).toContainText('2');
  await input.fill('absent');
  await input.press('Enter');
  await expect(page.locator('[data-search-status]')).toContainText('No results');
  await page.goto('/es/search/?q=caf%C3%A9');
  await expect(page.locator('[data-search-results] h2 a')).toHaveText('Guía de café');
  await expect(page.locator('form')).toHaveAttribute('action', '/es/search/');
  await page.getByRole('searchbox').fill('alpha');
  await page.getByRole('searchbox').press('Enter');
  await expect(page.locator('[data-search-results] h2 a')).toHaveCount(0);
});

test('SDK uses shared ranking and filter fixtures; static index is cached', async ({
  page,
}, testInfo) => {
  await page.goto('/search/');
  const actual = await page.evaluate(async (fixture) => {
    const { searchEntries } = await import('/__foundry/sdk/frontend/search.js');
    return fixture.cases.map((tc) => {
      const result = searchEntries(fixture.entries, tc.q, tc);
      return { urls: result.items.map((item) => item.url), total: result.total };
    });
  }, fixture);
  expect(actual).toEqual(fixture.cases.map(({ urls, total }) => ({ urls, total })));
  if (testInfo.project.name === 'static') {
    let downloads = 0;
    page.on('request', (req) => {
      if (req.url().endsWith('/__foundry/search.json')) downloads++;
    });
    await page.reload();
    const input = page.getByRole('searchbox');
    await input.fill('alpha');
    await input.press('Enter');
    await expect(page.locator('[data-search-results] h2 a')).toHaveCount(2);
    await input.fill('guide');
    await input.press('Enter');
    await expect(page.locator('[data-search-results] h2 a')).toHaveCount(1);
    expect(downloads).toBe(1);
  }
});

test('public indexes exclude unpublished content', async ({ request }) => {
  for (const path of ['/search.json', '/__foundry/search.json']) {
    const response = await request.get(path);
    expect(response.ok()).toBeTruthy();
    const body = await response.text();
    expect(body).not.toContain('Secret');
  }
});

test('loading failures are announced and can be retried', async ({ page }) => {
  await page.route('**/__foundry/api/search?**', (route) => route.abort());
  await page.route('**/__foundry/search.json', (route) =>
    route.fulfill({ status: 503, body: 'Unavailable' })
  );
  await page.goto('/search/?q=alpha');
  await expect(page.locator('[data-search-status]')).toContainText('could not be loaded');
  await page.unroute('**/__foundry/search.json');
  await page.getByRole('searchbox').press('Enter');
  await expect(page.locator('[data-search-results] h2 a')).toHaveCount(2);
});

test('server HTML works without JavaScript', async ({ browser }, testInfo) => {
  test.skip(testInfo.project.name !== 'live', 'Static searching requires JavaScript');
  const context = await browser.newContext({ javaScriptEnabled: false });
  const page = await context.newPage();
  await page.goto(`${testInfo.project.use.baseURL}/search/?q=alpha&type=page&limit=1`);
  await expect(page.locator('[data-search-results] h2 a')).toHaveCount(1);
  await expect(page.locator('[data-search-results] h2 a')).toHaveText('Alpha guide');
  await context.close();
});

test('new queries invalidate delayed results and announce loading', async ({ page }) => {
  await page.goto('/search/');
  await page.evaluate(async () => {
    const { mountSearch } = await import('/__foundry/sdk/frontend/search-ui.js');
    const root = document.createElement('section');
    root.id = 'race-search';
    root.innerHTML =
      '<form><label for="race-q">Race search</label><input id="race-q" name="q" type="search"><button>Find</button></form><p data-search-status role="status" aria-live="polite"></p><div data-search-results></div>';
    document.body.append(root);
    window.searchResolvers = {};
    mountSearch(root, {
      debounce: 1,
      client: {
        search: {
          query: (q) =>
            new Promise((resolve) => {
              window.searchResolvers[q] = resolve;
            }),
        },
      },
    });
  });
  const root = page.locator('#race-search');
  const input = root.getByRole('searchbox');
  await input.fill('old');
  await input.press('Enter');
  await expect(root.locator('[data-search-status]')).toHaveText('Searching…');
  await page.waitForFunction(() => !!window.searchResolvers.old);
  await input.fill('new');
  await input.press('Enter');
  await page.waitForFunction(() => !!window.searchResolvers.new);
  await page.evaluate(() =>
    window.searchResolvers.new({ total: 1, items: [{ title: 'New result', url: '/new/' }] })
  );
  await expect(root.locator('h2 a')).toHaveText('New result');
  await page.evaluate(() =>
    window.searchResolvers.old({ total: 1, items: [{ title: 'Old result', url: '/old/' }] })
  );
  await expect(root.locator('h2 a')).toHaveText('New result');
});

test('SDK default language, filters and limits agree across deployments', async ({ page }) => {
  await page.goto('/search/');
  const results = await page.evaluate(async () => {
    const { createFrontendClient } = await import('/__foundry/sdk/frontend/client.js');
    const client = createFrontendClient();
    return {
      english: await client.search.query('alpha', { limit: 1, type: 'page' }),
      spanish: await client.search.query('café', { lang: 'es' }),
      empty: await client.search.query(''),
      invalid: await client.search.query('alpha', { limit: '1e2' }),
    };
  });
  expect(results.english.total).toBe(2);
  expect(results.english.items).toHaveLength(1);
  expect(results.spanish.items.map((item) => item.title)).toEqual(['Guía de café']);
  expect(results.empty.items).toEqual([]);
  expect(results.empty.total).toBe(0);
  expect(results.invalid.limit).toBe(20);
});

test('search entry point is usable on mobile', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto('/');
  await page.getByRole('link', { name: 'Search', exact: true }).click();
  await page.getByRole('searchbox').fill('alpha');
  await page.getByRole('button', { name: 'Search', exact: true }).click();
  await expect(page.locator('[data-search-results] h2 a')).toHaveCount(2);
  expect(
    await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)
  ).toBeTruthy();
});
