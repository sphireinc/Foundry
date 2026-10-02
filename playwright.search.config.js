const { defineConfig, devices } = require('@playwright/test');
const live = process.env.FOUNDRY_SEARCH_LIVE_PORT || 18753;
const staticPort = process.env.FOUNDRY_SEARCH_STATIC_PORT || 18754;
module.exports = defineConfig({
  testDir: './tests/search-e2e',
  workers: 1,
  timeout: 30000,
  use: {
    ...devices['Desktop Chrome'],
    channel: process.env.FOUNDRY_SEARCH_BROWSER_CHANNEL || 'chrome',
  },
  projects: [
    { name: 'live', use: { baseURL: `http://127.0.0.1:${live}` } },
    { name: 'static', use: { baseURL: `http://127.0.0.1:${staticPort}` } },
  ],
  webServer: {
    command: 'node scripts/search-e2e.cjs',
    url: `http://127.0.0.1:${staticPort}/search/`,
    reuseExistingServer: false,
    timeout: 120000,
    env: { ...process.env, GOCACHE: process.env.GOCACHE || '/tmp/foundry-go-cache' },
  },
});
