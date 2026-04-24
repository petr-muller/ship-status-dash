# Component Monitor

The component-monitor is a service that periodically probes sub-components to detect outages and report their status to the dashboard API.

## Overview

The component-monitor supports these types of monitoring:

1. **HTTP Monitoring**: Probes HTTP endpoints and checks for expected status codes
2. **Prometheus Monitoring**: Executes Prometheus queries (both instant and range queries) to check component health
3. **JUnit Monitoring**: Fetches a Prow canary’s JUnit XML from GCS (or the GCSweb URL style) and derives health from that file
4. **Systemd Monitoring** (if enabled in config): Probes a systemd unit on a host

## Architecture

The component-monitor runs as a standalone service that:
- Loads configuration from a YAML file
- Creates probers for each configured component/sub-component
- Periodically executes probes at a configured frequency
- Sends probe results to the dashboard API via HTTP POST requests
- Does not expose any HTTP endpoints itself (only makes outbound requests)

## JUnit monitor (`junit_monitor`)

Use this for Prow jobs that write a canary JUnit file into `test-platform-results` (or another GCS bucket), under `logs/<job_name>/`.

If omitted, **junit_monitor.severity** defaults to **Degraded**.

**What the prober reads (no build-cluster login):**

1. **`latest-build.txt`** at `logs/<job_name>/` — a single line with the current Prow build id.
2. **`started.json`** for that id — if the start time is older than `max_age`, the probe reports unhealthy (stale canary), regardless of JUnit.
3. **`artifacts/junit_canary.xml`** for that id — parsed for total tests and failed `<testcase>` names (only `<failure/>` is treated as a failure in the current implementation).

**Single run (`history_runs: 1`, default):** the latest build (from `latest-build.txt`) is the only JUnit read. Failing = zero total JUnit tests, or any failed testcase. `failed_runs_threshold` is ignored (internally 1).

**History (`history_runs` N > 1):** the prober takes up to **N** recent build ids (GCS list API, merged with `latest-build.txt`, then sorted), fetches JUnit for each, and classifies every run. It then applies **`failed_runs_threshold` (Y) with the same failure pattern** — *not* “Y arbitrary red runs in N”:

- A **failure pattern** is the sorted set of failed testcase `name` values, or a shared bucket for runs with **no JUnit tests** (zero total tests in the sum of suite `tests` attributes).
- Let **K** = the size of the **largest** group of runs in the last N that share the **identical** pattern.
- If **K ≥ Y**, the sub-component is unhealthy for this prober. Otherwise it is healthy.
- So **Y=1** in a window of several runs: any one red run (with its own pattern) gives **K=1** for that pattern, so 1 ≥ 1 → reported unhealthy. **Y=2** requires the **same** pattern on at least two runs (e.g. the same canary test names flaking), not one failure on run A and a different failure on run B.

**Example:**

```yaml
junit_monitor:
  job_name: "periodic-build-farm-canary-build11"
  gcs_bucket: "test-platform-results"  # default if omitted
  max_age: "2h"                        # from started.json of latest only
  severity: "Degraded"
  artifact_url_style: "gcs"            # or "gcsweb" for the app.ci GCSweb host
  history_runs: 5
  failed_runs_threshold: 3            # 3+ runs in last 5 must share one failure pattern
```

## Configuration

The component-monitor is configured via command-line flags and a YAML configuration file:

**Command-Line Flags:**
- `--config-path` (required): Path to the component monitor configuration file (YAML)
- `--dashboard-url`: Base URL of the dashboard API
- `--name` (required): Name identifier for this component monitor instance
- `--kubeconfig-dir` (optional): Path to a directory containing kubeconfig files for different clusters
- `--report-auth-token-file` (required): Path to file containing bearer token for authenticating report requests to the dashboard API

**Configuration File Structure:**
```yaml
frequency: 5m
components:
  - component_slug: "prow"
    sub_component_slug: "deck"
    http_monitor:
      url: "https://prow.ci.openshift.org/"
      code: 200
      retry_after: 4m
      severity: "Down"  # Optional: severity when probe fails (defaults to "Down")
    prometheus_monitor:
      prometheus_location:
        cluster: "app.ci"
        namespace: "openshift-monitoring"
        route: "thanos-querier"
      queries:
        - query: "up{job=\"deck\"} == 1"
          failure_query: "up{job=\"deck\"}"
          duration: "5m"
          step: "30s"
          severity: "Down"  # Optional: severity when query fails (defaults to "Down")
```

**Prometheus Query Configuration:**
- `query`: The Prometheus query to run (must return results for healthy state)
- `failure_query`: Optional query to run when the main query fails, providing additional context
- `duration`: Optional duration string (e.g., `"5m"`, `"30s"`). If provided, the query will be executed as a range query
- `step`: Optional resolution for range queries (e.g., `"30s"`, `"15s"`). If not provided, a default step is calculated based on the duration
- `severity`: Optional severity level when the query fails. Valid values: `"Down"`, `"Degraded"`, `"CapacityExhausted"`, `"Suspected"`. Defaults to `"Down"` if not specified

**HTTP Monitor Configuration:**
- `url`: The URL to probe
- `code`: The expected HTTP status code
- `retry_after`: Duration to wait before retrying the probe when the status code is not as expected
- `severity`: Optional severity level when the probe fails. Valid values: `"Down"`, `"Degraded"`, `"CapacityExhausted"`, `"Suspected"`. Defaults to `"Down"` if not specified

## Prometheus Location Configuration

The `prometheus_location` field is a struct that specifies how to connect to a Prometheus instance. It can be configured in two ways:

### 1. URL-based (for local development and e2e testing)

Use the `url` field to connect directly to Prometheus without authentication:

```yaml
prometheus_monitor:
  prometheus_location:
    url: "http://localhost:9090"  # Direct URL to Prometheus
  queries:
    - query: "up{job=\"test\"} == 1"
```

**Requirements:**
- Only `url` field should be set (mutually exclusive with `cluster`, `namespace`, `route`)
- Do not provide `--kubeconfig-dir` flag
- The component-monitor connects directly to Prometheus without authentication

### 2. Cluster-based (for production deployments)

Use `cluster`, `namespace`, and `route` fields to connect via OpenShift Routes:

```yaml
prometheus_monitor:
  prometheus_location:
    cluster: "app.ci"                    # Cluster name (must match kubeconfig filename)
    namespace: "openshift-monitoring"   # Namespace where the Prometheus route exists
    route: "thanos-querier"             # Name of the OpenShift Route to Prometheus
  queries:
    - query: "up{job=\"deck\"} == 1"
      duration: "5m"
      step: "30s"
```

**Requirements:**
- All three fields (`cluster`, `namespace`, `route`) must be set together
- `url` field must not be set (mutually exclusive)
- Provide `--kubeconfig-dir` flag pointing to a directory with kubeconfig files
- Each kubeconfig file should be named after the cluster with a `.config` suffix (e.g., `app.ci.config`)

**How it works:**
1. Loads the kubeconfig file for the specified cluster
2. Uses the kubeconfig's authentication (bearer token, TLS certificates)
3. Discovers the Prometheus route via OpenShift Routes API using the provided namespace and route name
4. Creates an authenticated Prometheus client

### 3. In-cluster configuration

Use `"in-cluster"` as the cluster name to use the in-cluster Kubernetes configuration:

```yaml
prometheus_monitor:
  prometheus_location:
    cluster: "in-cluster"              # Special cluster name for in-cluster config
    namespace: "openshift-monitoring"   # Namespace where the Prometheus route exists
    route: "thanos-querier"             # Name of the OpenShift Route to Prometheus
  queries:
    - query: "up{job=\"deck\"} == 1"
      duration: "5m"
      step: "30s"
```

**Requirements:**
- Set `cluster` to `"in-cluster"`
- All three fields (`cluster`, `namespace`, `route`) must be set together
- `url` field must not be set (mutually exclusive)
- Do not provide `--kubeconfig-dir` flag (uses in-cluster service account credentials)

**How it works:**
1. Uses the in-cluster Kubernetes configuration (service account token and CA certificate)
2. Discovers the Prometheus route via OpenShift Routes API using the provided namespace and route name
3. Creates an authenticated Prometheus client

**Note:** Options 2 (cluster-based) and 3 (in-cluster) can be used together within the same deployment. You can configure some components to use cluster-based configuration (with kubeconfig files) and others to use in-cluster configuration, all in the same component-monitor instance.

## Service Account Authentication

The component-monitor authenticates to the dashboard API using OpenShift ServiceAccount bearer tokens:

1. **Token Configuration**: The component-monitor reads a bearer token from a file specified via the `--report-auth-token-file` command-line flag
2. **Request Authentication**: When sending reports to the dashboard API, the component-monitor includes the token in the `Authorization` header as `Bearer <token>`
3. **OAuth Proxy Processing**: In production, requests go through the OAuth proxy which:
   - Validates the bearer token
   - Extracts the service account name (e.g., `system:serviceaccount:ship-status:component-monitor`)
   - Sets the `X-Forwarded-User` header to the service account name
   - Signs the request with HMAC and adds the `GAP-Signature` header
4. **Dashboard Authorization**: The dashboard validates that:
   - The HMAC signature is valid
   - The service account (from `X-Forwarded-User`) is listed as an owner of the component in the dashboard configuration
   - Only service accounts that are owners of a component can report status for that component's sub-components

**Component Configuration**: Components must have the service account listed in their `owners` section with a `service_account` field. For example, in the Dashboard configuration:
```yaml
components:
  - slug: "prow"
    owners:
      - service_account: "system:serviceaccount:ship-status:component-monitor"
```

## How It Works

1. The component-monitor loads the configuration file and validates all settings
2. For each configured component, it creates appropriate probers (HTTP, Prometheus, JUnit, systemd, etc.)
3. At the configured frequency, it runs all probes concurrently
4. Probe results are aggregated and sent to the dashboard API via POST to `/api/component-monitor/report` with bearer token authentication
5. The dashboard API processes the reports and creates/resolves outages accordingly

## Status Reporting

The component-monitor reports status for each sub-component based on probe results. The status levels are configurable per query or monitor via the `severity` field.

### Available Severity Levels

When a probe fails, it reports a status based on the configured severity level:

- **Down**: Most critical severity level. Indicates the component is completely unavailable
- **Degraded**: Indicates the component is functioning but with reduced performance or capabilities
- **CapacityExhausted**: Indicates the component is unavailable due to resource exhaustion (e.g., no available cloud accounts)

If `severity` is not specified for a query or monitor, it defaults to `"Down"`.

### Status Determination

When multiple queries fail for the same sub-component, the most critical severity (highest level) is used:
- `Down` (level 4) > `Degraded` (level 3) > `CapacityExhausted` (level 2) > `Suspected` (level 1) is only used when the sub-component is configured to require `confirmation`

### Examples

**Example 1: HTTP monitor with Degraded severity**
```yaml
http_monitor:
  url: "https://example.com/api"
  code: 200
  retry_after: 4m
  severity: "Degraded"  # Reports Degraded status if probe fails
```

**Example 2: Prometheus queries with different severities**
```yaml
prometheus_monitor:
  queries:
    - query: "up{job=\"critical\"} == 1"
      severity: "Down"  # Critical failure
    - query: "response_time_seconds > 1"
      severity: "Degraded"  # Performance issue
    - query: "available_resources == 0"
      severity: "CapacityExhausted"  # Resource exhaustion
```

If the first query fails, the status will be `Down` (most critical). If only the second query fails, the status will be `Degraded`.

## Range Queries

When a `duration` is specified for a Prometheus query, the component-monitor executes it as a range query:
- The query looks back over the specified duration from the current time
- The `step` parameter controls the resolution (time between data points)
- If `step` is not provided, a default is calculated:
  - For durations ≤ 1 hour: 15 seconds
  - For longer durations: duration / 250
- Range queries return a `Matrix` type, which is evaluated by checking if any time series have data points

## Error Handling

- If a probe fails to execute (network error, etc.), an error is logged but the probe continues
- If the dashboard API is unavailable, errors are logged and the component-monitor continues running
- Configuration validation errors (invalid durations, steps, or prometheus locations) cause the component-monitor to exit immediately

## Configuration Testing

To test component-monitor configuration in dry-run mode, see [`hack/component-monitor-dry-run/`](../../hack/component-monitor-dry-run/README.md), and the `component-monitor-dry-run` make target.
