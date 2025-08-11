# Docker Guide for GPG Web Application

This guide covers how to build, run, and develop the GPG Web Application using Docker.

## 📋 Table of Contents

- [Overview](#overview)
- [Quick Start](#quick-start)
- [Building the Image](#building-the-image)
- [Running the Container](#running-the-container)
- [Development Workflow](#development-workflow)
- [Docker Compose](#docker-compose)
- [Environment Variables](#environment-variables)
- [Troubleshooting](#troubleshooting)
- [Best Practices](#best-practices)

## 🔍 Overview

The application uses a multi-stage Docker build process:

1. **CSS Builder Stage**: Builds Tailwind CSS assets using Node.js
2. **Go Builder Stage**: Compiles the Go binary
3. **Runtime Stage**: Creates a minimal production image using distroless

### Image Characteristics
- **Base**: `gcr.io/distroless/static:nonroot` (minimal, secure)
- **Size**: ~15-20MB (optimized)
- **Security**: Runs as non-root user
- **Architecture**: linux/amd64

## 🚀 Quick Start

### Using Make (Recommended)

```bash
# Build and run with Make
make docker-run

# For development environment
make docker-dev

# Stop containers
make docker-stop

# View logs
make docker-logs
```

### Using Docker Commands

```bash
# Build the image
docker build -t gpg-web:latest .

# Run the container
docker run -d \
  --name gpg-web-container \
  -p 8080:8080 \
  -e MASTER_KEY=your-secret-key \
  gpg-web:latest

# Access the application
open http://localhost:8080
```

## 🔨 Building the Image

### Standard Build

```bash
docker build -t gpg-web:latest .
```

### Build with Custom Tag

```bash
docker build -t gpg-web:v1.0.0 .
```

### Build Arguments

The Dockerfile supports the following build-time optimizations:

```bash
# Build with specific Node.js version
docker build --build-arg NODE_VERSION=20 -t gpg-web:latest .

# Build with specific Go version
docker build --build-arg GO_VERSION=1.24 -t gpg-web:latest .
```

### Multi-Platform Build

```bash
# Build for multiple architectures
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  -t gpg-web:latest \
  --push .
```

## 🏃 Running the Container

### Basic Run

```bash
docker run -d \
  --name gpg-web \
  -p 8080:8080 \
  -e MASTER_KEY=your-master-key \
  gpg-web:latest
```

### With Volume Mounts (Data Persistence)

```bash
docker run -d \
  --name gpg-web \
  -p 8080:8080 \
  -e MASTER_KEY=your-master-key \
  -v $(pwd)/data:/data \
  gpg-web:latest
```

### With Custom Configuration

```bash
docker run -d \
  --name gpg-web \
  -p 8080:8080 \
  -e MASTER_KEY=your-master-key \
  -e ENV=production \
  -v $(pwd)/data:/data \
  -v $(pwd)/custom-templates:/templates:ro \
  gpg-web:latest
```

## 💻 Development Workflow

### Option 1: Docker Compose (Recommended)

```bash
# Start development environment
docker-compose --profile dev up -d

# This starts:
# - Go application with live reload
# - CSS watcher for Tailwind builds
# - Volume mounts for hot reloading

# View logs
docker-compose logs -f app-dev

# Stop development environment
docker-compose --profile dev down
```

### Option 2: Development Container

```bash
# Build development image
docker build --target go_builder -t gpg-web:dev .

# Run with source mounted
docker run -it \
  --name gpg-web-dev \
  -p 8080:8080 \
  -v $(pwd):/src \
  -w /src \
  -e MASTER_KEY=dev-key \
  gpg-web:dev \
  go run ./cmd/gpgweb
```

### Live Reloading Setup

For the best development experience:

```bash
# Terminal 1: Start CSS watcher
make css-watch

# Terminal 2: Start Go application
make run-dev

# Or use Docker Compose for both
make docker-dev
```

## 🐳 Docker Compose

### Services Available

- **app**: Production application container
- **app-dev**: Development container with volume mounts
- **css-watch**: CSS build watcher (development profile)

### Common Commands

```bash
# Production
docker-compose up -d

# Development
docker-compose --profile dev up -d

# View logs
docker-compose logs -f [service-name]

# Rebuild and restart
docker-compose up --build -d

# Stop all services
docker-compose down

# Clean up volumes
docker-compose down -v
```

### Environment File

Create a `.env` file for environment variables:

```env
MASTER_KEY=your-super-secret-master-key
ENV=development
PORT=8080
```

## 🔧 Environment Variables

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `MASTER_KEY` | Master encryption key | - | ✅ |
| `ENV` | Environment mode | `production` | ❌ |
| `PORT` | Application port | `8080` | ❌ |

### Setting Environment Variables

```bash
# Via command line
docker run -e MASTER_KEY=secret -e ENV=dev gpg-web:latest

# Via environment file
docker run --env-file .env gpg-web:latest

# Via Docker Compose
# (automatically reads .env file)
docker-compose up
```

## 🔍 Troubleshooting

### Common Issues

#### Container Fails to Start

```bash
# Check logs
docker logs gpg-web-container

# Common causes:
# - Missing MASTER_KEY environment variable
# - Port 8080 already in use
# - Insufficient permissions
```

#### CSS Not Building

```bash
# Check if Node.js dependencies are installed
docker run --rm gpg-web:latest ls -la /static/dist/

# Rebuild with verbose output
docker build --no-cache -t gpg-web:latest .
```

#### Permission Denied Errors

```bash
# Ensure proper ownership (if using volume mounts)
sudo chown -R $(id -u):$(id -g) ./data

# Or run with user mapping
docker run --user $(id -u):$(id -g) gpg-web:latest
```

### Debug Commands

```bash
# Inspect the image
docker inspect gpg-web:latest

# Run interactive shell (debug)
docker run -it --rm gpg-web:latest sh

# Check container resources
docker stats gpg-web-container

# View container processes
docker exec gpg-web-container ps aux
```

### Performance Issues

```bash
# Monitor container resources
docker stats

# Check build cache usage
docker system df

# Clean up unused resources
docker system prune -f
```

## 📋 Best Practices

### Security

1. **Never hardcode secrets** in Dockerfiles
2. **Use environment variables** for configuration
3. **Run as non-root** (already implemented)
4. **Keep images minimal** (distroless base)
5. **Scan for vulnerabilities** regularly

```bash
# Scan image for vulnerabilities
docker scan gpg-web:latest
```

### Performance

1. **Use .dockerignore** to reduce build context
2. **Leverage layer caching** in CI/CD
3. **Multi-stage builds** for smaller images
4. **Use specific versions** for dependencies

### Development

1. **Use Docker Compose** for local development
2. **Mount volumes** for hot reloading
3. **Separate dev/prod** configurations
4. **Use health checks** in production

### Production Deployment

```bash
# Build optimized image
docker build \
  --target production \
  --build-arg BUILD_VERSION=$(git rev-parse HEAD) \
  -t gpg-web:$(git tag --sort=-version:refname | head -1) .

# Run with restart policy
docker run -d \
  --name gpg-web-prod \
  --restart unless-stopped \
  -p 8080:8080 \
  --env-file .env.production \
  gpg-web:latest

# Use Docker Compose for orchestration
docker-compose -f docker-compose.prod.yml up -d
```

## 📚 Additional Resources

- [Docker Documentation](https://docs.docker.com/)
- [Docker Compose Reference](https://docs.docker.com/compose/)
- [Distroless Images](https://github.com/GoogleContainerTools/distroless)
- [Go Docker Best Practices](https://docs.docker.com/language/golang/)

## 🆘 Need Help?

- Check the [main README](./README.md) for project-specific information
- Review [CONTRIBUTING.md](./CONTRIBUTING.md) for development guidelines
- Open an issue for Docker-specific problems

---

**Happy Dockerizing! 🐳**