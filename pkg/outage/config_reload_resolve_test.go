package outage

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ship-status-dash/pkg/types"

	"gorm.io/gorm"
)

func TestResolveActiveOutagesForMissingSubComponents(t *testing.T) {
	start := time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC)

	type compSub struct {
		component, sub string
	}

	tests := []struct {
		name                   string
		managerCfg             *types.DashboardConfig
		resolveCfg             *types.DashboardConfig
		seed                   func(t *testing.T, db *gorm.DB)
		wantStillActive        []compSub
		wantResolved           []compSub
		wantAuditUserOnResolve string
	}{
		{
			name: "orphan resolved and configured stays active",
			managerCfg: &types.DashboardConfig{
				Components: []*types.Component{
					{
						Slug: "build-farm",
						Subcomponents: []types.SubComponent{
							{Slug: "build01"},
							{Slug: "build03"},
						},
					},
				},
			},
			resolveCfg: &types.DashboardConfig{
				Components: []*types.Component{
					{
						Slug: "build-farm",
						Subcomponents: []types.SubComponent{
							{Slug: "build01"},
							{Slug: "build03"},
						},
					},
				},
			},
			seed: func(t *testing.T, db *gorm.DB) {
				t.Helper()
				ctx := context.WithValue(context.Background(), types.CurrentUserKey, "test-user")
				for _, sub := range []string{"build01", "build12"} {
					o := &types.Outage{
						ComponentName:    "build-farm",
						SubComponentName: sub,
						Severity:         types.SeverityDown,
						StartTime:        start,
						Description:      sub,
						CreatedBy:        "system",
						DiscoveredFrom:   "component-monitor",
					}
					require.NoError(t, db.WithContext(ctx).Create(o).Error)
				}
			},
			wantStillActive:        []compSub{{"build-farm", "build01"}},
			wantResolved:           []compSub{{"build-farm", "build12"}},
			wantAuditUserOnResolve: ConfigReloadResolverUser,
		},
		{
			name: "no change when every active pair is in config",
			managerCfg: &types.DashboardConfig{
				Components: []*types.Component{
					{
						Slug:          "comp",
						Subcomponents: []types.SubComponent{{Slug: "sub-a"}},
					},
				},
			},
			resolveCfg: &types.DashboardConfig{
				Components: []*types.Component{
					{
						Slug:          "comp",
						Subcomponents: []types.SubComponent{{Slug: "sub-a"}},
					},
				},
			},
			seed: func(t *testing.T, db *gorm.DB) {
				t.Helper()
				ctx := context.WithValue(context.Background(), types.CurrentUserKey, "test-user")
				o := &types.Outage{
					ComponentName:    "comp",
					SubComponentName: "sub-a",
					Severity:         types.SeverityDown,
					StartTime:        start,
					Description:      "ok",
					CreatedBy:        "system",
					DiscoveredFrom:   "component-monitor",
				}
				require.NoError(t, db.WithContext(ctx).Create(o).Error)
			},
			wantStillActive: []compSub{{"comp", "sub-a"}},
			wantResolved:    nil,
		},
		{
			name: "nil config resolves all active outages",
			managerCfg: &types.DashboardConfig{
				Components: []*types.Component{
					{
						Slug:          "comp",
						Subcomponents: []types.SubComponent{{Slug: "sub-a"}},
					},
				},
			},
			resolveCfg: nil,
			seed: func(t *testing.T, db *gorm.DB) {
				t.Helper()
				ctx := context.WithValue(context.Background(), types.CurrentUserKey, "test-user")
				o := &types.Outage{
					ComponentName:    "comp",
					SubComponentName: "sub-a",
					Severity:         types.SeverityDown,
					StartTime:        start,
					Description:      "orphan when cfg nil",
					CreatedBy:        "system",
					DiscoveredFrom:   "component-monitor",
				}
				require.NoError(t, db.WithContext(ctx).Create(o).Error)
			},
			wantStillActive:        nil,
			wantResolved:           []compSub{{"comp", "sub-a"}},
			wantAuditUserOnResolve: ConfigReloadResolverUser,
		},
		{
			name: "active outage for missing component resolved",
			managerCfg: &types.DashboardConfig{
				Components: []*types.Component{
					{
						Slug:          "still-here",
						Subcomponents: []types.SubComponent{{Slug: "sub"}},
					},
				},
			},
			resolveCfg: &types.DashboardConfig{
				Components: []*types.Component{
					{
						Slug:          "still-here",
						Subcomponents: []types.SubComponent{{Slug: "sub"}},
					},
				},
			},
			seed: func(t *testing.T, db *gorm.DB) {
				t.Helper()
				ctx := context.WithValue(context.Background(), types.CurrentUserKey, "test-user")
				o := &types.Outage{
					ComponentName:    "removed-component",
					SubComponentName: "sub",
					Severity:         types.SeverityDown,
					StartTime:        start,
					Description:      "component dropped from config",
					CreatedBy:        "system",
					DiscoveredFrom:   "component-monitor",
				}
				require.NoError(t, db.WithContext(ctx).Create(o).Error)
			},
			wantStillActive:        nil,
			wantResolved:           []compSub{{"removed-component", "sub"}},
			wantAuditUserOnResolve: ConfigReloadResolverUser,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tm := setupTestManager(t, tt.managerCfg)
			defer tm.close()

			dbMgr, ok := tm.manager.(*DBOutageManager)
			require.True(t, ok)

			tt.seed(t, tm.db)

			require.NoError(t, dbMgr.ResolveActiveOutagesForMissingSubComponents(tt.resolveCfg, ConfigReloadResolverUser))

			for _, cs := range tt.wantStillActive {
				var o types.Outage
				require.NoError(t, tm.db.Where("component_name = ? AND sub_component_name = ?", cs.component, cs.sub).First(&o).Error)
				assert.False(t, o.EndTime.Valid, "%s/%s should remain active", cs.component, cs.sub)
			}
			for _, cs := range tt.wantResolved {
				var o types.Outage
				require.NoError(t, tm.db.Where("component_name = ? AND sub_component_name = ?", cs.component, cs.sub).First(&o).Error)
				assert.True(t, o.EndTime.Valid, "%s/%s should be resolved", cs.component, cs.sub)

				if tt.wantAuditUserOnResolve != "" {
					var logs []types.OutageAuditLog
					require.NoError(t, tm.db.Where("outage_id = ?", o.ID).Order("created_at DESC").Find(&logs).Error)
					require.NotEmpty(t, logs)
					assert.Equal(t, "UPDATE", logs[0].Operation)
					assert.Equal(t, tt.wantAuditUserOnResolve, logs[0].User)
				}
			}
		})
	}
}
