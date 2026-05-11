---
description: "Common dev commands for database migration, linting, and testing"
applyTo: "**"
---

### Database migration

Run migrations: `go run ./cmd/migrate --dsn "$SHIP_STATUS_DSN"`

If `SHIP_STATUS_DSN` is not set, use the dev default: `postgres://postgres:password@localhost:5433/ship_status?sslmode=disable`

### Linting

Run lint: `make lint`

The lint script uses `golangci-lint` directly when available, falling back to a container otherwise.

### Testing

Run unit tests: `make test`

Run e2e tests: `make local-e2e`

### Frontend

Install dependencies: `cd frontend && npm ci --no-audit --ignore-scripts`

Start dev server: `cd frontend && npm run start`

Run lint/format: `cd frontend && npx eslint . --fix && npx prettier --write .`
