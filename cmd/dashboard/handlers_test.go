package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ship-status-dash/pkg/auth"
	"ship-status-dash/pkg/config"
	"ship-status-dash/pkg/outage"
	"ship-status-dash/pkg/repositories"
	"ship-status-dash/pkg/types"
)

// newTestHandlers returns Handlers backed by cfg, the given outage manager, mock pings, and an empty group cache.
func newTestHandlers(t *testing.T, cfg *types.DashboardConfig, om outage.OutageManager) *Handlers {
	t.Helper()
	cfgManager, err := config.NewManager("", func(string) (*types.DashboardConfig, error) {
		return cfg, nil
	}, logrus.New(), time.Second)
	require.NoError(t, err)
	cfgManager.Get()

	pingRepo := &repositories.MockComponentPingRepository{}
	cache := auth.NewGroupMembershipCache(logrus.New())
	return NewHandlers(logrus.New(), cfgManager, om, pingRepo, cache)
}

// minimalDashboardConfig is a tiny valid config (one component, one sub-component) for handler tests.
func minimalDashboardConfig() *types.DashboardConfig {
	return &types.DashboardConfig{
		Components: []*types.Component{
			{
				Name: "Alpha", Slug: "alpha", ShipTeam: "team-a",
				Subcomponents: []types.SubComponent{
					{Name: "One", Slug: "one"},
				},
			},
		},
	}
}

func TestGetOutagesDuringJSON(t *testing.T) {
	cfg := minimalDashboardConfig()
	t0 := time.Date(2025, 4, 1, 10, 0, 0, 0, time.UTC)
	t1 := t0.Add(time.Hour)
	t2 := t0.Add(2 * time.Hour)

	mockOM := &outage.MockOutageManager{}
	mockOM.GetOutagesDuringFn = func(queryStart, queryEnd time.Time, refs []types.SubComponentRef) ([]types.Outage, error) {
		if len(refs) == 0 {
			return []types.Outage{}, nil
		}
		if len(refs) == 1 && refs[0].ComponentSlug == "alpha" && refs[0].SubSlug == "one" &&
			queryStart.Equal(t1) && queryEnd.Equal(t1) {
			return []types.Outage{{
				ComponentName:    "alpha",
				SubComponentName: "one",
				Severity:         types.SeverityDown,
				StartTime:        t0,
				Description:      "x",
				DiscoveredFrom:   "test",
				CreatedBy:        "u",
			}}, nil
		}
		return []types.Outage{}, nil
	}

	h := newTestHandlers(t, cfg, mockOM)

	intPtr := func(n int) *int { return &n }

	tests := []struct {
		name            string
		query           string
		wantCode        int
		wantOutageCount *int
	}{
		{
			name:            "200_with_start_only",
			query:           "start=" + t1.UTC().Format(time.RFC3339),
			wantCode:        http.StatusOK,
			wantOutageCount: intPtr(1),
		},
		{
			name:     "400_no_time_params",
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "400_sub_without_component",
			query:    "start=" + t0.Format(time.RFC3339) + "&subComponentName=one",
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "400_start_after_end",
			query:    "start=" + t2.Format(time.RFC3339) + "&end=" + t0.Format(time.RFC3339),
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "404_unknown_component",
			query:    "start=" + t0.Format(time.RFC3339) + "&componentName=nope",
			wantCode: http.StatusNotFound,
		},
		{
			name:     "404_unknown_sub",
			query:    "start=" + t0.Format(time.RFC3339) + "&componentName=alpha&subComponentName=nope",
			wantCode: http.StatusNotFound,
		},
		{
			name:            "200_empty_when_tag_excludes",
			query:           "start=" + t1.Format(time.RFC3339) + "&tag=nonexistent-tag",
			wantCode:        http.StatusOK,
			wantOutageCount: intPtr(0),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := "/api/outages/during"
			if tt.query != "" {
				path += "?" + tt.query
			}
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()
			h.GetOutagesDuringJSON(rec, req)
			res := rec.Result()
			defer res.Body.Close()

			assert.Equal(t, tt.wantCode, res.StatusCode)
			if tt.wantOutageCount == nil {
				return
			}
			var got []types.Outage
			require.NoError(t, json.NewDecoder(res.Body).Decode(&got))
			assert.Len(t, got, *tt.wantOutageCount)
		})
	}
}
