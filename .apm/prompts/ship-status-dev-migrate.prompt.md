---
description: "Run Ship Status Dashboard database migrations"
---

# Ship Status dev — migrate

Use the **`run_migrate`** MCP tool (server: **`ship-status-dev`**). The tool runs:

```bash
go run ./cmd/migrate --dsn "$SHIP_STATUS_DSN"
```

If `SHIP_STATUS_DSN` is not set, it uses the dev default:

```text
postgres://postgres:password@localhost:5433/ship_status?sslmode=disable
```

Log: **`ship-status-dev-logs/run_migrate.log`**.

Note: migrations are also run automatically by `dashboard_serve` and during devcontainer setup.
