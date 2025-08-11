# ==============================================================================
# Multi-stage Dockerfile for GPG Web Application
# ==============================================================================

# ------------------------------------------------------------------------------
# Stage 1: Build CSS assets with Node.js and Tailwind CSS
# ------------------------------------------------------------------------------
FROM node:20-alpine AS css_builder

WORKDIR /app

# Copy package files for better layer caching
COPY package.json package-lock.json* ./

# Install Node.js dependencies
RUN npm ci --only=production --silent

# Copy CSS source files
COPY static/css/ ./static/css/
COPY templates/ ./templates/

# Build production CSS with Tailwind
RUN npm run build:css

# ------------------------------------------------------------------------------
# Stage 2: Build Go binary
# ------------------------------------------------------------------------------
FROM golang:1.24-alpine AS go_builder

WORKDIR /src

# Install build dependencies
RUN apk add --no-cache \
  ca-certificates \
  git \
  && update-ca-certificates

# Copy Go module files for better layer caching
COPY go.mod go.sum ./

# Configure Go proxy and download dependencies
RUN go env -w GOPROXY=https://proxy.golang.org,direct && \
  go mod download && \
  go mod verify

# Copy source code
COPY . .

# Copy built CSS from previous stage
COPY --from=css_builder /app/static/dist/ ./static/dist/

# Build the binary with optimization flags
ENV CGO_ENABLED=0 \
  GOOS=linux \
  GOARCH=amd64

RUN go build \
  -ldflags="-s -w -extldflags '-static'" \
  -trimpath \
  -o /web-gpg \
  ./cmd/gpgweb

# Verify the binary
RUN ls -l /easy-web-gpg && echo "Binary built successfully"

# ------------------------------------------------------------------------------
# Stage 3: Final minimal runtime image
# ------------------------------------------------------------------------------
FROM gcr.io/distroless/static:nonroot

# Add labels for better container management
LABEL maintainer="GPG Web Team" \
  description="Lightweight web UI for managing OpenPGP keys" \
  version="1.0"

# Copy the binary from builder stage
COPY --from=go_builder /web-gpg /web-gpg

# Copy application assets and templates
COPY --from=go_builder /src/templates/ /templates/
COPY --from=go_builder /src/static/ /static/
COPY --from=go_builder /src/migrations/sql/ /migrations/sql/

# Expose the application port
EXPOSE 8080



# Set the entrypoint
ENTRYPOINT ["/web-gpg"]
