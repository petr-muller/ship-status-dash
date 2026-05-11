//nolint:errcheck,unparam // Test helpers - error handling and unused parameters are acceptable in test code
package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"ship-status-dash/pkg/types"
	"ship-status-dash/pkg/utils"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	prowComponentName       = "Prow"
	componentMonitorSAToken = "component-monitor-sa-token"
)

func TestE2E_Dashboard(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	serverURL := os.Getenv("TEST_SERVER_URL")
	if serverURL == "" {
		t.Fatalf("TEST_SERVER_URL is not set")
	}
	mockOauthProxyURL := os.Getenv("TEST_MOCK_OAUTH_PROXY_URL")
	if mockOauthProxyURL == "" {
		t.Fatalf("TEST_MOCK_OAUTH_PROXY_URL is not set")
	}
	client, err := NewTestHTTPClient(serverURL, mockOauthProxyURL)
	require.NoError(t, err)

	cleanupAbsentReportOutages(t, client)

	t.Run("Health", testHealth(client))
	t.Run("Components", testComponents(client))
	t.Run("ComponentInfo", testComponentInfo(client))
	t.Run("Outages", testOutages(client))
	t.Run("UpdateOutage", testUpdateOutage(client))
	t.Run("DeleteOutage", testDeleteOutage(client))
	t.Run("GetOutage", testGetOutage(client))
	t.Run("OutageAuditLogs", testOutageAuditLogs(client))
	t.Run("SubComponentStatus", testSubComponentStatus(client))
	t.Run("ComponentStatus", testComponentStatus(client))
	t.Run("AllComponentsStatus", testAllComponentsStatus(client))
	t.Run("ListSubComponents", testListSubComponents(client))
	t.Run("Tags", testTags(client))
	t.Run("User", testUser(client))
	t.Run("ComponentMonitorReport", testComponentMonitorReport(client))
	t.Run("AbsentReport", testAbsentReport(client))
	// ConfigHotReload should be tested at the end, it attempts to clean up after itself, but due to the nature of timing,
	// and inspecting pod logs in ci, it is not guaranteed to do so successfully.
	t.Run("ConfigHotReload", testConfigHotReload(client))
}

func testHealth(client *TestHTTPClient) func(*testing.T) {
	return func(t *testing.T) {
		resp, err := client.Get("/health", false)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

		var health map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&health)
		require.NoError(t, err)

		assert.Equal(t, "ok", health["status"])
		assert.NotEmpty(t, health["time"])
	}
}

func testComponents(client *TestHTTPClient) func(*testing.T) {
	return func(t *testing.T) {
		components := getComponents(t, client)

		assert.Len(t, components, 6)
		assert.Equal(t, "Prow", components[0].Name)
		assert.Equal(t, "Backbone of the CI system", components[0].Description)
		assert.Equal(t, "TestPlatform", components[0].ShipTeam)
		assert.Len(t, components[0].SlackReporting, 1)
		assert.Equal(t, "#test-channel", components[0].SlackReporting[0].Channel)
		assert.Equal(t, types.SeverityDown, *components[0].SlackReporting[0].Severity)
		assert.Len(t, components[0].Subcomponents, 4)
		assert.Equal(t, "Tide", components[0].Subcomponents[0].Name)
		assert.Equal(t, "Deck", components[0].Subcomponents[1].Name)
		assert.Equal(t, "Hook", components[0].Subcomponents[2].Name)
		assert.Equal(t, "Plank", components[0].Subcomponents[3].Name)

		assert.Equal(t, "Downstream CI", components[1].Name)
		assert.Equal(t, "Downstream CI system", components[1].Description)
		assert.Equal(t, "TestPlatform", components[1].ShipTeam)
		assert.Len(t, components[1].SlackReporting, 1)
		assert.Equal(t, "#test-channel", components[1].SlackReporting[0].Channel)
		assert.Equal(t, types.SeverityDown, *components[1].SlackReporting[0].Severity)
		assert.Len(t, components[1].Subcomponents, 1)
		assert.Equal(t, "Retester", components[1].Subcomponents[0].Name)

		assert.Equal(t, "Build Farm", components[2].Name)
		assert.Equal(t, "Where the CI jobs are run", components[2].Description)
		assert.Equal(t, "DPTP", components[2].ShipTeam)
		assert.Len(t, components[2].SlackReporting, 1)
		assert.Equal(t, "#ops-testplatform", components[2].SlackReporting[0].Channel)
		assert.Equal(t, types.SeverityDown, *components[2].SlackReporting[0].Severity)
		assert.Len(t, components[2].Subcomponents, 2)
		assert.Equal(t, "Build01", components[2].Subcomponents[0].Name)
		assert.Equal(t, "Build02", components[2].Subcomponents[1].Name)

		assert.Equal(t, "Boskos", components[3].Name)
		assert.Equal(t, "Resource leasing for CI workloads", components[3].Description)
		assert.Equal(t, "DPTP", components[3].ShipTeam)
		assert.Len(t, components[3].SlackReporting, 1)
		assert.Equal(t, "#ops-testplatform", components[3].SlackReporting[0].Channel)
		assert.Len(t, components[3].Subcomponents, 2)
		assert.Equal(t, "Quota", components[3].Subcomponents[0].Name)
		assert.Equal(t, "Leases", components[3].Subcomponents[1].Name)

		assert.Equal(t, "Sippy", components[4].Name)
		assert.Equal(t, "CI private investigator", components[4].Description)
		assert.Equal(t, "TRT", components[4].ShipTeam)
		assert.Len(t, components[4].SlackReporting, 1)
		assert.Equal(t, "#trt-alert", components[4].SlackReporting[0].Channel)
		assert.Equal(t, types.SeverityDown, *components[4].SlackReporting[0].Severity)
		assert.Len(t, components[4].Subcomponents, 5)
		assert.Equal(t, "Sippy", components[4].Subcomponents[0].Name)
		assert.Equal(t, "api", components[4].Subcomponents[1].Name)
		assert.Equal(t, "data-load", components[4].Subcomponents[2].Name)

		assert.Equal(t, "Errata Reliability", components[5].Name)
		assert.Equal(t, "Services maintained by the Errata Reliability Team", components[5].Description)
		assert.Equal(t, "ERT", components[5].ShipTeam)
		assert.Len(t, components[5].Subcomponents, 1)
		assert.Equal(t, "systemd-test", components[5].Subcomponents[0].Name)
	}
}

func testComponentInfo(client *TestHTTPClient) func(*testing.T) {
	return func(t *testing.T) {
		t.Run("GET component info for existing component returns component details", func(t *testing.T) {
			component := getComponent(t, client, "Prow")

			assert.Equal(t, "Prow", component.Name)
			assert.Equal(t, "Backbone of the CI system", component.Description)
			assert.Equal(t, "TestPlatform", component.ShipTeam)
			assert.Len(t, component.SlackReporting, 1)
			assert.Equal(t, "#test-channel", component.SlackReporting[0].Channel)
			assert.Equal(t, types.SeverityDown, *component.SlackReporting[0].Severity)
			assert.Len(t, component.Subcomponents, 4)
			assert.Equal(t, "Tide", component.Subcomponents[0].Name)
			assert.Equal(t, "Deck", component.Subcomponents[1].Name)
			assert.Equal(t, "Hook", component.Subcomponents[2].Name)
			assert.Equal(t, "Plank", component.Subcomponents[3].Name)
		})

		t.Run("GET component info for non-existent component returns 404", func(t *testing.T) {
			expect404(t, client, "/api/components/"+utils.Slugify("NonExistentComponent"), false)
		})
	}
}

// createOutage is a helper function to create an outage for testing
func createOutage(t *testing.T, client *TestHTTPClient, componentName, subComponentName string) types.Outage {
	outagePayload := map[string]interface{}{
		"severity":        string(types.SeverityDown),
		"start_time":      time.Now().UTC().Format(time.RFC3339),
		"description":     "Test outage for " + subComponentName,
		"discovered_from": "e2e-test",
		"created_by":      "developer",
	}

	payloadBytes, err := json.Marshal(outagePayload)
	require.NoError(t, err)

	resp, err := client.Post(fmt.Sprintf("/api/components/%s/%s/outages", utils.Slugify(componentName), utils.Slugify(subComponentName)), payloadBytes)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var outage types.Outage
	err = json.NewDecoder(resp.Body).Decode(&outage)
	require.NoError(t, err)

	// Verify that created_by is set to the user from X-Forwarded-User header
	assert.Equal(t, "developer", outage.CreatedBy, "created_by should be set to the user from X-Forwarded-User header")

	return outage
}

// deleteOutage is a helper function to delete an outage for cleanup
func deleteOutage(t *testing.T, client *TestHTTPClient, componentName, subComponentName string, outageID uint) {
	resp, err := client.Delete(fmt.Sprintf("/api/components/%s/%s/outages/%d", utils.Slugify(componentName), utils.Slugify(subComponentName), outageID))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}

// updateOutage is a helper function to update an outage
func updateOutage(t *testing.T, client *TestHTTPClient, componentName, subComponentName string, outageID uint, payload map[string]interface{}) {
	payloadBytes, err := json.Marshal(payload)
	require.NoError(t, err)

	updateURL := fmt.Sprintf("/api/components/%s/%s/outages/%d", utils.Slugify(componentName), utils.Slugify(subComponentName), outageID)
	resp, err := client.Patch(updateURL, payloadBytes)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func testOutages(client *TestHTTPClient) func(*testing.T) {
	return func(t *testing.T) {
		t.Run("POST to sub-component succeeds", func(t *testing.T) {
			outage := createOutage(t, client, "Prow", "Tide")
			defer deleteOutage(t, client, "Prow", "Tide", outage.ID)

			assert.NotZero(t, outage.ID)
			assert.Equal(t, utils.Slugify("Prow"), outage.ComponentName)
			assert.Equal(t, utils.Slugify("Tide"), outage.SubComponentName)
			assert.Equal(t, string(types.SeverityDown), string(outage.Severity))
			assert.Equal(t, "e2e-test", outage.DiscoveredFrom)
		})

		t.Run("POST to non-existent sub-component fails", func(t *testing.T) {
			outagePayload := map[string]interface{}{
				"severity":        string(types.SeverityDown),
				"start_time":      time.Now().UTC().Format(time.RFC3339),
				"description":     "Test outage for non-existent sub-component",
				"discovered_from": "e2e-test",
				"created_by":      "developer",
			}

			payloadBytes, err := json.Marshal(outagePayload)
			require.NoError(t, err)

			resp, err := client.Post(fmt.Sprintf("/api/components/%s/%s/outages", utils.Slugify("Prow"), utils.Slugify("NonExistentSub")), payloadBytes)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusNotFound, resp.StatusCode)
		})

		t.Run("POST with invalid severity fails", func(t *testing.T) {
			outagePayload := map[string]interface{}{
				"severity":        "InvalidSeverity",
				"start_time":      time.Now().UTC().Format(time.RFC3339),
				"description":     "Test outage with invalid severity",
				"discovered_from": "e2e-test",
				"created_by":      "developer",
			}

			payloadBytes, err := json.Marshal(outagePayload)
			require.NoError(t, err)

			resp, err := client.Post(fmt.Sprintf("/api/components/%s/%s/outages", utils.Slugify("Prow"), utils.Slugify("Deck")), payloadBytes)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

			var errorResponse map[string]string
			err = json.NewDecoder(resp.Body).Decode(&errorResponse)
			require.NoError(t, err)
			assert.Contains(t, errorResponse["error"], "Invalid severity")
		})

		t.Run("GET on top-level component aggregates sub-components", func(t *testing.T) {
			// Create outages for different sub-components
			tideOutage := createOutage(t, client, "Prow", "Tide")
			defer deleteOutage(t, client, "Prow", "Tide", tideOutage.ID)
			deckOutage := createOutage(t, client, "Prow", "Deck")
			defer deleteOutage(t, client, "Prow", "Deck", deckOutage.ID)

			outages := getOutages(t, client, "Prow", "")

			// Should have exactly our 2 outages since we clean up after ourselves
			assert.Len(t, outages, 2)

			// Verify our specific outages are present
			outageIDs := make(map[uint]bool)
			for _, outage := range outages {
				outageIDs[outage.ID] = true
			}
			assert.True(t, outageIDs[tideOutage.ID], "Tide outage should be present")
			assert.True(t, outageIDs[deckOutage.ID], "Deck outage should be present")
		})

		t.Run("GET on sub-component returns only that sub-component's outages", func(t *testing.T) {
			// Create outages for different sub-components
			tideOutage1 := createOutage(t, client, "Prow", "Tide")
			defer deleteOutage(t, client, "Prow", "Tide", tideOutage1.ID)
			tideOutage2 := createOutage(t, client, "Prow", "Tide")
			defer deleteOutage(t, client, "Prow", "Tide", tideOutage2.ID)
			deckOutage := createOutage(t, client, "Prow", "Deck")
			defer deleteOutage(t, client, "Prow", "Deck", deckOutage.ID)

			outages := getOutages(t, client, "Prow", "Tide")

			// Should have exactly our 2 Tide outages since we clean up after ourselves
			assert.Len(t, outages, 2)

			// All outages should be for Tide only
			for _, outage := range outages {
				assert.Equal(t, utils.Slugify("Tide"), outage.SubComponentName)
			}

			// Verify our specific outages are present
			outageIDs := make(map[uint]bool)
			for _, outage := range outages {
				outageIDs[outage.ID] = true
			}
			assert.True(t, outageIDs[tideOutage1.ID], "First Tide outage should be present")
			assert.True(t, outageIDs[tideOutage2.ID], "Second Tide outage should be present")
			assert.False(t, outageIDs[deckOutage.ID], "Deck outage should not be included")
		})

		t.Run("GET on non-existent sub-component fails", func(t *testing.T) {
			// This test doesn't need any setup - it should fail regardless of existing data
			expect404(t, client, fmt.Sprintf("/api/components/%s/%s/outages", utils.Slugify("Prow"), utils.Slugify("NonExistentSub")), false)
		})

		t.Run("POST to unauthorized component returns 403", func(t *testing.T) {
			outagePayload := map[string]interface{}{
				"severity":        string(types.SeverityDown),
				"start_time":      time.Now().UTC().Format(time.RFC3339),
				"description":     "Test outage for unauthorized component",
				"discovered_from": "e2e-test",
				"created_by":      "developer",
			}

			payloadBytes, err := json.Marshal(outagePayload)
			require.NoError(t, err)

			expect403(t, client, "POST", fmt.Sprintf("/api/components/%s/%s/outages", utils.Slugify("Build Farm"), utils.Slugify("Build01")), payloadBytes)
		})
	}
}

func testUpdateOutage(client *TestHTTPClient) func(*testing.T) {
	return func(t *testing.T) {
		// Create an outage to update
		createdOutage := createOutage(t, client, "Prow", "Tide")
		defer deleteOutage(t, client, "Prow", "Tide", createdOutage.ID)

		// Verify that StartTime is rounded to the nearest second (no sub-second precision)
		assert.Equal(t, 0, createdOutage.StartTime.Nanosecond(), "StartTime should be rounded to the nearest second")

		// Now update the outage
		updatePayload := map[string]interface{}{
			"severity":     string(types.SeverityDegraded),
			"description":  "Updated description",
			"triage_notes": "Updated triage notes",
		}

		updateBytes, err := json.Marshal(updatePayload)
		require.NoError(t, err)

		updateURL := fmt.Sprintf("/api/components/%s/%s/outages/%d", utils.Slugify("Prow"), utils.Slugify("Tide"), createdOutage.ID)
		t.Logf("Making PATCH request to: %s", updateURL)

		updateResp, err := client.Patch(updateURL, updateBytes)
		require.NoError(t, err)
		defer updateResp.Body.Close()

		if updateResp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(updateResp.Body)
			t.Logf("Unexpected status %d, body: %s", updateResp.StatusCode, string(body))
		}

		assert.Equal(t, http.StatusOK, updateResp.StatusCode)
		assert.Equal(t, "application/json", updateResp.Header.Get("Content-Type"))

		var updatedOutage types.Outage
		err = json.NewDecoder(updateResp.Body).Decode(&updatedOutage)
		require.NoError(t, err)

		assert.Equal(t, createdOutage.ID, updatedOutage.ID)
		assert.Equal(t, string(types.SeverityDegraded), string(updatedOutage.Severity))
		assert.Equal(t, "Updated description", updatedOutage.Description)
		assert.NotNil(t, updatedOutage.TriageNotes)
		assert.Equal(t, "Updated triage notes", *updatedOutage.TriageNotes)
		assert.WithinDuration(t, createdOutage.StartTime.UTC(), updatedOutage.StartTime.UTC(), time.Second) // Should remain unchanged
		assert.Equal(t, createdOutage.CreatedBy, updatedOutage.CreatedBy)                                   // Should remain unchanged

		// Test updating non-existent outage
		nonExistentResp, err := client.Patch(fmt.Sprintf("/api/components/%s/%s/outages/99999", utils.Slugify("Prow"), utils.Slugify("Tide")), updateBytes)
		require.NoError(t, err)
		defer nonExistentResp.Body.Close()

		assert.Equal(t, http.StatusNotFound, nonExistentResp.StatusCode)

		// Test updating with invalid component
		invalidComponentResp, err := client.Patch(fmt.Sprintf("/api/components/%s/%s/outages/%d", utils.Slugify("NonExistentComponent"), utils.Slugify("Tide"), createdOutage.ID), updateBytes)
		require.NoError(t, err)
		defer invalidComponentResp.Body.Close()

		assert.Equal(t, http.StatusNotFound, invalidComponentResp.StatusCode)

		// Test updating with invalid severity
		invalidSeverityUpdate := map[string]interface{}{
			"severity": "InvalidSeverity",
		}
		invalidSeverityBytes, err := json.Marshal(invalidSeverityUpdate)
		require.NoError(t, err)

		invalidSeverityResp, err := client.Patch(fmt.Sprintf("/api/components/%s/%s/outages/%d", utils.Slugify("Prow"), utils.Slugify("Tide"), createdOutage.ID), invalidSeverityBytes)
		require.NoError(t, err)
		defer invalidSeverityResp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, invalidSeverityResp.StatusCode)

		var errorResponse map[string]string
		err = json.NewDecoder(invalidSeverityResp.Body).Decode(&errorResponse)
		require.NoError(t, err)
		assert.Contains(t, errorResponse["error"], "Invalid severity")

		// Test confirming an outage
		confirmPayload := map[string]interface{}{
			"confirmed": true,
		}
		confirmBytes, err := json.Marshal(confirmPayload)
		require.NoError(t, err)

		confirmResp, err := client.Patch(fmt.Sprintf("/api/components/%s/%s/outages/%d", utils.Slugify("Prow"), utils.Slugify("Tide"), createdOutage.ID), confirmBytes)
		require.NoError(t, err)
		defer confirmResp.Body.Close()

		assert.Equal(t, http.StatusOK, confirmResp.StatusCode)

		var confirmedOutage types.Outage
		err = json.NewDecoder(confirmResp.Body).Decode(&confirmedOutage)
		require.NoError(t, err)

		assert.True(t, confirmedOutage.ConfirmedAt.Valid, "confirmed_at should be set when confirmed is true")
		// Verify that ConfirmedAt is rounded to the nearest second (no sub-second precision)
		assert.Equal(t, 0, confirmedOutage.ConfirmedAt.Time.Nanosecond(), "ConfirmedAt should be rounded to the nearest second")

		t.Run("PATCH to unauthorized component returns 403", func(t *testing.T) {
			updatePayload := map[string]interface{}{
				"severity": string(types.SeverityDegraded),
			}

			updateBytes, err := json.Marshal(updatePayload)
			require.NoError(t, err)

			expect403(t, client, "PATCH", fmt.Sprintf("/api/components/%s/%s/outages/1", utils.Slugify("Build Farm"), utils.Slugify("Build01")), updateBytes)
		})

		t.Run("resolved_by should not change when end_time is unchanged, but should change when end_time is modified", func(t *testing.T) {
			serverURL := os.Getenv("TEST_SERVER_URL")
			require.NotEmpty(t, serverURL, "TEST_SERVER_URL must be set")
			mockOauthProxyURL := os.Getenv("TEST_MOCK_OAUTH_PROXY_URL")
			require.NotEmpty(t, mockOauthProxyURL, "TEST_MOCK_OAUTH_PROXY_URL must be set")

			// Create an outage with user1 (developer)
			createdOutage := createOutage(t, client, "Prow", "Tide")
			defer deleteOutage(t, client, "Prow", "Tide", createdOutage.ID)

			// Resolve the outage with user1 (developer) by setting end_time
			resolveTime := time.Now().UTC()
			resolvePayload := map[string]interface{}{
				"end_time": map[string]interface{}{
					"Time":  resolveTime.Format(time.RFC3339),
					"Valid": true,
				},
			}
			resolveBytes, err := json.Marshal(resolvePayload)
			require.NoError(t, err)

			resolveResp, err := client.Patch(fmt.Sprintf("/api/components/%s/%s/outages/%d", utils.Slugify("Prow"), utils.Slugify("Tide"), createdOutage.ID), resolveBytes)
			require.NoError(t, err)
			defer resolveResp.Body.Close()

			assert.Equal(t, http.StatusOK, resolveResp.StatusCode)

			var resolvedOutage types.Outage
			err = json.NewDecoder(resolveResp.Body).Decode(&resolvedOutage)
			require.NoError(t, err)

			assert.True(t, resolvedOutage.EndTime.Valid, "end_time should be valid after resolution")
			// Verify that EndTime is rounded to the nearest second (no sub-second precision)
			assert.Equal(t, 0, resolvedOutage.EndTime.Time.Nanosecond(), "EndTime should be rounded to the nearest second")
			originalEndTime := resolvedOutage.EndTime.Time

			// Now update the outage with user2 (editor) without changing end_time
			editorClient, err := NewTestHTTPClientWithUsername(serverURL, mockOauthProxyURL, "editor")
			require.NoError(t, err)

			updatePayload := map[string]interface{}{
				"description":  "Updated description by editor",
				"triage_notes": "Updated triage notes by editor",
			}
			updateBytes, err := json.Marshal(updatePayload)
			require.NoError(t, err)

			updateResp, err := editorClient.Patch(fmt.Sprintf("/api/components/%s/%s/outages/%d", utils.Slugify("Prow"), utils.Slugify("Tide"), createdOutage.ID), updateBytes)
			require.NoError(t, err)
			defer updateResp.Body.Close()

			assert.Equal(t, http.StatusOK, updateResp.StatusCode)

			var updatedOutage types.Outage
			err = json.NewDecoder(updateResp.Body).Decode(&updatedOutage)
			require.NoError(t, err)

			assert.True(t, updatedOutage.EndTime.Valid, "end_time should still be valid")
			assert.WithinDuration(t, originalEndTime, updatedOutage.EndTime.Time, time.Second, "end_time should not have changed")

			// Now update the outage with user2 (editor) by changing end_time
			newResolveTime := time.Now().UTC().Add(1 * time.Hour)
			changeEndTimePayload := map[string]interface{}{
				"end_time": map[string]interface{}{
					"Time":  newResolveTime.Format(time.RFC3339),
					"Valid": true,
				},
			}
			changeEndTimeBytes, err := json.Marshal(changeEndTimePayload)
			require.NoError(t, err)

			changeEndTimeResp, err := editorClient.Patch(fmt.Sprintf("/api/components/%s/%s/outages/%d", utils.Slugify("Prow"), utils.Slugify("Tide"), createdOutage.ID), changeEndTimeBytes)
			require.NoError(t, err)
			defer changeEndTimeResp.Body.Close()

			assert.Equal(t, http.StatusOK, changeEndTimeResp.StatusCode)

			var changedEndTimeOutage types.Outage
			err = json.NewDecoder(changeEndTimeResp.Body).Decode(&changedEndTimeOutage)
			require.NoError(t, err)

			assert.True(t, changedEndTimeOutage.EndTime.Valid, "end_time should still be valid")
			assert.WithinDuration(t, newResolveTime, changedEndTimeOutage.EndTime.Time, time.Second, "end_time should have been updated")
		})
	}
}

func testDeleteOutage(client *TestHTTPClient) func(*testing.T) {
	return func(t *testing.T) {
		t.Run("DELETE existing outage succeeds", func(t *testing.T) {
			// Create an outage to delete
			createdOutage := createOutage(t, client, "Prow", "Tide")

			// Delete the outage
			deleteOutage(t, client, "Prow", "Tide", createdOutage.ID)

			// Verify the outage is deleted by trying to get it
			resp, err := client.Get(fmt.Sprintf("/api/components/%s/%s/outages", utils.Slugify("Prow"), utils.Slugify("Tide")), false)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusOK, resp.StatusCode)

			var outages []types.Outage
			err = json.NewDecoder(resp.Body).Decode(&outages)
			require.NoError(t, err)

			// The deleted outage should not be in the list
			for _, outage := range outages {
				assert.NotEqual(t, createdOutage.ID, outage.ID, "Deleted outage should not be present")
			}
		})

		t.Run("DELETE non-existent outage returns 404", func(t *testing.T) {
			resp, err := client.Delete(fmt.Sprintf("/api/components/%s/%s/outages/99999", utils.Slugify("Prow"), utils.Slugify("Tide")))
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusNotFound, resp.StatusCode)
		})

		t.Run("DELETE outage from non-existent component returns 404", func(t *testing.T) {
			resp, err := client.Delete(fmt.Sprintf("/api/components/%s/%s/outages/1", utils.Slugify("NonExistentComponent"), utils.Slugify("Tide")))
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusNotFound, resp.StatusCode)
		})

		t.Run("DELETE outage from non-existent sub-component returns 404", func(t *testing.T) {
			resp, err := client.Delete(fmt.Sprintf("/api/components/%s/%s/outages/1", utils.Slugify("Prow"), utils.Slugify("NonExistentSub")))
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusNotFound, resp.StatusCode)
		})

		t.Run("DELETE outage from unauthorized component returns 403", func(t *testing.T) {
			expect403(t, client, "DELETE", fmt.Sprintf("/api/components/%s/%s/outages/1", utils.Slugify("Build Farm"), utils.Slugify("Build01")), nil)
		})
	}
}

func testGetOutage(client *TestHTTPClient) func(*testing.T) {
	return func(t *testing.T) {
		t.Run("GET existing outage succeeds", func(t *testing.T) {
			// Create an outage to retrieve
			createdOutage := createOutage(t, client, "Prow", "Tide")
			defer deleteOutage(t, client, "Prow", "Tide", createdOutage.ID)

			// Get the outage
			resp, err := client.Get(fmt.Sprintf("/api/components/%s/%s/outages/%d", utils.Slugify("Prow"), utils.Slugify("Tide"), createdOutage.ID), false)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusOK, resp.StatusCode)
			assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

			var outage types.Outage
			err = json.NewDecoder(resp.Body).Decode(&outage)
			require.NoError(t, err)

			assert.Equal(t, createdOutage.ID, outage.ID)
			assert.Equal(t, utils.Slugify("Tide"), outage.SubComponentName)
			assert.Equal(t, string(types.SeverityDown), string(outage.Severity))
			assert.Equal(t, "e2e-test", outage.DiscoveredFrom)
			assert.Equal(t, "developer", outage.CreatedBy)
		})

		t.Run("GET non-existent outage returns 404", func(t *testing.T) {
			resp, err := client.Get(fmt.Sprintf("/api/components/%s/%s/outages/99999", utils.Slugify("Prow"), utils.Slugify("Tide")), false)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusNotFound, resp.StatusCode)
		})

		t.Run("GET outage from non-existent component returns 404", func(t *testing.T) {
			resp, err := client.Get(fmt.Sprintf("/api/components/%s/%s/outages/1", utils.Slugify("NonExistentComponent"), utils.Slugify("Tide")), false)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusNotFound, resp.StatusCode)
		})

		t.Run("GET outage from non-existent sub-component returns 404", func(t *testing.T) {
			resp, err := client.Get(fmt.Sprintf("/api/components/%s/%s/outages/1", utils.Slugify("Prow"), utils.Slugify("NonExistentSub")), false)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusNotFound, resp.StatusCode)
		})

		t.Run("GET outage with wrong sub-component returns 404", func(t *testing.T) {
			// Create an outage for Tide
			tideOutage := createOutage(t, client, "Prow", "Tide")
			defer deleteOutage(t, client, "Prow", "Tide", tideOutage.ID)

			// Try to get it as if it were a Deck outage
			resp, err := client.Get(fmt.Sprintf("/api/components/%s/%s/outages/%d", utils.Slugify("Prow"), utils.Slugify("Deck"), tideOutage.ID), false)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusNotFound, resp.StatusCode)
		})
	}
}

func testOutageAuditLogs(client *TestHTTPClient) func(*testing.T) {
	return func(t *testing.T) {
		t.Run("GET audit-logs after create, update, and delete", func(t *testing.T) {
			createdOutage := createOutage(t, client, "Prow", "Tide")

			auditLogsURL := fmt.Sprintf("/api/components/%s/%s/outages/%d/audit-logs", utils.Slugify("Prow"), utils.Slugify("Tide"), createdOutage.ID)

			resp, err := client.Get(auditLogsURL, false)
			require.NoError(t, err)
			defer resp.Body.Close()
			require.Equal(t, http.StatusOK, resp.StatusCode)

			var logsAfterCreate []types.OutageAuditLog
			err = json.NewDecoder(resp.Body).Decode(&logsAfterCreate)
			require.NoError(t, err)
			require.Len(t, logsAfterCreate, 1, "audit logs after create should have one entry")
			assert.Equal(t, createdOutage.ID, logsAfterCreate[0].OutageID)
			assert.Equal(t, "CREATE", logsAfterCreate[0].Operation)
			assert.Equal(t, "developer", logsAfterCreate[0].User)

			updateOutage(t, client, "Prow", "Tide", createdOutage.ID, map[string]interface{}{
				"description": "Updated description for audit test",
			})

			resp2, err := client.Get(auditLogsURL, false)
			require.NoError(t, err)
			defer resp2.Body.Close()
			require.Equal(t, http.StatusOK, resp2.StatusCode)

			var logsAfterUpdate []types.OutageAuditLog
			err = json.NewDecoder(resp2.Body).Decode(&logsAfterUpdate)
			require.NoError(t, err)
			require.Len(t, logsAfterUpdate, 2, "audit logs after update should have two entries")
			// API returns logs newest first (created_at DESC)
			assert.Equal(t, "UPDATE", logsAfterUpdate[0].Operation)
			assert.Equal(t, "CREATE", logsAfterUpdate[1].Operation)
			assert.Equal(t, "developer", logsAfterUpdate[0].User)

			deleteOutage(t, client, "Prow", "Tide", createdOutage.ID)

			resp3, err := client.Get(auditLogsURL, false)
			require.NoError(t, err)
			defer resp3.Body.Close()
			assert.Equal(t, http.StatusNotFound, resp3.StatusCode, "audit-logs for deleted outage should return 404")
		})
	}
}

func testSubComponentStatus(client *TestHTTPClient) func(*testing.T) {
	return func(t *testing.T) {
		t.Run("GET status for healthy sub-component returns Healthy", func(t *testing.T) {
			status := getStatus(t, client, "Prow", "Deck")

			assert.Equal(t, types.StatusHealthy, status.Status)
			assert.Empty(t, status.ActiveOutages)
		})

		t.Run("GET status for sub-component with active outage returns outage severity", func(t *testing.T) {
			// Create an outage for Deck (should be auto-confirmed)
			outage := createOutage(t, client, "Prow", "Deck")
			defer deleteOutage(t, client, "Prow", "Deck", outage.ID)

			status := getStatus(t, client, "Prow", "Deck")

			assert.Equal(t, types.StatusDown, status.Status)
			assert.Len(t, status.ActiveOutages, 1)
			assert.Equal(t, string(types.SeverityDown), string(status.ActiveOutages[0].Severity))
		})

		t.Run("GET status for sub-component with multiple outages returns most critical", func(t *testing.T) {
			// Create a Degraded outage for Tide
			degradedOutage := createOutageWithSeverity(t, client, "Prow", "Tide", string(types.SeverityDegraded))
			defer deleteOutage(t, client, "Prow", "Tide", degradedOutage.ID)

			// Create a Down outage for Tide
			downOutage := createOutageWithSeverity(t, client, "Prow", "Tide", string(types.SeverityDown))
			defer deleteOutage(t, client, "Prow", "Tide", downOutage.ID)

			// Confirm both outages to test most critical logic
			updateOutage(t, client, "Prow", "Tide", degradedOutage.ID, map[string]interface{}{
				"confirmed": true,
			})
			updateOutage(t, client, "Prow", "Tide", downOutage.ID, map[string]interface{}{
				"confirmed": true,
			})

			status := getStatus(t, client, "Prow", "Tide")

			assert.Equal(t, types.StatusDown, status.Status)
			assert.Len(t, status.ActiveOutages, 2)
		})

		t.Run("GET status for non-existent component returns 404", func(t *testing.T) {
			expect404(t, client, fmt.Sprintf("/api/status/%s/%s", utils.Slugify("NonExistent"), utils.Slugify("Deck")), false)
		})

		t.Run("GET status for non-existent sub-component returns 404", func(t *testing.T) {
			expect404(t, client, fmt.Sprintf("/api/status/%s/%s", utils.Slugify("Prow"), utils.Slugify("NonExistent")), false)
		})

		t.Run("GET status for sub-component with future end_time still considers outage active", func(t *testing.T) {
			// Create an outage first (should be auto-confirmed)
			outage := createOutage(t, client, "Prow", "Deck")
			defer deleteOutage(t, client, "Prow", "Deck", outage.ID)

			// Update the outage to have a future end_time
			futureTime := time.Now().Add(24 * time.Hour) // 24 hours in the future
			updatePayload := map[string]interface{}{
				"end_time": map[string]interface{}{
					"Time":  futureTime.UTC().Format(time.RFC3339),
					"Valid": true,
				},
			}

			updateBytes, err := json.Marshal(updatePayload)
			require.NoError(t, err)

			updateResp, err := client.Patch(fmt.Sprintf("/api/components/%s/%s/outages/%d", utils.Slugify("Prow"), utils.Slugify("Deck"), outage.ID), updateBytes)
			require.NoError(t, err)
			defer updateResp.Body.Close()

			assert.Equal(t, http.StatusOK, updateResp.StatusCode)

			var updatedOutage types.Outage
			err = json.NewDecoder(updateResp.Body).Decode(&updatedOutage)
			require.NoError(t, err)

			// Check that the status endpoint still considers this outage active
			status := getStatus(t, client, "Prow", "Deck")

			assert.Equal(t, types.StatusDown, status.Status)
			assert.Len(t, status.ActiveOutages, 1)
			assert.Equal(t, outage.ID, status.ActiveOutages[0].ID)
		})

		t.Run("GET status for sub-component with unconfirmed outage returns Suspected", func(t *testing.T) {
			// Create an unconfirmed outage for Tide (which has requires_confirmation: true)
			outage := createOutage(t, client, "Prow", "Tide")
			defer deleteOutage(t, client, "Prow", "Tide", outage.ID)

			status := getStatus(t, client, "Prow", "Tide")

			assert.Equal(t, types.StatusSuspected, status.Status)
			assert.Len(t, status.ActiveOutages, 1)
			assert.False(t, status.ActiveOutages[0].ConfirmedAt.Valid)
		})

		t.Run("GET status for sub-component with mixed confirmed/unconfirmed outages returns confirmed severity", func(t *testing.T) {
			// Create confirmed degraded outage
			confirmedOutage := createOutageWithSeverity(t, client, "Prow", "Tide", string(types.SeverityDegraded))
			defer deleteOutage(t, client, "Prow", "Tide", confirmedOutage.ID)

			// Confirm the degraded outage
			updateOutage(t, client, "Prow", "Tide", confirmedOutage.ID, map[string]interface{}{
				"confirmed": true,
			})

			// Create unconfirmed down outage
			unconfirmedOutage := createOutageWithSeverity(t, client, "Prow", "Tide", string(types.SeverityDown))
			defer deleteOutage(t, client, "Prow", "Tide", unconfirmedOutage.ID)

			status := getStatus(t, client, "Prow", "Tide")

			// Should return Degraded (confirmed) not Suspected (unconfirmed)
			assert.Equal(t, types.StatusDegraded, status.Status)
			assert.Len(t, status.ActiveOutages, 2)
		})

		t.Run("GET status for sub-component without ping returns no last_ping_time", func(t *testing.T) {
			status := getStatus(t, client, "Prow", "Deck")

			assert.Equal(t, types.StatusHealthy, status.Status)
			assert.Nil(t, status.LastPingTime, "last_ping_time should be nil when no ping has been sent")
		})

		t.Run("GET status for sub-component with ping returns last_ping_time", func(t *testing.T) {
			// Send a component monitor report to create a ping
			reportPayload := types.ComponentMonitorReportRequest{
				ComponentMonitor: "app-ci-component-monitor",
				Statuses: []types.ComponentMonitorReportComponentStatus{
					{
						ComponentSlug:    utils.Slugify("Prow"),
						SubComponentSlug: utils.Slugify("Hook"),
						Status:           types.StatusHealthy,
						Reasons:          []types.Reason{{Type: types.CheckTypePrometheus}},
					},
				},
			}

			payloadBytes, err := json.Marshal(reportPayload)
			require.NoError(t, err)

			reportSentTime := time.Now()
			resp, err := client.PostWithBearerToken("/api/component-monitor/report", payloadBytes, componentMonitorSAToken)
			require.NoError(t, err)
			resp.Body.Close()
			assert.Equal(t, http.StatusOK, resp.StatusCode)

			status := getStatus(t, client, "Prow", "Hook")

			assert.NotNil(t, status.LastPingTime, "last_ping_time should be set after component monitor report")
			assert.WithinDuration(t, reportSentTime, *status.LastPingTime, 5*time.Second, "last_ping_time should be within 5 seconds of when report was sent")
		})

		t.Run("GET status for sub-component updates last_ping_time on subsequent reports", func(t *testing.T) {
			// Send first report
			reportPayload1 := types.ComponentMonitorReportRequest{
				ComponentMonitor: "app-ci-component-monitor",
				Statuses: []types.ComponentMonitorReportComponentStatus{
					{
						ComponentSlug:    utils.Slugify("Prow"),
						SubComponentSlug: utils.Slugify("Hook"),
						Status:           types.StatusHealthy,
						Reasons:          []types.Reason{{Type: types.CheckTypePrometheus}},
					},
				},
			}

			payloadBytes1, err := json.Marshal(reportPayload1)
			require.NoError(t, err)

			resp1, err := client.PostWithBearerToken("/api/component-monitor/report", payloadBytes1, componentMonitorSAToken)
			require.NoError(t, err)
			resp1.Body.Close()
			assert.Equal(t, http.StatusOK, resp1.StatusCode)

			status1 := getStatus(t, client, "Prow", "Hook")
			require.NotNil(t, status1.LastPingTime, "first report should set last_ping_time")
			firstPingTime := *status1.LastPingTime

			// Wait a moment to ensure timestamp is different
			time.Sleep(200 * time.Millisecond)

			// Send second report
			resp2, err := client.PostWithBearerToken("/api/component-monitor/report", payloadBytes1, componentMonitorSAToken)
			require.NoError(t, err)
			resp2.Body.Close()
			assert.Equal(t, http.StatusOK, resp2.StatusCode)

			status2 := getStatus(t, client, "Prow", "Hook")
			require.NotNil(t, status2.LastPingTime, "second report should update last_ping_time")
			secondPingTime := *status2.LastPingTime

			assert.True(t, secondPingTime.After(firstPingTime), "second ping time should be after first ping time")
		})
	}
}

func testComponentStatus(client *TestHTTPClient) func(*testing.T) {
	return func(t *testing.T) {
		t.Run("GET status for healthy component returns Healthy", func(t *testing.T) {
			status := getStatus(t, client, "Prow", "")

			assert.Equal(t, types.StatusHealthy, status.Status)
			assert.Empty(t, status.ActiveOutages)
		})

		t.Run("GET status for component with one degraded sub-component returns Partial", func(t *testing.T) {
			// Create a degraded outage for Deck (doesn't require confirmation)
			deckOutage := createOutageWithSeverity(t, client, "Prow", "Deck", string(types.SeverityDegraded))
			defer deleteOutage(t, client, "Prow", "Deck", deckOutage.ID)

			status := getStatus(t, client, "Prow", "")

			assert.Equal(t, types.StatusPartial, status.Status)
			assert.Len(t, status.ActiveOutages, 1)
			assert.Equal(t, string(types.SeverityDegraded), string(status.ActiveOutages[0].Severity))
		})

		t.Run("GET status for component with all sub-components down returns Down", func(t *testing.T) {
			// Create Down outages for all sub-components
			tideOutage := createOutageWithSeverity(t, client, "Prow", "Tide", string(types.SeverityDown))
			defer deleteOutage(t, client, "Prow", "Tide", tideOutage.ID)
			deckOutage := createOutageWithSeverity(t, client, "Prow", "Deck", string(types.SeverityDown))
			defer deleteOutage(t, client, "Prow", "Deck", deckOutage.ID)
			hookOutage := createOutageWithSeverity(t, client, "Prow", "Hook", string(types.SeverityDown))
			defer deleteOutage(t, client, "Prow", "Hook", hookOutage.ID)
			plankOutage := createOutageWithSeverity(t, client, "Prow", "Plank", string(types.SeverityDown))
			defer deleteOutage(t, client, "Prow", "Plank", plankOutage.ID)

			// Confirm Tide outage (others should be auto-confirmed)
			updateOutage(t, client, "Prow", "Tide", tideOutage.ID, map[string]interface{}{
				"confirmed": true,
			})

			status := getStatus(t, client, "Prow", "")

			assert.Equal(t, types.StatusDown, status.Status)
			assert.Len(t, status.ActiveOutages, 4)
			for _, outage := range status.ActiveOutages {
				assert.Equal(t, string(types.SeverityDown), string(outage.Severity))
			}
		})

		t.Run("GET status for component with mixed severity outages returns most severe", func(t *testing.T) {
			// Create outages with different severities for all sub-components
			tideOutage := createOutageWithSeverity(t, client, "Prow", "Tide", string(types.SeverityDown))
			defer deleteOutage(t, client, "Prow", "Tide", tideOutage.ID)
			deckOutage := createOutageWithSeverity(t, client, "Prow", "Deck", string(types.SeverityDegraded))
			defer deleteOutage(t, client, "Prow", "Deck", deckOutage.ID)
			hookOutage := createOutageWithSeverity(t, client, "Prow", "Hook", string(types.SeverityDegraded))
			defer deleteOutage(t, client, "Prow", "Hook", hookOutage.ID)
			plankOutage := createOutageWithSeverity(t, client, "Prow", "Plank", string(types.SeverityDegraded))
			defer deleteOutage(t, client, "Prow", "Plank", plankOutage.ID)

			// Confirm the Tide outage to test most severe logic
			updateOutage(t, client, "Prow", "Tide", tideOutage.ID, map[string]interface{}{
				"confirmed": true,
			})

			status := getStatus(t, client, "Prow", "")

			// Should return Down (most severe), not Degraded
			assert.Equal(t, types.StatusDown, status.Status)
			assert.Len(t, status.ActiveOutages, 4)
		})

		t.Run("GET status for component with unconfirmed outages on one sub-component returns Partial", func(t *testing.T) {
			// Create unconfirmed outages for Tide (requires_confirmation: true)
			tideOutage := createOutage(t, client, "Prow", "Tide")
			defer deleteOutage(t, client, "Prow", "Tide", tideOutage.ID)

			status := getStatus(t, client, "Prow", "")

			assert.Equal(t, types.StatusPartial, status.Status)
			assert.Len(t, status.ActiveOutages, 1)
			assert.False(t, status.ActiveOutages[0].ConfirmedAt.Valid)
		})

		t.Run("GET status for component with mixed confirmed/unconfirmed outages shows confirmed severity", func(t *testing.T) {
			// Create outages for all sub-components (auto-confirmed for Deck, Hook, Plank)
			deckOutage := createOutage(t, client, "Prow", "Deck")
			defer deleteOutage(t, client, "Prow", "Deck", deckOutage.ID)
			hookOutage := createOutage(t, client, "Prow", "Hook")
			defer deleteOutage(t, client, "Prow", "Hook", hookOutage.ID)
			plankOutage := createOutage(t, client, "Prow", "Plank")
			defer deleteOutage(t, client, "Prow", "Plank", plankOutage.ID)

			// Create unconfirmed outage for Tide (requires_confirmation: true)
			tideOutage := createOutage(t, client, "Prow", "Tide")
			defer deleteOutage(t, client, "Prow", "Tide", tideOutage.ID)

			status := getStatus(t, client, "Prow", "")

			// Should return Down (confirmed) not Suspected (unconfirmed)
			assert.Equal(t, types.StatusDown, status.Status)
			assert.Len(t, status.ActiveOutages, 4)
		})

		t.Run("GET status for non-existent component returns 404", func(t *testing.T) {
			expect404(t, client, "/api/status/"+utils.Slugify("NonExistent"), false)
		})
	}
}

func createOutageWithSeverity(t *testing.T, client *TestHTTPClient, componentName, subComponentName, severity string) types.Outage {
	outagePayload := map[string]interface{}{
		"severity":        severity,
		"start_time":      time.Now().UTC().Format(time.RFC3339),
		"description":     fmt.Sprintf("Test outage with %s severity", severity),
		"discovered_from": "e2e-test",
		"created_by":      "developer",
	}

	payloadBytes, err := json.Marshal(outagePayload)
	require.NoError(t, err)

	resp, err := client.Post(fmt.Sprintf("/api/components/%s/%s/outages", utils.Slugify(componentName), utils.Slugify(subComponentName)), payloadBytes)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var outage types.Outage
	err = json.NewDecoder(resp.Body).Decode(&outage)
	require.NoError(t, err)

	// Verify that created_by is set to the user from X-Forwarded-User header
	assert.Equal(t, "developer", outage.CreatedBy, "created_by should be set to the user from X-Forwarded-User header")

	return outage
}
func testAllComponentsStatus(client *TestHTTPClient) func(*testing.T) {
	return func(t *testing.T) {
		t.Run("GET status for all components returns all components with their status", func(t *testing.T) {
			allStatuses := getAllComponentsStatus(t, client)

			// Prow, Downstream CI, Build Farm, Boskos, Sippy, and Errata Reliability
			assert.Len(t, allStatuses, 6)
			// Find Prow component
			var prowStatus *types.ComponentStatus
			var buildFarmStatus *types.ComponentStatus
			var boskosStatus *types.ComponentStatus
			for i := range allStatuses {
				if allStatuses[i].ComponentName == prowComponentName {
					prowStatus = &allStatuses[i]
				}
				if allStatuses[i].ComponentName == "Build Farm" {
					buildFarmStatus = &allStatuses[i]
				}
				if allStatuses[i].ComponentName == "Boskos" {
					boskosStatus = &allStatuses[i]
				}
			}
			require.NotNil(t, prowStatus, "Prow component should be present")
			require.NotNil(t, buildFarmStatus, "Build Farm component should be present")
			require.NotNil(t, boskosStatus, "Boskos component should be present")
			assert.Equal(t, types.StatusHealthy, prowStatus.Status)
			assert.Empty(t, prowStatus.ActiveOutages)
			assert.Equal(t, types.StatusHealthy, buildFarmStatus.Status)
			assert.Empty(t, buildFarmStatus.ActiveOutages)
			assert.Equal(t, types.StatusHealthy, boskosStatus.Status)
			assert.Empty(t, boskosStatus.ActiveOutages)
		})

		t.Run("GET status for all components with outages shows correct statuses", func(t *testing.T) {
			// Create outages for all sub-components with mixed severities
			tideOutage := createOutageWithSeverity(t, client, "Prow", "Tide", string(types.SeverityDegraded))
			defer deleteOutage(t, client, "Prow", "Tide", tideOutage.ID)
			deckOutage := createOutageWithSeverity(t, client, "Prow", "Deck", string(types.SeverityDown))
			defer deleteOutage(t, client, "Prow", "Deck", deckOutage.ID)
			hookOutage := createOutageWithSeverity(t, client, "Prow", "Hook", string(types.SeverityDegraded))
			defer deleteOutage(t, client, "Prow", "Hook", hookOutage.ID)
			plankOutage := createOutageWithSeverity(t, client, "Prow", "Plank", string(types.SeverityDown))
			defer deleteOutage(t, client, "Prow", "Plank", plankOutage.ID)

			// Confirm Tide outage (others should be auto-confirmed)
			updateOutage(t, client, "Prow", "Tide", tideOutage.ID, map[string]interface{}{
				"confirmed": true,
			})

			allStatuses := getAllComponentsStatus(t, client)

			// Prow, Downstream CI, Build Farm, Boskos, Sippy, and Errata Reliability
			assert.Len(t, allStatuses, 6)
			// Find Prow component
			var prowStatus *types.ComponentStatus
			for i := range allStatuses {
				if allStatuses[i].ComponentName == prowComponentName {
					prowStatus = &allStatuses[i]
					break
				}
			}
			require.NotNil(t, prowStatus, "Prow component should be present")
			assert.Equal(t, "Prow", prowStatus.ComponentName)
			assert.Equal(t, types.StatusDown, prowStatus.Status) // Most severe status
			assert.Len(t, prowStatus.ActiveOutages, 4)
		})

		t.Run("GET status for all components with partial outages shows Partial status", func(t *testing.T) {
			// Create outage for only one sub-component (Deck doesn't require confirmation)
			deckOutage := createOutageWithSeverity(t, client, "Prow", "Deck", string(types.SeverityDegraded))
			defer deleteOutage(t, client, "Prow", "Deck", deckOutage.ID)

			allStatuses := getAllComponentsStatus(t, client)

			assert.Len(t, allStatuses, 6)
			// Find Prow component
			var prowStatus *types.ComponentStatus
			for i := range allStatuses {
				if allStatuses[i].ComponentName == prowComponentName {
					prowStatus = &allStatuses[i]
					break
				}
			}
			require.NotNil(t, prowStatus, "Prow component should be present")
			assert.Equal(t, "Prow", prowStatus.ComponentName)
			assert.Equal(t, types.StatusPartial, prowStatus.Status) // Only one sub-component affected
			assert.Len(t, prowStatus.ActiveOutages, 1)
			assert.Equal(t, string(types.SeverityDegraded), string(prowStatus.ActiveOutages[0].Severity))
		})

		t.Run("GET status for all components with unconfirmed outages shows Partial", func(t *testing.T) {
			// Create unconfirmed outage for Tide (requires_confirmation: true)
			tideOutage := createOutage(t, client, "Prow", "Tide")
			defer deleteOutage(t, client, "Prow", "Tide", tideOutage.ID)

			allStatuses := getAllComponentsStatus(t, client)

			assert.Len(t, allStatuses, 6)
			// Find Prow component
			var prowStatus *types.ComponentStatus
			for i := range allStatuses {
				if allStatuses[i].ComponentName == prowComponentName {
					prowStatus = &allStatuses[i]
					break
				}
			}
			require.NotNil(t, prowStatus, "Prow component should be present")
			assert.Equal(t, "Prow", prowStatus.ComponentName)
			assert.Equal(t, types.StatusPartial, prowStatus.Status)
			assert.Len(t, prowStatus.ActiveOutages, 1)
			assert.False(t, prowStatus.ActiveOutages[0].ConfirmedAt.Valid)
		})

		t.Run("GET status for all components with mixed confirmed/unconfirmed outages shows confirmed severity", func(t *testing.T) {
			// Create outages for all sub-components (auto-confirmed for Deck, Hook, Plank)
			deckOutage := createOutage(t, client, "Prow", "Deck")
			defer deleteOutage(t, client, "Prow", "Deck", deckOutage.ID)
			hookOutage := createOutage(t, client, "Prow", "Hook")
			defer deleteOutage(t, client, "Prow", "Hook", hookOutage.ID)
			plankOutage := createOutage(t, client, "Prow", "Plank")
			defer deleteOutage(t, client, "Prow", "Plank", plankOutage.ID)

			// Create unconfirmed outage for Tide (requires_confirmation: true)
			tideOutage := createOutage(t, client, "Prow", "Tide")
			defer deleteOutage(t, client, "Prow", "Tide", tideOutage.ID)

			allStatuses := getAllComponentsStatus(t, client)

			assert.Len(t, allStatuses, 6)
			// Find Prow component
			var prowStatus *types.ComponentStatus
			for i := range allStatuses {
				if allStatuses[i].ComponentName == prowComponentName {
					prowStatus = &allStatuses[i]
					break
				}
			}
			require.NotNil(t, prowStatus, "Prow component should be present")
			assert.Equal(t, "Prow", prowStatus.ComponentName)
			// Should return Down (confirmed) not Suspected (unconfirmed)
			assert.Equal(t, types.StatusDown, prowStatus.Status)
			assert.Len(t, prowStatus.ActiveOutages, 4)
		})

		t.Run("GET status for all components includes last_ping_time when available", func(t *testing.T) {
			// Prow should already have a last_ping_time from the initial seed, just verify it
			allStatuses := getAllComponentsStatus(t, client)

			// Find Prow and Build Farm components
			var prowStatus *types.ComponentStatus
			var buildFarmStatus *types.ComponentStatus
			for i := range allStatuses {
				if allStatuses[i].ComponentName == prowComponentName {
					prowStatus = &allStatuses[i]
				}
				if allStatuses[i].ComponentName == "Build Farm" {
					buildFarmStatus = &allStatuses[i]
				}
			}
			require.NotNil(t, prowStatus, "Prow component should be present")
			require.NotNil(t, buildFarmStatus, "Build Farm component should be present")

			// Prow should have last_ping_time since Hook and Plank were pinged at test start
			assert.NotNil(t, prowStatus.LastPingTime, "Prow should have last_ping_time from seeded pings")

			// Build Farm should not have last_ping_time since no report was sent
			assert.Nil(t, buildFarmStatus.LastPingTime, "Build Farm should not have last_ping_time without reports")
		})
	}
}

func testListSubComponents(client *TestHTTPClient) func(*testing.T) {
	return func(t *testing.T) {
		t.Run("no filters returns all sub-components", func(t *testing.T) {
			subs := getSubComponents(t, client, "", "", "")
			// Prow 4 + Downstream CI 1 + Build Farm 2 + Boskos 2 + Sippy 5 + Errata Reliability 1
			assert.Len(t, subs, 15)
		})
		t.Run("componentName filter returns only that component's sub-components", func(t *testing.T) {
			subs := getSubComponents(t, client, "prow", "", "")
			assert.Len(t, subs, 4)
			names := make([]string, len(subs))
			for i := range subs {
				names[i] = subs[i].Name
			}
			assert.ElementsMatch(t, []string{"Tide", "Deck", "Hook", "Plank"}, names)
		})
		t.Run("non-matching componentName returns empty", func(t *testing.T) {
			subs := getSubComponents(t, client, "non-existent", "", "")
			assert.Empty(t, subs)
		})

		t.Run("team filter returns sub-components from components with that team", func(t *testing.T) {
			subs := getSubComponents(t, client, "", "", "TestPlatform")
			// Prow + Downstream CI both have ship_team TestPlatform
			assert.Len(t, subs, 5)
		})
		t.Run("non-matching team returns empty", func(t *testing.T) {
			subs := getSubComponents(t, client, "", "", "NonExistentTeam")
			assert.Empty(t, subs)
		})

		t.Run("tag filter returns sub-components from different components for same tag", func(t *testing.T) {
			subs := getSubComponents(t, client, "", "jobs", "")
			// tag "jobs": Prow (Plank), Downstream CI (Retester), Build Farm (Build01, Build02), Boskos (Quota, Leases)
			assert.Len(t, subs, 6)
			names := make([]string, len(subs))
			for i := range subs {
				names[i] = subs[i].Name
			}
			assert.ElementsMatch(t, []string{"Plank", "Retester", "Build01", "Build02", "Quota", "Leases"}, names)
		})
		t.Run("non-matching tag returns empty", func(t *testing.T) {
			subs := getSubComponents(t, client, "", "nonexistent-tag", "")
			assert.Empty(t, subs)
		})

		t.Run("tag and componentName together filter correctly", func(t *testing.T) {
			subs := getSubComponents(t, client, "prow", "ci", "")
			// Prow sub-components with tag "ci": Tide, Hook, Plank
			assert.Len(t, subs, 3)
			names := make([]string, len(subs))
			for i := range subs {
				names[i] = subs[i].Name
			}
			assert.ElementsMatch(t, []string{"Tide", "Hook", "Plank"}, names)
		})
		t.Run("tag and team together filter correctly", func(t *testing.T) {
			subs := getSubComponents(t, client, "", "ci", "TestPlatform")
			// TestPlatform components with tag "ci": Prow (Tide, Hook, Plank), Downstream CI (Retester)
			assert.Len(t, subs, 4)
			names := make([]string, len(subs))
			for i := range subs {
				names[i] = subs[i].Name
			}
			assert.ElementsMatch(t, []string{"Tide", "Hook", "Plank", "Retester"}, names)
		})

	}
}

func testTags(client *TestHTTPClient) func(*testing.T) {
	return func(t *testing.T) {
		resp, err := client.Get("/api/tags", false)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var tags []types.Tag
		err = json.NewDecoder(resp.Body).Decode(&tags)
		require.NoError(t, err)

		// E2E config defines 7 tags with proper capitalization
		assert.Len(t, tags, 7)

		// Check that each tag has required fields
		tagNames := make([]string, len(tags))
		for i, tag := range tags {
			assert.NotEmpty(t, tag.Name)
			assert.NotEmpty(t, tag.Description)
			assert.NotEmpty(t, tag.Color)
			tagNames[i] = tag.Name
		}

		// Verify expected tags are present (with proper capitalization)
		assert.ElementsMatch(t, []string{"CI", "PR-Merging", "Frontend", "GitHub", "Jobs", "AWS", "GCP"}, tagNames)
	}
}

func testUser(client *TestHTTPClient) func(*testing.T) {
	return func(t *testing.T) {
		t.Run("GET /api/user returns authenticated user", func(t *testing.T) {
			resp, err := client.Get("/api/user", true)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusOK, resp.StatusCode)
			assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

			var userResponse struct {
				Username   string   `json:"username"`
				Components []string `json:"components"`
			}
			err = json.NewDecoder(resp.Body).Decode(&userResponse)
			require.NoError(t, err)

			assert.Equal(t, "developer", userResponse.Username)
			// Components should be a slice (can be empty)
			assert.NotNil(t, userResponse.Components)
			assert.Contains(t, userResponse.Components, utils.Slugify("Prow"), "developer should have access to Prow")
			assert.Contains(t, userResponse.Components, utils.Slugify("Boskos"), "developer should have access to Boskos")
			assert.NotContains(t, userResponse.Components, utils.Slugify("Build Farm"), "developer should not have access to Build Farm")
		})
	}
}

func testComponentMonitorReport(client *TestHTTPClient) func(*testing.T) {
	return func(t *testing.T) {
		t.Run("POST report with Down status creates outage", func(t *testing.T) {
			reportPayload := types.ComponentMonitorReportRequest{
				ComponentMonitor: "app-ci-component-monitor",
				Statuses: []types.ComponentMonitorReportComponentStatus{
					{
						ComponentSlug:    utils.Slugify("Prow"),
						SubComponentSlug: utils.Slugify("Hook"),
						Status:           types.StatusDown,
						Reasons: []types.Reason{
							{
								Type:    "prometheus",
								Check:   "up{job=\"hook\"} == 0",
								Results: "No healthy instances found",
							},
						},
					},
				},
			}

			payloadBytes, err := json.Marshal(reportPayload)
			require.NoError(t, err)

			reportSentTime := time.Now()
			resp, err := client.PostWithBearerToken("/api/component-monitor/report", payloadBytes, componentMonitorSAToken)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusOK, resp.StatusCode)

			var response map[string]string
			err = json.NewDecoder(resp.Body).Decode(&response)
			require.NoError(t, err)
			assert.Equal(t, "processed", response["status"])

			// Verify outage was created
			outages := getOutages(t, client, "Prow", "Hook")
			var foundOutage *types.Outage
			for i := range outages {
				if outages[i].DiscoveredFrom == "component-monitor" && len(outages[i].Reasons) > 0 && outages[i].Reasons[0].Type == types.CheckTypePrometheus {
					foundOutage = &outages[i]
					break
				}
			}
			require.NotNil(t, foundOutage, "Outage should be created")
			assert.Equal(t, string(types.SeverityDown), string(foundOutage.Severity))
			assert.Equal(t, "component-monitor", foundOutage.DiscoveredFrom)
			assert.Equal(t, "app-ci-component-monitor", foundOutage.CreatedBy)
			require.Len(t, foundOutage.Reasons, 1)
			assert.Equal(t, types.CheckTypePrometheus, foundOutage.Reasons[0].Type)
			assert.Equal(t, "up{job=\"hook\"} == 0", foundOutage.Reasons[0].Check)
			assert.Equal(t, "No healthy instances found", foundOutage.Reasons[0].Results)

			// Verify that ping time was set
			status := getStatus(t, client, "Prow", "Hook")
			assert.NotNil(t, status.LastPingTime, "last_ping_time should be set after component monitor report")
			assert.WithinDuration(t, reportSentTime, *status.LastPingTime, 5*time.Second, "last_ping_time should be within 5 seconds of when report was sent")

			// Cleanup
			deleteOutage(t, client, "Prow", "Hook", foundOutage.ID)
		})

		t.Run("POST report with Degraded status creates outage", func(t *testing.T) {
			reportPayload := types.ComponentMonitorReportRequest{
				ComponentMonitor: "app-ci-component-monitor",
				Statuses: []types.ComponentMonitorReportComponentStatus{
					{
						ComponentSlug:    utils.Slugify("Prow"),
						SubComponentSlug: utils.Slugify("Hook"),
						Status:           types.StatusDegraded,
						Reasons: []types.Reason{
							{
								Type:    "http",
								Check:   "https://hook.example.com/health",
								Results: "Response time > 5s",
							},
						},
					},
				},
			}

			payloadBytes, err := json.Marshal(reportPayload)
			require.NoError(t, err)

			resp, err := client.PostWithBearerToken("/api/component-monitor/report", payloadBytes, componentMonitorSAToken)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusOK, resp.StatusCode)

			// Verify outage was created
			outages := getOutages(t, client, "Prow", "Hook")
			var foundOutage *types.Outage
			for i := range outages {
				if outages[i].DiscoveredFrom == "component-monitor" && len(outages[i].Reasons) > 0 && outages[i].Reasons[0].Type == types.CheckTypeHTTP {
					foundOutage = &outages[i]
					break
				}
			}
			require.NotNil(t, foundOutage, "Outage should be created")
			assert.Equal(t, string(types.SeverityDegraded), string(foundOutage.Severity))

			// Cleanup
			deleteOutage(t, client, "Prow", "Hook", foundOutage.ID)
		})

		t.Run("POST report does not create duplicate outage for same Reason.Type", func(t *testing.T) {
			reportPayload := types.ComponentMonitorReportRequest{
				ComponentMonitor: "app-ci-component-monitor",
				Statuses: []types.ComponentMonitorReportComponentStatus{
					{
						ComponentSlug:    utils.Slugify("Prow"),
						SubComponentSlug: utils.Slugify("Hook"),
						Status:           types.StatusDown,
						Reasons: []types.Reason{
							{
								Type:    "prometheus",
								Check:   "up{job=\"hook\"} == 0",
								Results: "No healthy instances found",
							},
						},
					},
				},
			}

			payloadBytes, err := json.Marshal(reportPayload)
			require.NoError(t, err)

			// First report
			resp1, err := client.PostWithBearerToken("/api/component-monitor/report", payloadBytes, componentMonitorSAToken)
			require.NoError(t, err)
			resp1.Body.Close()
			assert.Equal(t, http.StatusOK, resp1.StatusCode)

			// Get the created outage
			outages1 := getOutages(t, client, "Prow", "Hook")
			var firstOutage *types.Outage
			for i := range outages1 {
				if outages1[i].DiscoveredFrom == "component-monitor" && len(outages1[i].Reasons) > 0 && outages1[i].Reasons[0].Type == "prometheus" {
					firstOutage = &outages1[i]
					break
				}
			}
			require.NotNil(t, firstOutage, "First outage should be created")

			// Second report with same Reason.Type
			resp2, err := client.PostWithBearerToken("/api/component-monitor/report", payloadBytes, componentMonitorSAToken)
			require.NoError(t, err)
			resp2.Body.Close()
			assert.Equal(t, http.StatusOK, resp2.StatusCode)

			// Verify no duplicate was created
			outages2 := getOutages(t, client, "Prow", "Hook")
			count := 0
			for i := range outages2 {
				if outages2[i].DiscoveredFrom == "component-monitor" && len(outages2[i].Reasons) > 0 && outages2[i].Reasons[0].Type == "prometheus" && outages2[i].EndTime.Valid == false {
					count++
				}
			}
			assert.Equal(t, 1, count, "Should only have one active outage created by the same component-monitor")

			// Cleanup
			deleteOutage(t, client, "Prow", "Hook", firstOutage.ID)
		})

		t.Run("POST report with Healthy status auto-resolves outage when auto_resolve is true", func(t *testing.T) {
			// Create an outage first
			downReport := types.ComponentMonitorReportRequest{
				ComponentMonitor: "app-ci-component-monitor",
				Statuses: []types.ComponentMonitorReportComponentStatus{
					{
						ComponentSlug:    utils.Slugify("Prow"),
						SubComponentSlug: utils.Slugify("Hook"), // Hook has auto_resolve: true
						Status:           types.StatusDown,
						Reasons: []types.Reason{
							{
								Type:    "prometheus",
								Check:   "up{job=\"hook\"} == 0",
								Results: "No healthy instances found",
							},
						},
					},
				},
			}

			downBytes, err := json.Marshal(downReport)
			require.NoError(t, err)

			resp, err := client.PostWithBearerToken("/api/component-monitor/report", downBytes, componentMonitorSAToken)
			require.NoError(t, err)
			resp.Body.Close()
			assert.Equal(t, http.StatusOK, resp.StatusCode)

			// Get the created outage
			outages := getOutages(t, client, "Prow", "Hook")
			var outage *types.Outage
			for i := range outages {
				if outages[i].DiscoveredFrom == "component-monitor" && len(outages[i].Reasons) > 0 && outages[i].Reasons[0].Type == "prometheus" && !outages[i].EndTime.Valid {
					outage = &outages[i]
					break
				}
			}
			require.NotNil(t, outage, "Outage should be created")
			assert.False(t, outage.EndTime.Valid, "Outage should be active")

			// Now report healthy status
			healthyReport := types.ComponentMonitorReportRequest{
				ComponentMonitor: "app-ci-component-monitor",
				Statuses: []types.ComponentMonitorReportComponentStatus{
					{
						ComponentSlug:    utils.Slugify("Prow"),
						SubComponentSlug: utils.Slugify("Hook"),
						Status:           types.StatusHealthy,
						Reasons: []types.Reason{
							{
								Type:    "prometheus",
								Check:   "up{job=\"hook\"} == 0",
								Results: "All instances healthy",
							},
						},
					},
				},
			}

			healthyBytes, err := json.Marshal(healthyReport)
			require.NoError(t, err)

			reportSentTime := time.Now()
			resp2, err := client.PostWithBearerToken("/api/component-monitor/report", healthyBytes, componentMonitorSAToken)
			require.NoError(t, err)
			resp2.Body.Close()
			assert.Equal(t, http.StatusOK, resp2.StatusCode)

			// Verify outage was resolved
			outages2 := getOutages(t, client, "Prow", "Hook")
			var resolvedOutage *types.Outage
			for i := range outages2 {
				if outages2[i].ID == outage.ID {
					resolvedOutage = &outages2[i]
					break
				}
			}
			require.NotNil(t, resolvedOutage, "Outage should still exist")
			assert.True(t, resolvedOutage.EndTime.Valid, "Outage should be resolved")

			// Verify that ping time was updated
			status := getStatus(t, client, "Prow", "Hook")
			assert.NotNil(t, status.LastPingTime, "last_ping_time should be set after component monitor report")
			assert.WithinDuration(t, reportSentTime, *status.LastPingTime, 5*time.Second, "last_ping_time should be within 5 seconds of when report was sent")
		})

		t.Run("POST report with Healthy status does not resolve when auto_resolve is false", func(t *testing.T) {
			// Create an outage first for Plank (which has auto_resolve: false)
			downReport := types.ComponentMonitorReportRequest{
				ComponentMonitor: "app-ci-component-monitor",
				Statuses: []types.ComponentMonitorReportComponentStatus{
					{
						ComponentSlug:    utils.Slugify("Prow"),
						SubComponentSlug: utils.Slugify("Plank"), // Plank has auto_resolve: false
						Status:           types.StatusDown,
						Reasons: []types.Reason{
							{
								Type:    "prometheus",
								Check:   "up{job=\"plank\"} == 0",
								Results: "No healthy instances found",
							},
						},
					},
				},
			}

			downBytes, err := json.Marshal(downReport)
			require.NoError(t, err)

			resp, err := client.PostWithBearerToken("/api/component-monitor/report", downBytes, componentMonitorSAToken)
			require.NoError(t, err)
			resp.Body.Close()
			assert.Equal(t, http.StatusOK, resp.StatusCode)

			// Get the created outage
			outages := getOutages(t, client, "Prow", "Plank")
			var outage *types.Outage
			for i := range outages {
				if outages[i].DiscoveredFrom == "component-monitor" && len(outages[i].Reasons) > 0 && outages[i].Reasons[0].Type == "prometheus" && !outages[i].EndTime.Valid {
					outage = &outages[i]
					break
				}
			}
			require.NotNil(t, outage, "Outage should be created")

			// Now report healthy status
			healthyReport := types.ComponentMonitorReportRequest{
				ComponentMonitor: "app-ci-component-monitor",
				Statuses: []types.ComponentMonitorReportComponentStatus{
					{
						ComponentSlug:    utils.Slugify("Prow"),
						SubComponentSlug: utils.Slugify("Plank"),
						Status:           types.StatusHealthy,
						Reasons: []types.Reason{
							{
								Type:    "prometheus",
								Check:   "up{job=\"plank\"} == 0",
								Results: "All instances healthy",
							},
						},
					},
				},
			}

			healthyBytes, err := json.Marshal(healthyReport)
			require.NoError(t, err)

			resp2, err := client.PostWithBearerToken("/api/component-monitor/report", healthyBytes, componentMonitorSAToken)
			require.NoError(t, err)
			resp2.Body.Close()
			assert.Equal(t, http.StatusOK, resp2.StatusCode)

			// Verify outage was NOT resolved
			outages2 := getOutages(t, client, "Prow", "Plank")
			var stillActiveOutage *types.Outage
			for i := range outages2 {
				if outages2[i].ID == outage.ID {
					stillActiveOutage = &outages2[i]
					break
				}
			}
			require.NotNil(t, stillActiveOutage, "Outage should still exist")
			assert.False(t, stillActiveOutage.EndTime.Valid, "Outage should still be active")

			// Cleanup
			deleteOutage(t, client, "Prow", "Plank", outage.ID)
		})

		t.Run("POST report with invalid component returns 400", func(t *testing.T) {
			reportPayload := types.ComponentMonitorReportRequest{
				ComponentMonitor: "app-ci-component-monitor",
				Statuses: []types.ComponentMonitorReportComponentStatus{
					{
						ComponentSlug:    utils.Slugify("NonExistentComponent"),
						SubComponentSlug: utils.Slugify("Deck"),
						Status:           types.StatusDown,
						Reasons: []types.Reason{
							{
								Type:    "prometheus",
								Check:   "up{job=\"deck\"} == 0",
								Results: "No healthy instances found",
							},
						},
					},
				},
			}

			payloadBytes, err := json.Marshal(reportPayload)
			require.NoError(t, err)

			resp, err := client.PostWithBearerToken("/api/component-monitor/report", payloadBytes, componentMonitorSAToken)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

			var errorResponse map[string]string
			err = json.NewDecoder(resp.Body).Decode(&errorResponse)
			require.NoError(t, err)
			assert.Contains(t, errorResponse["error"], "Component not found")
		})

		t.Run("POST report with invalid sub-component returns 400", func(t *testing.T) {
			reportPayload := types.ComponentMonitorReportRequest{
				ComponentMonitor: "app-ci-component-monitor",
				Statuses: []types.ComponentMonitorReportComponentStatus{
					{
						ComponentSlug:    utils.Slugify("Prow"),
						SubComponentSlug: utils.Slugify("NonExistentSub"),
						Status:           types.StatusDown,
						Reasons: []types.Reason{
							{
								Type:    "prometheus",
								Check:   "up{job=\"deck\"} == 0",
								Results: "No healthy instances found",
							},
						},
					},
				},
			}

			payloadBytes, err := json.Marshal(reportPayload)
			require.NoError(t, err)

			resp, err := client.PostWithBearerToken("/api/component-monitor/report", payloadBytes, componentMonitorSAToken)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

			var errorResponse map[string]string
			err = json.NewDecoder(resp.Body).Decode(&errorResponse)
			require.NoError(t, err)
			assert.Contains(t, errorResponse["error"], "Sub-component not found")
		})

		t.Run("POST report with multiple statuses processes all", func(t *testing.T) {
			reportPayload := types.ComponentMonitorReportRequest{
				ComponentMonitor: "app-ci-component-monitor",
				Statuses: []types.ComponentMonitorReportComponentStatus{
					{
						ComponentSlug:    utils.Slugify("Prow"),
						SubComponentSlug: utils.Slugify("Hook"),
						Status:           types.StatusDown,
						Reasons: []types.Reason{
							{
								Type:    "prometheus",
								Check:   "up{job=\"hook\"} == 0",
								Results: "No healthy instances found",
							},
						},
					},
					{
						ComponentSlug:    utils.Slugify("Prow"),
						SubComponentSlug: utils.Slugify("Plank"),
						Status:           types.StatusDegraded,
						Reasons: []types.Reason{
							{
								Type:    "http",
								Check:   "https://plank.example.com/health",
								Results: "Response time > 5s",
							},
						},
					},
				},
			}

			payloadBytes, err := json.Marshal(reportPayload)
			require.NoError(t, err)

			resp, err := client.PostWithBearerToken("/api/component-monitor/report", payloadBytes, componentMonitorSAToken)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusOK, resp.StatusCode)

			// Verify both outages were created
			hookOutages := getOutages(t, client, "Prow", "Hook")
			var hookOutage *types.Outage
			for i := range hookOutages {
				if hookOutages[i].DiscoveredFrom == "component-monitor" && len(hookOutages[i].Reasons) > 0 && hookOutages[i].Reasons[0].Type == "prometheus" {
					hookOutage = &hookOutages[i]
					break
				}
			}
			require.NotNil(t, hookOutage, "Hook outage should be created")

			plankOutages := getOutages(t, client, "Prow", "Plank")
			var plankOutage *types.Outage
			for i := range plankOutages {
				if plankOutages[i].DiscoveredFrom == "component-monitor" && len(plankOutages[i].Reasons) > 0 && plankOutages[i].Reasons[0].Type == "http" {
					plankOutage = &plankOutages[i]
					break
				}
			}
			require.NotNil(t, plankOutage, "Plank outage should be created")

			// Cleanup
			deleteOutage(t, client, "Prow", "Hook", hookOutage.ID)
			deleteOutage(t, client, "Prow", "Plank", plankOutage.ID)
		})

		t.Run("POST report with empty component_monitor returns 400", func(t *testing.T) {
			reportPayload := types.ComponentMonitorReportRequest{
				ComponentMonitor: "",
				Statuses: []types.ComponentMonitorReportComponentStatus{
					{
						ComponentSlug:    utils.Slugify("Prow"),
						SubComponentSlug: utils.Slugify("Hook"),
						Status:           types.StatusDown,
						Reasons: []types.Reason{
							{
								Type:    "prometheus",
								Check:   "up{job=\"hook\"} == 0",
								Results: "No healthy instances found",
							},
						},
					},
				},
			}

			payloadBytes, err := json.Marshal(reportPayload)
			require.NoError(t, err)

			resp, err := client.PostWithBearerToken("/api/component-monitor/report", payloadBytes, componentMonitorSAToken)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

			var errorResponse map[string]string
			err = json.NewDecoder(resp.Body).Decode(&errorResponse)
			require.NoError(t, err)
			assert.Contains(t, errorResponse["error"], "component_monitor is required")
		})

		t.Run("POST report with empty statuses returns 400", func(t *testing.T) {
			reportPayload := types.ComponentMonitorReportRequest{
				ComponentMonitor: "app-ci-component-monitor",
				Statuses:         []types.ComponentMonitorReportComponentStatus{},
			}

			payloadBytes, err := json.Marshal(reportPayload)
			require.NoError(t, err)

			resp, err := client.PostWithBearerToken("/api/component-monitor/report", payloadBytes, componentMonitorSAToken)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

			var errorResponse map[string]string
			err = json.NewDecoder(resp.Body).Decode(&errorResponse)
			require.NoError(t, err)
			assert.Contains(t, errorResponse["error"], "statuses cannot be empty")
		})

		t.Run("POST report with invalid token returns 401", func(t *testing.T) {
			reportPayload := types.ComponentMonitorReportRequest{
				ComponentMonitor: "app-ci-component-monitor",
				Statuses: []types.ComponentMonitorReportComponentStatus{
					{
						ComponentSlug:    utils.Slugify("Prow"),
						SubComponentSlug: utils.Slugify("Hook"),
						Status:           types.StatusDown,
						Reasons: []types.Reason{
							{
								Type:    "prometheus",
								Check:   "up{job=\"hook\"} == 0",
								Results: "No healthy instances found",
							},
						},
					},
				},
			}

			payloadBytes, err := json.Marshal(reportPayload)
			require.NoError(t, err)

			invalidToken := "invalid-token"
			resp, err := client.PostWithBearerToken("/api/component-monitor/report", payloadBytes, invalidToken)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		})

		t.Run("POST report with service account not an owner returns 400", func(t *testing.T) {
			// Build Farm component does not have the service account as an owner
			reportPayload := types.ComponentMonitorReportRequest{
				ComponentMonitor: "app-ci-component-monitor",
				Statuses: []types.ComponentMonitorReportComponentStatus{
					{
						ComponentSlug:    utils.Slugify("Build Farm"),
						SubComponentSlug: utils.Slugify("Build01"),
						Status:           types.StatusDown,
						Reasons: []types.Reason{
							{
								Type:    "prometheus",
								Check:   "up{job=\"build01\"} == 0",
								Results: "No healthy instances found",
							},
						},
					},
				},
			}

			payloadBytes, err := json.Marshal(reportPayload)
			require.NoError(t, err)

			resp, err := client.PostWithBearerToken("/api/component-monitor/report", payloadBytes, componentMonitorSAToken)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

			var errorResponse map[string]string
			err = json.NewDecoder(resp.Body).Decode(&errorResponse)
			require.NoError(t, err)
			assert.Equal(t, "Invalid request", errorResponse["error"])
		})

		t.Run("POST report with wrong component monitor instance returns 400", func(t *testing.T) {
			// Prow/Hook is configured for "app-ci-component-monitor", not "wrong-monitor"
			reportPayload := types.ComponentMonitorReportRequest{
				ComponentMonitor: "wrong-monitor",
				Statuses: []types.ComponentMonitorReportComponentStatus{
					{
						ComponentSlug:    utils.Slugify("Prow"),
						SubComponentSlug: utils.Slugify("Hook"),
						Status:           types.StatusDown,
						Reasons: []types.Reason{
							{
								Type:    "prometheus",
								Check:   "up{job=\"hook\"} == 0",
								Results: "No healthy instances found",
							},
						},
					},
				},
			}

			payloadBytes, err := json.Marshal(reportPayload)
			require.NoError(t, err)

			resp, err := client.PostWithBearerToken("/api/component-monitor/report", payloadBytes, componentMonitorSAToken)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

			var errorResponse map[string]string
			err = json.NewDecoder(resp.Body).Decode(&errorResponse)
			require.NoError(t, err)
			assert.Equal(t, "Invalid request", errorResponse["error"])
		})
	}
}

func testAbsentReport(client *TestHTTPClient) func(*testing.T) {
	return func(t *testing.T) {
		t.Run("Absent report checker creates outage when no ping exists, and resolves when ping is received", func(t *testing.T) {
			var outage *types.Outage
			ctx := context.Background()
			err := wait.PollUntilContextTimeout(ctx, 500*time.Millisecond, 20*time.Second, true, func(ctx context.Context) (bool, error) {
				outages := getOutages(t, client, "Downstream CI", "Retester")
				for i := range outages {
					if outages[i].DiscoveredFrom == "absent-monitored-component-report" && !outages[i].EndTime.Valid {
						outage = &outages[i]
						return true, nil
					}
				}
				return false, nil
			})

			require.NoError(t, err, "Absent report outage should be created within 20 seconds")
			require.NotNil(t, outage, "Absent report outage should exist")
			assert.Equal(t, "downstream-ci", outage.ComponentName)
			assert.Equal(t, "retester", outage.SubComponentName)
			assert.Equal(t, string(types.SeverityDown), string(outage.Severity))
			assert.Equal(t, "absent-monitored-component-report", outage.DiscoveredFrom)
			assert.Equal(t, "dashboard", outage.CreatedBy)
			assert.Contains(t, outage.Description, "Component-monitor has not reported status within expected time")
			assert.True(t, outage.ConfirmedAt.Valid, "Outage should be auto-confirmed since component doesn't require confirmation")

			// Verify the component status reflects the outage
			status := getStatus(t, client, "Downstream CI", "Retester")
			assert.Equal(t, types.StatusDown, status.Status)
			assert.Len(t, status.ActiveOutages, 1)

			// Store outage ID for later verification
			outageID := outage.ID

			// Send a healthy report to create a ping and trigger auto-resolve
			reportPayload := types.ComponentMonitorReportRequest{
				ComponentMonitor: "app-ci-component-monitor",
				Statuses: []types.ComponentMonitorReportComponentStatus{
					{
						ComponentSlug:    utils.Slugify("Downstream CI"),
						SubComponentSlug: utils.Slugify("Retester"),
						Status:           types.StatusHealthy,
						Reasons:          []types.Reason{{Type: types.CheckTypePrometheus}},
					},
				},
			}

			payloadBytes, err := json.Marshal(reportPayload)
			require.NoError(t, err)

			reportSentTime := time.Now()
			resp, err := client.PostWithBearerToken("/api/component-monitor/report", payloadBytes, componentMonitorSAToken)
			require.NoError(t, err)
			defer resp.Body.Close()
			assert.Equal(t, http.StatusOK, resp.StatusCode)

			// Wait for the absent report checker to run and auto-resolve the outage
			// The checker runs every 15s in e2e tests, so wait up to 20s
			var resolvedOutage *types.Outage
			err = wait.PollUntilContextTimeout(ctx, 500*time.Millisecond, 20*time.Second, true, func(ctx context.Context) (bool, error) {
				updatedOutages := getOutages(t, client, "Downstream CI", "Retester")
				for i := range updatedOutages {
					if updatedOutages[i].ID == outageID {
						resolvedOutage = &updatedOutages[i]
						if resolvedOutage.EndTime.Valid {
							return true, nil
						}
						break
					}
				}
				return false, nil
			})

			require.NoError(t, err, "Outage should be resolved within 20 seconds")
			require.NotNil(t, resolvedOutage, "Outage should still exist after resolution")
			assert.True(t, resolvedOutage.EndTime.Valid, "Outage should be resolved")

			// Verify the component status is now healthy
			updatedStatus := getStatus(t, client, "Downstream CI", "Retester")
			assert.Equal(t, types.StatusHealthy, updatedStatus.Status)
			assert.Len(t, updatedStatus.ActiveOutages, 0)
			assert.NotNil(t, updatedStatus.LastPingTime, "Should have a last ping time after report")
			assert.WithinDuration(t, reportSentTime, *updatedStatus.LastPingTime, 5*time.Second, "last_ping_time should be within 5 seconds of when report was sent")
		})
	}
}

// Prepares the test data by ensuring no outages exist for absent pings from prior to the tests starting
func cleanupAbsentReportOutages(t *testing.T, client *TestHTTPClient) {
	// Seed pings for monitored sub-components to prevent absent report checker from creating outages
	sendAllClearPing(t, client, "Prow", "Hook")
	sendAllClearPing(t, client, "Prow", "Plank")

	// Clean up any outages created by absent report checker before the pings were seeded
	deleteOutagesFromAbsentReport(t, client, "Prow", "Hook")
	deleteOutagesFromAbsentReport(t, client, "Prow", "Plank")
}

// sendAllClearPing sends a healthy component monitor report to seed a ping in the database,
// preventing the absent report checker from creating an immediate outage.
func sendAllClearPing(t *testing.T, client *TestHTTPClient, componentName, subComponentName string) {
	reportPayload := types.ComponentMonitorReportRequest{
		ComponentMonitor: "app-ci-component-monitor",
		Statuses: []types.ComponentMonitorReportComponentStatus{
			{
				ComponentSlug:    utils.Slugify(componentName),
				SubComponentSlug: utils.Slugify(subComponentName),
				Status:           types.StatusHealthy,
				Reasons:          []types.Reason{{Type: types.CheckTypePrometheus}},
			},
		},
	}

	payloadBytes, err := json.Marshal(reportPayload)
	require.NoError(t, err)

	resp, err := client.PostWithBearerToken("/api/component-monitor/report", payloadBytes, componentMonitorSAToken)
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode, "Failed to send all-clear ping for %s/%s", componentName, subComponentName)
}

// cleanupAbsentReportOutages deletes any outages created by the absent report checker for a given component.
func deleteOutagesFromAbsentReport(t *testing.T, client *TestHTTPClient, componentName, subComponentName string) {
	outages := getOutages(t, client, componentName, subComponentName)
	for _, outage := range outages {
		if outage.DiscoveredFrom == "absent-monitored-component-report" {
			deleteOutage(t, client, componentName, subComponentName, outage.ID)
		}
	}
}

func updateDashboardConfig(t *testing.T, modifier func(*types.DashboardConfig)) {
	configPath := os.Getenv("TEST_DASHBOARD_CONFIG_PATH")
	require.NotEmpty(t, configPath, "TEST_DASHBOARD_CONFIG_PATH must be set")
	modifyConfig(t, configPath, modifier)
}

func testConfigHotReload(client *TestHTTPClient) func(*testing.T) {
	return func(t *testing.T) {
		configPath := os.Getenv("TEST_DASHBOARD_CONFIG_PATH")
		require.NotEmpty(t, configPath, "TEST_DASHBOARD_CONFIG_PATH must be set")

		originalConfig := readConfig(t, configPath)

		defer func() {
			restoreConfig(t, configPath, originalConfig)
		}()

		t.Run("Config changes are reflected after reload", func(t *testing.T) {
			updateDashboardConfig(t, func(config *types.DashboardConfig) {
				// Update existing component description
				var prowFound bool
				for _, comp := range config.Components {
					if comp.Name == "Prow" {
						comp.Description = "Updated description for hot-reload test"
						prowFound = true
						break
					}
				}
				require.True(t, prowFound, "Prow component should exist in config")

				// Add new component
				newComponent := &types.Component{
					Name:        "Test Component",
					Description: "A test component for hot-reload",
					ShipTeam:    "TestTeam",
					SlackReporting: []types.SlackReportingConfig{
						{Channel: "#test-channel", Severity: &[]types.Severity{types.SeverityDown}[0]},
					},
					Subcomponents: []types.SubComponent{
						{
							Name:        "TestSub",
							Description: "Test sub-component",
						},
					},
					Owners: []types.Owner{
						{
							User: "developer",
						},
					},
				}
				config.Components = append(config.Components, newComponent)
			})

			// Wait for config to reload and verify both changes
			ctx := context.Background()
			err := wait.PollUntilContextTimeout(ctx, 200*time.Millisecond, 20*time.Second, true, func(ctx context.Context) (bool, error) {
				// Check that Prow description was updated
				prowComponent := getComponent(t, client, "Prow")
				if prowComponent.Description != "Updated description for hot-reload test" {
					return false, nil
				}

				// Check that new component appears
				components := getComponents(t, client)
				var testComponentFound bool
				for _, comp := range components {
					if comp.Name == "Test Component" {
						testComponentFound = true
						break
					}
				}
				return testComponentFound, nil
			})
			require.NoError(t, err, "Config changes should be reflected within 20 seconds")

			// Verify Prow description change
			prowComponent := getComponent(t, client, "Prow")
			assert.Equal(t, "Updated description for hot-reload test", prowComponent.Description)

			// Verify the new component exists
			testComponent := getComponent(t, client, "Test Component")
			assert.Equal(t, "Test Component", testComponent.Name)
			assert.Equal(t, "A test component for hot-reload", testComponent.Description)
			assert.Len(t, testComponent.Subcomponents, 1)
			assert.Equal(t, "TestSub", testComponent.Subcomponents[0].Name)
		})

		t.Run("removing sub-component with active outage resolves orphan and component becomes Healthy", func(t *testing.T) {
			createOutage(t, client, "Boskos", "Leases")

			statusBefore := getStatus(t, client, "Boskos", "")
			assert.Equal(t, types.StatusPartial, statusBefore.Status, "expected Partial when one of two sub-components has an active outage")

			updateDashboardConfig(t, func(config *types.DashboardConfig) {
				var boskosFound bool
				for _, comp := range config.Components {
					if comp.Name != "Boskos" {
						continue
					}
					boskosFound = true
					var kept []types.SubComponent
					for _, sub := range comp.Subcomponents {
						if sub.Name != "Leases" {
							kept = append(kept, sub)
						}
					}
					comp.Subcomponents = kept
					break
				}
				require.True(t, boskosFound, "Boskos component should exist in config")
			})

			ctx := context.Background()
			err := wait.PollUntilContextTimeout(ctx, 500*time.Millisecond, 90*time.Second, true, func(ctx context.Context) (bool, error) {
				st, ok := tryGetComponentStatus(client, "Boskos")
				if !ok {
					return false, nil
				}
				return st == types.StatusHealthy, nil
			})
			require.NoError(t, err, "Boskos should become Healthy after removed sub-component's outage is resolved")

			finalStatus := getStatus(t, client, "Boskos", "")
			assert.Equal(t, types.StatusHealthy, finalStatus.Status)
			assert.Empty(t, finalStatus.ActiveOutages)
		})
	}
}
