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

Please review the attached contract and
let me know if you have any concerns.

Best regards,
Bob`;

const PLAIN_2 = `Confidential: Q3 results are up 24%.
Do not share externally until the announcement
on Monday.

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

async function injectFakeContent(page: Page) {
  await page.evaluate(() => {
    // Disable transitions/animations for clean screenshots
    const style = document.createElement('style');
    style.textContent = '*, *::before, *::after { transition: none !important; animation: none !important; }';
    document.head.appendChild(style);

    // Inject fake key options into selector
    const sel = document.getElementById('key-select') as HTMLSelectElement;
    sel.innerHTML = `
      <option value="">Select a key...</option>
      <option value="1" data-is-private="true">🔐 Alice's Key</option>
      <option value="2" data-is-private="false">🔒 Bob's Public Key</option>
      <option value="3" data-is-private="true">🔐 Work Key</option>
    `;

    // Inject fake stored keys list
    const storedDiv = document.querySelector('section.border-t > div:last-child') as HTMLElement | null;
    if (storedDiv) {
      storedDiv.innerHTML = `
        <div class="flex items-center justify-between py-3 border-b border-[#292e42] group">
          <div class="flex items-center gap-2.5 min-w-0">
            <span class="text-sm font-medium text-[#c0caf5] truncate">Alice's Key</span>
            <span class="shrink-0 px-2 py-0.5 rounded text-[10px] font-medium bg-[#ff9e64]/15 text-[#ff9e64] border border-[#ff9e64]/25">Private</span>
            <span class="text-xs text-[#565f89] truncate">2024-01-15 09:12</span>
          </div>
        </div>
        <div class="flex items-center justify-between py-3 border-b border-[#292e42] group">
          <div class="flex items-center gap-2.5 min-w-0">
            <span class="text-sm font-medium text-[#c0caf5] truncate">Bob's Public Key</span>
            <span class="shrink-0 px-2 py-0.5 rounded text-[10px] font-medium bg-[#9ece6a]/15 text-[#9ece6a] border border-[#9ece6a]/25">Public</span>
            <span class="text-xs text-[#565f89] truncate">2024-01-20 14:30</span>
          </div>
        </div>
        <div class="flex items-center justify-between py-3 group">
          <div class="flex items-center gap-2.5 min-w-0">
            <span class="text-sm font-medium text-[#c0caf5] truncate">Work Key</span>
            <span class="shrink-0 px-2 py-0.5 rounded text-[10px] font-medium bg-[#ff9e64]/15 text-[#ff9e64] border border-[#ff9e64]/25">Private</span>
            <span class="text-xs text-[#565f89] truncate">2024-02-03 11:05</span>
          </div>
        </div>
      `;
    }
  });
}

async function pickKey(page: Page, value: string) {
  await page.evaluate((v) => {
    const sel = document.getElementById('key-select') as HTMLSelectElement;
    sel.value = v;
    sel.dispatchEvent(new Event('change', { bubbles: true }));
  }, value);
}

async function setTextareas(page: Page, input: string, output: string) {
  await page.evaluate(([inp, out]) => {
    const inputEl = document.getElementById('input-text') as HTMLTextAreaElement;
    inputEl.value = inp;
    inputEl.dispatchEvent(new Event('input', { bubbles: true }));
    const outputEl = document.getElementById('output-text') as HTMLTextAreaElement;
    outputEl.value = out;
  }, [input, output] as [string, string]);
}

test('generate demo frames', async ({ page }) => {
  fs.mkdirSync(FRAMES_DIR, { recursive: true });

  const clip = { x: 0, y: 0, width: W, height: H };

  // ── Frame 00: login screen ─────────────────────────────────────────
  // Use exact H viewport so min-h-screen = 700px → card is perfectly centred
  await page.setViewportSize({ width: W, height: H });
  await page.goto('/login');
  await page.screenshot({ path: path.join(FRAMES_DIR, 'frame-00.png'), clip });

  // ── Login, then expand viewport for the main page ──────────────────
  await page.fill('input[name="password"]', DEMO_PW);
  await page.click('button[type="submit"]');
  await page.waitForURL('/');
  await page.setViewportSize({ width: W, height: 900 });

  await injectFakeContent(page);

  // ── Frame 01: private key, plain text input, Encrypt mode ─────────
  await pickKey(page, '1');
  await setTextareas(page, PLAIN_1, '');
  await page.screenshot({ path: path.join(FRAMES_DIR, 'frame-01.png'), clip });

  // ── Frame 02: private key, encrypted output ───────────────────────
  await setTextareas(page, PLAIN_1, FAKE_PGP);
  await page.screenshot({ path: path.join(FRAMES_DIR, 'frame-02.png'), clip });

  // ── Frame 03: public key, plain text input, Encrypt mode ──────────
  await pickKey(page, '2');
  await setTextareas(page, PLAIN_2, '');
  await page.screenshot({ path: path.join(FRAMES_DIR, 'frame-03.png'), clip });

  // ── Frame 04: public key, encrypted output ────────────────────────
  await setTextareas(page, PLAIN_2, FAKE_PGP);
  await page.screenshot({ path: path.join(FRAMES_DIR, 'frame-04.png'), clip });

  // ── Frame 05: private key, PGP message input, Decrypt mode ────────
  await pickKey(page, '1');
  await setTextareas(page, FAKE_PGP, '');
  await page.screenshot({ path: path.join(FRAMES_DIR, 'frame-05.png'), clip });

  // ── Frame 06: private key, decrypted output ───────────────────────
  await setTextareas(page, FAKE_PGP, PLAIN_1);
  await page.screenshot({ path: path.join(FRAMES_DIR, 'frame-06.png'), clip });

  // ── Frame 07: key management section, same 1400×700 clip ──────────
  // Scroll so the key management section starts near the top of the viewport
  await page.evaluate(() => {
    const section = document.querySelector('section.border-t') as Element;
    const top = section.getBoundingClientRect().top + window.scrollY - 16;
    window.scrollTo({ top, behavior: 'instant' });
  });
  await page.screenshot({ path: path.join(FRAMES_DIR, 'frame-07.png'), clip });

  console.log(`\n✓ 8 frames written to: ${FRAMES_DIR}\n`);
});
