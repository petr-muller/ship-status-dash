package main

import (
	"context"
	"errors"
	"testing"
	"time"

	promclientv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"

	"ship-status-dash/pkg/types"
)

const (
	testComponentSlug    = "test-component"
	testSubComponentSlug = "test-subcomponent"
)

type mockPrometheusAPI struct {
	queryResults map[string]queryResult
	queryErrors  map[string]error
}

type queryResult struct {
	value    model.Value
	warnings promclientv1.Warnings
}

func (m *mockPrometheusAPI) Query(ctx context.Context, query string, ts time.Time, opts ...promclientv1.Option) (model.Value, promclientv1.Warnings, error) {
	if err, ok := m.queryErrors[query]; ok {
		return nil, nil, err
	}
	if result, ok := m.queryResults[query]; ok {
		return result.value, result.warnings, nil
	}
	return nil, nil, nil
}

func (m *mockPrometheusAPI) QueryRange(ctx context.Context, query string, r promclientv1.Range, opts ...promclientv1.Option) (model.Value, promclientv1.Warnings, error) {
	return nil, nil, errors.New("not implemented")
}

func (m *mockPrometheusAPI) Series(ctx context.Context, matches []string, startTime time.Time, endTime time.Time, opts ...promclientv1.Option) ([]model.LabelSet, promclientv1.Warnings, error) {
	return nil, nil, errors.New("not implemented")
}

func (m *mockPrometheusAPI) LabelNames(ctx context.Context, matches []string, startTime time.Time, endTime time.Time, opts ...promclientv1.Option) ([]string, promclientv1.Warnings, error) {
	return nil, nil, errors.New("not implemented")
}

func (m *mockPrometheusAPI) LabelValues(ctx context.Context, label string, matches []string, startTime time.Time, endTime time.Time, opts ...promclientv1.Option) (model.LabelValues, promclientv1.Warnings, error) {
	return nil, nil, errors.New("not implemented")
}

func (m *mockPrometheusAPI) Targets(ctx context.Context) (promclientv1.TargetsResult, error) {
	return promclientv1.TargetsResult{}, errors.New("not implemented")
}

func (m *mockPrometheusAPI) TargetsMetadata(ctx context.Context, matchTarget string, metric string, limit string) ([]promclientv1.MetricMetadata, error) {
	return nil, errors.New("not implemented")
}

func (m *mockPrometheusAPI) Metadata(ctx context.Context, metric string, limit string) (map[string][]promclientv1.Metadata, error) {
	return nil, errors.New("not implemented")
}

func (m *mockPrometheusAPI) TSDB(ctx context.Context, opts ...promclientv1.Option) (promclientv1.TSDBResult, error) {
	return promclientv1.TSDBResult{}, errors.New("not implemented")
}

func (m *mockPrometheusAPI) QueryExemplars(ctx context.Context, query string, startTime time.Time, endTime time.Time) ([]promclientv1.ExemplarQueryResult, error) {
	return nil, errors.New("not implemented")
}

func (m *mockPrometheusAPI) Buildinfo(ctx context.Context) (promclientv1.BuildinfoResult, error) {
	return promclientv1.BuildinfoResult{}, errors.New("not implemented")
}

func (m *mockPrometheusAPI) Runtimeinfo(ctx context.Context) (promclientv1.RuntimeinfoResult, error) {
	return promclientv1.RuntimeinfoResult{}, errors.New("not implemented")
}

func (m *mockPrometheusAPI) Rules(ctx context.Context) (promclientv1.RulesResult, error) {
	return promclientv1.RulesResult{}, errors.New("not implemented")
}

func (m *mockPrometheusAPI) Alerts(ctx context.Context) (promclientv1.AlertsResult, error) {
	return promclientv1.AlertsResult{}, errors.New("not implemented")
}

func (m *mockPrometheusAPI) AlertManagers(ctx context.Context) (promclientv1.AlertManagersResult, error) {
	return promclientv1.AlertManagersResult{}, errors.New("not implemented")
}

func (m *mockPrometheusAPI) CleanTombstones(ctx context.Context) error {
	return errors.New("not implemented")
}

func (m *mockPrometheusAPI) Config(ctx context.Context) (promclientv1.ConfigResult, error) {
	return promclientv1.ConfigResult{}, errors.New("not implemented")
}

func (m *mockPrometheusAPI) DeleteSeries(ctx context.Context, matches []string, startTime time.Time, endTime time.Time) error {
	return errors.New("not implemented")
}

func (m *mockPrometheusAPI) Flags(ctx context.Context) (promclientv1.FlagsResult, error) {
	return promclientv1.FlagsResult{}, errors.New("not implemented")
}

func (m *mockPrometheusAPI) Snapshot(ctx context.Context, skipHead bool) (promclientv1.SnapshotResult, error) {
	return promclientv1.SnapshotResult{}, errors.New("not implemented")
}

func (m *mockPrometheusAPI) WalReplay(ctx context.Context) (promclientv1.WalReplayStatus, error) {
	return promclientv1.WalReplayStatus{}, errors.New("not implemented")
}

func TestPrometheusProber_Probe(t *testing.T) {
	tests := []struct {
		name                string
		queries             []types.PrometheusQuery
		queryResults        map[string]model.Value
		queryErrors         map[string]error
		queryWarnings       map[string]promclientv1.Warnings
		expectStatus        types.Status
		expectedReasonCount int
		expectedReasonType  types.CheckType
		expectError         bool
	}{
		{
			name: "success - single query returns vector",
			queries: []types.PrometheusQuery{
				{Query: "up{job=\"test\"}"},
			},
			queryResults: map[string]model.Value{
				"up{job=\"test\"}": model.Vector{
					&model.Sample{Value: 1.0},
				},
			},
			expectStatus:        types.StatusHealthy,
			expectedReasonCount: 1,
			expectedReasonType:  types.CheckTypePrometheus,
		},
		{
			name: "success - single query returns scalar",
			queries: []types.PrometheusQuery{
				{Query: "scalar_query"},
			},
			queryResults: map[string]model.Value{
				"scalar_query": &model.Scalar{Value: 1.0},
			},
			expectStatus:        types.StatusHealthy,
			expectedReasonCount: 1,
			expectedReasonType:  types.CheckTypePrometheus,
		},
		{
			name: "success - multiple queries all succeed",
			queries: []types.PrometheusQuery{
				{Query: "up{job=\"test1\"}"},
				{Query: "up{job=\"test2\"}"},
				{Query: "up{job=\"test3\"}"},
			},
			queryResults: map[string]model.Value{
				"up{job=\"test1\"}": model.Vector{&model.Sample{Value: 1.0}},
				"up{job=\"test2\"}": model.Vector{&model.Sample{Value: 1.0}},
				"up{job=\"test3\"}": model.Vector{&model.Sample{Value: 1.0}},
			},
			expectStatus:        types.StatusHealthy,
			expectedReasonCount: 3,
			expectedReasonType:  types.CheckTypePrometheus,
		},
		{
			name: "failure - single query returns empty vector",
			queries: []types.PrometheusQuery{
				{Query: "up{job=\"test\"}", Severity: types.SeverityDown},
			},
			queryResults: map[string]model.Value{
				"up{job=\"test\"}": model.Vector{},
			},
			expectStatus:        types.StatusDown,
			expectedReasonCount: 1,
			expectedReasonType:  types.CheckTypePrometheus,
		},
		{
			name: "failure - single query returns nil",
			queries: []types.PrometheusQuery{
				{Query: "up{job=\"test\"}", Severity: types.SeverityDown},
			},
			queryResults:        map[string]model.Value{},
			expectStatus:        types.StatusDown,
			expectedReasonCount: 1,
			expectedReasonType:  types.CheckTypePrometheus,
		},
		{
			name: "failure - all queries fail",
			queries: []types.PrometheusQuery{
				{Query: "up{job=\"test1\"}", Severity: types.SeverityDown},
				{Query: "up{job=\"test2\"}", Severity: types.SeverityDown},
			},
			queryResults: map[string]model.Value{
				"up{job=\"test1\"}": model.Vector{},
				"up{job=\"test2\"}": model.Vector{},
			},
			expectStatus:        types.StatusDown,
			expectedReasonCount: 2,
			expectedReasonType:  types.CheckTypePrometheus,
		},
		{
			name: "degraded - some queries succeed, some fail",
			queries: []types.PrometheusQuery{
				{Query: "up{job=\"test1\"}"},
				{Query: "up{job=\"test2\"}", Severity: types.SeverityDegraded},
				{Query: "up{job=\"test3\"}"},
			},
			queryResults: map[string]model.Value{
				"up{job=\"test1\"}": model.Vector{&model.Sample{Value: 1.0}},
				"up{job=\"test2\"}": model.Vector{},
				"up{job=\"test3\"}": model.Vector{&model.Sample{Value: 1.0}},
			},
			expectStatus:        types.StatusDegraded,
			expectedReasonCount: 1,
			expectedReasonType:  types.CheckTypePrometheus,
		},
		{
			name: "query error sent to error channel",
			queries: []types.PrometheusQuery{
				{Query: "up{job=\"test\"}", Severity: types.SeverityDown},
			},
			queryErrors: map[string]error{
				"up{job=\"test\"}": errors.New("prometheus query error"),
			},
			expectError: true,
		},
		{
			name: "matrix query succeeds",
			queries: []types.PrometheusQuery{
				{Query: "rate(http_requests_total[5m])"},
			},
			queryResults: map[string]model.Value{
				"rate(http_requests_total[5m])": model.Matrix{
					&model.SampleStream{
						Values: []model.SamplePair{
							{Value: 1.0},
							{Value: 2.0},
						},
					},
				},
			},
			expectStatus:        types.StatusHealthy,
			expectedReasonCount: 1,
			expectedReasonType:  types.CheckTypePrometheus,
		},
		{
			name: "matrix query fails with empty matrix",
			queries: []types.PrometheusQuery{
				{Query: "rate(http_requests_total[5m])", Severity: types.SeverityDown},
			},
			queryResults: map[string]model.Value{
				"rate(http_requests_total[5m])": model.Matrix{},
			},
			expectStatus:        types.StatusDown,
			expectedReasonCount: 1,
			expectedReasonType:  types.CheckTypePrometheus,
		},
		{
			name: "multiple queries fail with different severities - most critical wins",
			queries: []types.PrometheusQuery{
				{Query: "up{job=\"test1\"}", Severity: types.SeverityDegraded},
				{Query: "up{job=\"test2\"}", Severity: types.SeverityDown},
				{Query: "up{job=\"test3\"}", Severity: types.SeverityCapacityExhausted},
			},
			queryResults: map[string]model.Value{
				"up{job=\"test1\"}": model.Vector{},
				"up{job=\"test2\"}": model.Vector{},
				"up{job=\"test3\"}": model.Vector{},
			},
			expectStatus:        types.StatusDown,
			expectedReasonCount: 3,
			expectedReasonType:  types.CheckTypePrometheus,
		},
		{
			name: "capacity exhausted severity returns CapacityExhausted status",
			queries: []types.PrometheusQuery{
				{Query: "up{job=\"test\"}", Severity: types.SeverityCapacityExhausted},
			},
			queryResults: map[string]model.Value{
				"up{job=\"test\"}": model.Vector{},
			},
			expectStatus:        types.StatusCapacityExhausted,
			expectedReasonCount: 1,
			expectedReasonType:  types.CheckTypePrometheus,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAPI := &mockPrometheusAPI{
				queryResults: make(map[string]queryResult),
				queryErrors:  tt.queryErrors,
			}

			for query, value := range tt.queryResults {
				warnings := tt.queryWarnings[query]
				mockAPI.queryResults[query] = queryResult{
					value:    value,
					warnings: warnings,
				}
			}

			prober := NewPrometheusProber(
				testComponentSlug,
				testSubComponentSlug,
				mockAPI,
				tt.queries,
			)

			ctx := context.Background()
			results := make(chan ProbeResult, 1)

			prober.Probe(ctx, results)

			select {
			case probeResult := <-results:
				if tt.expectError {
					assert.NotNil(t, probeResult.Error)
				} else {
					assert.Nil(t, probeResult.Error)
					result := probeResult.ComponentMonitorReportComponentStatus
					assert.Equal(t, testComponentSlug, result.ComponentSlug)
					assert.Equal(t, testSubComponentSlug, result.SubComponentSlug)
					assert.Equal(t, tt.expectStatus, result.Status)
					assert.Len(t, result.Reasons, tt.expectedReasonCount)

					for _, reason := range result.Reasons {
						assert.Equal(t, tt.expectedReasonType, reason.Type)
						assert.NotEmpty(t, reason.Check)
						assert.NotEmpty(t, reason.Results)
					}
				}
			case <-time.After(500 * time.Millisecond):
				t.Fatal("timeout waiting for result")
			}
		})
	}
}

func TestExtractValue(t *testing.T) {
	tests := []struct {
		name     string
		input    model.Value
		expected string
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: "no result",
		},
		{
			name:     "empty vector",
			input:    model.Vector{},
			expected: "empty vector",
		},
		{
			name: "single-element vector",
			input: model.Vector{
				&model.Sample{
					Metric: model.Metric{"job": "test"},
					Value:  1.0,
				},
			},
			expected: "1",
		},
		{
			name: "multi-element vector with single label",
			input: model.Vector{
				&model.Sample{
					Metric: model.Metric{"state": "free"},
					Value:  0.0,
				},
				&model.Sample{
					Metric: model.Metric{"state": "leased"},
					Value:  10.0,
				},
				&model.Sample{
					Metric: model.Metric{"state": "dirty"},
					Value:  2.0,
				},
			},
			expected: "{state=free}=0, {state=leased}=10, {state=dirty}=2",
		},
		{
			name: "multi-element vector with multiple labels",
			input: model.Vector{
				&model.Sample{
					Metric: model.Metric{"type": "aws-quota-slice", "state": "free"},
					Value:  5.0,
				},
				&model.Sample{
					Metric: model.Metric{"type": "aws-quota-slice", "state": "leased"},
					Value:  15.0,
				},
			},
			expected: "{state=free,type=aws-quota-slice}=5, {state=leased,type=aws-quota-slice}=15",
		},
		{
			name:     "scalar",
			input:    &model.Scalar{Value: 42.5},
			expected: "42.5",
		},
		{
			name:     "nil scalar",
			input:    (*model.Scalar)(nil),
			expected: "nil scalar",
		},
		{
			name: "matrix with values",
			input: model.Matrix{
				&model.SampleStream{
					Metric: model.Metric{"job": "test"},
					Values: []model.SamplePair{
						{Timestamp: 1000, Value: 1.0},
						{Timestamp: 2000, Value: 2.0},
						{Timestamp: 3000, Value: 3.0},
					},
				},
			},
			expected: "3",
		},
		{
			name:     "empty matrix",
			input:    model.Matrix{},
			expected: "empty matrix",
		},
		{
			name: "matrix with empty values",
			input: model.Matrix{
				&model.SampleStream{
					Metric: model.Metric{"job": "test"},
					Values: []model.SamplePair{},
				},
			},
			expected: "empty matrix",
		},
		{
			name:     "string",
			input:    &model.String{Value: "test string"},
			expected: "test string",
		},
		{
			name:     "nil string",
			input:    (*model.String)(nil),
			expected: "nil string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractValue(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
