package types

import (
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestStatusFromOutages(t *testing.T) {
	now := time.Now()
	confirmedTime := sql.NullTime{Time: now, Valid: true}
	unconfirmedTime := sql.NullTime{Valid: false}

	tests := []struct {
		name     string
		outages  []Outage
		expected Status
	}{
		{
			name: "single confirmed outage - down severity",
			outages: []Outage{
				{Severity: SeverityDown, ConfirmedAt: confirmedTime},
			},
			expected: StatusDown,
		},
		{
			name: "single unconfirmed outage - down severity",
			outages: []Outage{
				{Severity: SeverityDown, ConfirmedAt: unconfirmedTime},
			},
			expected: StatusSuspected,
		},
		{
			name: "multiple confirmed outages - highest severity wins",
			outages: []Outage{
				{Severity: SeveritySuspected, ConfirmedAt: confirmedTime},
				{Severity: SeverityDown, ConfirmedAt: confirmedTime},
				{Severity: SeverityDegraded, ConfirmedAt: confirmedTime},
			},
			expected: StatusDown,
		},
		{
			name: "mixed confirmed and unconfirmed - confirmed takes precedence",
			outages: []Outage{
				{Severity: SeverityDown, ConfirmedAt: unconfirmedTime},
				{Severity: SeverityDegraded, ConfirmedAt: confirmedTime},
			},
			expected: StatusDegraded,
		},
		{
			name: "only unconfirmed outages - shows suspected",
			outages: []Outage{
				{Severity: SeverityDown, ConfirmedAt: unconfirmedTime},
				{Severity: SeverityDegraded, ConfirmedAt: unconfirmedTime},
			},
			expected: StatusSuspected,
		},
		{
			name: "confirmed degraded with unconfirmed down - confirmed takes precedence",
			outages: []Outage{
				{Severity: SeverityDown, ConfirmedAt: unconfirmedTime},
				{Severity: SeverityDegraded, ConfirmedAt: confirmedTime},
			},
			expected: StatusDegraded,
		},
		{
			name:     "empty outages slice",
			outages:  []Outage{},
			expected: StatusHealthy,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := StatusFromOutages(tt.outages)
			assert.Equal(t, tt.expected, result)
		})
	}
}
