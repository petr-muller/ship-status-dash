package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"ship-status-dash/pkg/auth"
	"ship-status-dash/pkg/config"
	"ship-status-dash/pkg/outage"
	"ship-status-dash/pkg/repositories"
	"ship-status-dash/pkg/types"
)

// Handlers contains the HTTP request handlers for the dashboard API.
type Handlers struct {
	logger                 *logrus.Logger
	configManager          *config.Manager[types.DashboardConfig]
	outageManager          outage.OutageManager
	pingRepo               repositories.ComponentPingRepository
	groupCache             *auth.GroupMembershipCache
	monitorReportProcessor *ComponentMonitorReportProcessor
	externalPageCaches     map[string]*ExternalPageCache
}

// NewHandlers creates a new Handlers instance with the provided dependencies.
func NewHandlers(logger *logrus.Logger, configManager *config.Manager[types.DashboardConfig], outageManager outage.OutageManager, pingRepo repositories.ComponentPingRepository, groupCache *auth.GroupMembershipCache) *Handlers {
	return &Handlers{
		logger:                 logger,
		configManager:          configManager,
		outageManager:          outageManager,
		pingRepo:               pingRepo,
		groupCache:             groupCache,
		monitorReportProcessor: NewComponentMonitorReportProcessor(outageManager, pingRepo, configManager, logger),
		externalPageCaches: map[string]*ExternalPageCache{
			"spc-dashboard": NewExternalPageCache(
				"https://storage.googleapis.com/ship-spc-dashboard/index.html",
				1*time.Hour,
				logger,
			),
		},
	}
}

// config returns the current dashboard configuration.
func (h *Handlers) config() *types.DashboardConfig {
	return h.configManager.Get()
}

func respondWithJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(data) // Best effort - can't return error after writing headers
}

func respondWithError(w http.ResponseWriter, statusCode int, message string) {
	respondWithJSON(w, statusCode, map[string]string{
		"error": message,
	})
}

// IsUserAuthorizedForComponent checks if a user is authorized to perform mutating actions on a component.
// A user is authorized if they match any Owner.User field, or if they are a member of at least one rover_group configured for the component.
// Note that this does not check ServiceAccounts.
func (h *Handlers) IsUserAuthorizedForComponent(user string, component *types.Component) bool {
	for _, owner := range component.Owners {
		// Check if user matches the Owner.User field (for development/testing)
		if owner.User != "" && owner.User == user {
			return true
		}
		// Check if user is in any of the component's rover_groups
		if owner.RoverGroup != "" {
			if h.groupCache.IsUserInGroup(user, owner.RoverGroup) {
				return true
			}
		}
	}

	return false
}

// HealthJSON returns the health status of the dashboard service.
func (h *Handlers) HealthJSON(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"status": "ok",
		"time":   time.Now().UTC().Format(time.RFC3339),
	}
	respondWithJSON(w, http.StatusOK, response)
}

// GetComponentsJSON returns the list of configured components.
func (h *Handlers) GetComponentsJSON(w http.ResponseWriter, r *http.Request) {
	respondWithJSON(w, http.StatusOK, h.config().Components)
}

// GetComponentInfoJSON returns the information for a specific component.
func (h *Handlers) GetComponentInfoJSON(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	componentName := vars["componentName"]
	component := h.config().GetComponentBySlug(componentName)
	if component == nil {
		respondWithError(w, http.StatusNotFound, "Component not found")
		return
	}
	respondWithJSON(w, http.StatusOK, component)
}

// GetOutagesJSON retrieves outages for a specific component, aggregating sub-component outages for top-level components.
func (h *Handlers) GetOutagesJSON(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	componentName := vars["componentName"]

	logger := h.logger.WithField("component", componentName)

	component := h.config().GetComponentBySlug(componentName)
	if component == nil {
		respondWithError(w, http.StatusNotFound, "Component not found")
		return
	}
	subComponentSlugs := make([]string, len(component.Subcomponents))
	for i, subComponent := range component.Subcomponents {
		subComponentSlugs[i] = subComponent.Slug
	}

	outages, err := h.outageManager.GetOutagesForComponent(componentName, subComponentSlugs)
	if err != nil {
		logger.WithField("error", err).Error("Failed to query outages from database")
		respondWithError(w, http.StatusInternalServerError, "Failed to get outages")
		return
	}

	respondWithJSON(w, http.StatusOK, outages)
}

// GetSubComponentOutagesJSON retrieves outages for a specific sub-component.
func (h *Handlers) GetSubComponentOutagesJSON(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	componentName := vars["componentName"]
	subComponentName := vars["subComponentName"]

	logger := h.logger.WithFields(logrus.Fields{
		"component":     componentName,
		"sub_component": subComponentName,
	})

	component := h.config().GetComponentBySlug(componentName)
	if component == nil {
		respondWithError(w, http.StatusNotFound, "Component not found")
		return
	}

	subComponent := component.GetSubComponentBySlug(subComponentName)
	if subComponent == nil {
		respondWithError(w, http.StatusNotFound, "Sub-component not found")
		return
	}

	outages, err := h.outageManager.GetOutagesForSubComponent(componentName, subComponentName)
	if err != nil {
		logger.WithField("error", err).Error("Failed to query outages from database")
		respondWithError(w, http.StatusInternalServerError, "Failed to get outages")
		return
	}

	respondWithJSON(w, http.StatusOK, outages)
}

// CreateOutageJSON creates a new outage for a sub-component.
func (h *Handlers) CreateOutageJSON(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	componentName := vars["componentName"]
	subComponentName := vars["subComponentName"]

	activeUser, ok := GetUserFromContext(r.Context())
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "no active user found")
		return
	}

	logger := h.logger.WithFields(logrus.Fields{
		"component":     componentName,
		"sub_component": subComponentName,
		"active_user":   activeUser,
	})

	component := h.config().GetComponentBySlug(componentName)
	if component == nil {
		respondWithError(w, http.StatusNotFound, "Component not found")
		return
	}
	subComponent := component.GetSubComponentBySlug(subComponentName)
	if subComponent == nil {
		respondWithError(w, http.StatusNotFound, "Sub-Component not found")
		return
	}

	if !h.IsUserAuthorizedForComponent(activeUser, component) {
		logger.Warn("User not authorized to create outage")
		respondWithError(w, http.StatusForbidden, "You are not authorized to perform this action on this component")
		return
	}

	var outageReq types.UpsertOutageRequest
	if err := json.NewDecoder(r.Body).Decode(&outageReq); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	severity := ""
	if outageReq.Severity != nil {
		severity = *outageReq.Severity
	}
	discoveredFrom := ""
	if outageReq.DiscoveredFrom != nil {
		discoveredFrom = *outageReq.DiscoveredFrom
	}
	logger = logger.WithFields(logrus.Fields{
		"severity":        severity,
		"discovered_from": discoveredFrom,
	})

	var description string
	if outageReq.Description != nil {
		description = strings.TrimSpace(*outageReq.Description)
	}

	outage := types.Outage{
		ComponentName:    componentName,
		SubComponentName: subComponentName,
		Severity:         types.Severity(severity),
		Description:      description,
		StartTime:        *outageReq.StartTime,
		DiscoveredFrom:   discoveredFrom,
		TriageNotes:      outageReq.TriageNotes,
	}

	outage.CreatedBy = activeUser

	confirmed := (outageReq.Confirmed != nil && *outageReq.Confirmed)
	if confirmed || !subComponent.RequiresConfirmation {
		outage.ConfirmedAt = sql.NullTime{Time: time.Now(), Valid: true}
	}

	if message, valid := outage.Validate(); !valid {
		respondWithError(w, http.StatusBadRequest, message)
		return
	}

	if err := h.outageManager.CreateOutage(&outage, nil, activeUser); err != nil {
		logger.WithField("error", err).Error("Failed to create outage in database")
		respondWithError(w, http.StatusInternalServerError, "Failed to create outage")
		return
	}

	logger.Infof("Successfully created outage: %d", outage.ID)

	respondWithJSON(w, http.StatusCreated, outage)
}

// UpdateOutageJSON updates an existing outage with the provided fields.
func (h *Handlers) UpdateOutageJSON(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	componentName := vars["componentName"]
	subComponentName := vars["subComponentName"]
	outageIDStr := vars["outageId"]

	activeUser, ok := GetUserFromContext(r.Context())
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "no active user found")
		return
	}

	outageID, err := strconv.ParseUint(outageIDStr, 10, 32)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid outage ID")
		return
	}

	logger := h.logger.WithFields(logrus.Fields{
		"outage_id":     outageID,
		"component":     componentName,
		"sub_component": subComponentName,
		"active_user":   activeUser,
	})
	logger.Info("Updating outage")

	component := h.config().GetComponentBySlug(componentName)
	if component == nil {
		respondWithError(w, http.StatusNotFound, "Component not found")
		return
	}

	subComponent := component.GetSubComponentBySlug(subComponentName)
	if subComponent == nil {
		respondWithError(w, http.StatusNotFound, "Sub-Component not found")
		return
	}

	if !h.IsUserAuthorizedForComponent(activeUser, component) {
		logger.Warn("User not authorized to update outage")
		respondWithError(w, http.StatusForbidden, "You are not authorized to perform this action on this component")
		return
	}

	outage, err := h.outageManager.GetOutageByID(componentName, subComponentName, uint(outageID))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			respondWithError(w, http.StatusNotFound, "Outage not found")
			return
		}
		logger.WithField("error", err).Error("Failed to query outage from database")
		respondWithError(w, http.StatusInternalServerError, "Failed to get outage")
		return
	}

	var updateReq types.UpsertOutageRequest
	if err := json.NewDecoder(r.Body).Decode(&updateReq); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if updateReq.Severity != nil {
		if !types.IsValidSeverity(*updateReq.Severity) {
			respondWithError(w, http.StatusBadRequest, "Invalid severity. Must be one of: Down, Degraded, Suspected")
			return
		}
		outage.Severity = types.Severity(*updateReq.Severity)
	}
	if updateReq.StartTime != nil && !updateReq.StartTime.Equal(outage.StartTime) {
		outage.StartTime = *updateReq.StartTime
	}
	if updateReq.EndTime != nil {
		endTimeChanged := updateReq.EndTime.Valid != outage.EndTime.Valid || !updateReq.EndTime.Time.Equal(outage.EndTime.Time)
		if endTimeChanged {
			outage.EndTime = *updateReq.EndTime
		}
	}
	if updateReq.Description != nil {
		outage.Description = strings.TrimSpace(*updateReq.Description)
	}
	if updateReq.Confirmed != nil {
		if *updateReq.Confirmed && !outage.ConfirmedAt.Valid {
			outage.ConfirmedAt = sql.NullTime{Time: time.Now(), Valid: true}
		} else if !*updateReq.Confirmed && outage.ConfirmedAt.Valid {
			outage.ConfirmedAt = sql.NullTime{Valid: false}
		}
	}
	if updateReq.TriageNotes != nil {
		outage.TriageNotes = updateReq.TriageNotes
	}

	if message, valid := outage.Validate(); !valid {
		respondWithError(w, http.StatusBadRequest, message)
		return
	}

	if err := h.outageManager.UpdateOutage(outage, activeUser); err != nil {
		logger.WithField("error", err).Error("Failed to update outage in database")
		respondWithError(w, http.StatusInternalServerError, "Failed to update outage")
		return
	}

	logger.Info("Successfully updated outage")

	respondWithJSON(w, http.StatusOK, outage)
}

// GetOutageJSON retrieves a specific outage by ID for a specific sub-component.
func (h *Handlers) GetOutageJSON(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	componentName := vars["componentName"]
	subComponentName := vars["subComponentName"]
	outageIDStr := vars["outageId"]

	logger := h.logger.WithFields(logrus.Fields{
		"component":     componentName,
		"sub_component": subComponentName,
		"outage_id":     outageIDStr,
	})

	component := h.config().GetComponentBySlug(componentName)
	if component == nil {
		respondWithError(w, http.StatusNotFound, "Component not found")
		return
	}

	subComponent := component.GetSubComponentBySlug(subComponentName)
	if subComponent == nil {
		respondWithError(w, http.StatusNotFound, "Sub-component not found")
		return
	}

	outageID, err := strconv.ParseUint(outageIDStr, 10, 32)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid outage ID")
		return
	}

	outage, err := h.outageManager.GetOutageByID(componentName, subComponentName, uint(outageID))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			respondWithError(w, http.StatusNotFound, "Outage not found")
			return
		}
		logger.WithField("error", err).Error("Failed to query outage from database")
		respondWithError(w, http.StatusInternalServerError, "Failed to get outage")
		return
	}

	logger.Info("Successfully retrieved outage")
	respondWithJSON(w, http.StatusOK, outage)
}

// DeleteOutage deletes an outage by ID for a specific sub-component.
func (h *Handlers) DeleteOutage(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	componentName := vars["componentName"]
	subComponentName := vars["subComponentName"]
	outageIDStr := vars["outageId"]

	activeUser, ok := GetUserFromContext(r.Context())
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "no active user found")
		return
	}

	logger := h.logger.WithFields(logrus.Fields{
		"component":     componentName,
		"sub_component": subComponentName,
		"outage_id":     outageIDStr,
		"active_user":   activeUser,
	})

	component := h.config().GetComponentBySlug(componentName)
	if component == nil {
		respondWithError(w, http.StatusNotFound, "Component not found")
		return
	}

	subComponent := component.GetSubComponentBySlug(subComponentName)
	if subComponent == nil {
		respondWithError(w, http.StatusNotFound, "Sub-component not found")
		return
	}

	if !h.IsUserAuthorizedForComponent(activeUser, component) {
		logger.Warn("User not authorized to delete outage")
		respondWithError(w, http.StatusForbidden, "You are not authorized to perform this action on this component")
		return
	}

	outageID, err := strconv.ParseUint(outageIDStr, 10, 32)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid outage ID")
		return
	}

	outage, err := h.outageManager.GetOutageByID(componentName, subComponentName, uint(outageID))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			respondWithError(w, http.StatusNotFound, "Outage not found")
			return
		}
		logger.WithField("error", err).Error("Failed to query outage from database")
		respondWithError(w, http.StatusInternalServerError, "Failed to get outage")
		return
	}

	if err := h.outageManager.DeleteOutage(outage, activeUser); err != nil {
		logger.WithField("error", err).Error("Failed to delete outage from database")
		respondWithError(w, http.StatusInternalServerError, "Failed to delete outage")
		return
	}

	logger.Info("Successfully deleted outage")
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handlers) GetOutageAuditLogsJSON(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	componentName := vars["componentName"]
	subComponentName := vars["subComponentName"]
	outageIDStr := vars["outageId"]

	logger := h.logger.WithFields(logrus.Fields{
		"component":     componentName,
		"sub_component": subComponentName,
		"outage_id":     outageIDStr,
	})

	outageID, err := strconv.ParseUint(outageIDStr, 10, 32)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid outage ID")
		return
	}
	// Get the Outage using the component and subComponents to verify that the outage belongs to them
	outage, err := h.outageManager.GetOutageByID(componentName, subComponentName, uint(outageID))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			respondWithError(w, http.StatusNotFound, "Outage not found")
			return
		}
		logger.WithField("error", err).Error("Failed to query outage from database")
		respondWithError(w, http.StatusInternalServerError, "Failed to get outage")
		return
	}

	auditLogs, err := h.outageManager.GetOutageAuditLogs(outage.ID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to get outage audit logs")
		return
	}

	respondWithJSON(w, http.StatusOK, auditLogs)
}

// GetSubComponentStatusJSON returns the status of a subcomponent based on active outages
func (h *Handlers) GetSubComponentStatusJSON(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	componentName := vars["componentName"]
	subComponentName := vars["subComponentName"]

	logger := h.logger.WithFields(logrus.Fields{
		"component":     componentName,
		"sub_component": subComponentName,
	})

	component := h.config().GetComponentBySlug(componentName)
	if component == nil {
		respondWithError(w, http.StatusNotFound, "Component not found")
		return
	}

	subComponent := component.GetSubComponentBySlug(subComponentName)
	if subComponent == nil {
		respondWithError(w, http.StatusNotFound, "Sub-component not found")
		return
	}

	outages, err := h.outageManager.GetActiveOutagesForSubComponent(componentName, subComponentName)
	if err != nil {
		logger.WithField("error", err).Error("Failed to query active outages from database")
		respondWithError(w, http.StatusInternalServerError, "Failed to get subcomponent status")
		return
	}

	status := types.StatusHealthy
	if len(outages) > 0 {
		status = determineStatusFromSeverity(outages)
	}

	lastPingTime, err := h.pingRepo.GetLastPingTime(componentName, subComponentName)
	if err != nil {
		logger.WithField("error", err).Warn("Failed to query component report ping")
	}

	response := types.ComponentStatus{
		ComponentName: fmt.Sprintf("%s/%s", componentName, subComponentName),
		Status:        status,
		ActiveOutages: outages,
		LastPingTime:  lastPingTime,
	}
	respondWithJSON(w, http.StatusOK, response)
}

// GetComponentStatusJSON returns the status of a component based on active outages in all its sub-components
func (h *Handlers) GetComponentStatusJSON(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	componentName := vars["componentName"]

	logger := h.logger.WithField("component", componentName)

	component := h.config().GetComponentBySlug(componentName)
	if component == nil {
		respondWithError(w, http.StatusNotFound, "Component not found")
		return
	}

	response, err := h.getComponentStatus(component, logger)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to get component status")
		return
	}
	respondWithJSON(w, http.StatusOK, response)
}

// GetAllComponentsStatusJSON returns the status of all components
func (h *Handlers) GetAllComponentsStatusJSON(w http.ResponseWriter, r *http.Request) {
	logger := h.logger

	var allComponentStatuses []types.ComponentStatus

	for _, component := range h.config().Components {
		componentLogger := logger.WithField("component", component.Name)
		componentStatus, err := h.getComponentStatus(component, componentLogger)
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, "Failed to get component status")
			return
		}

		allComponentStatuses = append(allComponentStatuses, componentStatus)
	}
	respondWithJSON(w, http.StatusOK, allComponentStatuses)
}

// getComponentStatus calculates the status of a component based on its sub-components and active outages
func (h *Handlers) getComponentStatus(component *types.Component, logger *logrus.Entry) (types.ComponentStatus, error) {
	outages, err := h.outageManager.GetActiveOutagesForComponent(component.Slug)
	if err != nil {
		logger.WithField("error", err).Error("Failed to query active outages from database")
		return types.ComponentStatus{}, err
	}

	subComponentsWithOutages := make(map[string]bool)
	for _, outage := range outages {
		subComponentsWithOutages[outage.SubComponentName] = true
	}

	var status types.Status
	if len(outages) == 0 {
		status = types.StatusHealthy
	} else if len(subComponentsWithOutages) < len(component.Subcomponents) {
		status = types.StatusPartial
	} else {
		status = determineStatusFromSeverity(outages)
	}

	// The last ping time is the time of the most recent ping for ANY of the sub-components in the component.
	lastPingTime, err := h.pingRepo.GetMostRecentPingTimeForAnySubComponent(component.Slug)
	if err != nil {
		logger.WithField("error", err).Warn("Failed to query component report pings")
	}

	return types.ComponentStatus{
		ComponentName: component.Name,
		Status:        status,
		ActiveOutages: outages,
		LastPingTime:  lastPingTime,
	}, nil
}

func determineStatusFromSeverity(outages []types.Outage) types.Status {
	if len(outages) == 0 {
		return types.StatusHealthy
	}

	// First, determine status based on confirmed outages
	confirmedOutages := make([]types.Outage, 0)
	hasUnconfirmedOutage := false

	for _, outage := range outages {
		if outage.ConfirmedAt.Valid {
			confirmedOutages = append(confirmedOutages, outage)
		} else {
			hasUnconfirmedOutage = true
		}
	}

	// If there are confirmed outages, determine status by their severity
	if len(confirmedOutages) > 0 {
		mostCriticalSeverity := confirmedOutages[0].Severity
		highestLevel := types.GetSeverityLevel(mostCriticalSeverity)

		for _, outage := range confirmedOutages {
			level := types.GetSeverityLevel(outage.Severity)
			if level > highestLevel {
				highestLevel = level
				mostCriticalSeverity = outage.Severity
			}
		}
		return mostCriticalSeverity.ToStatus()
	}

	// Only unconfirmed outages - return Suspected
	if hasUnconfirmedOutage {
		return types.StatusSuspected
	}

	return types.StatusHealthy
}

// ListTagsJSON returns the list of configured tags.
func (h *Handlers) ListTagsJSON(w http.ResponseWriter, r *http.Request) {
	respondWithJSON(w, http.StatusOK, h.config().Tags)
}

// ListSubComponentsJSON handles HTTP requests to fetch a list of sub-components based on filters like componentName, team, or tag.
// All must be matched for a sub-component to be returned. If no filters are provided, all sub-components are returned.
func (h *Handlers) ListSubComponentsJSON(w http.ResponseWriter, r *http.Request) {
	componentSlug := r.URL.Query().Get("componentName")
	tag := r.URL.Query().Get("tag")
	team := r.URL.Query().Get("team")

	components := h.config().Components
	items := []types.SubComponentListItem{}
	for _, component := range components {
		if componentSlug != "" && component.Slug != componentSlug {
			continue
		}
		if team != "" && team != component.ShipTeam {
			continue
		}

		if tag == "" {
			for _, sub := range component.Subcomponents {
				items = append(items, types.SubComponentListItem{ComponentName: component.Name, SubComponent: sub})
			}
		} else {
		subComponentLoop:
			for _, sub := range component.Subcomponents {
				for _, t := range sub.Tags {
					if t == tag {
						items = append(items, types.SubComponentListItem{ComponentName: component.Name, SubComponent: sub})
						continue subComponentLoop
					}
				}
			}
		}
	}

	respondWithJSON(w, http.StatusOK, items)
}

func (h *Handlers) PostComponentMonitorReportJSON(w http.ResponseWriter, r *http.Request) {
	var req types.ComponentMonitorReportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.ComponentMonitor == "" {
		respondWithError(w, http.StatusBadRequest, "component_monitor is required")
		return
	}

	if len(req.Statuses) == 0 {
		respondWithError(w, http.StatusBadRequest, "statuses cannot be empty")
		return
	}

	for _, status := range req.Statuses {
		component := h.config().GetComponentBySlug(status.ComponentSlug)
		if component == nil {
			respondWithError(w, http.StatusBadRequest, fmt.Sprintf("Component not found: %s", status.ComponentSlug))
			return
		}

		subComponent := component.GetSubComponentBySlug(status.SubComponentSlug)
		if subComponent == nil {
			respondWithError(w, http.StatusBadRequest, fmt.Sprintf("Sub-component not found: %s/%s", status.ComponentSlug, status.SubComponentSlug))
			return
		}
	}

	user, authenticated := GetUserFromContext(r.Context())
	if !authenticated {
		respondWithError(w, http.StatusUnauthorized, "no Authenticated ServiceAccount user found")
		return
	}
	err := h.monitorReportProcessor.ValidateRequest(&req, user)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request")
		return
	}
	err = h.monitorReportProcessor.Process(&req)
	if err != nil {
		h.logger.WithField("error", err).Error("Failed to process component monitor report")
		respondWithError(w, http.StatusInternalServerError, "Failed to process report")
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{"status": "processed"})
}

type AuthenticatedUser struct {
	Username   string   `json:"username" yaml:"username"`
	Components []string `json:"components" yaml:"components"`
}

func (h *Handlers) GetAuthenticatedUserJSON(w http.ResponseWriter, r *http.Request) {
	user, authenticated := GetUserFromContext(r.Context())
	if !authenticated {
		respondWithError(w, http.StatusUnauthorized, "No Authenticated user found")
		return
	}

	response := AuthenticatedUser{
		Username:   user,
		Components: []string{},
	}

	// Return only components the user is authorized for
	for _, component := range h.config().Components {
		if h.IsUserAuthorizedForComponent(user, component) {
			response.Components = append(response.Components, component.Slug)
		}
	}

	respondWithJSON(w, http.StatusOK, response)
}
