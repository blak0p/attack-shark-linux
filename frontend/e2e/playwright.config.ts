import { defineConfig, devices } from "@playwright/test";

export default defineConfig({
  testDir: ".",
  testMatch: "*.spec.ts",
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  reporter: process.env.CI ? "github" : "list",
  outputDir: "../node_modules/.cache/routing-playwright",
  projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"], browserName: "chromium", channel: undefined } }],
  use: { baseURL: "http://127.0.0.1:4178", trace: "retain-on-failure" },
  webServer: {
    command: "npm run dev:e2e",
    url: "http://127.0.0.1:4178/e2e.html",
    reuseExistingServer: false,
    timeout: 30_000,
  },
});
