import { defineConfig, devices } from '@playwright/test';

// D-36: run against an already-running `npm run dev` stack. No `webServer` entry —
// deliberately not auto-starting the app so E2E stays decoupled from any one way of
// running it (local dev, Docker, etc.).
export default defineConfig({
  testDir: '.',
  fullyParallel: false,
  reporter: 'list',
  use: {
    baseURL: 'http://localhost:5173',
    trace: 'on-first-retry',
  },
  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }],
});
