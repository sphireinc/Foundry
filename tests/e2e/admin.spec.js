const fs = require('fs');
const path = require('path');
const { test, expect } = require('@playwright/test');

const ADMIN_USERNAME = process.env.FOUNDRY_E2E_ADMIN_USER || 'admin';
const ADMIN_PASSWORD = process.env.FOUNDRY_E2E_ADMIN_PASS || 'admin';
const LOGO_PATH = path.resolve(__dirname, '../../readme-assets/logo.png');

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
  await expect(page.getByText(/insufficient admin capabilities/i)).toHaveCount(0);
}

async function logout(page) {
  const logoutButton = page.locator('#logout').first();
  await expect(logoutButton).toBeVisible();
  await logoutButton.click();
  await expect(page.getByRole('heading', { name: /Foundry Admin/i })).toBeVisible();
}

async function adminRequest(page, method, requestPath, payload) {
  return page.evaluate(
    async ({ method: reqMethod, requestPath: reqPath, payload: reqPayload }) => {
      const headers = {};
      if (reqMethod !== 'GET' && reqMethod !== 'HEAD') {
        const sessionResp = await fetch('/__admin/api/session', { credentials: 'same-origin' });
        const session = await sessionResp.json();
        headers['Content-Type'] = 'application/json';
        headers['X-Foundry-CSRF-Token'] = session.csrf_token || '';
      }
      const response = await fetch(`/__admin${reqPath}`, {
        method: reqMethod,
        credentials: 'same-origin',
        headers,
        body:
          reqMethod === 'GET' || reqMethod === 'HEAD' || typeof reqPayload === 'undefined'
            ? undefined
            : JSON.stringify(reqPayload),
      });
      const body = await response.text();
      let json = null;
      try {
        json = body ? JSON.parse(body) : null;
      } catch (_error) {
        json = null;
      }
      return {
        ok: response.ok,
        status: response.status,
        body,
        json,
      };
    },
    { method, requestPath, payload }
  );
}

async function adminGet(page, requestPath) {
  return adminRequest(page, 'GET', requestPath);
}

async function adminPost(page, requestPath, payload) {
  return adminRequest(page, 'POST', requestPath, payload);
}

function expectAdminOK(result, context) {
  expect(result.ok, `${context}: ${result.status} ${result.body}`).toBeTruthy();
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
}

async function openHelloWorldInEditor(page) {
  await openDocumentInEditor(page, 'hello world', 'content/posts/hello-world.md');
}

async function getDocumentViaAdminAPI(page, sourcePath) {
  const result = await adminGet(
    page,
    `/api/document?id=${encodeURIComponent(sourcePath)}&include_drafts=1`
  );
  expectAdminOK(result, `get document ${sourcePath}`);
  return result.json;
}

async function saveDocumentViaAdminAPI(page, sourcePath, raw, versionComment = 'e2e restore') {
  const result = await adminPost(page, '/api/documents/save', {
    source_path: sourcePath,
    raw,
    version_comment: versionComment,
  });
  expectAdminOK(result, `save document ${sourcePath}`);
  return result.json;
}

async function createDocumentViaAdminAPI(page, kind, slug, lang = 'en', archetype = '') {
  const result = await adminPost(page, '/api/documents/create', {
    kind,
    slug,
    lang,
    archetype,
  });
  expectAdminOK(result, `create document ${kind}:${slug}`);
  return result.json;
}

function buildDocumentRaw({ title, slug, layout, body, draft = true, fields = null }) {
  const fieldBlock =
    fields && Object.keys(fields).length
      ? `fields:\n${Object.entries(fields)
          .map(([key, value]) => `  ${key}: ${JSON.stringify(value)}`)
          .join('\n')}\n`
      : '';
  return `---
title: ${title}
slug: ${slug}
layout: ${layout}
draft: ${draft ? 'true' : 'false'}
${fieldBlock}---

${body}
`;
}

async function listThemesViaAdminAPI(page) {
  const result = await adminGet(page, '/api/themes');
  expectAdminOK(result, 'list themes');
  return result.json;
}

async function switchFrontendThemeViaAdminAPI(page, name) {
  const result = await adminPost(page, '/api/themes/switch', { name, kind: 'frontend' });
  expectAdminOK(result, `switch frontend theme to ${name}`);
}

async function ensureFrontendTheme(page, name = 'default') {
  const themes = await listThemesViaAdminAPI(page);
  const current = themes.find((theme) => theme.kind === 'frontend' && theme.current)?.name || '';
  if (current !== name) {
    await switchFrontendThemeViaAdminAPI(page, name);
  }
}

async function createUserViaAdminAPI(page, input) {
  const result = await adminPost(page, '/api/users/save', input);
  expectAdminOK(result, `save user ${input.username}`);
  return result.json;
}

async function deleteUserViaAdminAPI(page, username) {
  const result = await adminPost(page, '/api/users/delete', { username });
  expectAdminOK(result, `delete user ${username}`);
}

async function deleteDocumentViaAdminAPI(page, sourcePath) {
  const result = await adminPost(page, '/api/documents/delete', {
    source_path: sourcePath,
    lock_token: '',
  });
  expectAdminOK(result, `delete document ${sourcePath}`);
}

async function updateDocumentStatusViaAdminAPI(page, sourcePath, status) {
  const result = await adminPost(page, '/api/documents/status', {
    source_path: sourcePath,
    status,
    lock_token: '',
  });
  expectAdminOK(result, `update document status ${sourcePath} -> ${status}`);
  return result.json;
}

async function listDocumentTrashViaAdminAPI(page) {
  const result = await adminGet(page, '/api/documents/trash');
  expectAdminOK(result, 'list document trash');
  return result.json;
}

async function restoreDocumentViaAdminAPI(page, documentPath) {
  const result = await adminPost(page, '/api/documents/restore', { path: documentPath });
  expectAdminOK(result, `restore document ${documentPath}`);
  return result.json;
}

async function purgeDocumentViaAdminAPI(page, documentPath) {
  const result = await adminPost(page, '/api/documents/purge', { path: documentPath });
  expectAdminOK(result, `purge document ${documentPath}`);
}

async function deleteMediaViaAdminAPI(page, reference) {
  const result = await adminPost(page, '/api/media/delete', { reference });
  expectAdminOK(result, `delete media ${reference}`);
}

test.describe('default admin theme', () => {
  test('admin login and shell bootstrap work', async ({ page }) => {
    await login(page);
    await ensureFrontendTheme(page, 'default');

    await expect(page.locator('.foundry-nav')).toContainText(/Overview/i);
    await expect(page.locator('.foundry-nav')).toContainText(/Documents/i);
  });

  test('operations view shows release and runtime controls', async ({ page }) => {
    await login(page);
    await ensureFrontendTheme(page, 'default');

    await page.getByRole('link', { name: /^Operations$/i }).click();

    await expect(page).toHaveURL(/\/__admin\/operations$/);
    await expect(page.getByRole('heading', { level: 2, name: /^Operations$/i })).toBeVisible();
    await expect(page.getByText(/Current Release/i)).toBeVisible();
    await expect(page.getByText(/Latest Release/i)).toBeVisible();
    await expect(page.getByText(/Install Mode/i)).toBeVisible();
    await expect(page.getByRole('button', { name: /Refresh Update Status/i })).toBeVisible();
    await expect(page.getByRole('button', { name: /Apply Update/i })).toBeVisible();
    await expect(page.getByRole('heading', { name: /^Zip Backups$/i })).toBeVisible();
    await expect(page.getByRole('heading', { name: /^Git Snapshots$/i })).toBeVisible();
  });

  test('documents search can open the editor for a specific document', async ({ page }) => {
    await login(page);
    await ensureFrontendTheme(page, 'default');

    await openHelloWorldInEditor(page);

    await expect(page.locator('#document-frontmatter-title')).toBeVisible();
    await expect(page.locator('#document-raw')).toBeVisible();
    await expect(page.locator('#document-frontmatter-title')).toHaveValue(/Hello World/i);
    await expect(page.getByText(/Structured Frontmatter/i)).toBeVisible();
  });

  test('editor save persists changes after reload', async ({ page }) => {
    await login(page);
    await ensureFrontendTheme(page, 'default');

    const slug = `e2e-save-${Date.now()}`;
    const sourcePath = `content/pages/${slug}.md`;
    const initialRaw = buildDocumentRaw({
      title: 'E2E Save Seed',
      slug,
      layout: 'page',
      body: '# E2E Save Seed\n\nInitial body.\n',
      draft: false,
    });
    const updatedTitle = `E2E Save ${Date.now()}`;

    try {
      await createDocumentViaAdminAPI(page, 'page', slug, 'en', 'page');
      await saveDocumentViaAdminAPI(page, sourcePath, initialRaw, 'e2e create save page');
      const updatedRaw = buildDocumentRaw({
        title: updatedTitle,
        slug,
        layout: 'page',
        body: '# E2E Save Seed\n\nInitial body.\n',
        draft: false,
      });
      await saveDocumentViaAdminAPI(page, sourcePath, updatedRaw, 'e2e save persistence');
      await page.goto(`/${slug}/`);
      await expect(page.getByRole('heading', { level: 1, name: updatedTitle })).toBeVisible();
      await page.reload();
      await expect(page.getByRole('heading', { level: 1, name: updatedTitle })).toBeVisible();
    } finally {
      if (page.isClosed()) {
        return;
      }
      const documents = await adminGet(
        page,
        `/api/documents?include_drafts=1&q=${encodeURIComponent(slug)}`
      );
      if (documents.ok && Array.isArray(documents.json) && documents.json.some((doc) => doc.source_path === sourcePath)) {
        await deleteDocumentViaAdminAPI(page, sourcePath);
      }
    }
  });

  test('theme field contracts drive custom fields end to end', async ({ page }) => {
    await login(page);
    await ensureFrontendTheme(page, 'default');

    const slug = `e2e-custom-${Date.now()}`;
    const sourcePath = `content/posts/${slug}.md`;
    const themes = await listThemesViaAdminAPI(page);
    const currentTheme =
      themes.find((theme) => theme.kind === 'frontend' && theme.current)?.name || 'default';
    const heroTitle = `E2E Hero ${Date.now()}`;
    const initialRaw = buildDocumentRaw({
      title: 'E2E Custom Seed',
      slug,
      layout: 'post',
      body: '# E2E Custom Seed\n\nA seeded post for custom fields.\n',
      draft: false,
    });

    try {
      await createDocumentViaAdminAPI(page, 'post', slug, 'en', 'post');
      await saveDocumentViaAdminAPI(page, sourcePath, initialRaw, 'e2e create custom field post');
      await switchFrontendThemeViaAdminAPI(page, 'Foundry-Cloud-Landing');
      await page.reload();

      await openDocumentInEditor(page, slug, sourcePath);
      await expect(page.getByText(/^Custom Fields$/i)).toBeVisible();
      await page.locator('[data-custom-field="hero_title"]').fill(heroTitle);
      await page.locator('#document-version-comment').fill('e2e custom field update');
      await page.getByRole('button', { name: /^Save Document$/i }).click();
      await expect(page.locator('.toast-stack')).toContainText(/Document saved\./i);
      const saved = await getDocumentViaAdminAPI(page, sourcePath);
      expect(saved.raw_body).toContain(heroTitle);

      await expect
        .poll(
          async () => {
            await page.goto(`/posts/${slug}/?e2e=${Date.now()}`);
            return (await page.locator('main h1').first().textContent())?.trim() || '';
          },
          { timeout: 15000 }
        )
        .toBe(heroTitle);
    } finally {
      await login(page);
      const documents = await adminGet(
        page,
        `/api/documents?include_drafts=1&q=${encodeURIComponent(slug)}`
      );
      if (documents.ok && Array.isArray(documents.json) && documents.json.some((doc) => doc.source_path === sourcePath)) {
        await deleteDocumentViaAdminAPI(page, sourcePath);
      }
      await switchFrontendThemeViaAdminAPI(page, currentTheme);
    }
  });

  test('editor preview flow works from the document editor', async ({ page }) => {
    await login(page);
    await ensureFrontendTheme(page, 'default');

    await openHelloWorldInEditor(page);
    await expect(page.locator('#document-raw')).toBeVisible();

    await page.getByRole('button', { name: /^Preview$/i }).click();

    await expect(page).toHaveURL(/\/__admin\/documents$/);
    await expect(page.getByRole('heading', { name: /^Preview$/i })).toBeVisible();
    await expect(page.locator('iframe').first()).toBeVisible();
  });

  test('media upload and metadata save flow work', async ({ page }) => {
    await login(page);
    await ensureFrontendTheme(page, 'default');

    const upload = { reference: '' };

    try {
      await page.getByRole('link', { name: /^Media$/i }).click();
      await expect(page.getByRole('heading', { name: /^Upload Media$/i })).toBeVisible();

      await page.locator('#media-collection').selectOption('images');
      await page.locator('#media-file').setInputFiles({
        name: `e2e-logo-${Date.now()}.png`,
        mimeType: 'image/png',
        buffer: fs.readFileSync(LOGO_PATH),
      });
      await page.getByRole('button', { name: /^Upload Media$/i }).click();

      await expect(page.locator('.toast-stack')).toContainText(/Media uploaded\./i);
      await expect(page.locator('.status-line.mono')).toContainText(/media:/i);
      upload.reference = ((await page.locator('.status-line.mono').first().textContent()) || '').trim();

      await page.locator('#media-title').fill('E2E Uploaded Logo');
      await page.locator('#media-alt').fill('E2E uploaded logo alt text');
      await page.getByRole('button', { name: /^Save Metadata$/i }).click();
      await expect(page.locator('.toast-stack')).toContainText(/Media metadata saved\./i);
      await expect(page.locator('#media-title')).toHaveValue('E2E Uploaded Logo');
    } finally {
      if (upload.reference) {
        await deleteMediaViaAdminAPI(page, upload.reference);
      }
    }
  });

  test('document lifecycle flow covers create, publish, trash, and restore', async ({ page }) => {
    await login(page);
    await ensureFrontendTheme(page, 'default');

    const slug = `e2e-lifecycle-${Date.now()}`;
    const sourcePath = `content/pages/${slug}.md`;

    try {
      const creation = await createDocumentViaAdminAPI(page, 'page', slug, 'en', 'page');
      expect(creation.source_path).toBe(sourcePath);

      const raw = `---
title: E2E Lifecycle Page
slug: ${slug}
layout: page
draft: true
---

# E2E Lifecycle

Created at ${Date.now()}
`;
      await saveDocumentViaAdminAPI(page, sourcePath, raw, 'e2e lifecycle seed');
      const created = await getDocumentViaAdminAPI(page, sourcePath);
      expect(created.raw_body).toContain('E2E Lifecycle Page');

      await updateDocumentStatusViaAdminAPI(page, sourcePath, 'published');
      const published = await getDocumentViaAdminAPI(page, sourcePath);
      expect(String(published.status || '').toLowerCase()).toBe('published');

      await deleteDocumentViaAdminAPI(page, sourcePath);
      const trashEntries = await listDocumentTrashViaAdminAPI(page);
      const trashed = trashEntries.find(
        (entry) => entry.original_path === sourcePath || entry.path === sourcePath
      );
      expect(trashed).toBeTruthy();

      await restoreDocumentViaAdminAPI(page, trashed.path);
      const restored = await adminGet(
        page,
        `/api/documents?include_drafts=1&q=${encodeURIComponent(slug)}`
      );
      expectAdminOK(restored, `search documents after restore ${slug}`);
      expect(restored.json.some((doc) => doc.source_path === sourcePath)).toBeTruthy();
    } finally {
      if (page.isClosed()) {
        return;
      }
      const documents = await adminGet(
        page,
        `/api/documents?include_drafts=1&q=${encodeURIComponent(slug)}`
      );
      if (documents.ok && Array.isArray(documents.json)) {
        const exists = documents.json.some((doc) => doc.source_path === sourcePath);
        if (exists) {
          await deleteDocumentViaAdminAPI(page, sourcePath);
        }
      }
      const trash = await adminGet(page, '/api/documents/trash');
      if (trash.ok && Array.isArray(trash.json)) {
        const trashed = trash.json.find(
          (entry) => entry.path === sourcePath || entry.original_path === sourcePath
        );
        if (trashed) {
          await purgeDocumentViaAdminAPI(page, trashed.path);
        }
      }
    }
  });

  test('users can be created and edited through the admin UI', async ({ page }) => {
    await login(page);
    await ensureFrontendTheme(page, 'default');

    const username = `e2e-user-${Date.now()}`;

    try {
      await page.getByRole('link', { name: /^Users$/i }).click();
      await expect(page.getByRole('heading', { level: 2, name: /^Users$/i })).toBeVisible();

      await page.getByLabel(/^Username$/i).fill(username);
      await page.getByLabel(/^Name$/i).fill('Initial Name');
      await page.getByLabel(/^Email$/i).fill(`${username}@example.com`);
      await page.getByLabel(/^Role$/i).fill('reviewer');
      await page.getByLabel(/^Password$/i).fill('FoundryUser123!');
      await page.getByRole('button', { name: /^Save User$/i }).click();
      await expect(page.locator('.toast-stack')).toContainText(/User saved\./i);

      const userRow = page.locator('.table-row', { hasText: username });
      await expect(userRow).toHaveCount(1);
      await userRow.locator('[data-edit-user]').click();
      await expect(page.getByLabel(/^Username$/i)).toHaveValue(username);

      await page.getByLabel(/^Name$/i).fill('Updated Reviewer');
      await page.getByRole('button', { name: /^Save User$/i }).click();
      await expect(page.locator('.toast-stack')).toContainText(/User saved\./i);

      const users = await adminGet(page, '/api/users');
      expectAdminOK(users, 'list users after update');
      const updated = users.json.find((entry) => entry.username === username);
      expect(updated?.name).toBe('Updated Reviewer');
    } finally {
      await deleteUserViaAdminAPI(page, username);
    }
  });

  test('plugin and theme validation actions work from admin', async ({ page }) => {
    await login(page);
    await ensureFrontendTheme(page, 'default');

    await page.getByRole('link', { name: /^Plugins$/i }).click();
    await expect(page.getByRole('heading', { level: 2, name: /^Plugins$/i })).toBeVisible();
    const pluginRow = page.locator('.table-row', { hasText: 'toc' }).first();
    await expect(pluginRow).toHaveCount(1);
    await pluginRow.locator('[data-validate-plugin]').click();
    await expect(page.locator('.toast-stack')).toContainText(/Plugin .*validated/i);

    await page.getByRole('link', { name: /^Themes$/i }).click();
    await expect(page.getByRole('heading', { level: 2, name: /^Themes$/i })).toBeVisible();
    const themeRow = page.locator('.table-row', { hasText: 'default' }).first();
    await expect(themeRow).toHaveCount(1);
    await themeRow.locator('[data-validate-theme]').click();
    await expect(page.locator('.toast-stack')).toContainText(/Theme .*validated/i);
  });

  test('operations can create a zip backup', async ({ page }) => {
    await login(page);
    await ensureFrontendTheme(page, 'default');

    await page.getByRole('link', { name: /^Operations$/i }).click();
    const restoreButtons = page.locator('[data-restore-backup]');
    const zipBackupsPanel = page
      .locator('.panel-pad', {
        has: page.getByRole('heading', { level: 3, name: /^Zip Backups$/i }),
      })
      .first();
    const beforeCount = await restoreButtons.count();
    const record = await adminPost(page, '/api/backups/create', {
      name: `e2e-backup-${Date.now()}.zip`,
    });
    expectAdminOK(record, 'create named e2e backup');

    await page.reload();
    await expect(page.locator('[data-restore-backup]')).toHaveCount(beforeCount + 1);
    await expect(zipBackupsPanel.locator('.table')).toContainText(record.json.name);
  });

  test('debug page exposes developer tooling panels', async ({ page }) => {
    await login(page);
    await ensureFrontendTheme(page, 'default');

    await page.getByRole('link', { name: /^Debug$/i }).click();
    await expect(page.getByRole('heading', { level: 1, name: /^Debug$/i })).toBeVisible();
    await expect(page.getByRole('heading', { level: 2, name: /^Runtime Event Stream$/i })).toBeVisible();
    await expect(page.getByRole('heading', { level: 2, name: /^Admin SDK Inspector$/i })).toBeVisible();
    await expect(page.getByRole('heading', { level: 2, name: /^Request \/ Command Console$/i })).toBeVisible();
    await expect(page.getByRole('heading', { level: 2, name: /^Feature Flags \/ Experiments$/i })).toBeVisible();
  });

  test('editor, reviewer, and author roles see the expected shell', async ({ page }) => {
    const stamp = Date.now();
    const users = [
      {
        username: `e2e-editor-${stamp}`,
        role: 'editor',
        password: 'FoundryEditor123!',
        visible: [/Documents/i, /Editor/i, /Media/i, /Audit/i],
        hidden: [/Users/i, /Settings/i, /Operations/i],
      },
      {
        username: `e2e-reviewer-${stamp}`,
        role: 'reviewer',
        password: 'FoundryReviewer123!',
        visible: [/Documents/i, /Editor/i, /Media/i, /Audit/i],
        hidden: [/Users/i, /Settings/i, /Operations/i],
      },
      {
        username: `e2e-author-${stamp}`,
        role: 'author',
        password: 'FoundryAuthor123!',
        visible: [/Documents/i, /Editor/i, /Media/i],
        hidden: [/Audit/i, /Users/i, /Settings/i, /Operations/i],
      },
    ];

    await login(page);
    await ensureFrontendTheme(page, 'default');
    try {
      for (const user of users) {
        await createUserViaAdminAPI(page, {
          username: user.username,
          name: user.username,
          email: `${user.username}@example.com`,
          role: user.role,
          password: user.password,
        });
      }

      for (const user of users) {
      await logout(page);
      await login(page, user.username, user.password);
        for (const pattern of user.visible) {
          await expect(page.locator('.foundry-nav')).toContainText(pattern);
        }
        for (const pattern of user.hidden) {
          await expect(page.locator('.foundry-nav')).not.toContainText(pattern);
        }
      }
    } finally {
      await logout(page);
      await login(page);
      await ensureFrontendTheme(page, 'default');
      for (const user of users) {
        await deleteUserViaAdminAPI(page, user.username);
      }
    }
  });
});
