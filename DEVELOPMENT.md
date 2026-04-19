# Development Guide

This guide covers the development workflow for Easy Web GPG with live reloading.

## Quick Start

### Development Mode (with Live Reload)

```bash
# Set your master password
export MASTER_PASSWORD=test123

# Start development environment
make dev
```

The development environment includes:
- **Go live reload** - Changes to `.go` files automatically rebuild and restart the app
- **Template live reload** - Changes to `.html` templates refresh automatically
- **CSS live reload** - Tailwind CSS rebuilds automatically when you edit CSS or templates

Access the app at: http://localhost:8080

## What Gets Reloaded?

### Automatically Reloaded
- ✅ Go source files (`**/*.go`)
- ✅ HTML templates (`templates/**/*.html`)
- ✅ CSS files (`static/css/**/*.css`)
- ✅ Tailwind classes in templates (triggers CSS rebuild)

### Requires Manual Restart
- ❌ `go.mod` / `go.sum` changes
- ❌ Docker configuration changes
- ❌ `.air.toml` configuration

## Development Commands

```bash
# Start development environment
make dev

# Build development images (only needed first time or after Dockerfile.dev changes)
make dev-build

# Stop development environment
make dev-down

# View logs in real-time
make dev-logs
```

## How It Works

### Architecture

The development setup uses two containers:

1. **app** - Go application with Air live reload
   - Mounts source code as volumes
   - Watches for changes and rebuilds automatically
   - Runs on port 8080

2. **css-watcher** - Tailwind CSS watcher
   - Watches `static/css/` and `templates/` directories
   - Rebuilds CSS automatically
   - Outputs to `static/dist/styles.css`

### Air Configuration

Air is configured via `.air.toml`:
- Build command: `go build -o ./tmp/main ./cmd/easywebgpg`
- Watch extensions: `.go`, `.html`, `.css`
- Excluded directories: `tmp`, `vendor`, `node_modules`, `data`, `tests`
- Build delay: 1 second (debounces rapid changes)

## Production vs Development

### Production (`make run`)
- Uses multi-stage Dockerfile
- Minimal distroless runtime image
- No development tools
- Optimized binary with `-ldflags="-s -w"`

### Development (`make dev`)
- Uses `Dockerfile.dev` with full Go toolchain
- Includes Air for live reload
- Volume mounts for instant updates
- Source maps and debugging symbols

## Troubleshooting

### Changes Not Detected

If Air isn't detecting changes:
```bash
# Restart the development environment
make dev-down
make dev
```

### CSS Not Updating

If Tailwind CSS isn't rebuilding:
```bash
# Check CSS watcher logs
docker logs easy-web-gpg-css-watcher-1

# Restart just the CSS watcher
docker compose -f docker-compose.dev.yml restart css-watcher
```

### Port Already in Use

If port 8080 is already taken:
```bash
# Stop any running instances
make dev-down
docker compose down

# Or change the port in docker-compose.dev.yml
```

### Database Issues

The development environment uses the same `./data` directory as production:
```bash
# Reset the database
rm -rf data/
make dev
```

## Tips & Best Practices

### Fast Feedback Loop

1. Keep `make dev` running in one terminal
2. Edit files in your IDE
3. Save changes
4. Watch the terminal for rebuild status
5. Refresh browser to see changes

### Debugging

Add debug logs:
```go
log.Printf("DEBUG: variable value: %v", myVar)
```

Logs appear immediately in the terminal running `make dev`.

### Testing Changes

Before committing:
```bash
# Stop dev environment
make dev-down

# Run tests
make test

# Build production image
make docker-build

# Test production build
make run
```

## File Structure

```
.
├── .air.toml              # Air configuration
├── Dockerfile             # Production Dockerfile
├── Dockerfile.dev         # Development Dockerfile
├── docker-compose.yml     # Production compose
├── docker-compose.dev.yml # Development compose
├── Makefile               # Build commands
└── DEVELOPMENT.md         # This file
```

## Next Steps

- See [README.md](README.md) for general project information
- See [tests/README.md](tests/README.md) for testing documentation
- Check `.air.toml` to customize live reload behavior
