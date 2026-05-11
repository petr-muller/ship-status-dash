# Development Guide

This document describes how to set up and run the Ship Status Dashboard for local development and testing.

## Prerequisites

Before starting the dashboard, you must set up a PostgreSQL database:

1. Start a PostgreSQL container:
   ```bash
   podman run -d \
     --name ship-status-db \
     -e POSTGRES_PASSWORD=yourpassword \
     -p 5432:5432 \
     quay.io/enterprisedb/postgresql:latest
   ```

2. Create the database:
   ```bash
   podman exec ship-status-db psql -U postgres -c "CREATE DATABASE ship_status;"
   ```
## Backend Setup

### Dashboard

Start the dashboard server and mock oauth-proxy using the local development script:

```bash
./hack/local/dashboard/local-dev.sh "postgres://postgres:yourpassword@localhost:5432/ship_status?sslmode=disable"
```

This script:
- Starts the dashboard server on port 8080 (public route, no auth)
- Starts the mock oauth-proxy on port 8443 (protected route, requires basic auth)
- Sets up a user with credentials: `developer:password`
- Generates a temporary HMAC secret for request signing

**Slack Integration**: To enable Slack integration for outage reporting, set the `SLACK_BOT_TOKEN` environment variable before running the script:

```bash
SLACK_BOT_TOKEN=xoxb-your-token ./hack/local/dashboard/local-dev.sh "postgres://postgres:yourpassword@localhost:5432/ship_status?sslmode=disable"
```

### Component Monitor

Start the component-monitor using the local development script:

```bash
./hack/local/component-monitor/local-dev.sh
```

This script:
- Starts a mock-monitored-component on port 8081
- Starts Prometheus in a podman container on port 9090
- Starts the component-monitor with the local configuration

**Note**: The component-monitor expects the dashboard to be running on `http://localhost:8080`. Make sure to start the dashboard first.

### Authentication Architecture

The dashboard local development script mimics the production architecture using `mock-oauth-proxy`:

#### Setup

1. **Dashboard Server** (port 8080)
   - Runs with full authentication enabled (no `SKIP_AUTH`)
   - Public routes are accessible without authentication (same as production)
   - Protected routes require authentication via mock-oauth-proxy

2. **Mock OAuth Proxy** (port 8443)
   - Implements basic authentication (username/password)
   - Proxies to dashboard on `localhost:8080`
   - Adds same headers as production oauth-proxy
   - Signs requests with HMAC using shared secret
   - Default credentials: `developer:password`

#### Architecture

```
Public Route:    http://localhost:8080 → Dashboard (no auth required)
Protected Route: http://localhost:8443 → Mock OAuth Proxy → Dashboard (localhost:8080)
```

**Note**: The `SKIP_AUTH=1` environment variable is available for a simpler (unrecommended) development setup without oauth-proxy, but the local development script uses the full authentication flow with mock-oauth-proxy to accurately mirror production.

The mock-oauth-proxy:
- Accepts Basic Auth credentials
- Validates against [YAML user configuration](hack/local/dashboard/mock-oauth-proxy-config.yaml)
- Forwards authenticated requests to dashboard
- Adds `X-Forwarded-User`, `X-Forwarded-Email`, and signs with HMAC

#### Shared HMAC Secret

Both processes use the same HMAC secret:
- Generated automatically by `hack/local/dashboard/local-dev.sh`
- Stored in temporary file
- Passed to both dashboard and mock-oauth-proxy via `--hmac-secret-file`

## Frontend Setup

1. Navigate to the frontend directory:
   ```bash
   cd frontend
   ```

2. Install dependencies:
   ```bash
   npm ci --ignore-scripts
   ```

3. Set environment variables (or use the .env.development file) and start the development server:
   ```bash
   REACT_APP_PUBLIC_DOMAIN=http://localhost:8080 \
   REACT_APP_PROTECTED_DOMAIN=http://localhost:8443 \
   npm start
   ```

The frontend will be available at `http://localhost:3030`.

## End-to-End Tests

The e2e test suite (`make local-e2e`) tests both the dashboard and component-monitor using the same architecture as local development:

- Dynamically assigns ports (8080-8099 for dashboard, 8443-8499 for proxy)
- Starts dashboard server without `SKIP_AUTH` (full authentication enabled)
- Starts mock-oauth-proxy with test user credentials
- Starts mock-monitored-component for component-monitor testing
- Starts component-monitor with test configuration
- Tests both public and protected routes
- Uses same HMAC signature verification as production

### Running E2E Tests

Run the complete e2e test suite:

```bash
make local-e2e
```

The e2e script (`test/e2e/scripts/local-e2e.sh`):
- Starts a PostgreSQL test container using podman
- Runs database migrations
- Starts the dashboard server on a dynamically assigned port (8080-8099)
- Starts the mock oauth-proxy on a dynamically assigned port (8443-8499)
- Starts the mock-monitored-component on port 9000
- Starts the component-monitor with test configuration
- Executes both dashboard and component-monitor e2e test suites
- Cleans up all processes and containers on completion

The test suite includes:
- **Dashboard tests** (`TestE2E_Dashboard`): Tests API endpoints, outages, component status, and user authentication
- **Component-monitor tests** (`TestE2E_ComponentMonitor`): Tests component monitoring probes and integration with the dashboard

