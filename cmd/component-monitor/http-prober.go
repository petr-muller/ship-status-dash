package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"ship-status-dash/pkg/types"
)

type HTTPProber struct {
	componentSlug      string
	subComponentSlug   string
	url                string
	expectedStatusCode int
	confirmAfter       time.Duration
	severity           types.Severity
}

func NewHTTPProber(componentSlug string, subComponentSlug string, url string, expectedStatusCode int, confirmAfter time.Duration, severity types.Severity) *HTTPProber {
	if severity == "" {
		severity = types.SeverityDown
	}
	return &HTTPProber{
		componentSlug:      componentSlug,
		subComponentSlug:   subComponentSlug,
		url:                url,
		expectedStatusCode: expectedStatusCode,
		confirmAfter:       confirmAfter,
		severity:           severity,
	}
}

func (p *HTTPProber) formatErrorResult(err error) ProbeResult {
	return ProbeResult{
		ComponentMonitorReportComponentStatus: types.ComponentMonitorReportComponentStatus{
			ComponentSlug:    p.componentSlug,
			SubComponentSlug: p.subComponentSlug,
		},
		ProbeType: ProbeTypeHTTP,
		Error:     fmt.Errorf("error running HTTP probe, for component: %s sub-component %s. url: %s. error: %w", p.componentSlug, p.subComponentSlug, p.url, err),
	}
}

func (p *HTTPProber) makeStatus(statusCode int) ProbeResult {
	var status types.Status
	if statusCode == p.expectedStatusCode {
		status = types.StatusHealthy
	} else {
		status = p.severity.ToStatus()
	}

	return ProbeResult{
		ComponentMonitorReportComponentStatus: types.ComponentMonitorReportComponentStatus{
			ComponentSlug:    p.componentSlug,
			SubComponentSlug: p.subComponentSlug,
			Status:           status,
			Reasons: []types.Reason{{
				Type:    types.CheckTypeHTTP,
				Check:   p.url,
				Results: fmt.Sprintf("Status code %d (expected %d)", statusCode, p.expectedStatusCode),
			}},
		},
		ProbeType: ProbeTypeHTTP,
	}
}

func (p *HTTPProber) Probe(ctx context.Context, results chan<- ProbeResult) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.url, nil)
	if err != nil {
		results <- p.formatErrorResult(err)
		return
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		results <- p.formatErrorResult(err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == p.expectedStatusCode {
		results <- p.makeStatus(resp.StatusCode)
		return
	}

	// Wait for the confirmAfter duration to see if the status code changes
	select {
	case <-ctx.Done():
		results <- p.formatErrorResult(ctx.Err())
		return
	case <-time.After(p.confirmAfter):
	}

	req, err = http.NewRequestWithContext(ctx, http.MethodGet, p.url, nil)
	if err != nil {
		results <- p.formatErrorResult(err)
		return
	}

	resp, err = client.Do(req)
	if err != nil {
		results <- p.formatErrorResult(err)
		return
	}
	defer resp.Body.Close()

	results <- p.makeStatus(resp.StatusCode)
}
