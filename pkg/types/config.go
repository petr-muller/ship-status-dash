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
