package main

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"time"

	routeclientset "github.com/openshift/client-go/route/clientset/versioned/typed/route/v1"
	promapi "github.com/prometheus/client_golang/api"
	promclientv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apimachineryerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"ship-status-dash/pkg/types"
)

// inClusterConfigName is the special cluster name that indicates the component monitor
// should use in-cluster Kubernetes configuration instead of a kubeconfig file.
const inClusterConfigName = "in-cluster"

// setDefaultStepValues sets default step values for Prometheus queries that have a duration but no step specified.
func setDefaultStepValues(config *types.ComponentMonitorConfig) {
	for i := range config.Components {
		if config.Components[i].PrometheusMonitor == nil {
			continue
		}
		for j := range config.Components[i].PrometheusMonitor.Queries {
			query := &config.Components[i].PrometheusMonitor.Queries[j]
			if query.Duration != "" && query.Step == "" {
				dur, err := time.ParseDuration(query.Duration)
				if err != nil {
					continue
				}
				if dur <= 1*time.Hour {
					query.Step = "15s"
				} else {
					stepDuration := dur / 250
					query.Step = stepDuration.String()
				}
			}
		}
	}
}

// setDefaultSeverityValues sets default severity values for Prometheus queries that have no severity specified.
func setDefaultSeverityValues(config *types.ComponentMonitorConfig) {
	for i := range config.Components {
		if config.Components[i].PrometheusMonitor != nil {
			for j := range config.Components[i].PrometheusMonitor.Queries {
				query := &config.Components[i].PrometheusMonitor.Queries[j]
				if query.Severity == "" {
					query.Severity = types.SeverityDown
				}
			}
		}
	}
}

// validatePrometheusConfiguration validates Prometheus monitor configuration including locations, durations, and steps.
func validatePrometheusConfiguration(components []types.MonitoringComponent, kubeconfigDir string) error {
	var errors []error
	for _, component := range components {
		if component.PrometheusMonitor == nil {
			continue
		}

		location := component.PrometheusMonitor.PrometheusLocation
		hasURL := location.URL != ""
		hasCluster := location.Cluster != ""
		hasNamespace := location.Namespace != ""
		hasRoute := location.Route != ""
		hasService := location.Service != ""

		if !hasURL && !hasCluster {
			errors = append(errors, fmt.Errorf("prometheusLocation must have either url or cluster set for component %s/%s", component.ComponentSlug, component.SubComponentSlug))
			continue
		}

		if hasURL && hasCluster {
			errors = append(errors, fmt.Errorf("prometheusLocation cannot have both url and cluster set for component %s/%s (they are mutually exclusive)", component.ComponentSlug, component.SubComponentSlug))
		}

		if hasURL && (hasNamespace || hasRoute || hasService) {
			errors = append(errors, fmt.Errorf("prometheusLocation cannot have url set together with namespace, route, or service for component %s/%s (url is mutually exclusive with cluster/namespace/route/service)", component.ComponentSlug, component.SubComponentSlug))
		}

		if hasCluster {
			if !hasNamespace {
				errors = append(errors, fmt.Errorf("prometheusLocation namespace is required when cluster is set for component %s/%s", component.ComponentSlug, component.SubComponentSlug))
			}
			if location.Cluster == inClusterConfigName {
				if !hasService {
					errors = append(errors, fmt.Errorf("prometheusLocation service is required when cluster is in-cluster for component %s/%s", component.ComponentSlug, component.SubComponentSlug))
				}
				if hasRoute {
					errors = append(errors, fmt.Errorf("prometheusLocation route must not be set when cluster is in-cluster for component %s/%s", component.ComponentSlug, component.SubComponentSlug))
				}
			} else {
				if !hasRoute {
					errors = append(errors, fmt.Errorf("prometheusLocation route is required when cluster is set for component %s/%s", component.ComponentSlug, component.SubComponentSlug))
				}
				if hasService {
					errors = append(errors, fmt.Errorf("prometheusLocation service must not be set when cluster is not in-cluster for component %s/%s", component.ComponentSlug, component.SubComponentSlug))
				}
				if kubeconfigDir == "" {
					errors = append(errors, fmt.Errorf("kubeconfig-dir is required when using cluster-based prometheusLocation for cluster %s (use %q as cluster name to use in-cluster config)", location.Cluster, inClusterConfigName))
				}
			}
		}

		// Validate URL format if provided
		if hasURL {
			if !isURL(location.URL) {
				errors = append(errors, fmt.Errorf("prometheusLocation url must be a valid URL for component %s/%s, got: %s", component.ComponentSlug, component.SubComponentSlug, location.URL))
			}
		}

		// If kubeconfigDir is provided and cluster is set (and not inClusterConfigName), check if kubeconfig file exists
		if hasCluster && kubeconfigDir != "" && location.Cluster != inClusterConfigName {
			kubeconfigPath := filepath.Join(kubeconfigDir, location.Cluster+".config")
			if _, err := os.Stat(kubeconfigPath); os.IsNotExist(err) {
				errors = append(errors, fmt.Errorf("kubeconfig file not found for cluster %s", location.Cluster))
			}
		}

		for _, query := range component.PrometheusMonitor.Queries {
			if query.Step != "" && query.Duration == "" {
				errors = append(errors, fmt.Errorf("step cannot be set without duration for component %s/%s, query %q", component.ComponentSlug, component.SubComponentSlug, query.Query))
			}
			if query.Duration != "" {
				if _, err := time.ParseDuration(query.Duration); err != nil {
					errors = append(errors, fmt.Errorf("failed to parse duration for component %s/%s, query %q: %w", component.ComponentSlug, component.SubComponentSlug, query.Query, err))
				}
			}
			if query.Step != "" {
				if _, err := time.ParseDuration(query.Step); err != nil {
					errors = append(errors, fmt.Errorf("failed to parse step for component %s/%s, query %q: %w", component.ComponentSlug, component.SubComponentSlug, query.Query, err))
				}
			}
			if query.Severity == "" {
				// This shouldn't ever happen due to setDefaultSeverityValues, but we'll check for it anyway.
				errors = append(errors, fmt.Errorf("severity is required for component %s/%s, query %q", component.ComponentSlug, component.SubComponentSlug, query.Query))
			}
		}
	}

	return apimachineryerrors.NewAggregate(errors)
}

// isURL checks if a string is a valid URL
func isURL(s string) bool {
	u, err := url.Parse(s)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https")
}

// getPrometheusLocationKey returns a unique key for a PrometheusLocation.
// For URL-based locations, returns the URL. For cluster-based locations, returns "cluster/namespace/route" or "cluster/namespace/service" for in-cluster.
func getPrometheusLocationKey(loc types.PrometheusLocation) string {
	if loc.URL != "" {
		return loc.URL
	}
	if loc.Cluster == inClusterConfigName {
		return fmt.Sprintf("%s/%s/%s", loc.Cluster, loc.Namespace, loc.Service)
	}
	return fmt.Sprintf("%s/%s/%s", loc.Cluster, loc.Namespace, loc.Route)
}

func createPrometheusClients(components []types.MonitoringComponent, kubeconfigDir string) (map[string]promclientv1.API, error) {
	clients := make(map[string]promclientv1.API)

	// Collect unique Prometheus locations with their keys
	locationMap := make(map[string]types.PrometheusLocation)
	for _, component := range components {
		if component.PrometheusMonitor == nil {
			continue
		}
		loc := component.PrometheusMonitor.PrometheusLocation
		key := getPrometheusLocationKey(loc)
		locationMap[key] = loc
	}

	if len(locationMap) == 0 {
		return clients, nil
	}

	for key, loc := range locationMap {
		if loc.Cluster != "" {
			var config *rest.Config
			var err error

			inCluster := loc.Cluster == inClusterConfigName
			if inCluster {
				config, err = rest.InClusterConfig()
				if err != nil {
					return nil, fmt.Errorf("failed to build in-cluster config: %w", err)
				}
			} else {
				if kubeconfigDir == "" {
					return nil, fmt.Errorf("kubeconfig-dir is required when using cluster-based prometheusLocation for cluster %s", loc.Cluster)
				}

				kubeconfigPath := filepath.Join(kubeconfigDir, loc.Cluster+".config")
				config, err = clientcmd.BuildConfigFromFlags("", kubeconfigPath)
				if err != nil {
					return nil, fmt.Errorf("failed to build config from kubeconfig for cluster %s: %w", loc.Cluster, err)
				}
			}

			roundTripper, err := rest.TransportFor(config)
			if err != nil {
				return nil, fmt.Errorf("failed to create transport for cluster %s: %w", loc.Cluster, err)
			}

			var prometheusURL string
			if inCluster {
				prometheusURL = buildInClusterPrometheusURL(loc)
			} else {
				prometheusURL, err = discoverPrometheusRoute(config, loc.Namespace, loc.Route)
				if err != nil {
					return nil, fmt.Errorf("failed to discover Prometheus route for cluster %s: %w", loc.Cluster, err)
				}
			}

			client, err := promapi.NewClient(promapi.Config{
				Address:      prometheusURL,
				RoundTripper: roundTripper,
			})
			if err != nil {
				return nil, fmt.Errorf("failed to create prometheus client for cluster %s: %w", loc.Cluster, err)
			}
			prometheusAPI := promclientv1.NewAPI(client)
			clients[key] = prometheusAPI
		} else if loc.URL != "" {
			client, err := promapi.NewClient(promapi.Config{
				Address: loc.URL,
			})
			if err != nil {
				return nil, fmt.Errorf("failed to create prometheus client for %s: %w", loc.URL, err)
			}
			prometheusAPI := promclientv1.NewAPI(client)
			clients[key] = prometheusAPI
		}
	}

	return clients, nil
}

func buildInClusterPrometheusURL(loc types.PrometheusLocation) string {
	return fmt.Sprintf("http://%s.%s.svc.cluster.local:9090", loc.Service, loc.Namespace)
}

func discoverPrometheusRoute(config *rest.Config, namespace, routeName string) (string, error) {
	routeClient, err := routeclientset.NewForConfig(config)
	if err != nil {
		return "", fmt.Errorf("failed to create route client: %w", err)
	}

	route, err := routeClient.Routes(namespace).Get(context.Background(), routeName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to get prometheus route %s/%s: %w", namespace, routeName, err)
	}

	var addr string
	if route.Spec.TLS != nil {
		addr = "https://" + route.Spec.Host
	} else {
		addr = "http://" + route.Spec.Host
	}

	return addr, nil
}
