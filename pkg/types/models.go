package types

import (
	"database/sql"
	"strings"
	"time"

	"gorm.io/gorm"
)

type Severity string

const (
	SeverityDown     Severity = "Down"
	SeverityDegraded Severity = "Degraded"
	//TODO: Suspected might be useful in the future (for the down-detector type voting), but it isn't used anywhere yet.
	SeveritySuspected Severity = "Suspected"
	// SeverityCapacityExhausted is for components that can go into outage due to lack of resources. For example, a boskos cloud-account.
	SeverityCapacityExhausted Severity = "CapacityExhausted"
)

func (s Severity) ToStatus() Status {
	switch s {
	case SeverityDown:
		return StatusDown
	case SeverityDegraded:
		return StatusDegraded
	case SeverityCapacityExhausted:
		return StatusCapacityExhausted
	case SeveritySuspected:
		return StatusSuspected
	default:
		return Status("Invalid")
	}
}

// IsValidSeverity checks if the provided severity string is a valid severity level
func IsValidSeverity(severity string) bool {
	switch Severity(severity) {
	case SeverityDown, SeverityDegraded, SeveritySuspected, SeverityCapacityExhausted:
		return true
	default:
		return false
	}
}

// GetSeverityLevel returns a numeric value for severity comparison (higher = more critical)
func GetSeverityLevel(severity Severity) int {
	switch severity {
	case SeverityDown:
		return 4
	case SeverityDegraded:
		return 3
	case SeverityCapacityExhausted:
		return 2
	case SeveritySuspected:
		return 1
	default:
		return 0
	}
}

// CheckType represents the type of monitoring check to perform.
type CheckType string

const (
	CheckTypePrometheus CheckType = "prometheus"
	CheckTypeHTTP       CheckType = "http"
	CheckTypeSystemd    CheckType = "systemd"
)

// Outage represents a component outage with tracking information for incident management.
type Outage struct {
	gorm.Model
	ComponentName    string       `json:"component_name" gorm:"column:component_name;not null;index"`
	SubComponentName string       `json:"sub_component_name" gorm:"column:sub_component_name;not null;index"`
	Severity         Severity     `json:"severity" gorm:"column:severity;not null"`
	StartTime        time.Time    `json:"start_time" gorm:"column:start_time;not null;index"`
	EndTime          sql.NullTime `json:"end_time" gorm:"column:end_time;index"`
	Description      string       `json:"description" gorm:"column:description;type:text"`
	// DiscoveredFrom describes where this outage was created: frontend, component-monitor, MCP, API
	DiscoveredFrom string       `json:"discovered_from" gorm:"column:discovered_from;not null"`
	CreatedBy      string       `json:"created_by" gorm:"column:created_by;not null"`
	ResolvedBy     *string      `json:"resolved_by,omitempty" gorm:"column:resolved_by"`
	ConfirmedBy    *string      `json:"confirmed_by,omitempty" gorm:"column:confirmed_by"`
	ConfirmedAt    sql.NullTime `json:"confirmed_at" gorm:"column:confirmed_at"`
	TriageNotes    *string      `json:"triage_notes,omitempty" gorm:"column:triage_notes;type:text"`
	// Reasons are the Reason records that describe the reason for the outage
	// this is utilized only by the component-monitor
	Reasons []Reason `json:"reasons,omitempty" gorm:"foreignKey:OutageID"`
	// SlackThreads are the Slack threads associated with the outage
	SlackThreads []SlackThread `json:"slack_threads,omitempty" gorm:"foreignKey:OutageID"`
	//TODO: Add optional link to jira card, and incident slack thread link for outage
}

// Validate validates the outage and returns an error message and whether it's valid.
// Returns an empty string and true if valid, otherwise returns an aggregated error message and false.
func (o *Outage) Validate() (string, bool) {
	var validationErrors []string

	if o.Severity == "" {
		validationErrors = append(validationErrors, "Severity is required")
	} else if !IsValidSeverity(string(o.Severity)) {
		validationErrors = append(validationErrors, "Invalid severity. Must be one of: Down, Degraded, Suspected")
	}

	if o.StartTime.IsZero() {
		validationErrors = append(validationErrors, "StartTime is required")
	}

	if o.DiscoveredFrom == "" {
		validationErrors = append(validationErrors, "DiscoveredFrom is required")
	}

	if o.CreatedBy == "" {
		validationErrors = append(validationErrors, "CreatedBy is required")
	}

	if len(validationErrors) > 0 {
		return strings.Join(validationErrors, "; "), false
	}

	return "", true
}

type Reason struct {
	gorm.Model
	OutageID uint `json:"-" gorm:"column:outage_id;not null;index"`
	// Type defines the type of monitoring check that was performed
	// either: prometheus, or http
	Type CheckType `json:"type"`
	// Check defines the specific check that was performed
	// a prometheus check will have a query, and a http check will have a url
	Check string `json:"check"`
	// Results summarizes the results of the check
	Results string `json:"results"`
}

// ComponentReportPing represents a ping report from a component monitor.
// This is used to track the last time that any status has been reported for a component/sub-component.
type ComponentReportPing struct {
	gorm.Model
	ComponentName    string    `json:"component_name" gorm:"column:component_name;not null;index;uniqueIndex:idx_component_subcomponent"`
	SubComponentName string    `json:"sub_component_name" gorm:"column:sub_component_name;not null;index;uniqueIndex:idx_component_subcomponent"`
	Time             time.Time `json:"time" gorm:"column:time;not null;index"`
}

// SlackThread represents a Slack thread associated with an outage in a specific channel.
type SlackThread struct {
	gorm.Model
	OutageID        uint   `json:"outage_id" gorm:"column:outage_id;not null;index:idx_outage_channel,unique"`
	Channel         string `json:"channel" gorm:"column:channel;not null;index:idx_outage_channel,unique"`
	ChannelID       string `json:"channel_id" gorm:"column:channel_id;not null"`
	ThreadTimestamp string `json:"thread_timestamp" gorm:"column:thread_timestamp;not null"`
	ThreadURL       string `json:"thread_url" gorm:"column:thread_url;not null"`
}
