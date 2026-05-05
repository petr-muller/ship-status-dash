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
	ConnectSystemBus(ctx context.Context) (*dbus.Conn, error)
}

// realDBusConnector connects to the real system D-Bus.
type realDBusConnector struct{}

func (r *realDBusConnector) ConnectSystemBus(ctx context.Context) (*dbus.Conn, error) {
	return dbus.ConnectSystemBus(dbus.WithContext(ctx))
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
// Systemd's escaping is byte-oriented: dashes become _2d, dots become _2e, etc.
func escapeUnitName(unit string) string {
	var b strings.Builder
	for _, c := range []byte(unit) {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') {
			b.WriteByte(c)
		} else {
			fmt.Fprintf(&b, "_%02x", c)
		}
	}
	return b.String()
}

func (p *SystemdProber) sendResult(ctx context.Context, results chan<- ProbeResult, result ProbeResult) {
	select {
	case results <- result:
	case <-ctx.Done():
	}
}

func (p *SystemdProber) Probe(ctx context.Context, results chan<- ProbeResult) {
	conn, err := p.connector.ConnectSystemBus(ctx)
	if err != nil {
		p.sendResult(ctx, results, p.formatErrorResult(fmt.Errorf("failed to connect to system D-Bus: %w", err)))
		return
	}
	defer conn.Close()

	objectPath := dbus.ObjectPath("/org/freedesktop/systemd1/unit/" + escapeUnitName(p.unit))
	obj := conn.Object("org.freedesktop.systemd1", objectPath)

	var prop dbus.Variant
	err = obj.CallWithContext(ctx, "org.freedesktop.DBus.Properties.Get", 0,
		"org.freedesktop.systemd1.Unit", "ActiveState").Store(&prop)
	if err != nil {
		p.sendResult(ctx, results, p.formatErrorResult(fmt.Errorf("failed to get ActiveState for unit %s: %w", p.unit, err)))
		return
	}

	activeState, ok := prop.Value().(string)
	if !ok {
		p.sendResult(ctx, results, p.formatErrorResult(fmt.Errorf("unexpected ActiveState type for unit %s: %v", p.unit, prop.Value())))
		return
	}

	var status types.Status
	if activeState == "active" {
		status = types.StatusHealthy
	} else {
		status = p.severity.ToStatus()
	}

	p.sendResult(ctx, results, ProbeResult{
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
		ProbeType: ProbeTypeSystemd,
	})
}

func (p *SystemdProber) formatErrorResult(err error) ProbeResult {
	return ProbeResult{
		ComponentMonitorReportComponentStatus: types.ComponentMonitorReportComponentStatus{
			ComponentSlug:    p.componentSlug,
			SubComponentSlug: p.subComponentSlug,
		},
		ProbeType: ProbeTypeSystemd,
		Error:     fmt.Errorf("error running systemd probe, for component: %s sub-component %s. unit: %s. error: %w", p.componentSlug, p.subComponentSlug, p.unit, err),
	}
}
