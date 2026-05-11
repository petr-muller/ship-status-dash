FROM registry.access.redhat.com/ubi9/ubi:latest

ARG GOLANGCI_LINT_VERSION=2.10.1

RUN dnf module enable nodejs:20 -y && \
    dnf install -y --allowerasing \
    git \
    make \
    jq \
    tar \
    gzip \
    wget \
    vim-minimal \
    findutils \
    procps-ng \
    sudo \
    go \
    npm \
    python3.12 \
    libstdc++ \
    postgresql \
    gettext \
    lsof \
    && dnf clean all

RUN ARCH=$(uname -m) && \
    if [ "$ARCH" = "aarch64" ]; then LINT_ARCH="arm64"; else LINT_ARCH="amd64"; fi && \
    rpm -i "https://github.com/golangci/golangci-lint/releases/download/v${GOLANGCI_LINT_VERSION}/golangci-lint-${GOLANGCI_LINT_VERSION}-linux-${LINT_ARCH}.rpm"

RUN groupadd -g 1000 vscode && \
    useradd -u 1000 -g 1000 -m -s /bin/bash vscode && \
    echo "vscode ALL=(ALL) NOPASSWD:ALL" > /etc/sudoers.d/vscode

ENV GOPATH=/home/vscode/go
ENV PATH="/home/vscode/.local/bin:${GOPATH}/bin:/usr/local/go/bin:${PATH}"

USER vscode

RUN go install gotest.tools/gotestsum@latest

ARG PROMETHEUS_VERSION=3.4.1
RUN ARCH=$(uname -m) && \
    if [ "$ARCH" = "aarch64" ]; then PROM_ARCH="arm64"; else PROM_ARCH="amd64"; fi && \
    wget -q "https://github.com/prometheus/prometheus/releases/download/v${PROMETHEUS_VERSION}/prometheus-${PROMETHEUS_VERSION}.linux-${PROM_ARCH}.tar.gz" -O /tmp/prometheus.tar.gz && \
    tar -xzf /tmp/prometheus.tar.gz -C /tmp && \
    sudo mv /tmp/prometheus-${PROMETHEUS_VERSION}.linux-${PROM_ARCH}/prometheus /usr/local/bin/prometheus && \
    sudo mv /tmp/prometheus-${PROMETHEUS_VERSION}.linux-${PROM_ARCH}/promtool /usr/local/bin/promtool && \
    rm -rf /tmp/prometheus*

# Cursor's install script calls base64 -D (macOS flag) but GNU coreutils uses -d
RUN sudo cp /usr/bin/base64 /usr/bin/base64.gnu && \
    printf '#!/bin/sh\nfor a in "$@"; do shift; case "$a" in -D) set -- "$@" -d;; *) set -- "$@" "$a";; esac; done\nexec /usr/bin/base64.gnu "$@"\n' | sudo tee /usr/bin/base64 > /dev/null && \
    sudo chmod +x /usr/bin/base64

ARG CLAUDE_INSTALLER_VERSION=latest
RUN curl -fsSL https://claude.ai/install.sh | sh

WORKDIR /workspace
