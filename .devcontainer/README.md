# Ship Status Dashboard — Dev Container

## Quick Start

Use the `/ship-status-dev-setup` slash command in Claude Code or Cursor to set up automatically.

## Prerequisites

- **Podman v4+** or **Docker** with Compose
- **devcontainer CLI**: `npm install -g @devcontainers/cli`

## Services

| Service | Container | Port | Notes |
|---------|-----------|------|-------|
| PostgreSQL | ship-status-postgres | 5433 | Auto-started by init-services.sh (host port 5433 → container 5432) |
| Dashboard API | (in devcontainer) | 8080 | Start with `/ship-status-dev-serve` |
| Mock OAuth Proxy | (in devcontainer) | 8443 | Started alongside dashboard |
| Vite Dev Server | (in devcontainer) | 3030 | Start with `/ship-status-dev-frontend` |
| Prometheus | (native, on demand) | 9090 | Started by `/ship-status-dev-app` |

## Manual Setup

### macOS (Podman)

```bash
podman machine init   # first time only
podman machine start
devcontainer up --workspace-folder . --docker-path podman
```

### macOS (Docker Desktop)

```bash
devcontainer up --workspace-folder .
```

### Linux (Podman)

```bash
systemctl --user enable --now podman.socket
devcontainer up --workspace-folder . --docker-path podman
```

### Linux (Docker)

```bash
devcontainer up --workspace-folder .
```

## Environment

Copy `.devcontainer/.env.example` to `.devcontainer/.env` and fill in any blank values.

## GCP Authentication

GCP credentials are mounted read-only from the host's `~/.config/gcloud`. Authenticate on the host:

```bash
gcloud auth application-default login
```

## Cleanup

### Podman

```bash
devcontainer down --workspace-folder .
podman stop ship-status-postgres && podman rm ship-status-postgres
podman network rm ship-status-net
```

### Docker

```bash
devcontainer down --workspace-folder .
docker stop ship-status-postgres && docker rm ship-status-postgres
docker network rm ship-status-net
```
