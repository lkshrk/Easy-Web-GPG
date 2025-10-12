# ==============================================================================
# Multi-stage Dockerfile for Easy Web GPG
# ==============================================================================

# ------------------------------------------------------------------------------
# Stage 1: Build CSS assets with Tailwind CSS standalone CLI
# ------------------------------------------------------------------------------
FROM debian:bookworm-slim AS css_builder

WORKDIR /app

# Install curl
RUN apt-get update && apt-get install -y curl && rm -rf /var/lib/apt/lists/*

# Copy CSS source files
COPY static/css/ ./static/css/
COPY templates/ ./templates/

# Download Tailwind standalone CLI and build CSS
ARG TARGETARCH
RUN TAILWIND_ARCH=$(case ${TARGETARCH:-amd64} in \
      amd64) echo "x64" ;; \
      arm64) echo "arm64" ;; \
      *) echo "x64" ;; \
    esac) && \
    curl -sLO "https://github.com/tailwindlabs/tailwindcss/releases/latest/download/tailwindcss-linux-${TAILWIND_ARCH}" && \
    chmod +x "tailwindcss-linux-${TAILWIND_ARCH}" && \
    mkdir -p static/dist && \
    "./tailwindcss-linux-${TAILWIND_ARCH}" -i ./static/css/input.css -o ./static/dist/styles.css --minify

# ------------------------------------------------------------------------------
# Stage 2: Build Go binary
# ------------------------------------------------------------------------------
FROM golang:1.24-alpine AS go_builder

WORKDIR /src

# Install build dependencies
RUN apk add --no-cache ca-certificates git && update-ca-certificates

# Copy Go module files
COPY go.mod go.sum ./
RUN go mod download && go mod verify

# Copy source code
COPY . .

# Copy built CSS
COPY --from=css_builder /app/static/dist/ ./static/dist/

# Build the binary
ENV CGO_ENABLED=0
RUN go build -ldflags="-s -w" -trimpath -o /easy-web-gpg ./cmd/easywebgpg

# ------------------------------------------------------------------------------
# Stage 3: Binary export (for `make build`)
# ------------------------------------------------------------------------------
FROM scratch AS binary-export
COPY --from=go_builder /easy-web-gpg /easy-web-gpg

# ------------------------------------------------------------------------------
# Stage 4: Final minimal runtime image
# ------------------------------------------------------------------------------
FROM gcr.io/distroless/static:nonroot AS runtime

COPY --from=go_builder /easy-web-gpg /easy-web-gpg
COPY --from=go_builder /src/templates/ /templates/
COPY --from=go_builder /src/static/ /static/
COPY --from=go_builder /src/migrations/sql/ /migrations/sql/

EXPOSE 8080

ENTRYPOINT ["/easy-web-gpg"]
