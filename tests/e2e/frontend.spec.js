const { test, expect } = require('@playwright/test');

const ADMIN_USERNAME = process.env.FOUNDRY_E2E_ADMIN_USER || 'admin';
const ADMIN_PASSWORD = process.env.FOUNDRY_E2E_ADMIN_PASS || 'admin';

async function ensureFrontendTheme(page, name = 'default') {
  await page.goto('/__admin');
  const usernameField = page.getByLabel(/Username/i);
  if (await usernameField.count()) {
    await expect(page.getByRole('heading', { name: /Foundry Admin/i })).toBeVisible();
    await usernameField.fill(ADMIN_USERNAME);
    await page.getByLabel(/Password/i).fill(ADMIN_PASSWORD);
    await page.getByRole('button', { name: /Log In/i }).click();
    await expect(page.getByRole('heading', { name: /Overview|Documents|Editor/i })).toBeVisible();
  }

  const result = await page.evaluate(async (targetName) => {
    const sessionResp = await fetch('/__admin/api/session', { credentials: 'same-origin' });
    if (!sessionResp.ok) {
      return { ok: false, body: await sessionResp.text() };
    }
    const session = await sessionResp.json();
    const themesResp = await fetch('/__admin/api/themes', { credentials: 'same-origin' });
    if (!themesResp.ok) {
      return { ok: false, body: await themesResp.text() };
    }
    const themes = await themesResp.json();
    const current = (themes || []).find((theme) => theme.kind === 'frontend' && theme.current)?.name || '';
    if (current === targetName) {
      return { ok: true };
    }
    const switchResp = await fetch('/__admin/api/themes/switch', {
      method: 'POST',
      credentials: 'same-origin',
      headers: {
        'Content-Type': 'application/json',
        'X-Foundry-CSRF-Token': session.csrf_token || '',
      },
      body: JSON.stringify({ name: targetName, kind: 'frontend' }),
    });
    return { ok: switchResp.ok, body: await switchResp.text() };
  }, name);

  expect(result.ok, result.body || `failed to switch theme to ${name}`).toBeTruthy();
}

test.describe('default frontend theme', () => {
  test('homepage renders the default theme shell', async ({ page }) => {
    await ensureFrontendTheme(page, 'default');
    await page.goto('/');

    await expect(page).toHaveTitle(/Home|Foundry CMS/i);
    await expect(page.locator('.page-header h1')).toHaveText(/^Home$/i);
    await expect(page.getByRole('link', { name: /About/i })).toBeVisible();
    await expect(page.locator('main')).toContainText(/Welcome|English homepage content/i);
  });

  test('about page renders published page content', async ({ page }) => {
    await ensureFrontendTheme(page, 'default');
    await page.goto('/about/');

    await expect(page.locator('.page-header h1')).toHaveText(/^About$/i);
    await expect(page.locator('main')).toContainText(
      /This CMS is built in Go with Markdown content and pluggable hooks/i
    );
  });

  test('post page renders plugin-driven sidebar metadata', async ({ page }) => {
    await ensureFrontendTheme(page, 'default');
    await page.goto('/posts/hello-world/');

    await expect(page.locator('.post-header h1')).toHaveText(/^Build with confidence$/i);
    await expect(page.locator('.meta-panel')).toContainText(/Overview/i);
    await expect(page.locator('.meta-panel')).toContainText(/On this page/i);
    await expect(page.locator('.meta-panel')).toContainText(/go/i);
    await expect(page.locator('.meta-panel')).toContainText(/engineering/i);
    await expect(page.locator('.meta-panel').getByRole('link', { name: /Why this theme exists/i })).toBeVisible();
  });
});
