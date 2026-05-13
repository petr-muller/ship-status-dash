# API Endpoints

This document lists all API endpoints available in the SHIP Status Dashboard.

For authentication details, see [cmd/dashboard/README.md](cmd/dashboard/README.md).

## Endpoints

### Component Status

- **GET** `/api/status` - Get status of all components
  - **Public:** Yes

- **GET** `/api/status/{componentName}` - Get status of a specific component
  - **Public:** Yes

- **GET** `/api/status/{componentName}/{subComponentName}` - Get status of a specific sub-component
  - **Public:** Yes

### Component Information

- **GET** `/api/components` - Get list of all configured components
  - **Public:** Yes

- **GET** `/api/components/{componentName}` - Get information for a specific component
  - **Public:** Yes

- **GET** `/api/sub-components` - List sub-components; optional query parameters `componentName`, `tag`, and `team` (when more than one is given, a sub-component must match all of them)
  - **Public:** Yes

### Tags

- **GET** `/api/tags` - Get the configured tag definitions
  - **Public:** Yes

### Outages

- **GET** `/api/components/{componentName}/outages` - Get all outages for a component
  - **Public:** Yes

- **GET** `/api/components/{componentName}/{subComponentName}/outages` - Get all outages for a sub-component
  - **Public:** Yes

- **GET** `/api/outages/during` - Get outages overlapping a time window or instant (query params: `start` and/or `end` as RFC3339 or RFC3339Nano — at least one required; optional `componentName`, `subComponentName`, `tag`, `team` — `componentName`, `tag`, and `team` use the same AND rules as **GET** `/api/sub-components`; `subComponentName` is only allowed when `componentName` is set and narrows to that sub-component)
  - **Public:** Yes

- **GET** `/api/components/{componentName}/{subComponentName}/outages/{outageId}` - Get a specific outage by ID
  - **Public:** Yes

- **POST** `/api/components/{componentName}/{subComponentName}/outages` - Create a new outage
  - **Public:** No (requires authentication and component authorization)

- **PATCH** `/api/components/{componentName}/{subComponentName}/outages/{outageId}` - Update an existing outage
  - **Public:** No (requires authentication and component authorization)

- **DELETE** `/api/components/{componentName}/{subComponentName}/outages/{outageId}` - Delete an outage
  - **Public:** No (requires authentication and component authorization)

### User Information

- **GET** `/api/user` - Get authenticated user information
  - **Public:** No (requires authentication)

### Component Monitor Reports

- **POST** `/api/component-monitor/report` - Submit component monitor status report
  - **Public:** No (requires service account authentication)
