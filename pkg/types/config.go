package types

// DashboardConfig contains the dashboardapplication configuration including component definitions.
type DashboardConfig struct {
	Components []*Component `json:"components" yaml:"components"`
	Tags       []Tag        `json:"tags" yaml:"tags"`
}

func (c *DashboardConfig) GetComponentBySlug(slug string) *Component {
	for i := range c.Components {
		if c.Components[i].Slug == slug {
			return c.Components[i]
		}
	}
	return nil
}

// SubComponentRef identifies a sub-component by config slugs for server-side filtering.
type SubComponentRef struct {
	ComponentSlug string
	SubSlug       string
}

// SubComponentRefsMatching returns component and sub-component slugs that satisfy the optional filters.
// Filters use AND semantics consistent with the sub-components list API: componentSlug, tag, and team
// narrow results; when subSlug is non-empty, only that sub-component is included (if it passes other filters).
func (c *DashboardConfig) SubComponentRefsMatching(componentSlug, subSlug, tag, team string) []SubComponentRef {
	var refs []SubComponentRef
	for _, component := range c.Components {
		if componentSlug != "" && component.Slug != componentSlug {
			continue
		}
		if team != "" && team != component.ShipTeam {
			continue
		}
		for i := range component.Subcomponents {
			sub := &component.Subcomponents[i]
			if subSlug != "" && sub.Slug != subSlug {
				continue
			}
			if tag != "" {
				var match bool
				for _, t := range sub.Tags {
					if t == tag {
						match = true
						break
					}
				}
				if !match {
					continue
				}
			}
			refs = append(refs, SubComponentRef{ComponentSlug: component.Slug, SubSlug: sub.Slug})
		}
	}
	return refs
}

// Component represents a top-level system component with sub-components and ownership information.
type Component struct {
	Name           string                 `json:"name" yaml:"name"`
	Slug           string                 `json:"slug"`
	Description    string                 `json:"description" yaml:"description"`
	ShipTeam       string                 `json:"ship_team" yaml:"ship_team"`
	SlackReporting []SlackReportingConfig `json:"slack_reporting,omitempty" yaml:"slack_reporting,omitempty"`
	Subcomponents  []SubComponent         `json:"sub_components" yaml:"sub_components"`
	Owners         []Owner                `json:"owners" yaml:"owners"`
}

func (c *Component) GetSubComponentBySlug(slug string) *SubComponent {
	for i := range c.Subcomponents {
		if c.Subcomponents[i].Slug == slug {
			return &c.Subcomponents[i]
		}
	}
	return nil
}

// SubComponent represents a sub-component that can have outages tracked against it.
type SubComponent struct {
	Name                 string                 `json:"name" yaml:"name"`
	Slug                 string                 `json:"slug"`
	Description          string                 `json:"description" yaml:"description"`
	LongDescription      string                 `json:"long_description,omitempty" yaml:"long_description,omitempty"`
	DocumentationURL     string                 `json:"documentation_url,omitempty" yaml:"documentation_url,omitempty"`
	Tags                 []string               `json:"tags,omitempty" yaml:"tags,omitempty"`
	Monitoring           *Monitoring            `json:"monitoring,omitempty" yaml:"monitoring,omitempty"`
	RequiresConfirmation bool                   `json:"requires_confirmation" yaml:"requires_confirmation"`
	SlackReporting       []SlackReportingConfig `json:"slack_reporting,omitempty" yaml:"slack_reporting,omitempty"`
}

// Monitoring defines how this sub-component is automatically monitored.
type Monitoring struct {
	Frequency string `json:"frequency" yaml:"frequency"`
	// ComponentMonitor is the name of the component monitor that will be used to report the status of this sub-component
	// It must match the component monitor name in the report request
	ComponentMonitor string `json:"component_monitor" yaml:"component_monitor"`
	// AutoResolve is a flag that indicates whether outages discovered by the component-monitor should be automatically resolved when
	// the component-monitor reports the sub-component is healthy.
	AutoResolve bool `json:"auto_resolve" yaml:"auto_resolve"`
}

// Owner represents ownership information for a component, either via Rover group or service account.
type Owner struct {
	RoverGroup string `json:"rover_group,omitempty" yaml:"rover_group,omitempty"`
	// ServiceAccount owners are used for the component-monitor.
	// In order to report the status of a sub-component, the service account must be an owner of the component.
	ServiceAccount string `json:"service_account,omitempty" yaml:"service_account,omitempty"`
	// User is a username of a user who is an admin of the component, this is used for development/testing purposes only
	User string `json:"user,omitempty" yaml:"user,omitempty"`
}

// ComponentMonitorConfig contains the configuration for the component monitor.
type ComponentMonitorConfig struct {
	Components []MonitoringComponent `json:"components" yaml:"components"`
	Frequency  string                `json:"frequency" yaml:"frequency"`
}

// MonitoringComponent contains the configuration for a sub-component monitor in the component monitor.
type MonitoringComponent struct {
	ComponentSlug    string `json:"component_slug" yaml:"component_slug"`
	SubComponentSlug string `json:"sub_component_slug" yaml:"sub_component_slug"`
	// PrometheusMonitors is the configuration for the Prometheus monitor
	PrometheusMonitor *PrometheusMonitor `json:"prometheus_monitor" yaml:"prometheus_monitor"`
	// HTTPMonitor is the configuration for the HTTP monitor
	HTTPMonitor *HTTPMonitor `json:"http_monitor,omitempty" yaml:"http_monitor,omitempty"`
	// SystemdMonitor is the configuration for the systemd unit monitor
	SystemdMonitor *SystemdMonitor `json:"systemd_monitor,omitempty" yaml:"systemd_monitor,omitempty"`
	// JUnitMonitor configures Prow GCS JUnit probing when set (optional).
	JUnitMonitor *JUnitMonitor `json:"junit_monitor,omitempty" yaml:"junit_monitor,omitempty"`
}

type PrometheusMonitor struct {
	PrometheusLocation PrometheusLocation `json:"prometheus_location" yaml:"prometheus_location"`
	// Queries is the list of Prometheus queries to perform
	Queries []PrometheusQuery `json:"queries" yaml:"queries"`
}

// PrometheusLocation specifies how to connect to a Prometheus instance.
// Either url (for e2e/local-dev) or cluster+namespace+route (for production) must be set, but not both.
type PrometheusLocation struct {
	// URL is the direct URL to Prometheus (for e2e and local development).
	// Mutually exclusive with cluster, namespace, service, and route.
	URL string `json:"url,omitempty" yaml:"url,omitempty"`
	// Cluster is the cluster name.
	// When set, namespace must also be set. For in-cluster, service is required; otherwise route is required.
	Cluster string `json:"cluster,omitempty" yaml:"cluster,omitempty"`
	// Namespace is the namespace where the Prometheus route or service exists.
	// Required when cluster is set.
	Namespace string `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	// Route is the name of the OpenShift Route to Prometheus.
	// Required when cluster is set and not in-cluster.
	Route string `json:"route,omitempty" yaml:"route,omitempty"`
	// Service is the Kubernetes service name for Prometheus. Used when cluster is in-cluster to connect via in-cluster DNS.
	// Required when cluster is in-cluster.
	Service string `json:"service,omitempty" yaml:"service,omitempty"`
}

type PrometheusQuery struct {
	// Query is the Prometheus query to perform
	Query string `json:"query" yaml:"query"`
	// Severity is the severity of the outage that will be created if the query returns no results.
	// If not provided, the severity will default to Down.
	Severity Severity `json:"severity,omitempty" yaml:"severity,omitempty"`
	// FailureQuery is the, optional, Prometheus (instant) query that runs when the Query returns no results
	// It can be used to provide more information as to the reason for the resulting Outage
	FailureQuery string `json:"failure_query,omitempty" yaml:"failure_query,omitempty"`
	// Duration is the duration to use in a range query.
	// If provided, the query will be a range query.
	// If not provided, the query will be an instant query.
	Duration string `json:"duration" yaml:"duration"`
	// Step is the resolution (time between data points) for range queries.
	// If not provided, a default step will be calculated based on the duration.
	// If provided, it must be a valid duration string (e.g., "15s", "1m").
	Step string `json:"step" yaml:"step"`
}

type SystemdMonitor struct {
	// Unit is the systemd unit name to monitor (e.g., "my-service.service")
	Unit string `json:"unit" yaml:"unit"`
	// Severity is the severity of the outage that will be created if the unit is not active.
	// If not provided, the severity will default to Down.
	Severity Severity `json:"severity,omitempty" yaml:"severity,omitempty"`
}

const (
	// JUnitArtifactStyleGCS uses the public GCS object URL.
	JUnitArtifactStyleGCS = "gcs"
	// JUnitArtifactStyleGCSWeb uses the GCSweb proxy (see JUnitDefaultGCSWebBase).
	JUnitArtifactStyleGCSWeb = "gcsweb"
)

// JUnitDefaultGCSWebBase is the default GCSweb host for artifact links (aligns with openshift/release ship-status and job report URLs).
const JUnitDefaultGCSWebBase = "https://gcsweb-ci.apps.ci.l2s4.p1.openshiftapps.com"

// JUnitMonitor configures reading JUnit XML that Prow uploads to GCS for a job (under logs/<job_name>/,
// e.g. artifacts/junit_canary.xml via latest-build.txt and started.json for staleness—see component-monitor docs).
//
// With history_runs 1 (default), only that latest build is evaluated for pass/fail.
// With history_runs N > 1 and failed_runs_threshold Y, the monitor evaluates up to N recent builds and
// reports unhealthy only when at least Y of those runs share the same failure pattern (the same set of
// failed testcase names, or runs all bucketed as zero JUnit tests)—not merely “any Y failing runs out of N,”
// which avoids noisy alerts from unrelated flake patterns.
type JUnitMonitor struct {
	// GCSBucket is the Prow GCS bucket name. If not provided, test-platform-results is used.
	GCSBucket string `json:"gcs_bucket,omitempty" yaml:"gcs_bucket,omitempty"`
	// JobName is the Prow job name (under logs/ in the bucket).
	JobName string `json:"job_name" yaml:"job_name"`
	// MaxAge is the maximum age of the build referenced by latest-build.txt before the probe reports unhealthy. Must be a valid Go duration.
	MaxAge string `json:"max_age" yaml:"max_age"`
	// Severity is the severity to report on failure or a stale build. If not provided, the severity will default to Degraded.
	Severity Severity `json:"severity,omitempty" yaml:"severity,omitempty"`
	// ArtifactURLStyle selects gcs (direct GCS URL) or gcsweb (GCSweb proxy URL). If not provided, gcs is used.
	ArtifactURLStyle string `json:"artifact_url_style,omitempty" yaml:"artifact_url_style,omitempty"`
	// GCSWebBaseURL is the GCSweb origin to use when ArtifactURLStyle is gcsweb. If not provided, JUnitDefaultGCSWebBase is used.
	GCSWebBaseURL string `json:"gcsweb_base_url,omitempty" yaml:"gcsweb_base_url,omitempty"`
	// HistoryRuns is the number of recent Prow build IDs to evaluate. If 0, 1 is used. When greater than 1, FailedRunsThreshold and GCS list responses apply; staleness (MaxAge) still uses only the latest build from latest-build.txt.
	HistoryRuns int `json:"history_runs,omitempty" yaml:"history_runs,omitempty"`
	// FailedRunsThreshold (Y) is used when HistoryRuns (N) is greater than 1. Unhealthy if the *largest* group
	// of runs that share the same failure pattern has size at least Y. A pattern is the sorted set of failed
	// testcase names, or a shared bucket for zero total JUnit tests. It is not "Y arbitrary red runs in N."
	// Y must be between 1 and N (inclusive). When HistoryRuns is 1, the field is ignored.
	FailedRunsThreshold int `json:"failed_runs_threshold,omitempty" yaml:"failed_runs_threshold,omitempty"`
}

type HTTPMonitor struct {
	// URL is the URL to probe
	URL string `json:"url" yaml:"url"`
	// Code is the expected HTTP status code
	Code int `json:"code" yaml:"code"`
	// RetryAfter is the duration to wait before retrying the probe only when the status code is not as expected
	RetryAfter string `json:"retry_after" yaml:"retry_after"`
	// Severity is the severity of the outage that will be created if the HTTP request fails.
	// If not provided, the severity will default to Down.
	Severity Severity `json:"severity,omitempty" yaml:"severity,omitempty"`
}

// SlackReportingConfig defines Slack reporting configuration for a channel with optional severity threshold.
type SlackReportingConfig struct {
	Channel  string    `json:"channel" yaml:"channel"`
	Severity *Severity `json:"severity,omitempty" yaml:"severity,omitempty"`
}

// GetSlackReporting returns the Slack reporting configuration for a sub-component.
// If the sub-component has its own SlackReporting config, it is returned.
// Otherwise, the component's SlackReporting config is returned.
func GetSlackReporting(component *Component, subComponent *SubComponent) []SlackReportingConfig {
	if subComponent != nil && len(subComponent.SlackReporting) > 0 {
		return subComponent.SlackReporting
	}
	if component != nil && len(component.SlackReporting) > 0 {
		return component.SlackReporting
	}
	return nil
}

type Tag struct {
	Name        string `json:"name" yaml:"name"`
	Description string `json:"description" yaml:"description"`
	Color       string `json:"color" yaml:"color"`
}
