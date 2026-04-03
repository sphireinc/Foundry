// Local-only headless UI coverage for Foundry's shipped frontend and admin themes.
// This runs against a real local Foundry server started from the repo root.
const { defineConfig, devices } = require('@playwright/test');

const PORT = process.env.PLAYWRIGHT_BASE_URL || 'http://127.0.0.1:8080';

module.exports = defineConfig({
  testDir: './tests/e2e',
  timeout: 30_000,
  expect: {
    timeout: 5_000,
  },
  fullyParallel: false,
  retries: 0,
  workers: 1,
  reporter: [['list'], ['html', { open: 'never' }]],
  use: {
    baseURL: PORT,
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
    video: 'retain-on-failure',
  },
  projects: [
    {
      name: 'chromium',
      use: {
        ...devices['Desktop Chrome'],
      },
    },
  ],
  webServer: {
    command: 'go run ./scripts/cmd/e2e-serve',
    url: PORT,
    reuseExistingServer: true,
    timeout: 120_000,
    env: {
      ...process.env,
      GOCACHE: process.env.GOCACHE || '/tmp/go-build-cache',
    },
  },
});
