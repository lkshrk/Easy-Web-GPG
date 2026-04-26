import { test, expect } from '@playwright/test';
import { compareSnapshot, compareVersions } from '../utils/visual-comparison';

const ROUTES = ['/', '/auth'];

const VIEWPORTS = [
  { name: 'mobile', width: 375, height: 667 },
  { name: 'tablet', width: 768, height: 1024 },
  { name: 'desktop', width: 1280, height: 720 },
];

const COMPARISON_CONFIG = {
  diffThresholdPercent: 0.5, // 0.5% difference allowed
  pixelmatchThreshold: 0.1,
};

// Get URLs from environment
const BASELINE_URL = process.env.BASELINE_URL || process.env.BASE_URL || 'http://localhost:8080';
const CURRENT_URL = process.env.BASE_URL || 'http://localhost:8080';
const USE_COMPARISON_MODE = !!process.env.BASELINE_URL;

test.describe('Visual Regression Tests', () => {
  test.describe.configure({ mode: 'parallel' });

  for (const viewport of VIEWPORTS) {
    test.describe(`Viewport: ${viewport.name}`, () => {
      test.use({ viewport });

      for (const route of ROUTES) {
        test(`${route} matches baseline`, async ({ page, browser }) => {
          if (USE_COMPARISON_MODE) {
            // Comparison mode: compare main branch vs current branch
            await compareVersions(
              page,
              browser,
              route,
              viewport,
              BASELINE_URL,
              CURRENT_URL,
              COMPARISON_CONFIG
            );
          } else {
            // Snapshot mode: compare against stored snapshots
            await compareSnapshot(page, route, viewport, COMPARISON_CONFIG);
          }
        });
      }
    });
  }
});

test.describe('Functional Tests', () => {
  test('login page loads', async ({ page }) => {
    await page.goto('/auth');
    await expect(page.locator('button[type="submit"]')).toContainText('Unlock');
  });

  test('shows login error on wrong password', async ({ page }) => {
    await page.goto('/auth');
    await page.fill('input[name="password"]', 'wrong-password');
    await page.click('button[type="submit"]');

    // Wait for error message
    await expect(page.getByText('invalid password')).toBeVisible();
  });
});
