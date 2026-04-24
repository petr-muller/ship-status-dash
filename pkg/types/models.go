package types

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
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
	CheckTypeJUnit      CheckType = "junit"
)

// Outage represents a component outage with tracking information for incident management.
type Outage struct {
	gorm.Model
	ComponentName    string       `json:"component_name" gorm:"column:component_name;not null;index"`
	SubComponentName string       `json:"sub_component_name" gorm:"column:sub_component_name;not null;index"`
	Severity         Severity     `json:"severity" gorm:"column:severity;not null"`
	StartTime        time.Time    `json:"start_time" gorm:"column:start_time;not null;index"`
	EndTime          sql.NullTime `json:"end_time" gorm:"column:end_time;index"`
	Description      string       `json:"description" gorm:"column:description;type:text;not null"`
	// DiscoveredFrom describes where this outage was created: frontend, component-monitor, MCP, API
	DiscoveredFrom string       `json:"discovered_from" gorm:"column:discovered_from;not null"`
	CreatedBy      string       `json:"created_by" gorm:"column:created_by;not null"`
	ConfirmedAt    sql.NullTime `json:"confirmed_at" gorm:"column:confirmed_at"`
	TriageNotes    *string      `json:"triage_notes,omitempty" gorm:"column:triage_notes;type:text"`
	// Reasons are the Reason records that describe the reason for the outage
	// this is utilized only by the component-monitor
	Reasons []Reason `json:"reasons,omitempty" gorm:"foreignKey:OutageID"`
	// SlackThreads are the Slack threads associated with the outage
	SlackThreads []SlackThread    `json:"slack_threads,omitempty" gorm:"foreignKey:OutageID"`
	AuditLogs    []OutageAuditLog `json:"audit_logs,omitempty" gorm:"foreignKey:OutageID"`
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

	if strings.TrimSpace(o.Description) == "" {
		validationErrors = append(validationErrors, "Description is required")
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

type contextKey string

const (
	OldOutageKey   contextKey = "old_outage"
	CurrentUserKey contextKey = "current_user"
)

// normalizeOutageTimesUTC converts outage timestamps to UTC so audit log diffs
// do not show spurious timezone changes (pgx returns times in the session TZ).
func normalizeOutageTimesUTC(o *Outage) {
	o.StartTime = o.StartTime.UTC()
	if o.EndTime.Valid {
		o.EndTime.Time = o.EndTime.Time.UTC()
	}
	if o.ConfirmedAt.Valid {
		o.ConfirmedAt.Time = o.ConfirmedAt.Time.UTC()
	}
}

func (o *Outage) BeforeUpdate(db *gorm.DB) error {
	return o.before(db)
}

func (o *Outage) BeforeDelete(db *gorm.DB) error {
	return o.before(db)
}

func (o *Outage) before(db *gorm.DB) error {
	// Check if we've already captured the old outage in this transaction
	if existing := db.Statement.Context.Value(OldOutageKey); existing != nil {
		return nil
	}

	var old Outage
	if err := db.Preload("Reasons").Preload("SlackThreads").First(&old, o.ID).Error; err != nil {
		return err
	}

	normalizeOutageTimesUTC(&old)

	db.Statement.Context = context.WithValue(db.Statement.Context, OldOutageKey, old)
	return nil
}

func (o *Outage) AfterUpdate(db *gorm.DB) error {
	return o.after(db, Update)
}

func (o *Outage) AfterCreate(db *gorm.DB) error {
	return o.after(db, Create)
}

func (o *Outage) AfterDelete(db *gorm.DB) error {
	return o.after(db, Delete)
}

func (o *Outage) after(db *gorm.DB, operation OperationType) error {
	var oldOutageJSON []byte
	if operation == Update || operation == Delete {
		var err error
		oldOutage, ok := db.Statement.Context.Value(OldOutageKey).(Outage)
		if !ok {
			return fmt.Errorf("value of old_outage is not an Outage type")
		}
		oldOutageJSON, err = json.Marshal(oldOutage)
		if err != nil {
			return fmt.Errorf("error marshaling old outage record: %w", err)
		}
	}

	var newTriageJSON []byte
	if operation != Delete {
		var fresh Outage
		if err := db.Preload("Reasons").Preload("SlackThreads").First(&fresh, o.ID).Error; err != nil {
			return fmt.Errorf("failed to reload outage for audit: %w", err)
		}
		normalizeOutageTimesUTC(&fresh)
		var err error
		newTriageJSON, err = json.Marshal(fresh)
		if err != nil {
			return fmt.Errorf("error marshaling new outage record: %w", err)
		}
	}
	userVal := db.Statement.Context.Value(CurrentUserKey)
	if userVal == nil {
		return fmt.Errorf("current user not found in context")
	}
	userStr, ok := userVal.(string)
	if !ok {
		return fmt.Errorf("current user in context has invalid type %T, expected string", userVal)
	}
	audit := OutageAuditLog{
		Operation: string(operation),
		OutageID:  o.ID,
		User:      userStr,
		Old:       oldOutageJSON,
		New:       newTriageJSON,
	}

	return db.Create(&audit).Error
}

type Reason struct {
	gorm.Model
	OutageID uint `json:"-" gorm:"column:outage_id;not null;index"`
	// Type defines the type of monitoring check that was performed.
	// Valid values are defined by CheckType: prometheus, http, systemd, or junit.
	Type CheckType `json:"type"`
	// Check defines the specific check that was performed
	// a prometheus check will have a query, an http check will have a url, and a systemd check will have a unit name
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

type OperationType string

const (
	Create OperationType = "CREATE"
	Update OperationType = "UPDATE"
	Delete OperationType = "DELETE"
)

type OutageAuditLog struct {
	gorm.Model
	OutageID  uint   `json:"outage_id" gorm:"column:outage_id;not null;index"`
	User      string `json:"user" gorm:"column:user;not null"`
	Operation string `json:"operation" gorm:"column:operation;not null"`
	Old       []byte `json:"old,omitempty" gorm:"column:old;type:jsonb"`
	New       []byte `json:"new,omitempty" gorm:"column:new;type:jsonb"`
}
