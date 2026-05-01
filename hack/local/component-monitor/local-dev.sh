#!/bin/bash

set -e

BACKGROUND=false
NATIVE_PROMETHEUS=false
while [ $# -gt 0 ]; do
  case "$1" in
    --background) BACKGROUND=true; shift ;;
    --native-prometheus) NATIVE_PROMETHEUS=true; shift ;;
    *) break ;;
  esac
done

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
cd "$PROJECT_ROOT"

LOG_DIR="${SHIP_STATUS_LOG_DIR:-/tmp}"
mkdir -p "$LOG_DIR"
PROMETHEUS_CONTAINER_NAME="prometheus-local-dev"
DASHBOARD_URL="${DASHBOARD_URL:-http://localhost:8443}"
PROMETHEUS_DATA_DIR="${PROMETHEUS_DATA_DIR:-/tmp/prometheus-local-dev}"

EXIT_CODE=0
cleanup() {
  EXIT_CODE="${EXIT_CODE:-$?}"
  set +e
  if [ "$BACKGROUND" = true ]; then
    exit "$EXIT_CODE"
  fi
  echo ""
  echo "Cleaning up..."
  if [ ! -z "$MOCK_COMPONENT_PID" ]; then
    kill -TERM $MOCK_COMPONENT_PID 2>/dev/null || true
    sleep 1
    kill -KILL $MOCK_COMPONENT_PID 2>/dev/null || true
  fi
  if [ ! -z "$PROMETHEUS_PID" ]; then
    kill -TERM $PROMETHEUS_PID 2>/dev/null || true
    sleep 1
    kill -KILL $PROMETHEUS_PID 2>/dev/null || true
  fi
  if [ "$NATIVE_PROMETHEUS" = false ]; then
    if podman ps -a --format "{{.Names}}" | grep -q "^${PROMETHEUS_CONTAINER_NAME}$"; then
      podman stop "$PROMETHEUS_CONTAINER_NAME" > /dev/null 2>&1 || true
      podman rm "$PROMETHEUS_CONTAINER_NAME" > /dev/null 2>&1 || true
    fi
  fi
  if [ ! -z "$COMPONENT_MONITOR_PID" ]; then
    kill -TERM $COMPONENT_MONITOR_PID 2>/dev/null || true
    sleep 1
    kill -KILL $COMPONENT_MONITOR_PID 2>/dev/null || true
  fi
  if [ ! -z "$COMPONENT_MONITOR_TOKEN" ] && [ -f "$COMPONENT_MONITOR_TOKEN" ]; then
    rm -f "$COMPONENT_MONITOR_TOKEN"
  fi
  echo "Cleanup complete"
  exit "$EXIT_CODE"
}

trap cleanup EXIT

echo "Starting mock-monitored-component on port 8081..."
MOCK_COMPONENT_LOG="$LOG_DIR/mock-component-local-dev.log"
go run ./cmd/mock-monitored-component --port 8081 > "$MOCK_COMPONENT_LOG" 2>&1 &
MOCK_COMPONENT_PID=$!

echo "Waiting for mock-monitored-component to be ready..."
for i in {1..10}; do
  if curl -s http://localhost:8081/health > /dev/null 2>&1; then
    echo "Mock-monitored-component is ready on port 8081 (pid $MOCK_COMPONENT_PID)"
    break
  fi
  if [ $i -eq 10 ]; then
    echo "Mock-monitored-component failed to start"
    exit 1
  fi
  sleep 1
done

PROMETHEUS_CONFIG_PATH="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/prometheus.yml"
PROMETHEUS_LOG="$LOG_DIR/prometheus-local-dev.log"

if [ "$NATIVE_PROMETHEUS" = true ]; then
  echo "Starting Prometheus (native)..."
  mkdir -p "$PROMETHEUS_DATA_DIR"

  prometheus \
    --config.file="$PROMETHEUS_CONFIG_PATH" \
    --storage.tsdb.path="$PROMETHEUS_DATA_DIR" \
    --web.listen-address=":9090" \
    --web.enable-lifecycle \
    > "$PROMETHEUS_LOG" 2>&1 &
  PROMETHEUS_PID=$!
else
  echo "Starting Prometheus in podman container..."
  if podman ps -a --format "{{.Names}}" | grep -q "^${PROMETHEUS_CONTAINER_NAME}$"; then
    podman rm -f "$PROMETHEUS_CONTAINER_NAME" > /dev/null 2>&1 || true
  fi

  # The checked-in config uses localhost, which won't work from inside a container.
  # Generate a temp config that uses the podman host gateway.
  PODMAN_PROMETHEUS_CONFIG=$(mktemp)
  sed 's/localhost:8081/host.containers.internal:8081/g' "$PROMETHEUS_CONFIG_PATH" > "$PODMAN_PROMETHEUS_CONFIG"

  podman run -d \
    --name "$PROMETHEUS_CONTAINER_NAME" \
    -p 9090:9090 \
    -v "$PODMAN_PROMETHEUS_CONFIG:/etc/prometheus/prometheus.yml:ro" \
    quay.io/prometheus/prometheus:latest \
    --config.file=/etc/prometheus/prometheus.yml \
    --storage.tsdb.path=/prometheus \
    --web.console.libraries=/usr/share/prometheus/console_libraries \
    --web.console.templates=/usr/share/prometheus/consoles \
    --web.enable-lifecycle \
    > /dev/null 2>&1
fi

echo "Waiting for Prometheus to complete initial scrape..."
for i in {1..60}; do
  if curl -s "http://localhost:9090/api/v1/query?query=success_rate" | grep -q "success_rate"; then
    echo "Prometheus has completed initial scrape"
    break
  fi
  if [ $i -eq 60 ]; then
    echo "Prometheus failed to complete initial scrape within 60 seconds"
    if [ "$NATIVE_PROMETHEUS" = true ]; then
      cat "$PROMETHEUS_LOG" 2>/dev/null || true
    else
      podman logs "$PROMETHEUS_CONTAINER_NAME" 2>/dev/null || true
    fi
    exit 1
  fi
  sleep 1
done

echo "Creating component-monitor auth token file..."
COMPONENT_MONITOR_TOKEN=$(mktemp)
echo "component-monitor-sa-token" > "$COMPONENT_MONITOR_TOKEN"

echo "Starting component-monitor..."
COMPONENT_MONITOR_LOG="$LOG_DIR/component-monitor-local-dev.log"
echo "Component-monitor logs: $COMPONENT_MONITOR_LOG"

go run ./cmd/component-monitor --config-path hack/local/component-monitor/config.yaml --dashboard-url "$DASHBOARD_URL" --name local-component-monitor --report-auth-token-file "$COMPONENT_MONITOR_TOKEN" > "$COMPONENT_MONITOR_LOG" 2>&1 &
COMPONENT_MONITOR_PID=$!

echo ""
echo "Component-monitor stack is running!"
echo "  Mock component: http://localhost:8081 (pid $MOCK_COMPONENT_PID)"
if [ "$NATIVE_PROMETHEUS" = true ]; then
  echo "  Prometheus: http://localhost:9090 (pid $PROMETHEUS_PID)"
else
  echo "  Prometheus: http://localhost:9090 (container $PROMETHEUS_CONTAINER_NAME)"
fi
echo "  Component monitor: pid $COMPONENT_MONITOR_PID"
echo ""
echo "Log files:"
echo "  Mock component: $MOCK_COMPONENT_LOG"
echo "  Prometheus: $PROMETHEUS_LOG"
echo "  Component monitor: $COMPONENT_MONITOR_LOG"
echo "  Auth token: $COMPONENT_MONITOR_TOKEN"

if [ "$BACKGROUND" = true ]; then
  echo ""
  if [ "$NATIVE_PROMETHEUS" = true ]; then
    echo "PIDs: mock_component=$MOCK_COMPONENT_PID prometheus=$PROMETHEUS_PID component_monitor=$COMPONENT_MONITOR_PID"
  else
    echo "PIDs: mock_component=$MOCK_COMPONENT_PID component_monitor=$COMPONENT_MONITOR_PID"
  fi
  exit 0
fi

echo ""
echo "Press Ctrl+C to stop"

set +e
while kill -0 $COMPONENT_MONITOR_PID 2>/dev/null; do
  sleep 1
done
exit 0
