#!/bin/sh
set -eu

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

if [ ! -d "$SCRIPT_DIR/.venv" ]; then
    python3.12 -m venv "$SCRIPT_DIR/.venv" 2>/dev/null || python3 -m venv "$SCRIPT_DIR/.venv"
    "$SCRIPT_DIR/.venv/bin/pip" install --upgrade pip -q
    "$SCRIPT_DIR/.venv/bin/pip" install -r "$SCRIPT_DIR/requirements.txt" -q
fi

exec "$SCRIPT_DIR/.venv/bin/python" "$SCRIPT_DIR/server.py"
