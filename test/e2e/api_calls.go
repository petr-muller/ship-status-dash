//nolint:errcheck,unparam // Test helpers - error handling and unused parameters are acceptable in test code
package e2e

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ship-status-dash/pkg/types"
	"ship-status-dash/pkg/utils"
)

// getStatus is a helper function to get component status and do basic assertions
func getStatus(t *testing.T, client *TestHTTPClient, componentName, subComponentName string) types.ComponentStatus {
	url := fmt.Sprintf("/api/status/%s", utils.Slugify(componentName))
	if subComponentName != "" {
		url += fmt.Sprintf("/%s", utils.Slugify(subComponentName))
	}

	resp, err := client.Get(url, false)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	var status types.ComponentStatus
	err = json.NewDecoder(resp.Body).Decode(&status)
	require.NoError(t, err)

	return status
}

// tryGetComponentStatus returns aggregate status for a component without asserting on HTTP status.
// It is intended for polling until status changes (e.g. after config reload).
func tryGetComponentStatus(client *TestHTTPClient, componentName string) (types.Status, bool) {
	resp, err := client.Get(fmt.Sprintf("/api/status/%s", utils.Slugify(componentName)), false)
	if err != nil {
		return "", false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", false
	}
	var status types.ComponentStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return "", false
	}
	return status.Status, true
}

// getComponents is a helper function to get all components and do basic assertions
func getComponents(t *testing.T, client *TestHTTPClient) []types.Component {
	resp, err := client.Get("/api/components", false)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	var components []types.Component
	err = json.NewDecoder(resp.Body).Decode(&components)
	require.NoError(t, err)

	return components
}

// getSubComponents returns sub-components from GET /api/sub-components with optional query params (componentName, tag, team).
func getSubComponents(t *testing.T, client *TestHTTPClient, componentName, tag, team string) []types.SubComponentListItem {
	params := url.Values{}
	if componentName != "" {
		params.Set("componentName", componentName)
	}
	if tag != "" {
		params.Set("tag", tag)
	}
	if team != "" {
		params.Set("team", team)
	}
	path := "/api/sub-components"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}
	resp, err := client.Get(path, false)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	var items []types.SubComponentListItem
	err = json.NewDecoder(resp.Body).Decode(&items)
	require.NoError(t, err)
	return items
}

// getComponent is a helper function to get a specific component and do basic assertions
func getComponent(t *testing.T, client *TestHTTPClient, componentName string) types.Component {
	resp, err := client.Get(fmt.Sprintf("/api/components/%s", utils.Slugify(componentName)), false)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	var component types.Component
	err = json.NewDecoder(resp.Body).Decode(&component)
	require.NoError(t, err)

	return component
}

// getOutages is a helper function to get outages for a component or sub-component
func getOutages(t *testing.T, client *TestHTTPClient, componentName, subComponentName string) []types.Outage {
	url := fmt.Sprintf("/api/components/%s", utils.Slugify(componentName))
	if subComponentName != "" {
		url += fmt.Sprintf("/%s", utils.Slugify(subComponentName))
	}
	url += "/outages"

	resp, err := client.Get(url, false)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var outages []types.Outage
	err = json.NewDecoder(resp.Body).Decode(&outages)
	require.NoError(t, err)

	return outages
}

// getAllComponentsStatus is a helper function to get all components status and do basic assertions
func getAllComponentsStatus(t *testing.T, client *TestHTTPClient) []types.ComponentStatus {
	resp, err := client.Get("/api/status", false)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	var allStatuses []types.ComponentStatus
	err = json.NewDecoder(resp.Body).Decode(&allStatuses)
	require.NoError(t, err)

	return allStatuses
}

// expect404 is a helper function to make a GET request and expect a 404 response
func expect404(t *testing.T, client *TestHTTPClient, url string, protected bool) {
	resp, err := client.Get(url, protected)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// expect403 is a helper function to make a request and expect a 403 Forbidden response
func expect403(t *testing.T, client *TestHTTPClient, method, url string, body []byte) {
	var resp *http.Response
	var err error

	switch method {
	case "POST":
		resp, err = client.Post(url, body)
	case "PATCH":
		resp, err = client.Patch(url, body)
	case "DELETE":
		resp, err = client.Delete(url)
	default:
		t.Fatalf("Unsupported method: %s", method)
	}

	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusForbidden, resp.StatusCode)

	var errorResponse map[string]string
	err = json.NewDecoder(resp.Body).Decode(&errorResponse)
	require.NoError(t, err)
	assert.Contains(t, errorResponse["error"], "not authorized")
}
