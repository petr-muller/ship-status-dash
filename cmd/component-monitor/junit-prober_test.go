package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"ship-status-dash/pkg/types"
)

// gcsListObjectURLTest must match the query string built by fetchGCSListPage so the mock can key URLs.
func gcsListObjectURLTest(bucket, job string) string {
	v := url.Values{}
	v.Set("prefix", "logs/"+job+"/")
	v.Set("delimiter", "/")
	v.Set("maxResults", "1000")
	v.Set("fields", "prefixes,nextPageToken")
	return fmt.Sprintf("https://www.googleapis.com/storage/v1/b/%s/o?%s", url.PathEscape(bucket), v.Encode())
}

// gcsProwObjectURL builds a direct GCS object URL (artifact_url_style gcs).
func gcsProwObjectURL(bucket, job string, segs ...string) string {
	tail := strings.Join(segs, "/")
	return fmt.Sprintf("https://storage.googleapis.com/%s/logs/%s/%s", bucket, job, tail)
}

type mockHTTPDoer struct {
	responses map[string]mockHTTPResponse
}

type mockHTTPResponse struct {
	body       string
	statusCode int
	err        error
}

func (m *mockHTTPDoer) Do(req *http.Request) (*http.Response, error) {
	r, ok := m.responses[req.URL.String()]
	if !ok {
		return nil, fmt.Errorf("unexpected URL: %s", req.URL.String())
	}
	if r.err != nil {
		return nil, r.err
	}
	code := r.statusCode
	if code == 0 {
		code = http.StatusOK
	}
	return &http.Response{
		StatusCode: code,
		Body:       io.NopCloser(strings.NewReader(r.body)),
	}, nil
}

func readJUnitFixture(t *testing.T, name string) string {
	t.Helper()
	p := filepath.Join("testdata", "junit", name)
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read %s: %v", p, err)
	}
	return string(b)
}

func TestAggregateJunitFromSuites_errorElement(t *testing.T) {
	const xmlBody = `<?xml version="1.0" encoding="UTF-8"?>
<testsuite tests="1">
  <testcase name="boom" classname="build-farm-canary"><error message="oops"/></testcase>
</testsuite>`
	suites, err := parseSuitesFromJunitBody(xmlBody)
	if err != nil {
		t.Fatalf("parseSuitesFromJunitBody: %v", err)
	}
	total, failed := aggregateJunitFromSuites(suites)
	if total != 1 {
		t.Errorf("total = %d, want 1", total)
	}
	want := []string{"boom"}
	if diff := cmp.Diff(want, failed); diff != "" {
		t.Errorf("failed (-want +got):\n%s", diff)
	}
}

func TestProwLogObjectURL(t *testing.T) {
	job := "periodic-ci-foo"
	bucket := "test-platform-results"
	cases := []struct {
		name  string
		p     *JUnitProber
		extra []string
		want  string
	}{
		{
			name:  "gcs",
			p:     NewJUnitProber("c", "s", bucket, job, time.Hour, types.SeverityDegraded, JUnitProberSettings{ArtifactURLStyle: types.JUnitArtifactStyleGCS, HistoryRuns: 1}, &http.Client{}),
			extra: []string{"1", "started.json"},
			want:  "https://storage.googleapis.com/test-platform-results/logs/periodic-ci-foo/1/started.json",
		},
		{
			name:  "gcsweb",
			p:     NewJUnitProber("c", "s", bucket, job, time.Hour, types.SeverityDegraded, JUnitProberSettings{ArtifactURLStyle: types.JUnitArtifactStyleGCSWeb, GCSWebBaseURL: "https://example-gcsweb.test", HistoryRuns: 1}, &http.Client{}),
			extra: []string{"1", "artifacts", "junit.xml"},
			want:  "https://example-gcsweb.test/gcs/test-platform-results/logs/periodic-ci-foo/1/artifacts/junit.xml",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.p.prowLogObjectURL(tc.extra...)
			if got != tc.want {
				t.Fatalf("prowLogObjectURL() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestJUnitProber_Probe(t *testing.T) {
	const job = "periodic-build-farm-canary-build01"
	const build = "123"
	bucket := defaultGCSBucket

	latestURL := gcsProwObjectURL(bucket, job, prowObjectLatestBuild)
	startedURL := gcsProwObjectURL(bucket, job, build, prowObjectStarted)
	finishedURL := gcsProwObjectURL(bucket, job, build, prowObjectFinished)
	xmlURL := gcsProwObjectURL(bucket, job, build, "artifacts", "junit_canary.xml")

	okSingle := readJUnitFixture(t, "ok_single.xml")
	failing := readJUnitFixture(t, "failing.xml")
	okTestSuites := readJUnitFixture(t, "ok_testsuites.xml")
	zero := readJUnitFixture(t, "zero_tests.xml")
	finishedBody := `{"timestamp":1,"result":"SUCCESS","passed":true}`

	customBucket := "my-bucket"
	customBase := fmt.Sprintf("https://storage.googleapis.com/%s/logs/%s", customBucket, job)
	customLatestURL := customBase + "/latest-build.txt"
	customStartedURL := customBase + "/456/started.json"
	customFinishedURL := customBase + "/456/finished.json"
	customXMLURL := customBase + "/456/artifacts/junit_canary.xml"

	tests := []struct {
		name           string
		bucket         string
		severity       types.Severity
		settings       JUnitProberSettings
		responses      map[string]mockHTTPResponse
		expectedError  bool
		expectedResult *types.ComponentMonitorReportComponentStatus
		expectedStatus types.Status
	}{
		{
			name:     "all tests pass",
			severity: types.SeverityDegraded,
			settings: JUnitProberSettings{HistoryRuns: 1, ArtifactURLStyle: types.JUnitArtifactStyleGCS},
			responses: map[string]mockHTTPResponse{
				latestURL:   {body: build},
				startedURL:  {body: recentStarted()},
				finishedURL: {body: finishedBody},
				xmlURL:      {body: okSingle},
			},
			expectedResult: &types.ComponentMonitorReportComponentStatus{
				ComponentSlug:    testComponentSlug,
				SubComponentSlug: testSubComponentSlug,
				Status:           types.StatusHealthy,
				Reasons:          nil,
			},
		},
		{
			name:     "some tests fail",
			severity: types.SeverityDegraded,
			settings: JUnitProberSettings{HistoryRuns: 1, ArtifactURLStyle: types.JUnitArtifactStyleGCS},
			responses: map[string]mockHTTPResponse{
				latestURL:   {body: build},
				startedURL:  {body: recentStarted()},
				finishedURL: {body: finishedBody},
				xmlURL:      {body: failing},
			},
			expectedResult: &types.ComponentMonitorReportComponentStatus{
				ComponentSlug:    testComponentSlug,
				SubComponentSlug: testSubComponentSlug,
				Status:           types.StatusDegraded,
				Reasons: []types.Reason{{
					Type:    types.CheckTypeJUnit,
					Check:   job,
					Results: "build 123: 2/4 tests failed: git-clone, quay-pull",
				}},
			},
		},
		{
			name:           "stale build",
			severity:       types.SeverityDegraded,
			settings:       JUnitProberSettings{HistoryRuns: 1, ArtifactURLStyle: types.JUnitArtifactStyleGCS},
			responses:      map[string]mockHTTPResponse{latestURL: {body: build}, startedURL: {body: staleStarted()}},
			expectedStatus: types.StatusDegraded,
		},
		{
			name:     "latest-build.txt fetch error",
			settings: JUnitProberSettings{HistoryRuns: 1, ArtifactURLStyle: types.JUnitArtifactStyleGCS},
			responses: map[string]mockHTTPResponse{
				latestURL: {err: fmt.Errorf("network error")},
			},
			expectedError: true,
		},
		{
			name:     "started.json returns 404",
			settings: JUnitProberSettings{HistoryRuns: 1, ArtifactURLStyle: types.JUnitArtifactStyleGCS},
			responses: map[string]mockHTTPResponse{
				latestURL:  {body: build},
				startedURL: {statusCode: 404, body: "not found"},
			},
			expectedError: true,
		},
		{
			name:     "junit xml fetch error on finished build",
			settings: JUnitProberSettings{HistoryRuns: 1, ArtifactURLStyle: types.JUnitArtifactStyleGCS},
			responses: map[string]mockHTTPResponse{
				latestURL:   {body: build},
				startedURL:  {body: recentStarted()},
				finishedURL: {body: finishedBody},
				xmlURL:      {statusCode: 404, body: "not found"},
			},
			expectedError: true,
		},
		{
			name:     "custom gcs bucket",
			bucket:   "my-bucket",
			severity: types.SeverityDegraded,
			settings: JUnitProberSettings{HistoryRuns: 1, ArtifactURLStyle: types.JUnitArtifactStyleGCS},
			responses: map[string]mockHTTPResponse{
				customLatestURL:   {body: "456"},
				customStartedURL:  {body: recentStarted()},
				customFinishedURL: {body: finishedBody},
				customXMLURL:      {body: okSingle},
			},
			expectedResult: &types.ComponentMonitorReportComponentStatus{
				ComponentSlug:    testComponentSlug,
				SubComponentSlug: testSubComponentSlug,
				Status:           types.StatusHealthy,
				Reasons:          nil,
			},
		},
		{
			name:     "default severity is degraded when unset",
			settings: JUnitProberSettings{HistoryRuns: 1, ArtifactURLStyle: types.JUnitArtifactStyleGCS},
			responses: map[string]mockHTTPResponse{
				latestURL:   {body: build},
				startedURL:  {body: recentStarted()},
				finishedURL: {body: finishedBody},
				xmlURL:      {body: failing},
			},
			expectedResult: &types.ComponentMonitorReportComponentStatus{
				ComponentSlug:    testComponentSlug,
				SubComponentSlug: testSubComponentSlug,
				Status:           types.StatusDegraded,
				Reasons: []types.Reason{{
					Type:    types.CheckTypeJUnit,
					Check:   job,
					Results: "build 123: 2/4 tests failed: git-clone, quay-pull",
				}},
			},
		},
		{
			name:     "testsuites wrapper root all pass",
			severity: types.SeverityDegraded,
			settings: JUnitProberSettings{HistoryRuns: 1, ArtifactURLStyle: types.JUnitArtifactStyleGCS},
			responses: map[string]mockHTTPResponse{
				latestURL:   {body: build},
				startedURL:  {body: recentStarted()},
				finishedURL: {body: finishedBody},
				xmlURL:      {body: okTestSuites},
			},
			expectedResult: &types.ComponentMonitorReportComponentStatus{
				ComponentSlug:    testComponentSlug,
				SubComponentSlug: testSubComponentSlug,
				Status:           types.StatusHealthy,
				Reasons:          nil,
			},
		},
		{
			name:     "invalid timestamp in started.json",
			severity: types.SeverityDegraded,
			settings: JUnitProberSettings{HistoryRuns: 1, ArtifactURLStyle: types.JUnitArtifactStyleGCS},
			responses: map[string]mockHTTPResponse{
				latestURL:  {body: build},
				startedURL: {body: `{"timestamp": 0}`},
			},
			expectedError: true,
		},
		{
			name:     "zero tests in junit xml",
			severity: types.SeverityDegraded,
			settings: JUnitProberSettings{HistoryRuns: 1, ArtifactURLStyle: types.JUnitArtifactStyleGCS},
			responses: map[string]mockHTTPResponse{
				latestURL:   {body: build},
				startedURL:  {body: recentStarted()},
				finishedURL: {body: finishedBody},
				xmlURL:      {body: zero},
			},
			expectedResult: &types.ComponentMonitorReportComponentStatus{
				ComponentSlug:    testComponentSlug,
				SubComponentSlug: testSubComponentSlug,
				Status:           types.StatusDegraded,
				Reasons: []types.Reason{{
					Type:    types.CheckTypeJUnit,
					Check:   job,
					Results: "build 123: zero tests found",
				}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := bucket
			if tt.bucket != "" {
				b = tt.bucket
			}
			prober := NewJUnitProber(
				testComponentSlug,
				testSubComponentSlug,
				b,
				job,
				2*time.Hour,
				tt.severity,
				tt.settings,
				&mockHTTPDoer{responses: tt.responses},
			)

			results := make(chan ProbeResult, 1)
			prober.Probe(context.Background(), results)

			var probeResult ProbeResult
			select {
			case probeResult = <-results:
			case <-time.After(500 * time.Millisecond):
				t.Fatal("timeout waiting for result")
			}

			var result types.ComponentMonitorReportComponentStatus
			var err error
			gotResult := false
			if probeResult.Error != nil {
				err = probeResult.Error
			} else {
				result = probeResult.ComponentMonitorReportComponentStatus
				gotResult = true
			}

			if tt.expectedError {
				if err == nil {
					t.Errorf("Probe() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("Probe() unexpected error: %v", err)
			}
			if !gotResult {
				t.Fatal("expected result but got none")
			}

			if tt.expectedResult != nil {
				if diff := cmp.Diff(tt.expectedResult, &result); diff != "" {
					t.Errorf("Probe() mismatch (-want +got):\n%s", diff)
				}
			} else if tt.expectedStatus != "" {
				if result.Status != tt.expectedStatus {
					t.Errorf("Probe() status = %q, want %q", result.Status, tt.expectedStatus)
				}
			}
		})
	}
}

func TestJUnitProber_Probe_fallback(t *testing.T) {
	const job = "periodic-build-farm-canary-build01"
	const latestBuild = "200"
	const prevBuild = "199"
	bucket := defaultGCSBucket

	latestURL := gcsProwObjectURL(bucket, job, prowObjectLatestBuild)
	startedLatest := gcsProwObjectURL(bucket, job, latestBuild, prowObjectStarted)
	finishedLatest := gcsProwObjectURL(bucket, job, latestBuild, prowObjectFinished)
	startedPrev := gcsProwObjectURL(bucket, job, prevBuild, prowObjectStarted)
	finishedPrev := gcsProwObjectURL(bucket, job, prevBuild, prowObjectFinished)
	xmlPrev := gcsProwObjectURL(bucket, job, prevBuild, "artifacts", "junit_canary.xml")
	listURL := gcsListObjectURLTest(bucket, job)

	okXML := readJUnitFixture(t, "ok_single.xml")
	failingXML := readJUnitFixture(t, "failing.xml")
	finishedBody := `{"timestamp":1,"result":"SUCCESS","passed":true}`

	listBody := fmt.Sprintf(`{"prefixes":["logs/%s/%s/","logs/%s/%s/"]}`, job, latestBuild, job, prevBuild)
	singleBuildList := fmt.Sprintf(`{"prefixes":["logs/%s/%s/"]}`, job, latestBuild)

	tests := []struct {
		name           string
		responses      map[string]mockHTTPResponse
		expectedError  bool
		expectedStatus types.Status
	}{
		{
			name: "falls back to previous build when latest not finished",
			responses: map[string]mockHTTPResponse{
				latestURL:      {body: latestBuild},
				startedLatest:  {body: recentStarted()},
				finishedLatest: {statusCode: 404, body: "not found"},
				listURL:        {body: listBody},
				startedPrev:    {body: recentStarted()},
				finishedPrev:   {body: finishedBody},
				xmlPrev:        {body: okXML},
			},
			expectedStatus: types.StatusHealthy,
		},
		{
			name: "fallback reports failures from previous build",
			responses: map[string]mockHTTPResponse{
				latestURL:      {body: latestBuild},
				startedLatest:  {body: recentStarted()},
				finishedLatest: {statusCode: 404, body: "not found"},
				listURL:        {body: listBody},
				startedPrev:    {body: recentStarted()},
				finishedPrev:   {body: finishedBody},
				xmlPrev:        {body: failingXML},
			},
			expectedStatus: types.StatusDegraded,
		},
		{
			name: "fallback to stale previous build reports staleness",
			responses: map[string]mockHTTPResponse{
				latestURL:      {body: latestBuild},
				startedLatest:  {body: recentStarted()},
				finishedLatest: {statusCode: 404, body: "not found"},
				listURL:        {body: listBody},
				startedPrev:    {body: staleStarted()},
			},
			expectedStatus: types.StatusDegraded,
		},
		{
			name: "fallback xml also missing returns error",
			responses: map[string]mockHTTPResponse{
				latestURL:      {body: latestBuild},
				startedLatest:  {body: recentStarted()},
				finishedLatest: {statusCode: 404, body: "not found"},
				listURL:        {body: listBody},
				startedPrev:    {body: recentStarted()},
				finishedPrev:   {body: finishedBody},
				xmlPrev:        {statusCode: 404, body: "not found"},
			},
			expectedError: true,
		},
		{
			name: "no previous build returns error",
			responses: map[string]mockHTTPResponse{
				latestURL:      {body: latestBuild},
				startedLatest:  {body: recentStarted()},
				finishedLatest: {statusCode: 404, body: "not found"},
				listURL:        {body: singleBuildList},
			},
			expectedError: true,
		},
		{
			name: "finished.json 500 propagates error instead of falling back",
			responses: map[string]mockHTTPResponse{
				latestURL:      {body: latestBuild},
				startedLatest:  {body: recentStarted()},
				finishedLatest: {statusCode: 500, body: "internal server error"},
			},
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewJUnitProber(testComponentSlug, testSubComponentSlug, bucket, job, 2*time.Hour, types.SeverityDegraded,
				JUnitProberSettings{HistoryRuns: 1, ArtifactURLStyle: types.JUnitArtifactStyleGCS},
				&mockHTTPDoer{responses: tt.responses})
			results := make(chan ProbeResult, 1)
			p.Probe(context.Background(), results)
			var res ProbeResult
			select {
			case res = <-results:
			case <-time.After(500 * time.Millisecond):
				t.Fatal("timeout waiting for result")
			}
			if tt.expectedError {
				if res.Error == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if res.Error != nil {
				t.Fatalf("unexpected error: %v", res.Error)
			}
			if res.Status != tt.expectedStatus {
				t.Errorf("want %s, got %s", tt.expectedStatus, res.Status)
			}
		})
	}
}

func TestJUnitProber_Probe_history(t *testing.T) {
	const job = "history-job"
	bucket := defaultGCSBucket
	const latest = "200"

	ok := readJUnitFixture(t, "ok_single.xml")
	failing := readJUnitFixture(t, "failing.xml")
	finishedBody := `{"timestamp":1,"result":"SUCCESS","passed":true}`

	latestURL := gcsProwObjectURL(bucket, job, prowObjectLatestBuild)
	startedURL := gcsProwObjectURL(bucket, job, latest, prowObjectStarted)
	finishedURL := gcsProwObjectURL(bucket, job, latest, prowObjectFinished)
	startedPrev := gcsProwObjectURL(bucket, job, "199", prowObjectStarted)
	listURL := gcsListObjectURLTest(bucket, job)
	xml200 := gcsProwObjectURL(bucket, job, "200", "artifacts", "junit_canary.xml")
	xml199 := gcsProwObjectURL(bucket, job, "199", "artifacts", "junit_canary.xml")

	listBody := fmt.Sprintf(`{"prefixes":["logs/%s/200/","logs/%s/199/"]}`, job, job)

	onlyA := `<?xml version="1.0" encoding="UTF-8"?>
<testsuite name="canary" tests="1" failures="1" errors="0">
  <testcase name="flaky-a" classname="c"><failure/></testcase>
</testsuite>`
	onlyB := `<?xml version="1.0" encoding="UTF-8"?>
<testsuite name="canary" tests="1" failures="1" errors="0">
  <testcase name="flaky-b" classname="c"><failure/></testcase>
</testsuite>`

	tests := []struct {
		name           string
		responses      map[string]mockHTTPResponse
		expectedError  bool
		expectedStatus types.Status
	}{
		{
			name: "both builds fail with same pattern reports degraded",
			responses: map[string]mockHTTPResponse{
				latestURL:   {body: latest},
				startedURL:  {body: recentStarted()},
				finishedURL: {body: finishedBody},
				listURL:     {body: listBody},
				xml200:      {body: failing},
				xml199:      {body: failing},
			},
			expectedStatus: types.StatusDegraded,
		},
		{
			name: "one pass one fail stays healthy below threshold",
			responses: map[string]mockHTTPResponse{
				latestURL:   {body: latest},
				startedURL:  {body: recentStarted()},
				finishedURL: {body: finishedBody},
				listURL:     {body: listBody},
				xml200:      {body: ok},
				xml199:      {body: failing},
			},
			expectedStatus: types.StatusHealthy,
		},
		{
			name: "different failure signatures stay healthy below threshold",
			responses: map[string]mockHTTPResponse{
				latestURL:   {body: latest},
				startedURL:  {body: recentStarted()},
				finishedURL: {body: finishedBody},
				listURL:     {body: listBody},
				xml200:      {body: onlyA},
				xml199:      {body: onlyB},
			},
			expectedStatus: types.StatusHealthy,
		},
		{
			name: "excludes unfinished latest from evaluation",
			responses: map[string]mockHTTPResponse{
				latestURL:   {body: latest},
				startedURL:  {body: recentStarted()},
				finishedURL: {statusCode: 404, body: "not found"},
				listURL:     {body: listBody},
				startedPrev: {body: recentStarted()},
				xml199:      {body: ok},
			},
			expectedStatus: types.StatusHealthy,
		},
		{
			name: "errors on non-latest build 404",
			responses: map[string]mockHTTPResponse{
				latestURL:   {body: latest},
				startedURL:  {body: recentStarted()},
				finishedURL: {body: finishedBody},
				listURL:     {body: listBody},
				xml200:      {body: ok},
				xml199:      {statusCode: 404, body: "not found"},
			},
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewJUnitProber(
				testComponentSlug, testSubComponentSlug, bucket, job, 2*time.Hour, types.SeverityDegraded,
				JUnitProberSettings{HistoryRuns: 2, FailedRunsThreshold: 2, ArtifactURLStyle: types.JUnitArtifactStyleGCS},
				&mockHTTPDoer{responses: tt.responses},
			)
			results := make(chan ProbeResult, 1)
			p.Probe(context.Background(), results)
			var res ProbeResult
			select {
			case res = <-results:
			case <-time.After(500 * time.Millisecond):
				t.Fatal("timeout waiting for result")
			}
			if tt.expectedError {
				if res.Error == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if res.Error != nil {
				t.Fatalf("unexpected error: %v", res.Error)
			}
			if res.Status != tt.expectedStatus {
				t.Errorf("want %s, got %s", tt.expectedStatus, res.Status)
			}
		})
	}
}

func recentStarted() string {
	return fmt.Sprintf(`{"timestamp": %d, "node": "node1"}`, time.Now().Add(-30*time.Minute).Unix())
}

func staleStarted() string {
	return fmt.Sprintf(`{"timestamp": %d, "node": "node1"}`, time.Now().Add(-3*time.Hour).Unix())
}
