import fs from 'fs';
import type { APIRequestContext, Page } from '@playwright/test';
import { test, expect } from '../utils/fixtures';
import {
  compareSnapshot,
  compareVersions,
  type Scenario,
  type Target,
} from '../utils/visual-comparison';

const VIEWPORTS = [
  { name: 'mobile', width: 375, height: 667 },
  { name: 'tablet', width: 768, height: 1024 },
  { name: 'desktop', width: 1280, height: 720 },
];

const COMPARISON_CONFIG = {
  diffThresholdPercent: 0.5,
  pixelmatchThreshold: 0.1,
};

const BASELINE_URL = process.env.BASELINE_URL || process.env.BASE_URL || 'http://localhost:8080';
const CURRENT_URL = process.env.BASE_URL || 'http://localhost:8080';
const USE_COMPARISON_MODE = !!process.env.BASELINE_URL;
const MASTER_PASSWORD = process.env.MASTER_PASSWORD || 'test-password';

const KEY_NAME = 'visual-test-key';
const PRIVATE_KEY = fs.readFileSync(new URL('../fixtures/visual-test-key.asc', import.meta.url), 'utf8');
const PLAINTEXT = 'Visual regression fixture message.';
const DATE_PATTERN = /^\d{1,2} [A-Z][a-z]{2} \d{4}$/;

async function login(request: APIRequestContext): Promise<void> {
  const res = await request.post('/auth', { form: { password: MASTER_PASSWORD }, maxRedirects: 0 });
  expect(res.status(), 'login response').toBe(303);
}

async function resetKeys(page: Page): Promise<void> {
  const html = await (await page.request.get('/')).text();
  for (const [, id] of html.matchAll(/data-key-id="(\d+)"/g)) {
    const res = await page.request.post('/keys/delete', { form: { id }, maxRedirects: 0 });
    expect(res.status(), `delete key ${id}`).toBe(303);
  }
}

async function addFixtureKey(page: Page): Promise<void> {
  const res = await page.request.post('/keys', {
    form: { name: KEY_NAME, armored: PRIVATE_KEY, password: '' },
    maxRedirects: 0,
  });
  expect(res.status(), 'add key response').toBe(303);
}

const maskKeyDates = (page: Page) => [page.getByText(DATE_PATTERN)];

const SCENARIOS: (Scenario & { loggedIn: boolean })[] = [
  {
    name: 'login',
    loggedIn: false,
    setup: async (page) => {
      await page.goto('/');
      await expect(page.locator('form[action="/auth"]')).toBeVisible();
    },
  },
  {
    name: 'empty',
    loggedIn: true,
    setup: async (page) => {
      await resetKeys(page);
      await page.goto('/');
      await expect(page.getByText('No keys stored yet.')).toBeVisible();
    },
  },
  {
    name: 'with-key',
    loggedIn: true,
    setup: async (page) => {
      await resetKeys(page);
      await addFixtureKey(page);
      await page.goto('/');
      await expect(page.locator('.delete-key-btn')).toHaveCount(1);
    },
    mask: maskKeyDates,
  },
  {
    name: 'round-trip',
    loggedIn: true,
    setup: async (page) => {
      await resetKeys(page);
      await addFixtureKey(page);
      await page.goto('/');

      const keyId = await page.locator('#key-select option', { hasText: KEY_NAME }).getAttribute('value');
      await page.selectOption('#key-select', keyId!);

      await page.fill('#input-text', PLAINTEXT);
      await page.click('#action-btn');
      await expect(page.locator('#output-text')).toHaveValue(/^-----BEGIN PGP MESSAGE-----/);

      await page.fill('#input-text', await page.inputValue('#output-text'));
      await expect(page.locator('#action-btn-text')).toHaveText('Decrypt');
      await page.click('#action-btn');
      await expect(page.locator('#output-text')).toHaveValue(PLAINTEXT);
      await expect(page.locator('#action-btn')).toBeEnabled();
    },
    mask: (page) => [...maskKeyDates(page), page.locator('#input-text')],
  },
];

test.describe('Visual Regression Tests', () => {
  test.describe.configure({ timeout: 30_000 });

  const baseline: Target = { baseURL: BASELINE_URL };
  const current: Target = { baseURL: CURRENT_URL };

  // /auth allows 10 attempts per IP per 15 minutes, so log in once per worker and reuse the cookie.
  test.beforeAll(async ({ playwright }) => {
    for (const target of USE_COMPARISON_MODE ? [baseline, current] : [current]) {
      const request = await playwright.request.newContext({ baseURL: target.baseURL });
      await login(request);
      target.storageState = await request.storageState();
      await request.dispose();
    }
  });

  for (const viewport of VIEWPORTS) {
    for (const scenario of SCENARIOS) {
      test(`${viewport.name} ${scenario.name} matches baseline`, { tag: '@visual' }, async ({ newTrackedContext }) => {
        const pick = (target: Target): Target =>
          scenario.loggedIn ? target : { baseURL: target.baseURL };
        if (USE_COMPARISON_MODE) {
          await compareVersions(newTrackedContext, pick(baseline), pick(current), viewport, scenario, COMPARISON_CONFIG);
        } else {
          await compareSnapshot(newTrackedContext, pick(current), viewport, scenario, COMPARISON_CONFIG);
        }
      });
    }
  }
});

test.describe('Functional Tests', () => {
  test('login page loads', async ({ page }) => {
    await page.goto('/');
    await expect(page.locator('button[type="submit"]')).toContainText('Unlock');
  });

  test('shows login error on wrong password', async ({ page }) => {
    await page.goto('/');
    await page.fill('input[name="password"]', 'wrong-password');
    await page.click('button[type="submit"]');
    await expect(page.getByText('invalid password')).toBeVisible();
  });
});
