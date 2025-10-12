import { test, expect, type Page } from '@playwright/test';
import { PNG } from 'pngjs';
import pixelmatch from 'pixelmatch';

const ROUTES = ['/', '/auth'];

const VIEWPORTS = [
  { name: 'mobile', width: 375, height: 667 },
  { name: 'tablet', width: 768, height: 1024 },
  { name: 'desktop', width: 1280, height: 720 },
];

const DIFF_THRESHOLD_PERCENT = 0.1; // 0.1% difference allowed
const PIXELMATCH_THRESHOLD = 0.1;

async function preparePageForScreenshot(page: Page) {
  // Disable animations
  await page.emulateMedia({ reducedMotion: 'reduce' });
  await page.addStyleTag({
    content: `
      *, *::before, *::after {
        animation: none !important;
        transition: none !important;
      }
    `,
  });

  // Wait for fonts
  await page.evaluate(async () => {
    if ('fonts' in document) {
      await document.fonts.ready;
    }
  });

  // Extra stabilization time
  await page.waitForTimeout(500);
}

test.describe('Visual Regression Tests', () => {
  test.describe.configure({ mode: 'parallel' });

  for (const viewport of VIEWPORTS) {
    test.describe(`Viewport: ${viewport.name}`, () => {
      test.use({ viewport });

      for (const route of ROUTES) {
        test(`${route} matches baseline`, async ({ page }) => {
          await page.goto(route);
          await preparePageForScreenshot(page);

          // Take screenshot
          const screenshot = await page.screenshot({ fullPage: true });

          // Compare with baseline
          // Note: On first run, this will save the baseline
          // On subsequent runs, it will compare
          await expect(screenshot).toMatchSnapshot(`${viewport.name}${route.replace(/\//g, '-') || '-home'}.png`, {
            maxDiffPixelRatio: DIFF_THRESHOLD_PERCENT / 100,
            threshold: PIXELMATCH_THRESHOLD,
          });
        });
      }
    });
  }
});

test.describe('Functional Tests', () => {
  test('login page loads', async ({ page }) => {
    await page.goto('/auth');
    await expect(page.locator('h1')).toContainText('Unlock PGP Web');
  });

  test('shows login error on wrong password', async ({ page }) => {
    await page.goto('/auth');
    await page.fill('input[name="password"]', 'wrong-password');
    await page.click('button[type="submit"]');

    // Wait for error message
    await expect(page.locator('.text-rose-600')).toBeVisible();
  });
});
