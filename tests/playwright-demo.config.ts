import { defineConfig } from '@playwright/test';

const BASE_URL = process.env.BASE_URL ?? 'http://localhost:18080';

export default defineConfig({
  testDir: '.',
  testMatch: 'demo.spec.ts',
  timeout: 30_000,
  workers: 1,
  retries: 0,
  use: {
    baseURL: BASE_URL,
    viewport: { width: 1400, height: 900 },
    screenshot: 'off',
    video: 'off',
  },
  reporter: 'list',
});
