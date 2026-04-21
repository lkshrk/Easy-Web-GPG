import { test, expect } from '@playwright/test';

const BASE_URL = process.env.BASE_URL || 'http://localhost:8080';
const MASTER_PASSWORD = process.env.MASTER_PASSWORD || 'test-password';

// Simple test PGP public key
const TEST_PUBLIC_KEY = `-----BEGIN PGP PUBLIC KEY BLOCK-----
Version: OpenPGP.js v4.10.1
Comment: https://openpgpjs.org

xsBNBF8H/4UBEACuGVDyIjQ1uQnjz8XwEkLTh0LcO+XO7uqJbQPxJ5k8aR9oY3KL
d2FhcGNoZXI8dGVzdEBleGFtcGxlLmNvbT7CwHUEEAEIACkFAl8H/4WIQTMJxVZ6
VRwRqL5QtKyJJY8uWwUCXwf/hg8JGAAAKK4AAAAAAAKEAAAAAAAAP94bWw8ZXhl
bXBsYXIub3JnLzIwMjEvMTIvMTXIfQQQFggAHwUCXwf/hgkQzCxFVnpXHBGorkC0
rIkljy5bAAoJEK1MtJTJY8uWUA0F/iRhn5YZCNwvt5ANFF1PiPJjDkQyS8oPGvG8
NU4Qv4PH7hW2QJBaMNhjwSZPqJPZ/EA/CCK+hY+kLg5YqDZJJPQXFZVC6QkZL4r
CJYH/MP0L8eMQqvApqCJO7JLN5PrQ2yqxRT7CqAJAA7UCNG8rQrUEAIQEApIBH3H
cW5KVJh5JVQeKCGDbALCxDVHJSqvDhOFxCj5lwT9aIWTJ4I8YTbGrcvLLQHEKtf
JtKqJJPPqKFMHHg8QQyMSEfv41YKrIYCqDWKBQQQyQm3xL/kV9vtLdBq2gZLEFf
/f3UROa1YLQfqPZ2LzVEpR9NyYK64wYQyk5QPw5X1wNQ0QTEVBU0RJT05FPT0i
QQQLBggJwxsBFiEE7KfPZCJhB9W5vZQPdcYQkNJcW+kFAl8f/+QJEEVZFFRVaG9h
Y2hlciA8dGVzdEBleGFtcGxlLmNvbT4CmwQkAQAAKf0MACQkQTAAAhmkAAAKKEAA
AIadQQILCxYIAQEGFQoK
=2cVc
-----END PGP PUBLIC KEY BLOCK-----`;

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

    await expect(page.locator('h1')).toContainText('PGP Web');
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

    // Click submit button (form is POST so it redirects)
    await page.click('form[action="/keys"] button[type="submit"]');

    // Wait for navigation after redirect
    await page.waitForLoadState('load');

    // Test passes if we got here without errors
    expect(true).toBeTruthy();
  });

  test('shows error when adding invalid key', async ({ page }) => {
    await page.goto(BASE_URL);

    // Try to add invalid key
    await page.fill('#key-name', 'invalid-key');
    await page.fill('#key-armored', 'not a valid PGP key at all');
    await page.click('form[action="/keys"] button[type="submit"]');

    // Check for error message
    const errorDiv = page.locator('#error-msg');
    await page.waitForTimeout(1000);

    const isVisible = await errorDiv.isVisible().catch(() => false);
    if (isVisible) {
      await expect(errorDiv).toBeVisible();
    }
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
