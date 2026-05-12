package config

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"ship-status-dash/pkg/testhelper"

	"github.com/google/go-cmp/cmp"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

type testConfig struct {
	Value string
}

func createTestConfigFile(t *testing.T, dir, content string) string {
	configPath := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(configPath, []byte(content), 0644)
	require.NoError(t, err)
	return configPath
}

func writeConfigFile(t *testing.T, path, content string) {
	err := os.WriteFile(path, []byte(content), 0644)
	require.NoError(t, err)
}

func createTestManager(t *testing.T, configPath string) *Manager[testConfig] {
	loadFunc := func(path string) (*testConfig, error) {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		content := string(data)
		value := "default"
		if len(content) > 7 && content[:7] == "value: " {
			end := len(content)
			if idx := len(content) - 1; idx >= 7 {
				value = content[7:end]
				if len(value) > 0 && value[len(value)-1] == '\n' {
					value = value[:len(value)-1]
				}
			}
		}
		return &testConfig{Value: value}, nil
	}
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	// Use a shorter poll interval for tests to make them run faster
	manager, err := NewManager(configPath, loadFunc, logger, 100*time.Millisecond)
	require.NoError(t, err)
	return manager
}

func TestNewManager(t *testing.T) {
	tests := []struct {
		name          string
		configContent string
		loadFunc      func(string) (*testConfig, error)
		wantErr       bool
		wantConfig    *testConfig
		checkHash     bool
	}{
		{
			name:          "successfully creates manager with valid config file",
			configContent: "value: test",
			loadFunc: func(path string) (*testConfig, error) {
				return &testConfig{Value: "test"}, nil
			},
			wantConfig: &testConfig{Value: "test"},
		},
		{
			name:          "initializes hash correctly",
			configContent: "value: test",
			loadFunc: func(path string) (*testConfig, error) {
				return &testConfig{Value: "test"}, nil
			},
			checkHash: true,
		},
		{
			name:          "returns error when loadFunc fails",
			configContent: "value: test",
			loadFunc: func(path string) (*testConfig, error) {
				return nil, os.ErrNotExist
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			configPath := createTestConfigFile(t, tmpDir, tt.configContent)

			logger := logrus.New()
			logger.SetLevel(logrus.ErrorLevel)

			manager, err := NewManager(configPath, tt.loadFunc, logger, DefaultPollInterval)

			if tt.wantErr {
				wantErr := os.ErrNotExist
				if diff := cmp.Diff(wantErr, err, testhelper.EquateErrorMessage); diff != "" {
					t.Errorf("Error mismatch (-want +got):\n%s", diff)
				}
				if diff := cmp.Diff((*Manager[testConfig])(nil), manager); diff != "" {
					t.Errorf("Manager mismatch (-want +got):\n%s", diff)
				}
			} else {
				var wantErr error = nil
				if diff := cmp.Diff(wantErr, err, testhelper.EquateErrorMessage); diff != "" {
					t.Errorf("Error mismatch (-want +got):\n%s", diff)
				}
				if manager == nil {
					t.Error("Expected non-nil manager but got nil")
				}

				if tt.wantConfig != nil {
					if diff := cmp.Diff(tt.wantConfig, manager.Get()); diff != "" {
						t.Errorf("Config mismatch (-want +got):\n%s", diff)
					}
				}

				if tt.checkHash {
					manager.mu.RLock()
					hash := manager.lastHash
					manager.mu.RUnlock()
					if diff := cmp.Diff("", hash); diff == "" {
						t.Error("Expected non-empty hash but got empty")
					}
				}
			}
		})
	}
}

func TestManager_OnUpdate(t *testing.T) {
	tests := []struct {
		name         string
		numCallbacks int
	}{
		{
			name:         "registers callbacks correctly",
			numCallbacks: 1,
		},
		{
			name:         "multiple callbacks can be registered",
			numCallbacks: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			configPath := createTestConfigFile(t, tmpDir, "value: test")
			manager := createTestManager(t, configPath)

			callback := func(cfg *testConfig) {}

			for i := 0; i < tt.numCallbacks; i++ {
				manager.OnUpdate(callback)
			}
		})
	}
}

func TestManager_Watch(t *testing.T) {
	tests := []struct {
		name           string
		initialContent string
		setupFile      func(*testing.T, string)
		wantCallback   bool
		wantConfig     *testConfig
	}{
		{
			name:           "file update triggers reload on next poll",
			initialContent: "value: initial",
			setupFile: func(t *testing.T, path string) {
				writeConfigFile(t, path, "value: updated")
			},
			wantCallback: true,
			wantConfig:   &testConfig{Value: "updated"},
		},
		{
			name:           "file recreation triggers reload on next poll",
			initialContent: "value: initial",
			setupFile: func(t *testing.T, path string) {
				os.Remove(path)
				time.Sleep(50 * time.Millisecond)
				writeConfigFile(t, path, "value: recreated")
			},
			wantCallback: true,
			wantConfig:   &testConfig{Value: "recreated"},
		},
		{
			name:           "file removal keeps existing config",
			initialContent: "value: initial",
			setupFile: func(t *testing.T, path string) {
				err := os.Remove(path)
				require.NoError(t, err)
			},
			wantCallback: false,
			wantConfig:   &testConfig{Value: "initial"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			configPath := createTestConfigFile(t, tmpDir, tt.initialContent)
			manager := createTestManager(t, configPath)

			callbackCalled := false
			var callbackConfig *testConfig
			manager.OnUpdate(func(cfg *testConfig) {
				callbackCalled = true
				callbackConfig = cfg
			})

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			err := manager.Watch(ctx)
			require.NoError(t, err)

			// Wait for initial poll to complete
			time.Sleep(100 * time.Millisecond)

			tt.setupFile(t, configPath)

			// Wait for next poll interval (100ms for tests)
			time.Sleep(150 * time.Millisecond)

			if diff := cmp.Diff(tt.wantCallback, callbackCalled); diff != "" {
				t.Errorf("Callback call expectation mismatch (-want +got):\n%s", diff)
			}

			if tt.wantConfig != nil {
				if tt.wantCallback {
					if diff := cmp.Diff(tt.wantConfig, callbackConfig); diff != "" {
						t.Errorf("Callback config mismatch (-want +got):\n%s", diff)
					}
				}
				if diff := cmp.Diff(tt.wantConfig, manager.Get()); diff != "" {
					t.Errorf("Manager config mismatch (-want +got):\n%s", diff)
				}
			}
		})
	}
}

func TestManager_ReloadCount(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := createTestConfigFile(t, tmpDir, "value: a")
	manager := createTestManager(t, configPath)

	tests := []struct {
		name        string
		fileContent string
		callReload  bool
		wantCount   uint64
	}{
		{
			name:      "initial_counter_after_NewManager",
			wantCount: 0,
		},
		{
			name:        "first_disk_change_increments",
			fileContent: "value: b",
			callReload:  true,
			wantCount:   1,
		},
		{
			name:        "second_disk_change_increments",
			fileContent: "value: c",
			callReload:  true,
			wantCount:   2,
		},
		{
			name:        "rewrite_same_bytes_does_not_increment",
			fileContent: "value: c",
			callReload:  true,
			wantCount:   2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.fileContent != "" {
				writeConfigFile(t, configPath, tt.fileContent)
			}
			if tt.callReload {
				manager.reloadIfChanged()
			}
			require.Equal(t, tt.wantCount, manager.reloadCount.Load())
		})
	}
}

func TestManager_HashValidation(t *testing.T) {
	tests := []struct {
		name           string
		initialContent string
		updateContent  string
		wantCallback   bool
		wantConfig     *testConfig
	}{
		{
			name:           "reload skipped when content unchanged",
			initialContent: "value: test",
			updateContent:  "value: test",
			wantCallback:   false,
		},
		{
			name:           "reload proceeds when content changed",
			initialContent: "value: initial",
			updateContent:  "value: updated",
			wantCallback:   true,
			wantConfig:     &testConfig{Value: "updated"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			configPath := createTestConfigFile(t, tmpDir, tt.initialContent)
			manager := createTestManager(t, configPath)

			callbackCalled := false
			manager.OnUpdate(func(cfg *testConfig) {
				callbackCalled = true
			})

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			err := manager.Watch(ctx)
			require.NoError(t, err)

			// Wait for first poll to complete
			time.Sleep(150 * time.Millisecond)

			// Cancel context to stop polling before file changes
			cancel()
			time.Sleep(50 * time.Millisecond) // Give goroutine time to stop

			writeConfigFile(t, configPath, tt.updateContent)

			// Manually trigger reload to test hash comparison
			manager.reloadIfChanged()

			if diff := cmp.Diff(tt.wantCallback, callbackCalled); diff != "" {
				t.Errorf("Callback expectation mismatch (-want +got):\n%s", diff)
			}

			if tt.wantConfig != nil {
				if diff := cmp.Diff(tt.wantConfig, manager.Get()); diff != "" {
					t.Errorf("Config mismatch (-want +got):\n%s", diff)
				}
			}
		})
	}
}

func TestManager_ErrorHandling(t *testing.T) {
	tests := []struct {
		name          string
		initialConfig *testConfig
		setupManager  func(*testing.T, *Manager[testConfig], string)
		wantConfig    *testConfig
	}{
		{
			name:          "failed reload keeps existing config",
			initialConfig: &testConfig{Value: "initial"},
			setupManager: func(t *testing.T, manager *Manager[testConfig], configPath string) {
				manager.loadFunc = func(path string) (*testConfig, error) {
					return nil, os.ErrNotExist
				}
				manager.reloadIfChanged()
			},
			wantConfig: &testConfig{Value: "initial"},
		},
		{
			name:          "file read errors are handled gracefully",
			initialConfig: &testConfig{Value: "initial"},
			setupManager: func(t *testing.T, manager *Manager[testConfig], configPath string) {
				err := os.Remove(configPath)
				require.NoError(t, err)
				manager.reloadIfChanged()
			},
			wantConfig: &testConfig{Value: "initial"},
		},
		{
			name:          "invalid config from loadFunc keeps existing config",
			initialConfig: &testConfig{Value: "initial"},
			setupManager: func(t *testing.T, manager *Manager[testConfig], configPath string) {
				writeConfigFile(t, configPath, "value: updated")
				manager.loadFunc = func(path string) (*testConfig, error) {
					return nil, os.ErrInvalid
				}
				manager.reloadIfChanged()
			},
			wantConfig: &testConfig{Value: "initial"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			configPath := createTestConfigFile(t, tmpDir, "value: initial")

			loadFunc := func(path string) (*testConfig, error) {
				return tt.initialConfig, nil
			}
			logger := logrus.New()
			logger.SetLevel(logrus.ErrorLevel)

			manager, err := NewManager(configPath, loadFunc, logger, DefaultPollInterval)
			require.NoError(t, err)

			tt.setupManager(t, manager, configPath)

			if diff := cmp.Diff(tt.wantConfig, manager.Get()); diff != "" {
				t.Errorf("Config mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
