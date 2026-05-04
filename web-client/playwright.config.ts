import { defineConfig, devices } from '@playwright/test'

/**
 * Playwright E2E config.
 * Assumes the backend runs on :8080 and the Vite dev server on :5173.
 *
 * Usage:
 *   npx playwright test          # run headless
 *   npx playwright test --ui     # interactive UI mode
 *   npx playwright test --debug  # step-through debugging
 */
export default defineConfig({
  testDir: './e2e',
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: process.env.CI ? 1 : undefined,
  reporter: 'html',
  use: {
    baseURL: 'http://localhost:5173',
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
    video: 'on-first-retry',
  },

  projects: [
    { name: 'chromium', use: { ...devices['Desktop Chrome'] } },
    // { name: 'firefox', use: { ...devices['Desktop Firefox'] } },
  ],

  /* Run the local dev server before starting the tests */
  webServer: {
    command: 'vite --port 5173',
    url: 'http://localhost:5173',
    reuseExistingServer: !process.env.CI,
  },
})
