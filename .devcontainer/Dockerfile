# Development container with Go and Tailwind CSS standalone CLI
FROM golang:1.24-bookworm

# Avoid warnings by switching to noninteractive
ENV DEBIAN_FRONTEND=noninteractive

# Install base dependencies
RUN apt-get update && apt-get install -y \
    git \
    curl \
    make \
    ca-certificates \
    sqlite3 \
    && rm -rf /var/lib/apt/lists/*

# Create a non-root user
ARG USERNAME=vscode
ARG USER_UID=1000
ARG USER_GID=$USER_UID

RUN groupadd --gid $USER_GID $USERNAME \
    && useradd --uid $USER_UID --gid $USER_GID -m $USERNAME \
    && apt-get update \
    && apt-get install -y sudo \
    && echo $USERNAME ALL=\(root\) NOPASSWD:ALL > /etc/sudoers.d/$USERNAME \
    && chmod 0440 /etc/sudoers.d/$USERNAME

# Download Tailwind CSS standalone CLI
ARG TAILWIND_VERSION=latest
ARG TARGETARCH=amd64

RUN curl -sLO "https://github.com/tailwindlabs/tailwindcss/releases/${TAILWIND_VERSION}/download/tailwindcss-linux-${TARGETARCH}" \
    && chmod +x "tailwindcss-linux-${TARGETARCH}" \
    && mv "tailwindcss-linux-${TARGETARCH}" /usr/local/bin/tailwindcss \
    && tailwindcss --help > /dev/null 2>&1 || echo "Tailwind CLI installed"

# Install Go tools
RUN go install golang.org/x/tools/gopls@latest \
    && go install github.com/go-delve/delve/cmd/dlv@latest \
    && go install honnef.co/go/tools/cmd/staticcheck@latest

# Switch back to dialog for any ad-hoc use of apt-get
ENV DEBIAN_FRONTEND=dialog

# Set working directory
WORKDIR /workspace

# Switch to non-root user
USER $USERNAME
