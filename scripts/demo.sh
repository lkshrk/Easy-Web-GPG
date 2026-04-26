#!/usr/bin/env bash
# Generate demo.gif for the README.
# Requirements: Go, ImageMagick (brew install imagemagick), Node + npm
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
FRAMES_DIR="$SCRIPT_DIR/demo-frames"
OUTPUT="$ROOT_DIR/.github/assets/demo.gif"
PORT=18080
BIN="$ROOT_DIR/bin/easywebgpg-demo"
DB="$ROOT_DIR/data.db"
DB_BACKUP="$ROOT_DIR/data.db.demo-bak"
SERVER_PID=""

cleanup() {
  [[ -n "$SERVER_PID" ]] && kill "$SERVER_PID" 2>/dev/null || true
  [[ -f "$DB_BACKUP" ]] && mv "$DB_BACKUP" "$DB" || true
  rm -f "$BIN"
}
trap cleanup EXIT

# ── Preflight checks ───────────────────────────────────────────────────────────
if ! command -v go &>/dev/null; then
  echo "error: go not found" >&2; exit 1
fi
if ! command -v convert &>/dev/null; then
  echo "error: ImageMagick 'convert' not found. Install: brew install imagemagick" >&2; exit 1
fi
if ! command -v npx &>/dev/null; then
  echo "error: npx not found (Node.js required)" >&2; exit 1
fi

# ── Build CSS if needed ────────────────────────────────────────────────────────
if [[ ! -f "$ROOT_DIR/static/dist/styles.css" ]]; then
  echo "→ Building CSS..."
  make -C "$ROOT_DIR" css
fi

# ── Build binary ───────────────────────────────────────────────────────────────
echo "→ Building server..."
go build -o "$BIN" "$ROOT_DIR/cmd/easywebgpg"

# ── Back up existing DB ────────────────────────────────────────────────────────
[[ -f "$DB" ]] && cp "$DB" "$DB_BACKUP" && rm "$DB"

# ── Start server ───────────────────────────────────────────────────────────────
echo "→ Starting server on :$PORT..."
MASTER_PASSWORD=demo123 PORT=$PORT "$BIN" &
SERVER_PID=$!

# Wait until ready (up to 10s)
for i in $(seq 1 20); do
  if curl -sf "http://localhost:$PORT/" >/dev/null 2>&1; then
    break
  fi
  sleep 0.5
done
if ! curl -sf "http://localhost:$PORT/" >/dev/null 2>&1; then
  echo "error: server did not start in time" >&2; exit 1
fi

# ── Run Playwright ─────────────────────────────────────────────────────────────
echo "→ Capturing screenshots..."
mkdir -p "$FRAMES_DIR"
cd "$ROOT_DIR/tests"
npm install --silent
BASE_URL="http://localhost:$PORT" \
DEMO_PW=demo123 \
DEMO_FRAMES_DIR="$FRAMES_DIR" \
  npx playwright test demo.spec.ts --config playwright-demo.config.ts

# ── Stitch into GIF ────────────────────────────────────────────────────────────
echo "→ Creating GIF..."
mkdir -p "$(dirname "$OUTPUT")"
convert \
  -delay 300 -loop 0 \
  "$FRAMES_DIR/frame-00.png" \
  -delay 250 \
  "$FRAMES_DIR/frame-01.png" \
  "$FRAMES_DIR/frame-02.png" \
  "$FRAMES_DIR/frame-03.png" \
  "$FRAMES_DIR/frame-04.png" \
  "$FRAMES_DIR/frame-05.png" \
  "$FRAMES_DIR/frame-06.png" \
  -delay 350 \
  "$FRAMES_DIR/frame-07.png" \
  -layers optimize \
  -resize '900x>' \
  "$OUTPUT"

echo ""
echo "✓  demo.gif created: $OUTPUT"
echo "   $(du -sh "$OUTPUT" | cut -f1)"
