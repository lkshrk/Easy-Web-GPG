# Visual Regression Tests

This directory contains visual regression tests for Easy Web GPG using Playwright.

## How It Works

The visual regression tests run in two modes:

### Comparison Mode (CI - Non-Main Branches)
When running on any branch other than `main`, the tests:
1. Build the Docker image for the `main` branch
2. Build the Docker image for the current branch
3. Start both applications on different ports (main on 8081, current on 8080)
4. Take screenshots of both versions simultaneously
5. Compare them pixel-by-pixel using `pixelmatch`
6. Fail if differences exceed the threshold (0.5%)

### Snapshot Mode (CI - Main Branch or Local)
When running on the `main` branch or locally without `BASELINE_URL`:
- Uses Playwright's built-in snapshot testing
- Compares screenshots against stored baseline images
- Useful for updating baselines after intentional UI changes

## Running Tests Locally

### Quick Test (Snapshot Mode)
```bash
cd tests
npm install
npm run test:visual
```

### Full Comparison Test (Recommended - Like CI)
Use the make target to automatically build and compare both versions:
```bash
make test-visual
```

This will:
1. Clone and checkout the main branch to `/tmp/easy-web-gpg-main`
2. Build Docker images for both main and current branch
3. Start both applications (main on 8081, current on 8080)
4. Run visual regression tests comparing both
5. Automatically clean up containers when done

### Manual Comparison Test
If you want more control:
```bash
# Setup containers
make test-visual-setup

# Run tests manually
cd tests
BASELINE_URL=http://localhost:8081 BASE_URL=http://localhost:8080 npm run test:visual

# Cleanup when done
make test-visual-cleanup
```

## Configuration

- **Routes tested**: `/` (home) and `/auth` (login)
- **Viewports**: mobile (375x667), tablet (768x1024), desktop (1280x720)
- **Diff threshold**: 0.5% (percentage of pixels that can differ)
- **Pixelmatch threshold**: 0.1 (sensitivity for individual pixel comparison)

## Viewing Results

When tests fail, artifacts are uploaded to GitHub Actions:
- `baseline.png` - Screenshot from main branch
- `current.png` - Screenshot from current branch
- `diff.png` - Visual diff highlighting differences in pink

## Skipping Visual Tests

Add `[ui-change]` to your commit message to skip visual regression tests when you intentionally changed the UI:

```bash
git commit -m "Update button styles [ui-change]"
```

## Test Structure

```
tests/
├── visual.spec.ts              # Main test file (routes, viewports, test orchestration)
├── utils/
│   └── visual-comparison.ts    # Reusable comparison utilities
├── playwright.config.ts        # Playwright configuration
├── tsconfig.json              # TypeScript configuration
├── package.json               # Test dependencies
└── README.md                  # This file
```

### Key Files

- **`visual.spec.ts`** - Defines test cases, routes, and viewports to test
- **`utils/visual-comparison.ts`** - Reusable comparison functions:
  - `compareSnapshot()` - Compare against stored snapshots
  - `compareVersions()` - Compare two live versions
  - `preparePageForScreenshot()` - Disable animations and stabilize pages
- **`playwright.config.ts`** - Test configuration (timeouts, reporters, etc.)
