package e2e

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"ship-status-dash/pkg/config"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/util/wait"
)

// logrus TextFormatter emits reload_count as reload_count=N on the same line as msg=...
var configReloadReloadCountRe = regexp.MustCompile(`reload_count=(\d+)`)

const configReloadLogTailLines = 2000

// configMapToPodName maps ConfigMap names to their corresponding pod names
var configMapToPodName = map[string]string{
	"dashboard-config":         "dashboard",
	"component-monitor-config": "component-monitor",
}

// isCI returns true if running in OpenShift CI environment
func isCI() bool {
	return os.Getenv("OPENSHIFT_CI") == "true"
}

// getNamespace returns the E2E namespace, failing if not set
func getNamespace(t *testing.T) string {
	namespace := os.Getenv("E2E_NS")
	require.NotEmpty(t, namespace, "E2E_NS environment variable must be set")
	return namespace
}

// getKubectlCmd returns the kubectl command, failing if not set
func getKubectlCmd(t *testing.T) string {
	kubectlCmd := os.Getenv("KUBECTL_CMD")
	require.NotEmpty(t, kubectlCmd, "KUBECTL_CMD environment variable must be set")
	return kubectlCmd
}

// getConfigMapContents retrieves the config.yaml content from a ConfigMap
func getConfigMapContents(t *testing.T, namespace, configMapName string) ([]byte, error) {
	kubectlCmd := getKubectlCmd(t)
	args := []string{"-n", namespace, "get", "configmap", configMapName, "-o", "jsonpath={.data.config\\.yaml}"}

	cmd := exec.Command(kubectlCmd, args...)
	configData, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			t.Logf("kubectl get command failed with stderr: %s", string(exitErr.Stderr))
		}
		return nil, err
	}

	return configData, nil
}

// patchConfigMap patches a ConfigMap with the provided config data and waits for propagation
func patchConfigMap(t *testing.T, namespace, configMapName string, configData []byte) {
	kubectlCmd := getKubectlCmd(t)
	podName := getPodNameForConfigMap(t, namespace, configMapName)

	baselineTail, err := getPodLogsTail(kubectlCmd, namespace, podName, configReloadLogTailLines)
	require.NoError(t, err, "failed to read pod logs for reload baseline (namespace=%s pod=%s)", namespace, podName)
	baselineReloadCount := maxReloadCountOnConfigReloadLines(baselineTail)

	patchData := map[string]interface{}{
		"data": map[string]string{
			"config.yaml": string(configData),
		},
	}
	patchJSON, err := json.Marshal(patchData)
	require.NoError(t, err, "Failed to marshal patch JSON")

	args := []string{"-n", namespace, "patch", "configmap", configMapName, "--type", "merge", "--patch", string(patchJSON)}
	patchCmd := exec.Command(kubectlCmd, args...)
	output, err := patchCmd.CombinedOutput()
	if err != nil {
		require.NoError(t, err, "Failed to patch ConfigMap %s/%s: %s", namespace, configMapName, string(output))
	}

	waitForConfigReloadInPod(t, namespace, podName, baselineReloadCount)
}

// getPodNameForConfigMap returns the pod name for a given ConfigMap
func getPodNameForConfigMap(t *testing.T, namespace, configMapName string) string {
	podName, ok := configMapToPodName[configMapName]
	if !ok {
		t.Fatalf("Unknown ConfigMap name: %s", configMapName)
	}
	return podName
}

// waitForConfigReloadInPod polls recent pod logs until reload_count exceeds baselineReloadCount.
func waitForConfigReloadInPod(t *testing.T, namespace, podName string, baselineReloadCount int) {
	kubectlCmd := getKubectlCmd(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	err := wait.PollUntilContextTimeout(ctx, 2*time.Second, 5*time.Minute, true, func(ctx context.Context) (bool, error) {
		logs, err := getPodLogsTail(kubectlCmd, namespace, podName, configReloadLogTailLines)
		if err != nil {
			return false, nil // Continue polling
		}

		if configReloadObservedSincePatch(logs, baselineReloadCount) {
			return true, nil
		}

		return false, nil // Continue polling
	})

	if err != nil {
		require.NoError(t, err, "Config reload not detected in pod %s/%s logs within 5 minutes", namespace, podName)
	}
}

func configReloadObservedSincePatch(logs string, baselineReloadCount int) bool {
	return maxReloadCountOnConfigReloadLines(logs) > baselineReloadCount
}

func maxReloadCountOnConfigReloadLines(logs string) int {
	lines := strings.Split(logs, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := lines[i]
		if !strings.Contains(line, config.ConfigReloadedMessage) {
			continue
		}
		m := configReloadReloadCountRe.FindStringSubmatch(line)
		if len(m) < 2 {
			continue
		}
		n, err := strconv.Atoi(m[1])
		if err != nil {
			continue
		}
		return n
	}
	return 0
}

func getPodLogsTail(kubectlCmd, namespace, podName string, tailLines int) (string, error) {
	args := []string{"-n", namespace, "logs", podName, "--tail", strconv.Itoa(tailLines)}
	cmd := exec.Command(kubectlCmd, args...)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(output), nil
}

// readConfig reads the config from either a file (local) or ConfigMap (CI)
func readConfig(t *testing.T, location string) []byte {
	if isCI() {
		namespace := getNamespace(t)
		configData, err := getConfigMapContents(t, namespace, location)
		require.NoError(t, err, "Failed to read ConfigMap %s/%s", namespace, location)
		return configData
	}

	configData, err := os.ReadFile(location)
	require.NoError(t, err, "Failed to read config file %s", location)
	return configData
}

// restoreConfig restores the config to either a file (local) or ConfigMap (CI)
func restoreConfig(t *testing.T, location string, configData []byte) {
	if isCI() {
		namespace := getNamespace(t)
		patchConfigMap(t, namespace, location, configData)
	} else {
		err := os.WriteFile(location, configData, 0644)
		require.NoError(t, err, "Failed to restore config file %s", location)
	}
}

// modifyConfig is a generic function that modifies a config using a typed modifier function.
// It handles both file-based (local) and ConfigMap-based (CI) configs.
func modifyConfig[T any](t *testing.T, location string, modifier func(*T)) {
	require.NotEmpty(t, location, "location must be set")
	if isCI() {
		namespace := getNamespace(t)
		updateConfigMap(t, namespace, location, modifier)
	} else {
		updateConfigFile(t, location, modifier)
	}
}

// updateConfigFile reads a config file, applies the modifier function, and writes it back.
func updateConfigFile[T any](t *testing.T, configPath string, modifier func(*T)) {
	require.NotEmpty(t, configPath, "configPath must be set")

	configData, err := os.ReadFile(configPath)
	require.NoError(t, err)

	var cfg T
	err = yaml.Unmarshal(configData, &cfg)
	require.NoError(t, err)

	modifier(&cfg)

	updatedConfig, err := yaml.Marshal(&cfg)
	require.NoError(t, err)
	err = os.WriteFile(configPath, updatedConfig, 0644)
	require.NoError(t, err)
}

// updateConfigMap updates a ConfigMap with a typed modifier.
func updateConfigMap[T any](t *testing.T, namespace string, configMapName string, modifier func(*T)) {
	configData, err := getConfigMapContents(t, namespace, configMapName)
	require.NoError(t, err, "Failed to get ConfigMap %s/%s", namespace, configMapName)

	var cfg T
	err = yaml.Unmarshal(configData, &cfg)
	require.NoError(t, err, "Failed to parse ConfigMap data")

	modifier(&cfg)

	updatedConfig, err := yaml.Marshal(&cfg)
	require.NoError(t, err, "Failed to marshal updated config")

	patchConfigMap(t, namespace, configMapName, updatedConfig)
}
