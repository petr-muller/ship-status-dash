---
description: "Run lint and unit tests (full local CI suite, excluding e2e)"
---

# Ship Status dev — tests

Use the **`run_tests`** MCP tool (server: **`ship-status-dev`**). This runs two steps in order, stopping if any step fails:

1. **Lint** — `CI=true make lint`
   - `CI=true` makes `hack/go-lint.sh` use the locally installed `golangci-lint` instead of spawning a container.

2. **Unit tests** — `make test`
   - Runs Go tests via gotestsum.

This does **not** run e2e tests. Use `/ship-status-dev-e2e` for that.

Logs: **`ship-status-dev-logs/run_lint.log`** and **`ship-status-dev-logs/run_test.log`**.
