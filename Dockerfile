### Multi-stage Dockerfile
### Stage: build Go binary and embed static assets (expecting static/dist to be present in context)
FROM golang:1.24-alpine AS go_builder
WORKDIR /src

# Install git (for potential module fetching) and ca-certificates
RUN apk add --no-cache ca-certificates git

# Copy Go modules and source
COPY go.mod go.sum* ./
RUN go env -w GOPROXY=https://proxy.golang.org
COPY . .

ENV CGO_ENABLED=0 GOOS=linux GOARCH=amd64
RUN go build -ldflags="-s -w" -o /web-gpg

### Final image: minimal
FROM gcr.io/distroless/static:nonroot
# ensure older binary name still exists if referenced; keep web-gpg as main
COPY --from=go_builder /web-gpg /web-gpg
# Copy templates, static assets and migrations into known absolute paths
COPY --from=go_builder /src/templates /templates
COPY --from=go_builder /src/static/dist /static/dist
COPY --from=go_builder /src/migrations/sql /migrations/sql

EXPOSE 8080
ENTRYPOINT ["/web-gpg"]
