import { defineConfig } from '@playwright/test';
export default defineConfig({
  testDir: './e2e',
  workers: 1,
  retries: 0,
  timeout: 90000,
  use: {
    baseURL: `http://127.0.0.1:${process.env.UPTIME_LAB_WEB_PORT || '4173'}`,
    browserName: 'chromium',
    trace: 'retain-on-failure',
  },
  reporter: 'list',
});
