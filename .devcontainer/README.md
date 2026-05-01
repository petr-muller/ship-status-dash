# Ship Status Dashboard — Dev Container

## Quick Start

Use the `/ship-status-dev-setup` slash command in Claude Code or Cursor to set up automatically.

## Prerequisites

- **Podman v4+** (or Docker)
- **devcontainer CLI**: `npm install -g @devcontainers/cli`

## Services

| Service | Container | Port | Notes |
|---------|-----------|------|-------|
| PostgreSQL | ship-status-postgres | 5433 | Auto-started by init-services.sh (host port 5433 → container 5432) |
| Dashboard API | (in devcontainer) | 8080 | Start with `/ship-status-dev-serve` |
| Mock OAuth Proxy | (in devcontainer) | 8443 | Started alongside dashboard |
| Vite Dev Server | (in devcontainer) | 3000 | Start with `/ship-status-dev-frontend` |
| Prometheus | (podman, on demand) | 9090 | Started by `/ship-status-dev-monitor` |

## Manual Setup

### macOS

```bash
podman machine init   # first time only
podman machine start
devcontainer up --workspace-folder . --docker-path podman
```

### Linux

```bash
systemctl --user enable --now podman.socket
devcontainer up --workspace-folder . --docker-path podman
```

## Environment

Copy `.devcontainer/.env.example` to `.devcontainer/.env` and fill in any blank values.

## GCP Authentication

GCP credentials are mounted read-only from the host's `~/.config/gcloud`. Authenticate on the host:

```bash
gcloud auth application-default login
```

## Known Limitations

- `make local-e2e` requires Podman on the host (not inside the devcontainer) since it starts its own PostgreSQL container.
- `make lint` outside `CI=true` mode spawns a containerized linter, which doesn't work with nested Podman.

## Cleanup

```bash
devcontainer down --workspace-folder .
podman stop ship-status-postgres && podman rm ship-status-postgres
podman network rm ship-status-net
```
