package main

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"ship-status-dash/pkg/types"
)

const (
	defaultGCSBucket      = "test-platform-results"
	prowObjectLatestBuild = "latest-build.txt"
	prowObjectStarted     = "started.json"
	prowObjectFinished    = "finished.json"
	junitProwPath         = "artifacts/junit_canary.xml"
	maxTextBodyBytes      = 4 * 1024 * 1024
	defaultGCSWebBase     = types.JUnitDefaultGCSWebBase

	// History grouping: runs count toward the threshold when they share the
	// same normalized failure (sorted unique failing test names, or a zero-test run).
	junitSignatureZero      = "zero_tests"
	junitSignatureFailedPfx = "failed:" // + sorted, comma-joined test names
)

type httpStatusError struct {
	StatusCode int
	URL        string
}

func (e *httpStatusError) Error() string {
	return fmt.Sprintf("HTTP %d from %s", e.StatusCode, e.URL)
}

// junitHTTPClient is the HTTP client used to GET Prow and GCS resources. *http.Client implements it.
type junitHTTPClient interface {
	Do(*http.Request) (*http.Response, error)
}

// JUnitProberSettings holds JUnit prober options parsed from JUnitMonitor.
type JUnitProberSettings struct {
	ArtifactURLStyle    string
	GCSWebBaseURL       string
	HistoryRuns         int
	FailedRunsThreshold int
}

// JUnitProber fetches Prow JUnit XML from GCS or GCSweb and reports component health.
type JUnitProber struct {
	componentSlug    string
	subComponentSlug string
	bucket           string
	jobName          string
	maxAge           time.Duration
	severity         types.Severity
	client           junitHTTPClient
	settings         JUnitProberSettings
}

// NewJUnitProber returns a JUnitProber. It uses default bucket, severity, and artifact style when empty,
// enforces a minimum history of 1, and sets FailedRunsThreshold to 1 when a single run is configured.
// jobName is trimmed of leading and trailing whitespace before use in URLs and reasons.
func NewJUnitProber(componentSlug, subComponentSlug, bucket, jobName string, maxAge time.Duration, severity types.Severity, settings JUnitProberSettings, client junitHTTPClient) *JUnitProber {
	jobName = strings.TrimSpace(jobName)
	if bucket == "" {
		bucket = defaultGCSBucket
	}
	if severity == "" {
		severity = types.SeverityDegraded
	}
	if settings.HistoryRuns < 1 {
		settings.HistoryRuns = 1
	}
	if settings.HistoryRuns == 1 {
		settings.FailedRunsThreshold = 1
	}
	if settings.ArtifactURLStyle == "" {
		settings.ArtifactURLStyle = types.JUnitArtifactStyleGCS
	}
	return &JUnitProber{
		componentSlug:    componentSlug,
		subComponentSlug: subComponentSlug,
		bucket:           bucket,
		jobName:          jobName,
		maxAge:           maxAge,
		severity:         severity,
		client:           client,
		settings:         settings,
	}
}

func (p *JUnitProber) prowLogObjectURL(segments ...string) string {
	tail := strings.Join(segments, "/")
	switch p.settings.ArtifactURLStyle {
	case types.JUnitArtifactStyleGCSWeb:
		base := strings.TrimRight(p.gcswebBase(), "/")
		return fmt.Sprintf("%s/gcs/%s/logs/%s/%s", base, p.bucket, p.jobName, tail)
	default:
		return fmt.Sprintf("https://storage.googleapis.com/%s/logs/%s/%s", p.bucket, p.jobName, tail)
	}
}

func (p *JUnitProber) gcswebBase() string {
	if strings.TrimSpace(p.settings.GCSWebBaseURL) != "" {
		return strings.TrimRight(p.settings.GCSWebBaseURL, "/")
	}
	return defaultGCSWebBase
}

func (p *JUnitProber) formatErrorResult(err error) ProbeResult {
	return ProbeResult{
		ComponentMonitorReportComponentStatus: types.ComponentMonitorReportComponentStatus{
			ComponentSlug:    p.componentSlug,
			SubComponentSlug: p.subComponentSlug,
		},
		Error: fmt.Errorf("error running JUnit probe, for component: %s sub-component %s. job: %s. error: %w", p.componentSlug, p.subComponentSlug, p.jobName, err),
	}
}

// checkBuildStaleness fetches started.json for the given build and checks
// whether it exceeds maxAge. Returns a non-nil *ProbeResult when the build
// is stale; returns (nil, nil) when fresh; returns (nil, err) on fetch/parse
// failures.
func (p *JUnitProber) checkBuildStaleness(ctx context.Context, buildID string) (*ProbeResult, error) {
	startedBody, err := p.fetchText(ctx, p.prowLogObjectURL(buildID, prowObjectStarted))
	if err != nil {
		return nil, fmt.Errorf("fetching started.json for build %s: %w", buildID, err)
	}
	var started prowStarted
	if err := json.Unmarshal([]byte(startedBody), &started); err != nil {
		return nil, fmt.Errorf("parsing started.json for build %s: %w", buildID, err)
	}
	if started.Timestamp <= 0 {
		return nil, fmt.Errorf("invalid or missing timestamp in started.json for build %s", buildID)
	}
	if age := time.Since(time.Unix(started.Timestamp, 0)); age > p.maxAge {
		return &ProbeResult{
			ComponentMonitorReportComponentStatus: types.ComponentMonitorReportComponentStatus{
				ComponentSlug:    p.componentSlug,
				SubComponentSlug: p.subComponentSlug,
				Status:           p.severity.ToStatus(),
				Reasons: []types.Reason{{
					Type:    types.CheckTypeJUnit,
					Check:   p.jobName,
					Results: fmt.Sprintf("latest build %s started %s ago (max age %s)", buildID, age.Round(time.Minute), p.maxAge),
				}},
			},
		}, nil
	}
	return nil, nil
}

func (p *JUnitProber) isBuildFinished(ctx context.Context, buildID string) (bool, error) {
	_, err := p.fetchText(ctx, p.prowLogObjectURL(buildID, prowObjectFinished))
	if err == nil {
		return true, nil
	}
	var httpErr *httpStatusError
	if errors.As(err, &httpErr) && httpErr.StatusCode == http.StatusNotFound {
		return false, nil
	}
	return false, err
}

func (p *JUnitProber) Probe(ctx context.Context, results chan<- ProbeResult) {
	latest, err := p.fetchText(ctx, p.prowLogObjectURL(prowObjectLatestBuild))
	if err != nil {
		results <- p.formatErrorResult(fmt.Errorf("fetching latest build ID: %w", err))
		return
	}
	latest = strings.TrimSpace(latest)
	if latest == "" {
		results <- p.formatErrorResult(fmt.Errorf("empty latest build ID from %s", prowObjectLatestBuild))
		return
	}

	staleResult, err := p.checkBuildStaleness(ctx, latest)
	if err != nil {
		results <- p.formatErrorResult(err)
		return
	}
	if staleResult != nil {
		results <- *staleResult
		return
	}

	// If the latest build hasn't finished yet, fall back to the previous
	// completed build so we don't error on missing artifacts.
	unfinishedBuild := ""
	finished, finErr := p.isBuildFinished(ctx, latest)
	if finErr != nil {
		results <- p.formatErrorResult(fmt.Errorf("checking finished.json for build %s: %w", latest, finErr))
		return
	}
	if !finished {
		unfinishedBuild = latest
		prevID, findErr := p.findPreviousBuildID(ctx, latest)
		if findErr != nil {
			results <- p.formatErrorResult(fmt.Errorf("latest build %s not finished and %w", latest, findErr))
			return
		}
		staleResult, err = p.checkBuildStaleness(ctx, prevID)
		if err != nil {
			results <- p.formatErrorResult(err)
			return
		}
		if staleResult != nil {
			results <- *staleResult
			return
		}
		latest = prevID
	}

	builds, err := p.resolveBuildIDsToEvaluate(ctx, latest)
	if err != nil {
		results <- p.formatErrorResult(err)
		return
	}

	// Exclude the unfinished build if it appeared in the GCS listing.
	if unfinishedBuild != "" {
		filtered := builds[:0]
		for _, b := range builds {
			if b != unfinishedBuild {
				filtered = append(filtered, b)
			}
		}
		builds = filtered
	}

	if p.settings.HistoryRuns == 1 {
		r, perr := p.probeJunitForBuildID(ctx, builds[0])
		if perr != nil {
			results <- p.formatErrorResult(perr)
			return
		}
		results <- r
		return
	}

	threshold := p.settings.FailedRunsThreshold // validated in loadAndValidateConfig when history_runs > 1

	// For each Prow run, the failure signature is the sorted set of failing
	// test names (or a dedicated zero-tests key). A run counts toward
	// the threshold only together with other runs that share the same key.
	signatureCount := make(map[string]int)
	summaries := make([]string, 0, len(builds))
	for _, b := range builds {
		total, failed, perr := p.junitStatsForBuild(ctx, b)
		if perr != nil {
			results <- p.formatErrorResult(fmt.Errorf("build %s: %w", b, perr))
			return
		}
		summaries = append(summaries, fmt.Sprintf("build %s: %s", b, formatJunitBuildSummary(total, failed)))
		if !junitUnhealthy(total, failed) {
			continue
		}
		sig := junitFailureSignature(total, failed)
		signatureCount[sig]++
	}

	highestSig, highestCount := "", 0
	for sig, n := range signatureCount {
		if n > highestCount {
			highestCount, highestSig = n, sig
		} else if n == highestCount && n > 0 && (highestSig == "" || sig < highestSig) {
			highestSig = sig
		}
	}
	if highestCount < threshold {
		// Healthy: omit Reasons — mergeStatuses strips them for Healthy in the report anyway.
		results <- ProbeResult{
			ComponentMonitorReportComponentStatus: types.ComponentMonitorReportComponentStatus{
				ComponentSlug:    p.componentSlug,
				SubComponentSlug: p.subComponentSlug,
				Status:           types.StatusHealthy,
				Reasons:          nil,
			},
		}
		return
	}
	reason := fmt.Sprintf(
		"junit: %d of the last %d run(s) share the same failure pattern: %s (threshold %d) — %s",
		highestCount, len(builds), formatJunitSignatureShort(highestSig), threshold, strings.Join(summaries, " | "),
	)
	results <- ProbeResult{
		ComponentMonitorReportComponentStatus: types.ComponentMonitorReportComponentStatus{
			ComponentSlug:    p.componentSlug,
			SubComponentSlug: p.subComponentSlug,
			Status:           p.severity.ToStatus(),
			Reasons:          []types.Reason{{Type: types.CheckTypeJUnit, Check: p.jobName, Results: reason}},
		},
	}
}

func (p *JUnitProber) resolveBuildIDsToEvaluate(ctx context.Context, latestFromFile string) ([]string, error) {
	if p.settings.HistoryRuns <= 1 {
		return []string{latestFromFile}, nil
	}
	ids, err := p.listBuildIDPrefixes(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing recent builds: %w", err)
	}
	seen := make(map[string]struct{})
	seen[latestFromFile] = struct{}{}
	for _, id := range ids {
		seen[id] = struct{}{}
	}
	var all []string
	for id := range seen {
		all = append(all, id)
	}
	sortBuildIDsDesc(all)
	n := p.settings.HistoryRuns
	if len(all) < n {
		return all, nil
	}
	return all[:n], nil
}

func (p *JUnitProber) findPreviousBuildID(ctx context.Context, currentBuildID string) (string, error) {
	ids, err := p.listBuildIDPrefixes(ctx)
	if err != nil {
		return "", fmt.Errorf("listing builds for fallback: %w", err)
	}
	seen := map[string]struct{}{currentBuildID: {}}
	for _, id := range ids {
		seen[id] = struct{}{}
	}
	all := make([]string, 0, len(seen))
	for id := range seen {
		all = append(all, id)
	}
	sortBuildIDsDesc(all)
	for i, id := range all {
		if id == currentBuildID && i+1 < len(all) {
			return all[i+1], nil
		}
	}
	return "", fmt.Errorf("no previous build found before %s", currentBuildID)
}

func (p *JUnitProber) listBuildIDPrefixes(ctx context.Context) ([]string, error) {
	jobPrefix := "logs/" + p.jobName + "/"
	unique := make(map[string]struct{})
	for page, token := 0, ""; page < 50; page++ {
		body, err := p.fetchGCSListPage(ctx, jobPrefix, token)
		if err != nil {
			return nil, err
		}
		var r gcsListObjectResponse
		if err := json.Unmarshal([]byte(body), &r); err != nil {
			return nil, fmt.Errorf("parsing GCS list: %w", err)
		}
		for _, pr := range r.Prefixes {
			id, ok := buildIDFromPrefixPath(pr, p.jobName)
			if ok {
				unique[id] = struct{}{}
			}
		}
		if r.NextPageToken == "" {
			break
		}
		token = r.NextPageToken
	}
	ids := make([]string, 0, len(unique))
	for k := range unique {
		ids = append(ids, k)
	}
	return ids, nil
}

func (p *JUnitProber) fetchGCSListPage(ctx context.Context, prefix, pageToken string) (string, error) {
	const pageSize = 1000
	v := url.Values{}
	v.Set("prefix", prefix)
	v.Set("delimiter", "/")
	v.Set("maxResults", fmt.Sprint(pageSize))
	v.Set("fields", "prefixes,nextPageToken")
	if pageToken != "" {
		v.Set("pageToken", pageToken)
	}
	escapedBucket := url.PathEscape(p.bucket)
	u := fmt.Sprintf("https://www.googleapis.com/storage/v1/b/%s/o?%s", escapedBucket, v.Encode())
	return p.fetchText(ctx, u)
}

type gcsListObjectResponse struct {
	Prefixes      []string `json:"prefixes"`
	NextPageToken string   `json:"nextPageToken"`
}

func buildIDFromPrefixPath(prefix, jobName string) (string, bool) {
	trim := strings.TrimLeft(prefix, "/")
	expected := "logs/" + jobName + "/"
	if !strings.HasPrefix(trim, expected) {
		return "", false
	}
	rest := strings.TrimPrefix(trim, expected)
	if rest == "" {
		return "", false
	}
	id := strings.SplitN(rest, "/", 2)[0]
	if id == "" {
		return "", false
	}
	return id, true
}

func sortBuildIDsDesc(ids []string) {
	sort.Slice(ids, func(i, j int) bool { return buildIDGreater(ids[i], ids[j]) })
}

func buildIDGreater(a, b string) bool {
	ai, ok1 := new(big.Int).SetString(a, 10)
	bi, ok2 := new(big.Int).SetString(b, 10)
	if ok1 && ok2 {
		return ai.Cmp(bi) > 0
	}
	return a > b
}

func (p *JUnitProber) probeJunitForBuildID(ctx context.Context, buildID string) (ProbeResult, error) {
	xmlBody, err := p.fetchJunitBody(ctx, buildID)
	if err != nil {
		return ProbeResult{}, err
	}
	return p.makeStatusFromXMLBody(buildID, xmlBody)
}

func (p *JUnitProber) fetchJunitBody(ctx context.Context, buildID string) (string, error) {
	segs := append([]string{buildID}, strings.Split(junitProwPath, "/")...)
	u := p.prowLogObjectURL(segs...)
	return p.fetchText(ctx, u)
}

func (p *JUnitProber) makeStatusFromXMLBody(buildID, xmlBody string) (ProbeResult, error) {
	suites, err := parseSuitesFromJunitBody(xmlBody)
	if err != nil {
		return ProbeResult{}, fmt.Errorf("parsing junit XML for build %s: %w", buildID, err)
	}
	total, failed := aggregateJunitFromSuites(suites)
	return p.makeStatusFromAggregates(buildID, total, failed), nil
}

// parseSuitesFromJunitBody decodes a JUnit XML body into one or more suites
// (testsuite root or a single synthetic suite from a testsuite payload).
func parseSuitesFromJunitBody(xmlBody string) ([]junitSuite, error) {
	if strings.TrimSpace(xmlBody) == "" {
		return nil, fmt.Errorf("empty JUnit body")
	}
	var doc junitDoc
	if err := xml.NewDecoder(strings.NewReader(xmlBody)).Decode(&doc); err != nil {
		return nil, err
	}
	suites := doc.Suites
	if len(suites) == 0 {
		suites = []junitSuite{{Tests: doc.Tests, TestCases: doc.TestCases}}
	}
	return suites, nil
}

func aggregateJunitFromSuites(suites []junitSuite) (total int, failedSorted []string) {
	seen := make(map[string]struct{}, 4)
	for _, s := range suites {
		total += s.Tests
		for _, tc := range s.TestCases {
			if tc.Failure == nil && tc.Error == nil {
				continue
			}
			if tc.Name != "" {
				seen[tc.Name] = struct{}{}
			}
		}
	}
	if len(seen) == 0 {
		return total, nil
	}
	failedSorted = make([]string, 0, len(seen))
	for n := range seen {
		failedSorted = append(failedSorted, n)
	}
	sort.Strings(failedSorted)
	return total, failedSorted
}

// junitUnhealthy is true for zero total tests (no coverage) or any failed testcase.
func junitUnhealthy(total int, failed []string) bool {
	return total == 0 || len(failed) > 0
}

// junitFailureSignature is a key for the same failure "reason" across runs. Only call when junitUnhealthy.
// totalJUnitCases is the sum of suite tests attributes from JUnit XML (same as aggregateJunitFromSuites).
func junitFailureSignature(totalJUnitCases int, failed []string) string {
	if totalJUnitCases == 0 {
		return junitSignatureZero
	}
	// Unhealthy, totalJUnitCases>0, non-empty failed (sorted unique) per aggregateJunitFromSuites.
	if len(failed) == 0 {
		return ""
	}
	return junitSignatureFailedPfx + strings.Join(failed, ",")
}

func formatJunitSignatureShort(sig string) string {
	if sig == junitSignatureZero {
		return "zero tests (no JUnit test cases, or total tests=0)"
	}
	if !strings.HasPrefix(sig, junitSignatureFailedPfx) {
		return sig
	}
	names := strings.Split(strings.TrimPrefix(sig, junitSignatureFailedPfx), ",")
	if len(names) == 0 {
		return "failed: (unknown cases)"
	}
	return "failed: " + strings.Join(names, ", ")
}

func (p *JUnitProber) junitStatsForBuild(ctx context.Context, buildID string) (total int, failed []string, err error) {
	xml, err := p.fetchJunitBody(ctx, buildID)
	if err != nil {
		return 0, nil, err
	}
	suites, err := parseSuitesFromJunitBody(xml)
	if err != nil {
		return 0, nil, err
	}
	t, f := aggregateJunitFromSuites(suites)
	return t, f, nil
}

func formatJunitBuildSummary(total int, failed []string) string {
	switch {
	case total == 0:
		return "zero tests found"
	case len(failed) == 0:
		return fmt.Sprintf("all %d tests passed", total)
	default:
		return fmt.Sprintf("%d/%d tests failed: %s", len(failed), total, strings.Join(failed, ", "))
	}
}

func (p *JUnitProber) makeStatusFromAggregates(buildID string, total int, failed []string) ProbeResult {
	var status types.Status
	var reasons []types.Reason
	switch {
	case total == 0:
		status = p.severity.ToStatus()
		reasons = []types.Reason{{
			Type:    types.CheckTypeJUnit,
			Check:   p.jobName,
			Results: fmt.Sprintf("build %s: zero tests found", buildID),
		}}
	case len(failed) == 0:
		status = types.StatusHealthy
		// Omit Reasons for Healthy — mergeStatuses strips them in the report anyway.
		reasons = nil
	default:
		status = p.severity.ToStatus()
		reasons = []types.Reason{{
			Type:    types.CheckTypeJUnit,
			Check:   p.jobName,
			Results: fmt.Sprintf("build %s: %d/%d tests failed: %s", buildID, len(failed), total, strings.Join(failed, ", ")),
		}}
	}
	return ProbeResult{
		ComponentMonitorReportComponentStatus: types.ComponentMonitorReportComponentStatus{
			ComponentSlug:    p.componentSlug,
			SubComponentSlug: p.subComponentSlug,
			Status:           status,
			Reasons:          reasons,
		},
	}
}

func (p *JUnitProber) fetchText(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, resp.Body)
		return "", &httpStatusError{StatusCode: resp.StatusCode, URL: url}
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxTextBodyBytes))
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// junitDoc unmarshals Prow JUnit with either a testsuite or testsuites root element.
type junitDoc struct {
	Suites    []junitSuite `xml:"testsuite"`
	Tests     int          `xml:"tests,attr"`
	TestCases []junitCase  `xml:"testcase"`
}

type junitSuite struct {
	Tests     int         `xml:"tests,attr"`
	TestCases []junitCase `xml:"testcase"`
}

type junitCase struct {
	Name    string    `xml:"name,attr"`
	Failure *struct{} `xml:"failure"`
	Error   *struct{} `xml:"error"`
}

type prowStarted struct {
	Timestamp int64 `json:"timestamp"`
}
