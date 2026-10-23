import { defineConfig, devices } from "@playwright/test";

/**
 * Sentinel end-to-end suite.
 *
 * Expects a running stack (docker-compose.demo.yml or local):
 *   SENTINEL_URL  dashboard/API          default http://localhost:8080
 *   MOCK_URL      mock webhook inbox     default http://localhost:9911
 *   DEMO_DSN      DSN Sentinel uses to reach the demo ERP DB (as seen FROM the Sentinel container)
 *   MOCK_URL_FROM_SENTINEL  mock URL as seen from Sentinel (default = MOCK_URL)
 *   SENTINEL_TOKEN  bearer token when auth is enabled (omit with AUTH_DISABLED=true)
 *
 * Global setup resets Sentinel config and loads the demo configuration in "fast" mode
 * (5-second schedules) so detections happen quickly.
 */
const baseURL = process.env.SENTINEL_URL ?? "http://localhost:8080";

export default defineConfig({
  testDir: "./specs",
  globalSetup: "./global-setup.ts",
  timeout: 90_000,
  expect: { timeout: 15_000 },
  fullyParallel: false,
  workers: 1, // specs share one live stack
  retries: process.env.CI ? 1 : 0,
  reporter: [[process.env.CI ? "github" : "list"], ["list"], ["html", { open: "never", outputFolder: "playwright-report" }], ["junit", { outputFile: "results/junit.xml" }]],
  use: {
    baseURL,
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
    video: "retain-on-failure",
    extraHTTPHeaders: process.env.SENTINEL_TOKEN ? { Authorization: `Bearer ${process.env.SENTINEL_TOKEN}` } : {},
  },
  projects: [
    { name: "api", testMatch: /api\/.*\.spec\.ts/ },
    { name: "ui-desktop", testMatch: /ui\/(?!mobile).*\.spec\.ts/, use: { ...devices["Desktop Chrome"], viewport: { width: 1440, height: 900 } }, dependencies: ["api"] },
    { name: "ui-mobile", testMatch: /ui\/mobile\.spec\.ts/, use: { ...devices["Pixel 7"] }, dependencies: ["api"] },
    { name: "docs", testMatch: /docs\/.*\.spec\.ts/, use: { ...devices["Desktop Chrome"], viewport: { width: 1440, height: 900 } } },
  ],
});
