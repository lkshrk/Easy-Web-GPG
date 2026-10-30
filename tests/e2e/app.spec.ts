import fs from 'fs';
import { test, expect } from '../utils/fixtures';

const BASE_URL = process.env.BASE_URL || 'http://localhost:8080';
const MASTER_PASSWORD = process.env.MASTER_PASSWORD || 'test-password';

const TEST_PUBLIC_KEY = fs.readFileSync(new URL('../fixtures/visual-test-key.pub.asc', import.meta.url), 'utf8');

test.describe('Easy Web GPG - Smoke Tests', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto(BASE_URL);

    // Check if we're on the login page
    const loginForm = page.locator('form[action="/auth"]');
    const isVisible = await loginForm.isVisible().catch(() => false);

    if (isVisible) {
      await page.fill('input[name="password"]', MASTER_PASSWORD);
      await page.click('button[type="submit"]');
      await page.waitForLoadState('networkidle');
    }
  });

  test('homepage loads successfully', async ({ page }) => {
    const response = await page.goto(BASE_URL);
    expect(response?.status()).toBe(200);

    await expect(page).toHaveTitle('PGP Web');
  });

  test('main UI elements are present', async ({ page }) => {
    await page.goto(BASE_URL);

    // Key management section
    await expect(page.locator('#key-name')).toBeVisible();
    await expect(page.locator('#key-armored')).toBeVisible();
    await expect(page.locator('#key-password')).toBeVisible();

    // Encrypt/Decrypt section
    await expect(page.locator('#input-text')).toBeVisible();
    await expect(page.locator('#output-text')).toBeVisible();
    await expect(page.locator('#action-btn')).toBeVisible();
  });
});

test.describe('Easy Web GPG - Key Management', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto(BASE_URL);

    // Check if we're on the login page
    const loginForm = page.locator('form[action="/auth"]');
    const isVisible = await loginForm.isVisible().catch(() => false);

    if (isVisible) {
      await page.fill('input[name="password"]', MASTER_PASSWORD);
      await page.click('button[type="submit"]');
      await page.waitForLoadState('networkidle');
    }
  });

  test('can add a new key', async ({ page }) => {
    await page.goto(BASE_URL);

    const keyName = 'test-key-' + Date.now();

    // Fill in the add key form
    await page.fill('#key-name', keyName);
    await page.fill('#key-armored', TEST_PUBLIC_KEY);

    await page.click('form[action="/keys"] button[type="submit"]');

    await expect(page.getByText('Key added successfully')).toBeVisible();
    await expect(page.locator('#key-select option', { hasText: keyName })).toHaveCount(1);
  });

  test.describe('invalid key', () => {
    test.use({ expectedBrowserErrors: ['Failed to load resource: the server responded with a status of 400 (Bad Request)'] });

    test('shows error when adding invalid key', async ({ page }) => {
      await page.goto(BASE_URL);

      await page.fill('#key-name', 'invalid-key');
      await page.fill('#key-armored', 'not a valid PGP key at all');
      await page.click('form[action="/keys"] button[type="submit"]');

      await expect(page.getByText(/invalid PGP key/)).toBeVisible();
    });
  });
});

test.describe('Easy Web GPG - Input Handling', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto(BASE_URL);

    // Check if we're on the login page
    const loginForm = page.locator('form[action="/auth"]');
    const isVisible = await loginForm.isVisible().catch(() => false);

    if (isVisible) {
      await page.fill('input[name="password"]', MASTER_PASSWORD);
      await page.click('button[type="submit"]');
      await page.waitForLoadState('networkidle');
    }
  });

  test('input and output fields work', async ({ page }) => {
    await page.goto(BASE_URL);

    const testInput = 'Test message for encryption';

    // Type in input field
    await page.fill('#input-text', testInput);
    const inputValue = await page.inputValue('#input-text');
    expect(inputValue).toBe(testInput);

    // Clear button works
    await page.click('#clear-btn');
    const clearedValue = await page.inputValue('#input-text');
    expect(clearedValue).toBe('');
  });

  test('action button is disabled without key and input', async ({ page }) => {
    await page.goto(BASE_URL);

    const actionBtn = page.locator('#action-btn');
    await expect(actionBtn).toBeDisabled();
  });
});
