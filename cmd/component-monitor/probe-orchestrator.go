package main

import (
	"context"
	"sort"
	"time"

	"ship-status-dash/pkg/types"

	"github.com/sirupsen/logrus"
)

// Prober is an interface for component probes.
type Prober interface {
	Probe(ctx context.Context, results chan<- ProbeResult)
}

const (
	ProbeTypeHTTP       = "http"
	ProbeTypeJUnit      = "junit"
	ProbeTypePrometheus = "prometheus"
	ProbeTypeSystemd    = "systemd"
)

type ProbeResult struct {
	types.ComponentMonitorReportComponentStatus
	ProbeType string
	Error     error
}

// ProbeOrchestrator manages the execution of component probes.
type ProbeOrchestrator struct {
	probers      []Prober
	results      chan ProbeResult
	frequency    time.Duration
	reportClient *ReportClient
	log          *logrus.Logger
}

// NewProbeOrchestrator creates a new ProbeOrchestrator.
func NewProbeOrchestrator(probers []Prober, frequency time.Duration, dashboardURL string, componentMonitorName string, authToken string, log *logrus.Logger) *ProbeOrchestrator {
	return &ProbeOrchestrator{
		probers:      probers,
		results:      make(chan ProbeResult),
		frequency:    frequency,
		reportClient: NewReportClient(dashboardURL, componentMonitorName, authToken),
		log:          log,
	}
}

// Run starts the probe orchestration loop.
func (o *ProbeOrchestrator) Run(ctx context.Context) {
	for {
		if ctx.Err() != nil {
			o.log.Warn("Context canceled, exiting")
			return
		}

		o.drainChannels()

		startTime := time.Now()
		o.startProbes(ctx)
		results := o.collectProbeResults(ctx)
		mergedResults := mergeStatuses(results)
		if err := o.reportClient.SendReport(mergedResults); err != nil {
			o.log.Errorf("Error sending report: %v", err)
		} else {
			o.log.Infof("Report sent successfully")
		}
		elapsed := time.Since(startTime)
		o.log.Infof("Probing completed in %s", elapsed)
		if !o.waitForNextCycle(ctx, elapsed) {
			return
		}
	}
}

// DryRun runs probes once and outputs the report as JSON to stdout.
func (o *ProbeOrchestrator) DryRun(ctx context.Context) {
	o.startProbes(ctx)
	results := o.collectProbeResults(ctx)
	mergedResults := mergeStatuses(results)
	if err := o.reportClient.PrintReport(mergedResults); err != nil {
		o.log.Errorf("Error outputting report: %v", err)
	}
}

func (o *ProbeOrchestrator) startProbes(ctx context.Context) {
	o.log.Infof("Probing %d components...", len(o.probers))
	for _, prober := range o.probers {
		go prober.Probe(ctx, o.results)
	}
}

func (o *ProbeOrchestrator) collectProbeResults(ctx context.Context) []ProbeResult {
	probesCompleted := 0
	results := []ProbeResult{}
	timeout := time.After(o.frequency)

	for probesCompleted < len(o.probers) {
		select {
		case probeResult := <-o.results:
			resultLog := o.log.WithFields(logrus.Fields{
				"component":     probeResult.ComponentSlug,
				"sub_component": probeResult.SubComponentSlug,
				"status":        probeResult.Status,
				"probe_type":    probeResult.ProbeType,
			})
			if probeResult.Error != nil {
				resultLog.Errorf("Error: %v", probeResult.Error)
			} else {
				resultLog.Info("Component monitor probe result received")
			}
			results = append(results, probeResult)
			probesCompleted++
		case <-ctx.Done():
			o.log.Warn("Context canceled during probe collection, exiting")
			return results
		case <-timeout:
			o.log.Warnf("Timeout waiting for probe results after %s, restarting probe cycle", o.frequency)
			return results
		}
	}

	return results
}

func (o *ProbeOrchestrator) drainChannels() {
	o.log.Infof("Draining channels before next cycle...")
	for {
		select {
		case probeResult := <-o.results:
			if probeResult.Error != nil {
				o.log.Warnf("Discarding old error for component %s sub-component %s: %v", probeResult.ComponentSlug, probeResult.SubComponentSlug, probeResult.Error)
			} else {
				o.log.Warnf("Discarding old result for component %s sub-component %s", probeResult.ComponentSlug, probeResult.SubComponentSlug)
			}
		default:
			o.log.Infof("Channels drained")
			return
		}
	}
}

func (o *ProbeOrchestrator) waitForNextCycle(ctx context.Context, elapsed time.Duration) bool {
	if elapsed < o.frequency {
		sleepDuration := o.frequency - elapsed
		o.log.Infof("Will probe again in %s", sleepDuration)
		select {
		case <-ctx.Done():
			o.log.Warn("Context canceled during sleep, exiting")
			return false
		case <-time.After(sleepDuration):
		}
	}
	return true
}

// mergeStatuses merges multiple status reports for the same component/sub-component
// into a single unified status. It groups by (ComponentSlug, SubComponentSlug), combines all
// reasons from unhealthy probes, and determines the most critical status when multiple probes report different statuses.
// If there are any errored statuses and all non-errored statuses are Healthy, the component/sub-component
// is omitted from the report.
func mergeStatuses(probeResults []ProbeResult) []types.ComponentMonitorReportComponentStatus {
	if len(probeResults) == 0 {
		return []types.ComponentMonitorReportComponentStatus{}
	}

	type componentKey struct {
		component    string
		subComponent string
	}

	grouped := make(map[componentKey][]ProbeResult)
	for _, probeResult := range probeResults {
		key := componentKey{
			component:    probeResult.ComponentSlug,
			subComponent: probeResult.SubComponentSlug,
		}
		grouped[key] = append(grouped[key], probeResult)
	}

	merged := make([]types.ComponentMonitorReportComponentStatus, 0, len(grouped))
	for key, group := range grouped {
		if result := mergeStatusesForSubComponent(key.component, key.subComponent, group); result != nil {
			merged = append(merged, *result)
		}
	}

	// Sort results for deterministic output
	sort.Slice(merged, func(i, j int) bool {
		if merged[i].ComponentSlug != merged[j].ComponentSlug {
			return merged[i].ComponentSlug < merged[j].ComponentSlug
		}
		return merged[i].SubComponentSlug < merged[j].SubComponentSlug
	})

	return merged
}

// mergeStatusesForSubComponent merges probe results for a single component/sub-component.
// Returns nil if the component/sub-component should be omitted from the report.
func mergeStatusesForSubComponent(componentSlug, subComponentSlug string, group []ProbeResult) *types.ComponentMonitorReportComponentStatus {
	hasError := false
	var nonErroredStatuses []types.ComponentMonitorReportComponentStatus

	for _, probeResult := range group {
		if probeResult.Error != nil {
			hasError = true
		} else {
			nonErroredStatuses = append(nonErroredStatuses, probeResult.ComponentMonitorReportComponentStatus)
		}
	}

	if hasError {
		allHealthy := true
		for _, status := range nonErroredStatuses {
			if status.Status != types.StatusHealthy {
				allHealthy = false
				break
			}
		}
		// If we have an error in a probe, and the sub-component would otherwise be healthy, we omit the status from the report so that the absent-report-checker can create an outage if it continues to error.
		// This allows an admin to look into the error with the probe.
		if allHealthy {
			return nil
		}
	}

	if len(nonErroredStatuses) == 0 {
		return nil
	}

	var allFailedReasons []types.Reason
	mostCriticalStatus := types.StatusHealthy

	for _, status := range nonErroredStatuses {
		if status.Status != types.StatusHealthy {
			allFailedReasons = append(allFailedReasons, status.Reasons...)
		}
		currentLevel := types.GetSeverityLevel(status.Status.ToSeverity())
		mostCriticalLevel := types.GetSeverityLevel(mostCriticalStatus.ToSeverity())
		if currentLevel > mostCriticalLevel {
			mostCriticalStatus = status.Status
		}
	}

	result := &types.ComponentMonitorReportComponentStatus{
		ComponentSlug:    componentSlug,
		SubComponentSlug: subComponentSlug,
		Status:           mostCriticalStatus,
		Reasons:          allFailedReasons,
	}
	if mostCriticalStatus == types.StatusHealthy {
		result.Reasons = nil
	}
	return result
}
