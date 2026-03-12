//go:build linux && integration

package main

import (
	"context"
	"testing"
	"time"

	"ship-status-dash/pkg/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These integration tests verify that the systemd prober can query D-Bus
// from inside a container with the host D-Bus socket volume-mounted.
//
// Build and run in a container:
//
//	go test -c -tags integration -o /tmp/systemd-prober-test ./cmd/component-monitor/
//	sudo podman run --rm \
//	  --security-opt label=disable \
//	  -v /run/dbus/system_bus_socket:/run/dbus/system_bus_socket:ro \
//	  -v /tmp/systemd-prober-test:/systemd-prober-test:ro \
//	  registry.access.redhat.com/ubi9/ubi-minimal:latest \
//	  /systemd-prober-test -test.v -test.run TestSystemdProber_Integration

func probeUnit(t *testing.T, unit string, severity types.Severity) ProbeResult {
	t.Helper()
	prober := NewSystemdProber("test-component", "test-subcomponent", unit, severity)

	results := make(chan ProbeResult, 1)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	prober.Probe(ctx, results)

	select {
	case result := <-results:
		return result
	case <-ctx.Done():
		t.Fatalf("timeout waiting for probe result for %s", unit)
		return ProbeResult{}
	}
}

func TestSystemdProber_Integration_ActiveUnit(t *testing.T) {
	// dbus.service is guaranteed to be active when D-Bus socket is available
	result := probeUnit(t, "dbus.service", types.SeverityDown)
	require.NoError(t, result.Error, "Probe should not error for dbus.service")
	assert.Equal(t, types.StatusHealthy, result.Status, "dbus.service should be active/healthy")
	require.Len(t, result.Reasons, 1)
	assert.Equal(t, types.CheckTypeSystemd, result.Reasons[0].Type)
	assert.Equal(t, "dbus.service", result.Reasons[0].Check)
	assert.Equal(t, "ActiveState: active", result.Reasons[0].Results)
}

func TestSystemdProber_Integration_InactiveUnit(t *testing.T) {
	// Nonexistent units return "inactive" via D-Bus rather than erroring
	result := probeUnit(t, "nonexistent-unit-12345.service", types.SeverityDown)
	require.NoError(t, result.Error, "Probe should not error for nonexistent unit (systemd returns inactive)")
	assert.Equal(t, types.StatusDown, result.Status, "Nonexistent unit should report Down status")
	require.Len(t, result.Reasons, 1)
	assert.Equal(t, "ActiveState: inactive", result.Reasons[0].Results)
}

func TestSystemdProber_Integration_SeverityMapping(t *testing.T) {
	tests := []struct {
		name           string
		severity       types.Severity
		expectedStatus types.Status
	}{
		{
			name:           "Down",
			severity:       types.SeverityDown,
			expectedStatus: types.StatusDown,
		},
		{
			name:           "Degraded",
			severity:       types.SeverityDegraded,
			expectedStatus: types.StatusDegraded,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := probeUnit(t, "nonexistent-unit-12345.service", tt.severity)
			require.NoError(t, result.Error)
			assert.Equal(t, tt.expectedStatus, result.Status)
		})
	}
}
