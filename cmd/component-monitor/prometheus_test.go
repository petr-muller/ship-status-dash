package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"

	"ship-status-dash/pkg/testhelper"
	"ship-status-dash/pkg/types"
)

func TestIsURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "valid http URL",
			input:    "http://localhost:9090",
			expected: true,
		},
		{
			name:     "valid https URL",
			input:    "https://prometheus.example.com",
			expected: true,
		},
		{
			name:     "valid https URL with path",
			input:    "https://prometheus.example.com/api/v1",
			expected: true,
		},
		{
			name:     "invalid - no scheme",
			input:    "localhost:9090",
			expected: false,
		},
		{
			name:     "invalid - not http/https",
			input:    "ftp://example.com",
			expected: false,
		},
		{
			name:     "invalid - empty string",
			input:    "",
			expected: false,
		},
		{
			name:     "invalid - cluster name",
			input:    "app.ci",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isURL(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestValidatePrometheusLocations(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a test kubeconfig file
	kubeconfigPath := filepath.Join(tmpDir, "app.ci.config")
	err := os.WriteFile(kubeconfigPath, []byte("test kubeconfig content"), 0644)
	assert.NoError(t, err)

	tests := []struct {
		name          string
		components    []types.MonitoringComponent
		kubeconfigDir string
		expectedErr   error
	}{
		{
			name: "valid - URL when kubeconfigDir not set",
			components: []types.MonitoringComponent{
				{
					ComponentSlug:    "test",
					SubComponentSlug: "test",
					PrometheusMonitor: &types.PrometheusMonitor{
						PrometheusLocation: types.PrometheusLocation{
							URL: "http://localhost:9090",
						},
						Queries: []types.PrometheusQuery{{Query: "up", Severity: types.SeverityDown}},
					},
				},
			},
		},
		{
			name: "valid - cluster name when kubeconfigDir set",
			components: []types.MonitoringComponent{
				{
					ComponentSlug:    "test",
					SubComponentSlug: "test",
					PrometheusMonitor: &types.PrometheusMonitor{
						PrometheusLocation: types.PrometheusLocation{
							Cluster:   "app.ci",
							Namespace: "openshift-monitoring",
							Route:     "thanos-querier",
						},
						Queries: []types.PrometheusQuery{{Query: "up", Severity: types.SeverityDown}},
					},
				},
			},
			kubeconfigDir: tmpDir,
		},
		{
			name: "invalid - URL when kubeconfigDir set",
			components: []types.MonitoringComponent{
				{
					ComponentSlug:    "test",
					SubComponentSlug: "test",
					PrometheusMonitor: &types.PrometheusMonitor{
						PrometheusLocation: types.PrometheusLocation{
							URL: "http://localhost:9090",
						},
						Queries: []types.PrometheusQuery{{Query: "up", Severity: types.SeverityDown}},
					},
				},
			},
			kubeconfigDir: tmpDir,
		},
		{
			name: "invalid - cluster name when kubeconfigDir not set",
			components: []types.MonitoringComponent{
				{
					ComponentSlug:    "test",
					SubComponentSlug: "test",
					PrometheusMonitor: &types.PrometheusMonitor{
						PrometheusLocation: types.PrometheusLocation{
							Cluster:   "app.ci",
							Namespace: "openshift-monitoring",
							Route:     "thanos-querier",
						},
						Queries: []types.PrometheusQuery{{Query: "up", Severity: types.SeverityDown}},
					},
				},
			},
			expectedErr: errors.New(`kubeconfig-dir is required when using cluster-based prometheusLocation for cluster app.ci (use "in-cluster" as cluster name to use in-cluster config)`),
		},
		{
			name: "valid - in-cluster config when kubeconfigDir not set",
			components: []types.MonitoringComponent{
				{
					ComponentSlug:    "test",
					SubComponentSlug: "test",
					PrometheusMonitor: &types.PrometheusMonitor{
						PrometheusLocation: types.PrometheusLocation{
							Cluster:   "in-cluster",
							Namespace: "openshift-monitoring",
							Service:   "prometheus-operated",
						},
						Queries: []types.PrometheusQuery{{Query: "up", Severity: types.SeverityDown}},
					},
				},
			},
		},
		{
			name: "valid - in-cluster config when kubeconfigDir is set (kubeconfig file check skipped)",
			components: []types.MonitoringComponent{
				{
					ComponentSlug:    "test",
					SubComponentSlug: "test",
					PrometheusMonitor: &types.PrometheusMonitor{
						PrometheusLocation: types.PrometheusLocation{
							Cluster:   "in-cluster",
							Namespace: "openshift-monitoring",
							Service:   "prometheus-operated",
						},
						Queries: []types.PrometheusQuery{{Query: "up", Severity: types.SeverityDown}},
					},
				},
			},
			kubeconfigDir: tmpDir,
		},
		{
			name: "invalid - in-cluster config without namespace",
			components: []types.MonitoringComponent{
				{
					ComponentSlug:    "test",
					SubComponentSlug: "test",
					PrometheusMonitor: &types.PrometheusMonitor{
						PrometheusLocation: types.PrometheusLocation{
							Cluster: "in-cluster",
							Service: "prometheus-operated",
						},
						Queries: []types.PrometheusQuery{{Query: "up", Severity: types.SeverityDown}},
					},
				},
			},
			expectedErr: errors.New("prometheusLocation namespace is required when cluster is set for component test/test"),
		},
		{
			name: "invalid - in-cluster config without service",
			components: []types.MonitoringComponent{
				{
					ComponentSlug:    "test",
					SubComponentSlug: "test",
					PrometheusMonitor: &types.PrometheusMonitor{
						PrometheusLocation: types.PrometheusLocation{
							Cluster:   "in-cluster",
							Namespace: "openshift-monitoring",
						},
						Queries: []types.PrometheusQuery{{Query: "up", Severity: types.SeverityDown}},
					},
				},
			},
			expectedErr: errors.New("prometheusLocation service is required when cluster is in-cluster for component test/test"),
		},
		{
			name: "invalid - in-cluster with route but no service",
			components: []types.MonitoringComponent{
				{
					ComponentSlug:    "test",
					SubComponentSlug: "test",
					PrometheusMonitor: &types.PrometheusMonitor{
						PrometheusLocation: types.PrometheusLocation{
							Cluster:   "in-cluster",
							Namespace: "openshift-monitoring",
							Route:     "thanos-querier",
						},
						Queries: []types.PrometheusQuery{{Query: "up", Severity: types.SeverityDown}},
					},
				},
			},
			expectedErr: errors.New("[prometheusLocation service is required when cluster is in-cluster for component test/test, prometheusLocation route must not be set when cluster is in-cluster for component test/test]"),
		},
		{
			name: "invalid - non-in-cluster with service but no route",
			components: []types.MonitoringComponent{
				{
					ComponentSlug:    "test",
					SubComponentSlug: "test",
					PrometheusMonitor: &types.PrometheusMonitor{
						PrometheusLocation: types.PrometheusLocation{
							Cluster:   "app.ci",
							Namespace: "openshift-monitoring",
							Service:   "prometheus-operated",
						},
						Queries: []types.PrometheusQuery{{Query: "up", Severity: types.SeverityDown}},
					},
				},
			},
			kubeconfigDir: tmpDir,
			expectedErr:   errors.New("[prometheusLocation route is required when cluster is set for component test/test, prometheusLocation service must not be set when cluster is not in-cluster for component test/test]"),
		},
		{
			name: "invalid - in-cluster with both route and service",
			components: []types.MonitoringComponent{
				{
					ComponentSlug:    "test",
					SubComponentSlug: "test",
					PrometheusMonitor: &types.PrometheusMonitor{
						PrometheusLocation: types.PrometheusLocation{
							Cluster:   "in-cluster",
							Namespace: "openshift-monitoring",
							Route:     "thanos-querier",
							Service:   "prometheus-operated",
						},
						Queries: []types.PrometheusQuery{{Query: "up", Severity: types.SeverityDown}},
					},
				},
			},
			expectedErr: errors.New("prometheusLocation route must not be set when cluster is in-cluster for component test/test"),
		},
		{
			name: "invalid - non-in-cluster with both route and service",
			components: []types.MonitoringComponent{
				{
					ComponentSlug:    "test",
					SubComponentSlug: "test",
					PrometheusMonitor: &types.PrometheusMonitor{
						PrometheusLocation: types.PrometheusLocation{
							Cluster:   "app.ci",
							Namespace: "openshift-monitoring",
							Route:     "thanos-querier",
							Service:   "prometheus-operated",
						},
						Queries: []types.PrometheusQuery{{Query: "up", Severity: types.SeverityDown}},
					},
				},
			},
			kubeconfigDir: tmpDir,
			expectedErr:   errors.New("prometheusLocation service must not be set when cluster is not in-cluster for component test/test"),
		},
		{
			name: "invalid - empty prometheusLocation",
			components: []types.MonitoringComponent{
				{
					ComponentSlug:    "test",
					SubComponentSlug: "test",
					PrometheusMonitor: &types.PrometheusMonitor{
						PrometheusLocation: types.PrometheusLocation{},
						Queries:            []types.PrometheusQuery{{Query: "up", Severity: types.SeverityDown}},
					},
				},
			},
			kubeconfigDir: tmpDir,
			expectedErr:   errors.New("prometheusLocation must have either url or cluster set for component test/test"),
		},
		{
			name: "invalid - kubeconfig file not found",
			components: []types.MonitoringComponent{
				{
					ComponentSlug:    "test",
					SubComponentSlug: "test",
					PrometheusMonitor: &types.PrometheusMonitor{
						PrometheusLocation: types.PrometheusLocation{
							Cluster:   "nonexistent",
							Namespace: "openshift-monitoring",
							Route:     "thanos-querier",
						},
						Queries: []types.PrometheusQuery{{Query: "up", Severity: types.SeverityDown}},
					},
				},
			},
			kubeconfigDir: tmpDir,
			expectedErr:   errors.New("kubeconfig file not found for cluster nonexistent"),
		},
		{
			name: "valid - component without PrometheusMonitor",
			components: []types.MonitoringComponent{
				{
					ComponentSlug:    "test",
					SubComponentSlug: "test",
					HTTPMonitor: &types.HTTPMonitor{
						URL: "http://example.com",
					},
				},
			},
			kubeconfigDir: tmpDir,
		},
		{
			name: "valid - multiple components with mixed monitors",
			components: []types.MonitoringComponent{
				{
					ComponentSlug:    "test1",
					SubComponentSlug: "test1",
					HTTPMonitor: &types.HTTPMonitor{
						URL: "http://example.com",
					},
				},
				{
					ComponentSlug:    "test2",
					SubComponentSlug: "test2",
					PrometheusMonitor: &types.PrometheusMonitor{
						PrometheusLocation: types.PrometheusLocation{
							Cluster:   "app.ci",
							Namespace: "openshift-monitoring",
							Route:     "thanos-querier",
						},
						Queries: []types.PrometheusQuery{{Query: "up", Severity: types.SeverityDown}},
					},
				},
			},
			kubeconfigDir: tmpDir,
		},
		{
			name: "invalid - unparseable duration",
			components: []types.MonitoringComponent{
				{
					ComponentSlug:    "test",
					SubComponentSlug: "test",
					PrometheusMonitor: &types.PrometheusMonitor{
						PrometheusLocation: types.PrometheusLocation{
							URL: "http://localhost:9090",
						},
						Queries: []types.PrometheusQuery{
							{
								Query:    "up",
								Duration: "invalid-duration",
								Severity: types.SeverityDown,
							},
						},
					},
				},
			},
			expectedErr: errors.New(`failed to parse duration for component test/test, query "up": time: invalid duration "invalid-duration"`),
		},
		{
			name: "invalid - unparseable step",
			components: []types.MonitoringComponent{
				{
					ComponentSlug:    "test",
					SubComponentSlug: "test",
					PrometheusMonitor: &types.PrometheusMonitor{
						PrometheusLocation: types.PrometheusLocation{
							URL: "http://localhost:9090",
						},
						Queries: []types.PrometheusQuery{
							{
								Query:    "up",
								Duration: "1h",
								Step:     "invalid-step",
								Severity: types.SeverityDown,
							},
						},
					},
				},
			},
			expectedErr: errors.New(`failed to parse step for component test/test, query "up": time: invalid duration "invalid-step"`),
		},
		{
			name: "invalid - step set without duration",
			components: []types.MonitoringComponent{
				{
					ComponentSlug:    "test",
					SubComponentSlug: "test",
					PrometheusMonitor: &types.PrometheusMonitor{
						PrometheusLocation: types.PrometheusLocation{
							URL: "http://localhost:9090",
						},
						Queries: []types.PrometheusQuery{
							{
								Query:    "up",
								Step:     "15s",
								Severity: types.SeverityDown,
							},
						},
					},
				},
			},
			expectedErr: errors.New(`step cannot be set without duration for component test/test, query "up"`),
		},
		{
			name: "invalid - url and cluster both set",
			components: []types.MonitoringComponent{
				{
					ComponentSlug:    "test",
					SubComponentSlug: "test",
					PrometheusMonitor: &types.PrometheusMonitor{
						PrometheusLocation: types.PrometheusLocation{
							URL:     "http://localhost:9090",
							Cluster: "app.ci",
						},
						Queries: []types.PrometheusQuery{{Query: "up", Severity: types.SeverityDown}},
					},
				},
			},
			expectedErr: errors.New("[prometheusLocation cannot have both url and cluster set for component test/test (they are mutually exclusive), prometheusLocation namespace is required when cluster is set for component test/test, prometheusLocation route is required when cluster is set for component test/test, kubeconfig-dir is required when using cluster-based prometheusLocation for cluster app.ci (use \"in-cluster\" as cluster name to use in-cluster config)]"),
		},
		{
			name: "invalid - url and namespace both set",
			components: []types.MonitoringComponent{
				{
					ComponentSlug:    "test",
					SubComponentSlug: "test",
					PrometheusMonitor: &types.PrometheusMonitor{
						PrometheusLocation: types.PrometheusLocation{
							URL:       "http://localhost:9090",
							Namespace: "openshift-monitoring",
						},
						Queries: []types.PrometheusQuery{{Query: "up", Severity: types.SeverityDown}},
					},
				},
			},
			expectedErr: errors.New("prometheusLocation cannot have url set together with namespace, route, or service for component test/test (url is mutually exclusive with cluster/namespace/route/service)"),
		},
		{
			name: "invalid - cluster set without namespace",
			components: []types.MonitoringComponent{
				{
					ComponentSlug:    "test",
					SubComponentSlug: "test",
					PrometheusMonitor: &types.PrometheusMonitor{
						PrometheusLocation: types.PrometheusLocation{
							Cluster: "app.ci",
							Route:   "thanos-querier",
						},
						Queries: []types.PrometheusQuery{{Query: "up", Severity: types.SeverityDown}},
					},
				},
			},
			kubeconfigDir: tmpDir,
			expectedErr:   errors.New("prometheusLocation namespace is required when cluster is set for component test/test"),
		},
		{
			name: "invalid - cluster set without route",
			components: []types.MonitoringComponent{
				{
					ComponentSlug:    "test",
					SubComponentSlug: "test",
					PrometheusMonitor: &types.PrometheusMonitor{
						PrometheusLocation: types.PrometheusLocation{
							Cluster:   "app.ci",
							Namespace: "openshift-monitoring",
						},
						Queries: []types.PrometheusQuery{{Query: "up", Severity: types.SeverityDown}},
					},
				},
			},
			kubeconfigDir: tmpDir,
			expectedErr:   errors.New("prometheusLocation route is required when cluster is set for component test/test"),
		},
		{
			name: "invalid - invalid URL format",
			components: []types.MonitoringComponent{
				{
					ComponentSlug:    "test",
					SubComponentSlug: "test",
					PrometheusMonitor: &types.PrometheusMonitor{
						PrometheusLocation: types.PrometheusLocation{
							URL: "not-a-url",
						},
						Queries: []types.PrometheusQuery{{Query: "up", Severity: types.SeverityDown}},
					},
				},
			},
			expectedErr: errors.New("prometheusLocation url must be a valid URL for component test/test, got: not-a-url"),
		},
		{
			name: "invalid - missing severity",
			components: []types.MonitoringComponent{
				{
					ComponentSlug:    "test",
					SubComponentSlug: "test",
					PrometheusMonitor: &types.PrometheusMonitor{
						PrometheusLocation: types.PrometheusLocation{
							URL: "http://localhost:9090",
						},
						Queries: []types.PrometheusQuery{{Query: "up"}},
					},
				},
			},
			expectedErr: errors.New(`severity is required for component test/test, query "up"`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePrometheusConfiguration(tt.components, tt.kubeconfigDir)
			diff := cmp.Diff(tt.expectedErr, err, testhelper.EquateErrorMessage)
			if diff != "" {
				t.Errorf("validatePrometheusConfiguration() error mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestSetDefaultStepValues(t *testing.T) {
	tests := []struct {
		name     string
		config   *types.ComponentMonitorConfig
		expected *types.ComponentMonitorConfig
	}{
		{
			name: "sets default step for short duration",
			config: &types.ComponentMonitorConfig{
				Components: []types.MonitoringComponent{
					{
						PrometheusMonitor: &types.PrometheusMonitor{
							Queries: []types.PrometheusQuery{
								{
									Query:    "up",
									Duration: "30m",
								},
							},
						},
					},
				},
			},
			expected: &types.ComponentMonitorConfig{
				Components: []types.MonitoringComponent{
					{
						PrometheusMonitor: &types.PrometheusMonitor{
							Queries: []types.PrometheusQuery{
								{
									Query:    "up",
									Duration: "30m",
									Step:     "15s",
								},
							},
						},
					},
				},
			},
		},
		{
			name: "sets default step for long duration",
			config: &types.ComponentMonitorConfig{
				Components: []types.MonitoringComponent{
					{
						PrometheusMonitor: &types.PrometheusMonitor{
							Queries: []types.PrometheusQuery{
								{
									Query:    "up",
									Duration: "24h",
								},
							},
						},
					},
				},
			},
			expected: &types.ComponentMonitorConfig{
				Components: []types.MonitoringComponent{
					{
						PrometheusMonitor: &types.PrometheusMonitor{
							Queries: []types.PrometheusQuery{
								{
									Query:    "up",
									Duration: "24h",
									Step:     "5m45.6s",
								},
							},
						},
					},
				},
			},
		},
		{
			name: "does not modify query with existing step",
			config: &types.ComponentMonitorConfig{
				Components: []types.MonitoringComponent{
					{
						PrometheusMonitor: &types.PrometheusMonitor{
							Queries: []types.PrometheusQuery{
								{
									Query:    "up",
									Duration: "1h",
									Step:     "30s",
								},
							},
						},
					},
				},
			},
			expected: &types.ComponentMonitorConfig{
				Components: []types.MonitoringComponent{
					{
						PrometheusMonitor: &types.PrometheusMonitor{
							Queries: []types.PrometheusQuery{
								{
									Query:    "up",
									Duration: "1h",
									Step:     "30s",
								},
							},
						},
					},
				},
			},
		},
		{
			name: "does not modify query without duration",
			config: &types.ComponentMonitorConfig{
				Components: []types.MonitoringComponent{
					{
						PrometheusMonitor: &types.PrometheusMonitor{
							Queries: []types.PrometheusQuery{
								{
									Query: "up",
								},
							},
						},
					},
				},
			},
			expected: &types.ComponentMonitorConfig{
				Components: []types.MonitoringComponent{
					{
						PrometheusMonitor: &types.PrometheusMonitor{
							Queries: []types.PrometheusQuery{
								{
									Query: "up",
								},
							},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setDefaultStepValues(tt.config)
			diff := cmp.Diff(tt.expected, tt.config)
			if diff != "" {
				t.Errorf("setDefaultStepValues() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestSetDefaultSeverityValues(t *testing.T) {
	tests := []struct {
		name     string
		config   *types.ComponentMonitorConfig
		expected *types.ComponentMonitorConfig
	}{
		{
			name: "sets default severity when not specified",
			config: &types.ComponentMonitorConfig{
				Components: []types.MonitoringComponent{
					{
						PrometheusMonitor: &types.PrometheusMonitor{
							Queries: []types.PrometheusQuery{
								{
									Query: "up",
								},
							},
						},
					},
				},
			},
			expected: &types.ComponentMonitorConfig{
				Components: []types.MonitoringComponent{
					{
						PrometheusMonitor: &types.PrometheusMonitor{
							Queries: []types.PrometheusQuery{
								{
									Query:    "up",
									Severity: types.SeverityDown,
								},
							},
						},
					},
				},
			},
		},
		{
			name: "does not modify query with existing severity",
			config: &types.ComponentMonitorConfig{
				Components: []types.MonitoringComponent{
					{
						PrometheusMonitor: &types.PrometheusMonitor{
							Queries: []types.PrometheusQuery{
								{
									Query:    "up",
									Severity: types.SeverityDegraded,
								},
							},
						},
					},
				},
			},
			expected: &types.ComponentMonitorConfig{
				Components: []types.MonitoringComponent{
					{
						PrometheusMonitor: &types.PrometheusMonitor{
							Queries: []types.PrometheusQuery{
								{
									Query:    "up",
									Severity: types.SeverityDegraded,
								},
							},
						},
					},
				},
			},
		},
		{
			name: "sets default severity for multiple queries",
			config: &types.ComponentMonitorConfig{
				Components: []types.MonitoringComponent{
					{
						PrometheusMonitor: &types.PrometheusMonitor{
							Queries: []types.PrometheusQuery{
								{
									Query: "up",
								},
								{
									Query:    "down",
									Severity: types.SeverityDegraded,
								},
								{
									Query: "other",
								},
							},
						},
					},
				},
			},
			expected: &types.ComponentMonitorConfig{
				Components: []types.MonitoringComponent{
					{
						PrometheusMonitor: &types.PrometheusMonitor{
							Queries: []types.PrometheusQuery{
								{
									Query:    "up",
									Severity: types.SeverityDown,
								},
								{
									Query:    "down",
									Severity: types.SeverityDegraded,
								},
								{
									Query:    "other",
									Severity: types.SeverityDown,
								},
							},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setDefaultSeverityValues(tt.config)
			diff := cmp.Diff(tt.expected, tt.config)
			if diff != "" {
				t.Errorf("setDefaultSeverityValues() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
