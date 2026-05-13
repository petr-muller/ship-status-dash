#!/bin/bash
set -eu

echo "==> Installing Go IDE tools..."
go install golang.org/x/tools/gopls@v0.18.1
go install github.com/go-delve/delve/cmd/dlv@v1.24.2
go install honnef.co/go/tools/cmd/staticcheck@v0.6.1

echo "==> Downloading Go module dependencies..."
go mod download

echo "==> Installing frontend dependencies..."
make npm

echo "==> Setting up MCP server venv..."
rm -rf mcp/.venv
python3.12 -m venv mcp/.venv
mcp/.venv/bin/pip install --upgrade pip -q
mcp/.venv/bin/pip install -r mcp/requirements.txt -q

echo "==> Running database migrations..."
go run ./cmd/migrate --dsn "$SHIP_STATUS_DSN"

echo "==> Dev environment ready."
