// Playwright config for generating README screenshots from a local Foundry instance.
const { defineConfig, devices } = require('@playwright/test');

const PORT = process.env.PLAYWRIGHT_BASE_URL || 'http://127.0.0.1:8080';

module.exports = defineConfig({
  testDir: './tests/readme-screenshots',
  timeout: 30_000,
  expect: {
    timeout: 5_000,
  },
  fullyParallel: false,
  retries: 0,
  workers: 1,
  reporter: [['list']],
  use: {
    baseURL: PORT,
    trace: 'off',
    screenshot: 'off',
    video: 'off',
    viewport: { width: 1600, height: 1200 },
  },
  projects: [
    {
      name: 'chromium',
      use: {
        ...devices['Desktop Chrome'],
        viewport: { width: 1600, height: 1200 },
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
