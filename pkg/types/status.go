package types

import "time"

type Status string

const (
	StatusHealthy           Status = "Healthy"
	StatusDegraded          Status = "Degraded"
	StatusDown              Status = "Down"
	StatusCapacityExhausted Status = "CapacityExhausted"
	StatusSuspected         Status = "Suspected"
	StatusPartial           Status = "Partial" // Indicates that some sub-components are healthy, and some are degraded or down
)

// ToSeverity converts a Status to a Severity. Returns an empty string if the status cannot be converted to a severity.
func (s Status) ToSeverity() Severity {
	switch s {
	case StatusDown:
		return SeverityDown
	case StatusDegraded:
		return SeverityDegraded
	case StatusCapacityExhausted:
		return SeverityCapacityExhausted
	case StatusSuspected:
		return SeveritySuspected
	default:
		return ""
	}
}

type ComponentStatus struct {
	ComponentName string     `json:"component_name"`
	Status        Status     `json:"status"`
	ActiveOutages []Outage   `json:"active_outages"`
	LastPingTime  *time.Time `json:"last_ping_time,omitempty"`
}

// StatusFromOutages returns the roll-up status from active outages when the caller has already
// narrowed the case (for example, every sub-component has at least one outage).
func StatusFromOutages(outages []Outage) Status {
	if len(outages) == 0 {
		return StatusHealthy
	}

	confirmedOutages := make([]Outage, 0)
	hasUnconfirmedOutage := false

	for _, outage := range outages {
		if outage.ConfirmedAt.Valid {
			confirmedOutages = append(confirmedOutages, outage)
		} else {
			hasUnconfirmedOutage = true
		}
	}

	if len(confirmedOutages) > 0 {
		mostCriticalSeverity := confirmedOutages[0].Severity
		highestLevel := GetSeverityLevel(mostCriticalSeverity)

		for _, outage := range confirmedOutages {
			level := GetSeverityLevel(outage.Severity)
			if level > highestLevel {
				highestLevel = level
				mostCriticalSeverity = outage.Severity
			}
		}
		return mostCriticalSeverity.ToStatus()
	}

	if hasUnconfirmedOutage {
		return StatusSuspected
	}

	return StatusHealthy
}
