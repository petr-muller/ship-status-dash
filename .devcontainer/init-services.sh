#!/bin/sh
set -eu

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
if [ ! -f "$SCRIPT_DIR/.env" ]; then
    cp "$SCRIPT_DIR/.env.example" "$SCRIPT_DIR/.env"
fi

podman network create ship-status-net 2>/dev/null || true

if ! podman start ship-status-postgres 2>/dev/null; then
    podman rm -f ship-status-postgres 2>/dev/null || true
    podman run -d --name ship-status-postgres \
        --network ship-status-net \
        -e POSTGRES_PASSWORD=password \
        -e POSTGRES_DB=ship_status \
        -p 127.0.0.1:5433:5432 \
        quay.io/enterprisedb/postgresql \
        -c listen_addresses='*'
fi

echo "Waiting for PostgreSQL..."
pg_ready=false
for i in $(seq 1 30); do
    if podman exec ship-status-postgres pg_isready -U postgres >/dev/null 2>&1; then
        pg_ready=true
        break
    fi
    sleep 1
done
if [ "$pg_ready" = false ]; then
    echo "ERROR: PostgreSQL did not become ready within 30 seconds."
    exit 1
fi

echo "Services ready."
