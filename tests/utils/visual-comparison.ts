import { type Page, type Browser, expect } from '@playwright/test';
import { PNG } from 'pngjs';
import pixelmatch from 'pixelmatch';
import fs from 'fs';
import path from 'path';

export interface ComparisonConfig {
  diffThresholdPercent: number;
  pixelmatchThreshold: number;
}

export interface ViewportConfig {
  name: string;
  width: number;
  height: number;
}

/**
 * Prepares a page for screenshot by disabling animations and waiting for fonts
 */
export async function preparePageForScreenshot(page: Page): Promise<void> {
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

/**
 * Compares a page screenshot against stored Playwright snapshots
 */
export async function compareSnapshot(
  page: Page,
  route: string,
  viewport: ViewportConfig,
  config: ComparisonConfig
): Promise<void> {
  await page.goto(route);
  await preparePageForScreenshot(page);

  const screenshot = await page.screenshot({ fullPage: true });

  await expect(screenshot).toMatchSnapshot(
    `${viewport.name}${route.replace(/\//g, '-') || '-home'}.png`,
    {
      maxDiffPixelRatio: config.diffThresholdPercent / 100,
      threshold: config.pixelmatchThreshold,
    }
  );
}

/**
 * Compares two versions of a page (e.g., main branch vs current branch)
 */
export async function compareVersions(
  currentPage: Page,
  browser: Browser,
  route: string,
  viewport: ViewportConfig,
  baselineUrl: string,
  currentUrl: string,
  config: ComparisonConfig
): Promise<void> {
  // Create a separate context for baseline
  const baselineContext = await browser.newContext({ viewport });
  const baselinePage = await baselineContext.newPage();

  try {
    // Navigate both pages
    await Promise.all([
      baselinePage.goto(`${baselineUrl}${route}`),
      currentPage.goto(`${currentUrl}${route}`),
    ]);

    // Prepare both pages
    await Promise.all([
      preparePageForScreenshot(baselinePage),
      preparePageForScreenshot(currentPage),
    ]);

    // Take screenshots
    const [baselineScreenshot, currentScreenshot] = await Promise.all([
      baselinePage.screenshot({ fullPage: true }),
      currentPage.screenshot({ fullPage: true }),
    ]);

    // Compare images
    const comparisonResult = compareImages(
      baselineScreenshot,
      currentScreenshot,
      config
    );

    // Save screenshots for debugging
    const testName = `${viewport.name}${route.replace(/\//g, '-') || '-home'}`;
    saveComparisonArtifacts(
      testName,
      baselineScreenshot,
      currentScreenshot,
      comparisonResult.diffImage
    );

    // Check if difference is within threshold
    expect(comparisonResult.diffPercent).toBeLessThan(config.diffThresholdPercent);

    if (comparisonResult.diffPercent > 0) {
      console.log(
        `✓ ${testName}: ${comparisonResult.diffPercent.toFixed(3)}% difference (${comparisonResult.numDiffPixels} pixels)`
      );
    }
  } finally {
    await baselineContext.close();
  }
}

/**
 * Compares two images using pixelmatch
 */
function compareImages(
  baselineBuffer: Buffer,
  currentBuffer: Buffer,
  config: ComparisonConfig
): {
  numDiffPixels: number;
  diffPercent: number;
  diffImage: PNG;
} {
  const baselinePng = PNG.sync.read(baselineBuffer);
  const currentPng = PNG.sync.read(currentBuffer);

  // Ensure dimensions match
  if (
    baselinePng.width !== currentPng.width ||
    baselinePng.height !== currentPng.height
  ) {
    throw new Error(
      `Screenshot dimensions don't match: baseline ${baselinePng.width}x${baselinePng.height} vs current ${currentPng.width}x${currentPng.height}`
    );
  }

  // Create diff image
  const diff = new PNG({ width: baselinePng.width, height: baselinePng.height });
  const numDiffPixels = pixelmatch(
    baselinePng.data,
    currentPng.data,
    diff.data,
    baselinePng.width,
    baselinePng.height,
    { threshold: config.pixelmatchThreshold }
  );

  const totalPixels = baselinePng.width * baselinePng.height;
  const diffPercent = (numDiffPixels / totalPixels) * 100;

  return {
    numDiffPixels,
    diffPercent,
    diffImage: diff,
  };
}

/**
 * Saves comparison artifacts to disk for debugging
 */
function saveComparisonArtifacts(
  testName: string,
  baselineBuffer: Buffer,
  currentBuffer: Buffer,
  diffImage: PNG
): void {
  const screenshotDir = path.join(process.cwd(), 'test-results', testName);
  fs.mkdirSync(screenshotDir, { recursive: true });

  fs.writeFileSync(path.join(screenshotDir, 'baseline.png'), baselineBuffer);
  fs.writeFileSync(path.join(screenshotDir, 'current.png'), currentBuffer);
  fs.writeFileSync(path.join(screenshotDir, 'diff.png'), PNG.sync.write(diffImage));
}
