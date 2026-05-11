package config

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
)

// ConfigReloadedMessage is the log message emitted after a config reload and all OnUpdate callbacks have run.
const ConfigReloadedMessage = "Config reloaded successfully"

// DefaultPollInterval is the default interval at which the config file is checked for changes.
const DefaultPollInterval = 5 * time.Minute

// Manager provides thread-safe configuration management with hot-reload support.
type Manager[T any] struct {
	mu              sync.RWMutex
	config          *T
	configPath      string
	loadFunc        func(string) (*T, error)
	logger          *logrus.Logger
	updateCallbacks []func(*T)
	lastHash        string
	pollInterval    time.Duration
	// reloadCount is the number of times the config has been reloaded.
	// It is helpful in e2e testing when we are waiting/verifying reloads.
	reloadCount atomic.Uint64
}

// NewManager creates a new config manager with the specified load function and poll interval.
func NewManager[T any](configPath string, loadFunc func(string) (*T, error), logger *logrus.Logger, pollInterval time.Duration) (*Manager[T], error) {
	manager := &Manager[T]{
		configPath:      configPath,
		loadFunc:        loadFunc,
		logger:          logger,
		updateCallbacks: make([]func(*T), 0),
		pollInterval:    pollInterval,
	}

	if err := manager.load(); err != nil {
		return nil, err
	}

	// Initialize hash from initial config load
	configBytes, err := os.ReadFile(configPath)
	if err == nil {
		hash := sha256.Sum256(configBytes)
		manager.lastHash = hex.EncodeToString(hash[:])
	}

	return manager, nil
}

// Get returns the current configuration in a thread-safe manner.
func (m *Manager[T]) Get() *T {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// load loads and validates the configuration from the file.
func (m *Manager[T]) load() error {
	config, err := m.loadFunc(m.configPath)
	if err != nil {
		return err
	}

	m.mu.Lock()
	m.config = config
	m.mu.Unlock()

	return nil
}

// reloadIfChanged attempts to reload the configuration if it has changed and update callbacks if successful.
func (m *Manager[T]) reloadIfChanged() {
	m.mu.Lock()
	// Read the file content to compute hash
	configBytes, err := os.ReadFile(m.configPath)
	if err != nil {
		m.mu.Unlock()
		m.logger.WithFields(logrus.Fields{
			"config_path": m.configPath,
			"error":       err,
		}).Error("Failed to read config file for hash validation")
		return
	}

	// Compute hash of current content
	newHash := sha256.Sum256(configBytes)
	newHashStr := hex.EncodeToString(newHash[:])

	// Skip reload if content hasn't changed
	if newHashStr == m.lastHash {
		m.mu.Unlock()
		m.logger.WithField("config_path", m.configPath).Info("Config file content unchanged, skipping reload")
		return
	}

	// Load and validate the new configuration
	newConfig, err := m.loadFunc(m.configPath)
	if err != nil {
		m.mu.Unlock()
		m.logger.WithFields(logrus.Fields{
			"config_path": m.configPath,
			"error":       err,
		}).Error("Failed to reload config, keeping existing config")
		return
	}

	m.config = newConfig
	m.lastHash = newHashStr
	// Copy callbacks to avoid race conditions
	callbacks := make([]func(*T), len(m.updateCallbacks))
	copy(callbacks, m.updateCallbacks)

	// Release lock before calling callbacks to avoid holding lock during potentially slow operations
	m.mu.Unlock()
	for _, callback := range callbacks {
		callback(newConfig)
	}

	n := m.reloadCount.Add(1)
	m.logger.WithFields(logrus.Fields{
		"config_path":  m.configPath,
		"reload_count": n,
	}).Info(ConfigReloadedMessage)
}

// OnUpdate registers a callback function that will be called when the configuration is updated.
func (m *Manager[T]) OnUpdate(callback func(*T)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.updateCallbacks = append(m.updateCallbacks, callback)
}

// Watch starts polling the configuration file for changes and reloads when content changes are detected.
func (m *Manager[T]) Watch(ctx context.Context) error {
	go func() {
		ticker := time.NewTicker(m.pollInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				m.reloadIfChanged()
			}
		}
	}()

	return nil
}
