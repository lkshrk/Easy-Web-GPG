import { test, type Page } from '@playwright/test';
import path from 'path';
import fs from 'fs';
import { fileURLToPath } from 'url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));

const DEMO_PW = process.env.DEMO_PW ?? 'demo123';
const FRAMES_DIR = process.env.DEMO_FRAMES_DIR
  ? path.resolve(process.env.DEMO_FRAMES_DIR)
  : path.join(__dirname, '..', 'scripts', 'demo-frames');

const W = 1400;
const H = 700;

const PLAIN_1 = `Hey Alice,

Please review the attached contract and let
me know if you have any concerns.

Best regards,
Bob`;

const PLAIN_2 = `Confidential: Q3 results are up 24%.
Do not share externally until Monday's
announcement.

— Finance Team`;

const FAKE_PGP = `-----BEGIN PGP MESSAGE-----

hQEMAzIVTlHk5+MBAQD+Ky8n3K7PVsxAUbBpYJlRNnJkOq3m9vT8W1xE
5YOPZgD7mFjT2KxBl9rQVp8C6wNnE3mPjLdXuR4HsZVKkTQkBCwME4bH
hJ/Y+AAALwEA7cxn5M2OUz3lJF8Rh/v+X9JtKdVZm3qN6PcWBkL0Q8QA
nR3MjKOpIzD0fGh2mNWqX7pL1CsEbKRyAUjVoT5wDiPx9lYH3BnZ8eMk
4QxSJo6ruW1vCFtNgaheD2wKPmXA1oTqRbLS0YH7nsVjMKEu5cIz3d+F
xWp9Gt4ORL2pKjN8aBvCqZeY5wDsFmXoHt1u3U6iEVMgcTPl7skn0QAA
=xY7p
-----END PGP MESSAGE-----`;

async function disableAnimations(page: Page) {
  await page.evaluate(() => {
    const s = document.createElement('style');
    s.textContent = '*, *::before, *::after { transition: none !important; animation: none !important; }';
    document.head.appendChild(s);
  });
}

// Prepare the crypto panel (frames 01–06):
//   - kill animations
//   - hide key management section
//   - add bottom padding so content doesn't press against the clip edge
//   - inject fake key options into the selector
async function setupCryptoPanel(page: Page) {
  await page.evaluate(() => {
    const s = document.createElement('style');
    s.textContent = '*, *::before, *::after { transition: none !important; animation: none !important; }';
    document.head.appendChild(s);

    // Extra space at the bottom of the visible area
    const main = document.querySelector('main') as HTMLElement;
    if (main) main.style.paddingBottom = '60px';

    // Completely hide the key management section
    const mgmt = document.querySelector('section.border-t') as HTMLElement | null;
    if (mgmt) mgmt.style.display = 'none';

    // Inject three fake keys into the <select>
    const sel = document.getElementById('key-select') as HTMLSelectElement;
    sel.innerHTML = `
      <option value="">Select a key...</option>
      <option value="1" data-is-private="true">🔐 Alice's Key</option>
      <option value="2" data-is-private="false">🔒 Bob's Public Key</option>
      <option value="3" data-is-private="true">🔐 Work Key</option>
    `;
  });
}

// Select a key by value and fire the change event so the badge updates.
async function pickKey(page: Page, value: string) {
  await page.evaluate((v) => {
    const sel = document.getElementById('key-select') as HTMLSelectElement;
    sel.value = v;
    sel.dispatchEvent(new Event('change', { bubbles: true }));
  }, value);
}

// Set textarea values and fire input so the button state recalculates.
async function setTextareas(page: Page, input: string, output: string) {
  await page.evaluate(([inp, out]) => {
    const inputEl = document.getElementById('input-text') as HTMLTextAreaElement;
    inputEl.value = inp;
    inputEl.dispatchEvent(new Event('input', { bubbles: true }));
    const outputEl = document.getElementById('output-text') as HTMLTextAreaElement;
    outputEl.value = out;
  }, [input, output] as [string, string]);
}

// Prepare the add-key panel (frame 07):
//   - hide header, crypto sections, error div, action buttons
//   - show only the "Add New Key" form from the key management section
//   - centre the form vertically in the viewport
async function setupAddKeyPanel(page: Page) {
  await page.evaluate(() => {
    const s = document.createElement('style');
    s.textContent = '*, *::before, *::after { transition: none !important; animation: none !important; }';
    document.head.appendChild(s);

    // Hide page header
    const header = document.querySelector('header') as HTMLElement | null;
    if (header) header.style.display = 'none';

    // Hide both sections that aren't key-management, plus error div and buttons
    document.querySelectorAll('main > section:not(.border-t)').forEach(
      (el) => ((el as HTMLElement).style.display = 'none'),
    );
    const errorDiv = document.getElementById('error-msg') as HTMLElement | null;
    if (errorDiv) errorDiv.style.display = 'none';
    // action-buttons div is the direct flex div in main
    document.querySelectorAll('main > div').forEach((el) => {
      if ((el as HTMLElement).classList.contains('flex')) {
        (el as HTMLElement).style.display = 'none';
      }
    });

    // Show key management; hide stored-keys div, hide the section heading
    const mgmt = document.querySelector('section.border-t') as HTMLElement | null;
    if (!mgmt) return;
    mgmt.style.display = 'block';
    mgmt.style.borderTop = 'none';
    mgmt.style.paddingTop = '0';

    // Hide section heading (h2 "Key Management")
    const h2 = mgmt.querySelector('h2') as HTMLElement | null;
    if (h2) h2.style.display = 'none';

    // Hide stored keys div (second div child)
    const divs = mgmt.querySelectorAll(':scope > div');
    divs.forEach((d, i) => {
      if (i > 0) (d as HTMLElement).style.display = 'none';
    });

    // Centre the add-key card vertically — the page body acts as the flex container
    const body = document.body as HTMLElement;
    body.style.display = 'flex';
    body.style.alignItems = 'center';
    body.style.justifyContent = 'center';
    body.style.minHeight = '700px';
    body.style.padding = '0';

    // Constrain the main element to a readable width
    const main = document.querySelector('main') as HTMLElement;
    if (main) {
      main.style.width = '100%';
      main.style.maxWidth = '700px';
      main.style.padding = '40px 32px';
      main.style.margin = '0 auto';
    }
  });
}

test('generate demo frames', async ({ page }) => {
  fs.mkdirSync(FRAMES_DIR, { recursive: true });

  const clip = { x: 0, y: 0, width: W, height: H };

  // ── Frame 00: login screen ─────────────────────────────────────────────────
  await page.setViewportSize({ width: W, height: H });
  await page.goto('/login');
  await disableAnimations(page);
  await page.screenshot({ path: path.join(FRAMES_DIR, 'frame-00.png'), clip });

  // ── Login ──────────────────────────────────────────────────────────────────
  await page.fill('input[name="password"]', DEMO_PW);
  await page.click('button[type="submit"]');
  await page.waitForURL('/');
  // Taller viewport so the full crypto panel is rendered off-clip
  await page.setViewportSize({ width: W, height: 900 });

  await setupCryptoPanel(page);

  // ── Frame 01: Alice's key selected, plain text, Encrypt mode ──────────────
  await pickKey(page, '1');
  await setTextareas(page, PLAIN_1, '');
  await page.screenshot({ path: path.join(FRAMES_DIR, 'frame-01.png'), clip });

  // ── Frame 02: same key, encrypted output ──────────────────────────────────
  await setTextareas(page, PLAIN_1, FAKE_PGP);
  await page.screenshot({ path: path.join(FRAMES_DIR, 'frame-02.png'), clip });

  // ── Frame 03: Bob's public key, plain text, Encrypt mode ──────────────────
  await pickKey(page, '2');
  await setTextareas(page, PLAIN_2, '');
  await page.screenshot({ path: path.join(FRAMES_DIR, 'frame-03.png'), clip });

  // ── Frame 04: public key, encrypted output ────────────────────────────────
  await setTextareas(page, PLAIN_2, FAKE_PGP);
  await page.screenshot({ path: path.join(FRAMES_DIR, 'frame-04.png'), clip });

  // ── Frame 05: Alice's key, PGP message in, Decrypt mode ───────────────────
  await pickKey(page, '1');
  await setTextareas(page, FAKE_PGP, '');
  await page.screenshot({ path: path.join(FRAMES_DIR, 'frame-05.png'), clip });

  // ── Frame 06: decrypted output ────────────────────────────────────────────
  await setTextareas(page, FAKE_PGP, PLAIN_1);
  await page.screenshot({ path: path.join(FRAMES_DIR, 'frame-06.png'), clip });

  // ── Frame 07: Add Key form, centred ───────────────────────────────────────
  await page.reload();
  await page.waitForURL('/');
  await page.setViewportSize({ width: W, height: H });
  await setupAddKeyPanel(page);
  await page.screenshot({ path: path.join(FRAMES_DIR, 'frame-07.png'), clip });

  console.log(`\n✓ 8 frames written to: ${FRAMES_DIR}\n`);
});
