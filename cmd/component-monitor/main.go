package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	promclientv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"

	"ship-status-dash/pkg/config"
	"ship-status-dash/pkg/types"
)

// Options contains command-line configuration options for the component monitor.
type Options struct {
	ConfigPath   string
	DashboardURL string
	Name         string
	// KubeconfigDir is the path to a directory containing kubeconfig files for different clusters.
	// Each file should be named after the cluster with a ".config" suffix (e.g., "app.ci.config" for the app.ci cluster).
	// When this is set, prometheusLocation in the config must be a cluster name, not a URL.
	KubeconfigDir            string
	ReportAuthTokenFile      string
	DryRun                   bool
	ConfigUpdatePollInterval time.Duration
}

// NewOptions parses command-line flags and returns a new Options instance.
func NewOptions() *Options {
	opts := &Options{}

	flag.StringVar(&opts.ConfigPath, "config-path", "", "Path to component monitor config file")
	flag.StringVar(&opts.DashboardURL, "dashboard-url", "", "Dashboard API base URL")
	flag.StringVar(&opts.Name, "name", "", "Name of the component monitor")
	flag.StringVar(&opts.KubeconfigDir, "kubeconfig-dir", "", "Path to directory containing kubeconfig files for different clusters (each file named after the cluster)")
	flag.StringVar(&opts.ReportAuthTokenFile, "report-auth-token-file", "", "Path to file containing bearer token for authenticating report requests")
	flag.BoolVar(&opts.DryRun, "dry-run", false, "Run probes once and output JSON report instead of sending to dashboard")
	flag.DurationVar(&opts.ConfigUpdatePollInterval, "config-update-poll-interval", config.DefaultPollInterval, "Interval for polling config file for changes")
	flag.Parse()

	return opts
}

// Validate checks that all required options are provided and valid.
func (o *Options) Validate() error {
	if o.ConfigPath == "" {
		return errors.New("config path is required (use --config-path flag)")
	}

	if _, err := os.Stat(o.ConfigPath); os.IsNotExist(err) {
		return errors.New("config file does not exist: " + o.ConfigPath)
	}

	if o.Name == "" {
		return errors.New("name is required (use --name flag)")
	}

	if !o.DryRun {
		if o.ReportAuthTokenFile == "" {
			return errors.New("report auth token file is required (use --report-auth-token-file flag)")
		}

		if _, err := os.Stat(o.ReportAuthTokenFile); os.IsNotExist(err) {
			return errors.New("report auth token file does not exist: " + o.ReportAuthTokenFile)
		}
	}

	return nil
}

func loadAndValidateConfig(log *logrus.Logger, configPath string, kubeconfigDir string) (*types.ComponentMonitorConfig, error) {
	log.Infof("Loading config from %s", configPath)

	configFile, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var cfg types.ComponentMonitorConfig
	if err := yaml.Unmarshal(configFile, &cfg); err != nil {
		return nil, err
	}

	frequency, err := time.ParseDuration(cfg.Frequency)
	if err != nil {
		return nil, fmt.Errorf("failed to parse frequency: %w", err)
	}
	log.Infof("Probing Frequency configured to: %s", frequency)

	for _, component := range cfg.Components {
		if component.SystemdMonitor != nil && strings.TrimSpace(component.SystemdMonitor.Unit) == "" {
			return nil, fmt.Errorf("systemd unit is required for component %s/%s", component.ComponentSlug, component.SubComponentSlug)
		}

		if component.HTTPMonitor != nil {
			retryAfter, err := time.ParseDuration(component.HTTPMonitor.RetryAfter)
			if err != nil {
				return nil, fmt.Errorf("failed to parse retry after duration for component %s/%s: %w", component.ComponentSlug, component.SubComponentSlug, err)
			}
			if retryAfter > frequency {
				return nil, fmt.Errorf("retry after duration is greater than frequency for component %s/%s: %s > %s", component.ComponentSlug, component.SubComponentSlug, component.HTTPMonitor.RetryAfter, frequency)
			}
		}

		if component.JUnitMonitor != nil {
			if strings.TrimSpace(component.JUnitMonitor.JobName) == "" {
				return nil, fmt.Errorf("job_name is required for junit_monitor on component %s/%s", component.ComponentSlug, component.SubComponentSlug)
			}
			maxAge, err := time.ParseDuration(component.JUnitMonitor.MaxAge)
			if err != nil {
				return nil, fmt.Errorf("invalid max_age for junit_monitor on component %s/%s: %w", component.ComponentSlug, component.SubComponentSlug, err)
			}
			if maxAge <= 0 {
				return nil, fmt.Errorf("max_age for junit_monitor on component %s/%s must be a positive duration, got %q", component.ComponentSlug, component.SubComponentSlug, component.JUnitMonitor.MaxAge)
			}
			switch strings.ToLower(strings.TrimSpace(component.JUnitMonitor.ArtifactURLStyle)) {
			case "", types.JUnitArtifactStyleGCS, types.JUnitArtifactStyleGCSWeb:
			default:
				return nil, fmt.Errorf("artifact_url_style for junit_monitor on component %s/%s must be %q or %q, got %q", component.ComponentSlug, component.SubComponentSlug, types.JUnitArtifactStyleGCS, types.JUnitArtifactStyleGCSWeb, component.JUnitMonitor.ArtifactURLStyle)
			}
			hr := component.JUnitMonitor.HistoryRuns
			if hr < 0 {
				return nil, fmt.Errorf("history_runs for junit_monitor on component %s/%s must be non-negative", component.ComponentSlug, component.SubComponentSlug)
			}
			if hr <= 0 {
				hr = 1
			}
			if hr > 1 {
				ft := component.JUnitMonitor.FailedRunsThreshold
				if ft < 1 {
					return nil, fmt.Errorf("failed_runs_threshold is required and must be >=1 when history_runs >1 for junit_monitor on component %s/%s", component.ComponentSlug, component.SubComponentSlug)
				}
				if ft > hr {
					return nil, fmt.Errorf("failed_runs_threshold for junit_monitor on component %s/%s must be <= history_runs (%d), got %d", component.ComponentSlug, component.SubComponentSlug, hr, ft)
				}
			}
		}
	}

	setDefaultStepValues(&cfg)
	setDefaultSeverityValues(&cfg)

	if err := validatePrometheusConfiguration(cfg.Components, kubeconfigDir); err != nil {
		return nil, fmt.Errorf("invalid prometheus location configuration: %w", err)
	}

	log.Infof("Loaded configuration with %d components", len(cfg.Components))
	return &cfg, nil
}

func createProbers(components []types.MonitoringComponent, prometheusClients map[string]promclientv1.API, log *logrus.Logger) []Prober {
	var probers []Prober
	for _, component := range components {
		componentLogger := log.WithFields(logrus.Fields{
			"component":     component.ComponentSlug,
			"sub_component": component.SubComponentSlug,
		})
		componentLogger.Info("Configuring component monitor probe")
		if component.HTTPMonitor != nil {
			retryAfter, err := time.ParseDuration(component.HTTPMonitor.RetryAfter)
			if err != nil {
				componentLogger.WithField("error", err).Fatal("Failed to parse retry after duration")
			}
			prober := NewHTTPProber(component.ComponentSlug, component.SubComponentSlug, component.HTTPMonitor.URL, component.HTTPMonitor.Code, retryAfter, component.HTTPMonitor.Severity)
			componentLogger.Info("Added HTTP prober for component")
			probers = append(probers, prober)
		}
		if component.PrometheusMonitor != nil {
			locationKey := getPrometheusLocationKey(component.PrometheusMonitor.PrometheusLocation)
			prometheusProber := NewPrometheusProber(component.ComponentSlug, component.SubComponentSlug, prometheusClients[locationKey], component.PrometheusMonitor.Queries)
			componentLogger.Info("Added Prometheus prober for component")
			probers = append(probers, prometheusProber)
		}
		if component.SystemdMonitor != nil {
			systemdProber := NewSystemdProber(component.ComponentSlug, component.SubComponentSlug, component.SystemdMonitor.Unit, component.SystemdMonitor.Severity)
			componentLogger.Info("Added systemd prober for component")
			probers = append(probers, systemdProber)
		}
		if component.JUnitMonitor != nil {
			maxAge, err := time.ParseDuration(component.JUnitMonitor.MaxAge)
			if err != nil {
				componentLogger.WithField("error", err).Fatal("Failed to parse max_age duration")
			}
			style := strings.ToLower(strings.TrimSpace(component.JUnitMonitor.ArtifactURLStyle))
			if style == "" {
				style = types.JUnitArtifactStyleGCS
			}
			hr := component.JUnitMonitor.HistoryRuns
			if hr <= 0 {
				hr = 1
			}
			failedRunsThreshold := component.JUnitMonitor.FailedRunsThreshold
			if hr == 1 {
				failedRunsThreshold = 1
			}
			junitSt := JUnitProberSettings{
				ArtifactURLStyle:    style,
				GCSWebBaseURL:       strings.TrimSpace(component.JUnitMonitor.GCSWebBaseURL),
				HistoryRuns:         hr,
				FailedRunsThreshold: failedRunsThreshold,
			}
			junitJobName := strings.TrimSpace(component.JUnitMonitor.JobName)
			junitProber := NewJUnitProber(component.ComponentSlug, component.SubComponentSlug, component.JUnitMonitor.GCSBucket, junitJobName, maxAge, component.JUnitMonitor.Severity, junitSt, &http.Client{Timeout: 30 * time.Second})
			componentLogger.Info("Added JUnit prober for component")
			probers = append(probers, junitProber)
		}
	}
	return probers
}

func startOrchestratorWithConfig(config *types.ComponentMonitorConfig, kubeconfigDir, dashboardURL, componentMonitorName, reportAuthToken string, log *logrus.Logger, parentCtx context.Context) (context.CancelFunc, error) {
	frequency, err := time.ParseDuration(config.Frequency)
	if err != nil {
		return nil, fmt.Errorf("failed to parse frequency: %w", err)
	}

	prometheusClients, err := createPrometheusClients(config.Components, kubeconfigDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create prometheus clients: %w", err)
	}

	probers := createProbers(config.Components, prometheusClients, log)
	if len(probers) == 0 {
		return nil, fmt.Errorf("no probers configured")
	}

	orchestratorCtx, orchestratorCancel := context.WithCancel(parentCtx)
	orchestrator := NewProbeOrchestrator(probers, frequency, dashboardURL, componentMonitorName, reportAuthToken, log)
	go orchestrator.Run(orchestratorCtx)

	return orchestratorCancel, nil
}

func main() {
	log := logrus.New()
	log.SetLevel(logrus.InfoLevel)
	log.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})

	opts := NewOptions()

	if err := opts.Validate(); err != nil {
		log.WithField("error", err).Fatal("Invalid command-line options")
	}

	loadFunc := func(path string) (*types.ComponentMonitorConfig, error) {
		return loadAndValidateConfig(log, path, opts.KubeconfigDir)
	}

	configManager, err := config.NewManager(opts.ConfigPath, loadFunc, log, opts.ConfigUpdatePollInterval)
	if err != nil {
		log.WithField("error", err).Fatal("Failed to create config manager")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Info("Received interrupt signal, shutting down...")
		cancel()
	}()

	if err := configManager.Watch(ctx); err != nil {
		log.WithField("error", err).Fatal("Failed to start config watcher")
	}

	if opts.DryRun {
		log.Info("Running in dry run mode, will not send report to dashboard")
		monitoringConfig := configManager.Get()
		frequency, err := time.ParseDuration(monitoringConfig.Frequency)
		if err != nil {
			log.WithField("error", err).Fatal("Failed to parse frequency")
		}
		prometheusClients, err := createPrometheusClients(monitoringConfig.Components, opts.KubeconfigDir)
		if err != nil {
			log.WithField("error", err).Fatal("Failed to create prometheus clients")
		}
		probers := createProbers(monitoringConfig.Components, prometheusClients, log)
		if len(probers) == 0 {
			log.Warn("No probers configured, exiting")
			return
		}
		orchestrator := NewProbeOrchestrator(probers, frequency, opts.DashboardURL, opts.Name, "", log)
		orchestrator.DryRun(ctx)
		return
	}

	tokenBytes, err := os.ReadFile(opts.ReportAuthTokenFile)
	if err != nil {
		log.WithFields(logrus.Fields{
			"token_file": opts.ReportAuthTokenFile,
			"error":      err,
		}).Fatal("Failed to read report auth token file")
	}
	reportAuthToken := strings.TrimSpace(string(tokenBytes))

	orchestratorCancel, err := startOrchestratorWithConfig(configManager.Get(), opts.KubeconfigDir, opts.DashboardURL, opts.Name, reportAuthToken, log, ctx)
	if err != nil {
		log.WithField("error", err).Fatal("Failed to start orchestrator")
	}
	defer orchestratorCancel()

	// If the monitoring config changes, we need to stop the current orchestrator and start a new one with the new config
	startOrchestrator := func() {
		newCancel, startErr := startOrchestratorWithConfig(configManager.Get(), opts.KubeconfigDir, opts.DashboardURL, opts.Name, reportAuthToken, log, ctx)
		if startErr != nil {
			log.WithField("error", startErr).Error("Failed to start orchestrator with updated monitoringConfig, keeping existing orchestrator")
			return
		}

		log.Info("Stopping current orchestrator and starting new one with updated monitoringConfig")
		orchestratorCancel()
		orchestratorCancel = newCancel
	}

	configManager.OnUpdate(func(newConfig *types.ComponentMonitorConfig) {
		startOrchestrator()
	})

	<-ctx.Done()
}
