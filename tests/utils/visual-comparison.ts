import { type BrowserContext, type BrowserContextOptions, type Locator, type Page, expect } from '@playwright/test';
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

export type StorageState = Exclude<BrowserContextOptions['storageState'], string | undefined>;

export interface Target {
  baseURL: string;
  storageState?: StorageState;
}

export interface Scenario {
  name: string;
  setup: (page: Page) => Promise<void>;
  mask?: (page: Page) => Locator[];
}

export type ContextFactory = (options?: BrowserContextOptions) => Promise<BrowserContext>;

export async function preparePageForScreenshot(page: Page): Promise<void> {
  await page.emulateMedia({ reducedMotion: 'reduce' });
  await page.addStyleTag({
    content: `
      *, *::before, *::after {
        animation: none !important;
        transition: none !important;
      }
    `,
  });
  await page.evaluate(async () => {
    (document.activeElement as HTMLElement | null)?.blur();
    await document.fonts.ready;
  });
  await page.waitForTimeout(500);
}

async function capture(
  newContext: ContextFactory,
  target: Target,
  viewport: ViewportConfig,
  scenario: Scenario
): Promise<Buffer> {
  const context = await newContext({
    viewport: { width: viewport.width, height: viewport.height },
    baseURL: target.baseURL,
    storageState: target.storageState,
  });
  const page = await context.newPage();
  await scenario.setup(page);
  await preparePageForScreenshot(page);
  return page.screenshot({ fullPage: true, mask: scenario.mask?.(page) ?? [], maskColor: '#ff00ff' });
}

export async function compareSnapshot(
  newContext: ContextFactory,
  target: Target,
  viewport: ViewportConfig,
  scenario: Scenario,
  config: ComparisonConfig
): Promise<void> {
  const screenshot = await capture(newContext, target, viewport, scenario);
  expect(screenshot).toMatchSnapshot(`${viewport.name}-${scenario.name}.png`, {
    maxDiffPixelRatio: config.diffThresholdPercent / 100,
    threshold: config.pixelmatchThreshold,
  });
}

export async function compareVersions(
  newContext: ContextFactory,
  baseline: Target,
  current: Target,
  viewport: ViewportConfig,
  scenario: Scenario,
  config: ComparisonConfig
): Promise<void> {
  const [baselineScreenshot, currentScreenshot] = await Promise.all([
    capture(newContext, baseline, viewport, scenario),
    capture(newContext, current, viewport, scenario),
  ]);

  const testName = `${viewport.name}-${scenario.name}`;
  const result = compareImages(baselineScreenshot, currentScreenshot, config);
  saveComparisonArtifacts(testName, baselineScreenshot, currentScreenshot, result.diffImage);

  expect(result.currentSize, 'screenshot dimensions').toBe(result.baselineSize);
  expect(result.diffPercent).toBeLessThan(config.diffThresholdPercent);

  if (result.diffPercent > 0) {
    console.log(`${testName}: ${result.diffPercent.toFixed(3)}% difference (${result.numDiffPixels} pixels)`);
  }
}

function compareImages(baselineBuffer: Buffer, currentBuffer: Buffer, config: ComparisonConfig) {
  const baselinePng = PNG.sync.read(baselineBuffer);
  const currentPng = PNG.sync.read(currentBuffer);
  const baselineSize = `${baselinePng.width}x${baselinePng.height}`;
  const currentSize = `${currentPng.width}x${currentPng.height}`;

  if (baselineSize !== currentSize) {
    return { numDiffPixels: 0, diffPercent: 100, diffImage: undefined, baselineSize, currentSize };
  }

  const diff = new PNG({ width: baselinePng.width, height: baselinePng.height });
  const numDiffPixels = pixelmatch(
    baselinePng.data,
    currentPng.data,
    diff.data,
    baselinePng.width,
    baselinePng.height,
    { threshold: config.pixelmatchThreshold }
  );

  return {
    numDiffPixels,
    diffPercent: (numDiffPixels / (baselinePng.width * baselinePng.height)) * 100,
    diffImage: diff,
    baselineSize,
    currentSize,
  };
}

function saveComparisonArtifacts(
  testName: string,
  baselineBuffer: Buffer,
  currentBuffer: Buffer,
  diffImage: PNG | undefined
): void {
  const screenshotDir = path.join(process.cwd(), 'test-results', testName);
  fs.mkdirSync(screenshotDir, { recursive: true });

  fs.writeFileSync(path.join(screenshotDir, 'baseline.png'), baselineBuffer);
  fs.writeFileSync(path.join(screenshotDir, 'current.png'), currentBuffer);
  if (diffImage) {
    fs.writeFileSync(path.join(screenshotDir, 'diff.png'), PNG.sync.write(diffImage));
  }
}
