#!/bin/sh
set -eu

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

REQ_HASH=""
if command -v shasum >/dev/null 2>&1; then
    REQ_HASH=$(shasum -a 256 "$SCRIPT_DIR/requirements.txt" | cut -d' ' -f1)
elif command -v sha256sum >/dev/null 2>&1; then
    REQ_HASH=$(sha256sum "$SCRIPT_DIR/requirements.txt" | cut -d' ' -f1)
fi

HASH_FILE="$SCRIPT_DIR/.venv/.requirements.hash"
NEED_INSTALL=false

if [ ! -d "$SCRIPT_DIR/.venv" ]; then
    NEED_INSTALL=true
elif [ -n "$REQ_HASH" ] && { [ ! -f "$HASH_FILE" ] || [ "$(cat "$HASH_FILE" 2>/dev/null)" != "$REQ_HASH" ]; }; then
    NEED_INSTALL=true
fi

if [ "$NEED_INSTALL" = true ]; then
    python3.12 -m venv "$SCRIPT_DIR/.venv" 2>/dev/null || python3 -m venv "$SCRIPT_DIR/.venv"
    "$SCRIPT_DIR/.venv/bin/pip" install --upgrade pip -q
    "$SCRIPT_DIR/.venv/bin/pip" install -r "$SCRIPT_DIR/requirements.txt" -q
    if [ -n "$REQ_HASH" ]; then
        echo "$REQ_HASH" > "$HASH_FILE"
    fi
fi

exec "$SCRIPT_DIR/.venv/bin/python" "$SCRIPT_DIR/server.py"
