FROM golang:1.24-bullseye

# Install system dependencies
RUN apt-get update \
  && apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    git \
    build-essential \
    make \
    sqlite3 \
    pkg-config \
    libsqlite3-dev \
  && rm -rf /var/lib/apt/lists/*

# Node.js and pnpm removed from the devcontainer configuration

# Install Delve (Go debugger)
RUN go install github.com/go-delve/delve/cmd/dlv@latest || true

ENV GOPATH=/go
ENV PATH=$GOPATH/bin:/usr/local/go/bin:$PATH

WORKDIR /workspace

CMD ["/bin/bash"]
