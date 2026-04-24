# Ship Status Dashboard

SHIP Status and Availability Dashboard monitor

## Project Structure

This project consists of multiple components:

- **Dashboard**: Web application for viewing and managing component status, availability, and outages
  - Backend: Go server (`cmd/dashboard`)
  - Frontend: React application (`frontend/`)
- **Component Monitor**: Monitoring service that periodically probes components and reports their status to the dashboard
  - Go service (`cmd/component-monitor`)
  - Supports HTTP, Prometheus, and JUnit (Prow GCS canary) monitoring; see [`cmd/component-monitor/README.md`](cmd/component-monitor/README.md)

For local development setup, see [DEVELOPMENT.md](DEVELOPMENT.md).

## Dashboard Component

The dashboard is a web application for viewing and managing component status, availability, and outages. It consists of:
- Backend: Go server (`cmd/dashboard`)
- Frontend: React application (`frontend/`)

For detailed documentation, see [`cmd/dashboard/README.md`](cmd/dashboard/README.md).

## Component Monitor

The component-monitor is a service that periodically probes sub-components to detect outages and report their status to the dashboard API.
  - Go service (`cmd/component-monitor`)
  - Supports HTTP, Prometheus, and JUnit monitoring

For detailed documentation, see [`cmd/component-monitor/README.md`](cmd/component-monitor/README.md).