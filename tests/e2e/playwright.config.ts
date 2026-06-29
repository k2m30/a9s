import { defineConfig, devices } from "@playwright/test";

// The web UI uses a single per-session controller + a session cookie, so tests
// share one server and must run serially (workers: 1). global-setup builds the
// binary, boots `a9s --demo --web`, and writes the tokenized URL to .runtime.json;
// global-teardown kills the server.
export default defineConfig({
  testDir: ".",
  fullyParallel: false,
  workers: 1,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  timeout: 30_000,
  reporter: process.env.CI
    ? [["github"], ["html", { open: "never" }]]
    : [["list"], ["html", { open: "never" }]],
  globalSetup: "./global-setup.ts",
  globalTeardown: "./global-teardown.ts",
  use: {
    trace: "on-first-retry",
    screenshot: "only-on-failure",
    video: "retain-on-failure",
  },
  projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"] } }],
});
