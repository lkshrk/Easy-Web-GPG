import { test as base, expect, type BrowserContext, type BrowserContextOptions } from '@playwright/test';

// Exact substrings of browser errors that are expected in CI; each entry needs a one-line reason.
const ALLOWED_BROWSER_ERRORS: string[] = [];

type Fixtures = {
  expectedBrowserErrors: string[];
  browserErrorLog: string[];
  newTrackedContext: (options?: BrowserContextOptions) => Promise<BrowserContext>;
  failOnBrowserErrors: void;
};

function track(context: BrowserContext, log: string[], expected: string[]): void {
  const allowed = [...ALLOWED_BROWSER_ERRORS, ...expected];
  const record = (message: string) => {
    if (!allowed.some((s) => message.includes(s))) log.push(message);
  };
  context.on('weberror', (e) => record(`pageerror: ${e.error().message}`));
  context.on('console', (m) => {
    if (m.type() === 'error') record(`console.error: ${m.text()} (${m.location().url})`);
  });
}

export const test = base.extend<Fixtures>({
  expectedBrowserErrors: [[], { option: true }],

  browserErrorLog: async ({}, use) => {
    await use([]);
  },

  context: async ({ context, browserErrorLog, expectedBrowserErrors }, use) => {
    track(context, browserErrorLog, expectedBrowserErrors);
    await use(context);
  },

  newTrackedContext: async ({ browser, browserErrorLog, expectedBrowserErrors }, use) => {
    const contexts: BrowserContext[] = [];
    await use(async (options) => {
      const ctx = await browser.newContext(options);
      track(ctx, browserErrorLog, expectedBrowserErrors);
      contexts.push(ctx);
      return ctx;
    });
    await Promise.all(contexts.map((c) => c.close()));
  },

  failOnBrowserErrors: [
    async ({ browserErrorLog }, use) => {
      await use();
      expect(browserErrorLog, 'browser errors (pageerror / console.error)').toEqual([]);
    },
    { auto: true },
  ],
});

export { expect };
