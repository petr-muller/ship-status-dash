package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/google/go-cmp/cmp"

	"ship-status-dash/pkg/testhelper"
	"ship-status-dash/pkg/types"
)

func TestEscapeUnitName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"sshd.service", "sshd_2eservice"},
		{"my-service.service", "my_2dservice_2eservice"},
		{"simple", "simple"},
		{"a.b-c_d", "a_2eb_2dc_5fd"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := escapeUnitName(tt.input)
			if got != tt.expected {
				t.Errorf("escapeUnitName(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

// fakeDBusConnector is a test double for dbusConnector that returns a preconfigured error.
type fakeDBusConnector struct {
	err error
}

func (f *fakeDBusConnector) SystemBus() (*dbus.Conn, error) {
	return nil, f.err
}

func TestSystemdProber_Probe_ConnectionError(t *testing.T) {
	connErr := errors.New("connection refused")
	prober := NewSystemdProber("test-component", "test-subcomponent", "test.service", types.SeverityDown)
	prober.connector = &fakeDBusConnector{err: connErr}

	results := make(chan ProbeResult, 1)
	ctx := context.Background()

	prober.Probe(ctx, results)

	select {
	case result := <-results:
		if result.Error == nil {
			t.Fatal("expected error but got nil")
		}
		wantErr := errors.New("error running systemd probe, for component: test-component sub-component test-subcomponent. unit: test.service. error: failed to connect to system D-Bus: connection refused")
		if diff := cmp.Diff(wantErr, result.Error, testhelper.EquateErrorMessage); diff != "" {
			t.Errorf("error mismatch (-want +got):\n%s", diff)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for result")
	}
}

func TestSystemdProber_DefaultSeverity(t *testing.T) {
	prober := NewSystemdProber("comp", "sub", "test.service", "")
	if prober.severity != types.SeverityDown {
		t.Errorf("expected default severity Down, got %s", prober.severity)
	}
}

func TestSystemdProber_CustomSeverity(t *testing.T) {
	prober := NewSystemdProber("comp", "sub", "test.service", types.SeverityDegraded)
	if prober.severity != types.SeverityDegraded {
		t.Errorf("expected severity Degraded, got %s", prober.severity)
	}
}
