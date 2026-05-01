const fs = require('fs');
const path = require('path');
const { test, expect } = require('@playwright/test');

const ADMIN_USERNAME = process.env.FOUNDRY_E2E_ADMIN_USER || 'admin';
const ADMIN_PASSWORD = process.env.FOUNDRY_E2E_ADMIN_PASS || 'admin';

test.describe.configure({ mode: 'serial' });

async function login(page, username = ADMIN_USERNAME, password = ADMIN_PASSWORD) {
  await page.goto('/__admin');
  const loginHeading = page.getByRole('heading', { name: /Foundry Admin/i });
  if (await loginHeading.count()) {
    await expect(loginHeading).toBeVisible();
    await page.getByLabel(/Username/i).fill(username);
    await page.getByLabel(/Password/i).fill(password);
    await page.getByRole('button', { name: /Log In/i }).click();
  }
  await expect(page.locator('.foundry-shell')).toBeVisible();
  await expect(page.locator('.foundry-nav')).toBeVisible();
}

async function openDocumentInEditor(page, query, sourcePath) {
  await page.getByRole('link', { name: /^Documents$/i }).click();
  await expect(page.getByRole('heading', { name: /^Find Documents$/i })).toBeVisible();

  await page.getByLabel(/Search Documents/i).fill(query);
  await page.getByRole('button', { name: /^Search$/i }).click();

  const documentRow = page.locator('.table-row', { hasText: sourcePath });
  await expect(documentRow).toHaveCount(1);
  await documentRow.locator('[data-edit-document]').click();
  await expect(page.locator('.breadcrumbs')).toContainText(sourcePath);
  await page.getByRole('button', { name: /^Open Editor$/i }).click();

  await expect(page).toHaveURL(/\/__admin\/editor$/);
  await expect(page.getByRole('heading', { level: 1, name: /^Editor$/i })).toBeVisible();
  await expect(page.locator('#document-source-path')).toHaveValue(sourcePath);
  await expect(page.locator('#document-quill-editor .ql-editor')).toBeVisible();
}

async function addCaptureBanner(page) {
  const capturedAt = new Date().toLocaleString('en-US', {
    month: '2-digit',
    day: '2-digit',
    year: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
    hour12: true,
  });
  const currentURL = page.url();
  await page.evaluate(
    ({ capturedAt: stamp, currentURL: url }) => {
      document.body.style.paddingTop = '40px';
      const existing = document.getElementById('readme-capture-banner');
      if (existing) existing.remove();

      const banner = document.createElement('div');
      banner.id = 'readme-capture-banner';
      banner.textContent = `Captured at: ${stamp}   URL: ${url}`;
      Object.assign(banner.style, {
        position: 'fixed',
        inset: '0 0 auto 0',
        height: '40px',
        background: '#000',
        color: '#fff',
        zIndex: '2147483647',
        display: 'flex',
        alignItems: 'center',
        padding: '0 16px',
        fontFamily: 'monospace',
        fontSize: '18px',
        fontWeight: '700',
        letterSpacing: '0.02em',
      });
      document.body.prepend(banner);
    },
    { capturedAt, currentURL }
  );
}

async function screenshot(page, fileName) {
  await addCaptureBanner(page);
  await page.evaluate(() => {
    document.querySelectorAll('.toast-stack').forEach((node) => node.remove());
  });
  const outputDir = path.resolve(__dirname, '../../readme-assets/admin-screenshots');
  fs.mkdirSync(outputDir, { recursive: true });
  await page.screenshot({
    path: path.join(outputDir, fileName),
    fullPage: true,
    animations: 'disabled',
    caret: 'hide',
  });
}

test('capture updated admin screenshots for the README', async ({ page }) => {
  await login(page);
  await openDocumentInEditor(page, 'hello world', 'content/posts/hello-world.md');
  await page.evaluate(() => {
    if (document.activeElement && typeof document.activeElement.blur === 'function') {
      document.activeElement.blur();
    }
  });
  await screenshot(page, '04_editor.png');

  await page.getByRole('button', { name: /^Zen Mode$/i }).click();
  await expect(page.locator('.zen-overlay')).toBeVisible();
  await expect(page.locator('#zen-preview iframe.preview-frame')).toBeVisible({
    timeout: 10_000,
  });
  await page.evaluate(() => {
    if (document.activeElement && typeof document.activeElement.blur === 'function') {
      document.activeElement.blur();
    }
  });
  await screenshot(page, '15_zen_mode.png');
});
