# ==============================================================================
# Multi-stage Dockerfile for Easy Web GPG
# ==============================================================================

# ------------------------------------------------------------------------------
# Stage 1: Build CSS assets
# ------------------------------------------------------------------------------
FROM --platform=$BUILDPLATFORM debian:bookworm-slim AS css_builder

WORKDIR /app
RUN apt-get update && apt-get install -y curl && rm -rf /var/lib/apt/lists/*
COPY static/css/ ./static/css/
COPY templates/ ./templates/

ARG BUILDARCH
RUN TAILWIND_ARCH=$(case ${BUILDARCH:-amd64} in \
      amd64) echo "x64" ;; \
      arm64) echo "arm64" ;; \
      *) echo "x64" ;; \
    esac) && \
    curl -sLO "https://github.com/tailwindlabs/tailwindcss/releases/latest/download/tailwindcss-linux-${TAILWIND_ARCH}" && \
    chmod +x "tailwindcss-linux-${TAILWIND_ARCH}" && \
    mkdir -p static/dist && \
    "./tailwindcss-linux-${TAILWIND_ARCH}" -i ./static/css/input.css -o ./static/dist/styles.css --minify

# ------------------------------------------------------------------------------
# Stage 2: Go dependencies (shared cache layer)
# ------------------------------------------------------------------------------
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS go_deps

WORKDIR /src
RUN apk add --no-cache ca-certificates git && update-ca-certificates
COPY go.mod go.sum ./
RUN go mod download && go mod verify

# ------------------------------------------------------------------------------
# Stage 3: Build binary
# ------------------------------------------------------------------------------
FROM go_deps AS go_builder

COPY . .
COPY --from=css_builder /app/static/dist/ ./static/dist/
ARG TARGETOS TARGETARCH
ENV CGO_ENABLED=0
RUN GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -ldflags="-s -w" -trimpath -o /easy-web-gpg ./cmd/easywebgpg

# ------------------------------------------------------------------------------
# Stage 4: Go tests  (docker build --target go-test .)
# ------------------------------------------------------------------------------
FROM go_deps AS go-test

RUN apk add --no-cache gcc musl-dev
COPY . .
CMD ["go", "test", "-race", "-v", "./..."]

# ------------------------------------------------------------------------------
# Stage 5: Development — Air live reload  (docker build --target dev .)
# ------------------------------------------------------------------------------
FROM golang:1.26-alpine AS dev

WORKDIR /app
RUN apk add --no-cache ca-certificates git && update-ca-certificates && \
    go install github.com/air-verse/air@latest
COPY go.mod go.sum ./
RUN go mod download && go mod verify
RUN mkdir -p tmp
EXPOSE 8080
CMD ["air", "-c", ".air.toml"]

# ------------------------------------------------------------------------------
# Stage 6: Playwright E2E tests  (docker build --target playwright-test .)
# ------------------------------------------------------------------------------
FROM mcr.microsoft.com/playwright:v1.45.0-jammy AS playwright-test

WORKDIR /app/tests
COPY tests/package*.json ./
RUN npm ci
COPY tests/ ./
RUN npx playwright install --with-deps chromium
CMD ["npx", "playwright", "test", "--reporter=list"]

# ------------------------------------------------------------------------------
# Stage 7: Dev container — dev + IDE tools  (target: devcontainer)
# ------------------------------------------------------------------------------
FROM dev AS devcontainer

RUN go install golang.org/x/tools/gopls@latest \
    && go install github.com/go-delve/delve/cmd/dlv@latest \
    && go install honnef.co/go/tools/cmd/staticcheck@latest \
    && apk add --no-cache sqlite

# ------------------------------------------------------------------------------
# Stage 9: Binary export  (docker build --target binary-export --output bin/ .)
# ------------------------------------------------------------------------------
FROM scratch AS binary-export
COPY --from=go_builder /easy-web-gpg /easy-web-gpg

# ------------------------------------------------------------------------------
# Stage 10: Runtime image  (default target)
# ------------------------------------------------------------------------------
FROM gcr.io/distroless/static:nonroot AS runtime

COPY --from=go_builder /easy-web-gpg /easy-web-gpg
COPY --from=go_builder /src/templates/ /templates/
COPY --from=go_builder /src/static/ /static/
COPY --from=go_builder /src/migrations/sql/ /migrations/sql/

EXPOSE 8080
ENTRYPOINT ["/easy-web-gpg"]
