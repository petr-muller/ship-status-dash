package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/godbus/dbus/v5"

	"ship-status-dash/pkg/types"
)

// dbusConnector is an interface for creating D-Bus connections, allowing for testing.
type dbusConnector interface {
	SystemBus() (*dbus.Conn, error)
}

// realDBusConnector connects to the real system D-Bus.
type realDBusConnector struct{}

func (r *realDBusConnector) SystemBus() (*dbus.Conn, error) {
	return dbus.SystemBus()
}

// SystemdProber monitors a systemd unit via D-Bus.
type SystemdProber struct {
	componentSlug    string
	subComponentSlug string
	unit             string
	severity         types.Severity
	connector        dbusConnector
}

func NewSystemdProber(componentSlug, subComponentSlug, unit string, severity types.Severity) *SystemdProber {
	if severity == "" {
		severity = types.SeverityDown
	}
	return &SystemdProber{
		componentSlug:    componentSlug,
		subComponentSlug: subComponentSlug,
		unit:             unit,
		severity:         severity,
		connector:        &realDBusConnector{},
	}
}

// escapeUnitName converts a systemd unit name to a D-Bus object path component.
// Dashes become _2d, dots become _2e, etc.
func escapeUnitName(unit string) string {
	var b strings.Builder
	for _, c := range unit {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') {
			b.WriteRune(c)
		} else {
			b.WriteString(fmt.Sprintf("_%x", c))
		}
	}
	return b.String()
}

func (p *SystemdProber) Probe(ctx context.Context, results chan<- ProbeResult) {
	conn, err := p.connector.SystemBus()
	if err != nil {
		results <- p.formatErrorResult(fmt.Errorf("failed to connect to system D-Bus: %w", err))
		return
	}
	defer conn.Close()

	objectPath := dbus.ObjectPath("/org/freedesktop/systemd1/unit/" + escapeUnitName(p.unit))
	prop, err := conn.Object("org.freedesktop.systemd1", objectPath).GetProperty("org.freedesktop.systemd1.Unit.ActiveState")
	if err != nil {
		results <- p.formatErrorResult(fmt.Errorf("failed to get ActiveState for unit %s: %w", p.unit, err))
		return
	}

	activeState, ok := prop.Value().(string)
	if !ok {
		results <- p.formatErrorResult(fmt.Errorf("unexpected ActiveState type for unit %s: %v", p.unit, prop.Value()))
		return
	}

	var status types.Status
	if activeState == "active" {
		status = types.StatusHealthy
	} else {
		status = p.severity.ToStatus()
	}

	results <- ProbeResult{
		ComponentMonitorReportComponentStatus: types.ComponentMonitorReportComponentStatus{
			ComponentSlug:    p.componentSlug,
			SubComponentSlug: p.subComponentSlug,
			Status:           status,
			Reasons: []types.Reason{{
				Type:    types.CheckTypeSystemd,
				Check:   p.unit,
				Results: fmt.Sprintf("ActiveState: %s", activeState),
			}},
		},
	}
}

func (p *SystemdProber) formatErrorResult(err error) ProbeResult {
	return ProbeResult{
		ComponentMonitorReportComponentStatus: types.ComponentMonitorReportComponentStatus{
			ComponentSlug:    p.componentSlug,
			SubComponentSlug: p.subComponentSlug,
		},
		Error: fmt.Errorf("error running systemd probe, for component: %s sub-component %s. unit: %s. error: %w", p.componentSlug, p.subComponentSlug, p.unit, err),
	}
}
