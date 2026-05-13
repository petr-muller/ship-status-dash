package outage

import (
	"context"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"ship-status-dash/pkg/config"
	"ship-status-dash/pkg/repositories"
	"ship-status-dash/pkg/testhelper"
	"ship-status-dash/pkg/types"

	"github.com/google/go-cmp/cmp"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestDB creates an in-memory SQLite database for testing and migrates the standard outage-related models.
// The database is automatically closed when the test completes.
func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}
	err = db.AutoMigrate(&types.Outage{}, &types.Reason{}, &types.SlackThread{}, &types.OutageAuditLog{})
	if err != nil {
		t.Fatalf("Failed to migrate test database: %v", err)
	}
	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err != nil {
			t.Fatalf("test database sql.DB: %v", err)
		}
		if err := sqlDB.Close(); err != nil {
			t.Fatalf("test database close: %v", err)
		}
	})
	return db
}

type testManager struct {
	manager    OutageManager
	db         *gorm.DB
	mockServer *MockSlackServer
}

func setupTestManager(t *testing.T, cfg *types.DashboardConfig) *testManager {
	db := setupTestDB(t)

	cfgManager, err := config.NewManager("", func(string) (*types.DashboardConfig, error) {
		return cfg, nil
	}, logrus.New(), time.Second)
	if err != nil {
		t.Fatalf("Failed to create config manager: %v", err)
	}
	cfgManager.Get()

	mockServer := NewMockSlackServer(t)
	slackClient := mockServer.Client()

	manager := NewDBOutageManager(db, slackClient, cfgManager, "https://test.example.com/", "https://rhsandbox.slack.com/", logrus.New())

	return &testManager{
		manager:    manager,
		db:         db,
		mockServer: mockServer,
	}
}

func (tm *testManager) close() {
	tm.mockServer.Close()
}

func assertSlackMessages(t *testing.T, mockServer *MockSlackServer, want []PostedMessage) {
	postedMsgs := mockServer.PostedMessages()
	if diff := cmp.Diff(want, postedMsgs, testhelper.EquateNilEmpty); diff != "" {
		t.Errorf("Slack messages mismatch (-want +got):\n%s", diff)
	}
}

func TestOutageManager_CreateOutage(t *testing.T) {
	tests := []struct {
		name              string
		outage            *types.Outage
		reasons           []types.Reason
		config            *types.DashboardConfig
		verifyOutage      func(t *testing.T, outage *types.Outage)
		verifyReasons     func(t *testing.T, db *gorm.DB, outageID uint)
		wantSlackMessages []PostedMessage
	}{
		{
			name: "successful creation with slack reporting",
			outage: &types.Outage{
				Model:            gorm.Model{ID: 1},
				ComponentName:    "test-component",
				SubComponentName: "test-sub",
				Severity:         types.SeverityDown,
				StartTime:        time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
				Description:      "Automated test outage",
				CreatedBy:        "system",
				DiscoveredFrom:   "component-monitor",
			},
			config: &types.DashboardConfig{
				Components: []*types.Component{
					{
						Slug: "test-component",
						Name: "Test Component",
						SlackReporting: []types.SlackReportingConfig{
							{Channel: "#test-channel"},
						},
						Subcomponents: []types.SubComponent{
							{Slug: "test-sub", Name: "Test Sub"},
						},
					},
				},
			},
			verifyOutage: func(t *testing.T, outage *types.Outage) {
				assert.Equal(t, "test-component", outage.ComponentName)
				assert.Equal(t, "test-sub", outage.SubComponentName)
				assert.Equal(t, types.SeverityDown, outage.Severity)
				wantStartTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
				assert.True(t, outage.StartTime.Equal(wantStartTime), "StartTime = %v, want %v", outage.StartTime, wantStartTime)
				assert.Equal(t, "system", outage.CreatedBy)
				assert.Equal(t, "component-monitor", outage.DiscoveredFrom)
				assert.NotZero(t, outage.ID, "ID should be set")
			},
			wantSlackMessages: []PostedMessage{
				{
					Channel:         "#test-channel",
					Text:            "🚨 Outage Detected: Test Component/Test Sub\n\nSeverity: `Down`\nDescription:\n>Automated test outage\nStarted: `2024-01-15T10:30:00Z`\nCreated by: `system`\nDiscovered from: `component-monitor`\n\n<https://test.example.com/test-component/test-sub/outages/1|View Outage>",
					ThreadTimestamp: "",
					ResponseTS:      "1234567890.000001",
				},
			},
		},
		{
			name: "successful creation with reasons",
			outage: &types.Outage{
				Model:            gorm.Model{ID: 1},
				ComponentName:    "test-component",
				SubComponentName: "test-sub",
				Severity:         types.SeverityDown,
				StartTime:        time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
				Description:      "Automated test outage",
				CreatedBy:        "system",
				DiscoveredFrom:   "component-monitor",
			},
			reasons: []types.Reason{
				{
					Type:    types.CheckTypePrometheus,
					Check:   "up{job=\"test\"} == 0",
					Results: "No healthy instances found",
				},
				{
					Type:    types.CheckTypeHTTP,
					Check:   "https://test.example.com/health",
					Results: "Response time > 5s",
				},
			},
			config: &types.DashboardConfig{
				Components: []*types.Component{
					{
						Slug: "test-component",
						Name: "Test Component",
						Subcomponents: []types.SubComponent{
							{Slug: "test-sub", Name: "Test Sub"},
						},
					},
				},
			},
			verifyOutage: func(t *testing.T, outage *types.Outage) {
				assert.Equal(t, "test-component", outage.ComponentName)
				assert.Equal(t, "test-sub", outage.SubComponentName)
				assert.Equal(t, types.SeverityDown, outage.Severity)
				assert.NotZero(t, outage.ID, "ID should be set")
			},
			verifyReasons: func(t *testing.T, db *gorm.DB, outageID uint) {
				var createdReasons []types.Reason
				err := db.Where("outage_id = ?", outageID).Find(&createdReasons).Error
				assert.NoError(t, err)
				assert.Len(t, createdReasons, 2, "Should create 2 reasons")

				assert.Equal(t, types.CheckTypePrometheus, createdReasons[0].Type)
				assert.Equal(t, "up{job=\"test\"} == 0", createdReasons[0].Check)
				assert.Equal(t, "No healthy instances found", createdReasons[0].Results)
				assert.Equal(t, outageID, createdReasons[0].OutageID)

				assert.Equal(t, types.CheckTypeHTTP, createdReasons[1].Type)
				assert.Equal(t, "https://test.example.com/health", createdReasons[1].Check)
				assert.Equal(t, "Response time > 5s", createdReasons[1].Results)
				assert.Equal(t, outageID, createdReasons[1].OutageID)
			},
			wantSlackMessages: []PostedMessage{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tm := setupTestManager(t, tt.config)
			defer tm.close()

			err := tm.manager.CreateOutage(tt.outage, tt.reasons, "test-user")
			if err != nil {
				t.Fatalf("Failed to create outage: %v", err)
			}

			var createdOutages []types.Outage
			err = tm.db.Find(&createdOutages).Error
			if err != nil {
				t.Fatalf("Failed to query created outages: %v", err)
			}

			if len(createdOutages) != 1 {
				t.Fatalf("Expected 1 outage, got %d", len(createdOutages))
			}

			if tt.verifyOutage != nil {
				tt.verifyOutage(t, &createdOutages[0])
			}

			if tt.verifyReasons != nil {
				tt.verifyReasons(t, tm.db, createdOutages[0].ID)
			}

			assertSlackMessages(t, tm.mockServer, tt.wantSlackMessages)

			var logs []types.OutageAuditLog
			err = tm.db.Where("outage_id = ?", createdOutages[0].ID).Find(&logs).Error
			require.NoError(t, err)
			require.Len(t, logs, 1)
			assert.Equal(t, "CREATE", logs[0].Operation)
			assert.Equal(t, "test-user", logs[0].User)
		})
	}
}

func TestOutageManager_UpdateOutage(t *testing.T) {
	tests := []struct {
		name              string
		outage            *types.Outage
		mutateOutage      func(*types.Outage)
		config            *types.DashboardConfig
		slackThreadRepo   *repositories.MockSlackThreadRepository
		verifyOutage      func(t *testing.T, outage *types.Outage)
		wantSlackMessages []PostedMessage
	}{
		{
			name: "successful update with slack reporting",
			outage: &types.Outage{
				Model:            gorm.Model{ID: 1},
				ComponentName:    "test-component",
				SubComponentName: "test-sub",
				Severity:         types.SeverityDown,
				StartTime:        time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
				Description:      "Automated test outage",
				CreatedBy:        "system",
				DiscoveredFrom:   "component-monitor",
			},
			mutateOutage: func(o *types.Outage) {
				o.Severity = types.SeverityDegraded
			},
			config: &types.DashboardConfig{
				Components: []*types.Component{
					{
						Slug: "test-component",
						Name: "Test Component",
						Subcomponents: []types.SubComponent{
							{Slug: "test-sub", Name: "Test Sub"},
						},
					},
				},
			},
			slackThreadRepo: &repositories.MockSlackThreadRepository{
				ThreadsForOutage: []types.SlackThread{
					{
						Channel:         "#test-channel",
						ThreadTimestamp: "1234567890.123456",
					},
				},
			},
			verifyOutage: func(t *testing.T, outage *types.Outage) {
				assert.Equal(t, uint(1), outage.ID)
				assert.Equal(t, "test-component", outage.ComponentName)
				assert.Equal(t, "test-sub", outage.SubComponentName)
				assert.Equal(t, types.SeverityDown, outage.Severity)
				assert.Equal(t, "Automated test outage", outage.Description)
			},
			wantSlackMessages: []PostedMessage{
				{
					Channel:         "#test-channel",
					Text:            "📝 Outage Updated: Test Component/Test Sub (#1)\n\nSeverity changed: `Degraded` → `Down`\n\n<https://test.example.com/test-component/test-sub/outages/1|View Outage>",
					ThreadTimestamp: "1234567890.123456",
					ResponseTS:      "1234567890.000001",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tm := setupTestManager(t, tt.config)
			defer tm.close()

			oldOutage := *tt.outage
			tt.mutateOutage(&oldOutage)
			ctx := context.WithValue(context.Background(), types.CurrentUserKey, "test-user")
			err := tm.db.WithContext(ctx).Create(&oldOutage).Error
			if err != nil {
				t.Fatalf("Failed to create old outage: %v", err)
			}

			if tt.slackThreadRepo != nil && len(tt.slackThreadRepo.ThreadsForOutage) > 0 {
				thread := tt.slackThreadRepo.ThreadsForOutage[0]
				thread.OutageID = tt.outage.ID
				err := tm.db.Create(&thread).Error
				if err != nil {
					t.Fatalf("Failed to create slack thread: %v", err)
				}
			}

			err = tm.manager.UpdateOutage(tt.outage, "test-user")
			if err != nil {
				t.Fatalf("Failed to update outage: %v", err)
			}

			var savedOutages []types.Outage
			err = tm.db.Where("id = ?", tt.outage.ID).Find(&savedOutages).Error
			if err != nil {
				t.Fatalf("Failed to query saved outages: %v", err)
			}

			if len(savedOutages) != 1 {
				t.Fatalf("Expected 1 outage, got %d", len(savedOutages))
			}

			if tt.verifyOutage != nil {
				tt.verifyOutage(t, &savedOutages[0])
			}

			assertSlackMessages(t, tm.mockServer, tt.wantSlackMessages)

			var logs []types.OutageAuditLog
			err = tm.db.Where("outage_id = ?", tt.outage.ID).Order("created_at DESC").Find(&logs).Error
			require.NoError(t, err)
			require.Len(t, logs, 2)
			assert.Equal(t, "UPDATE", logs[0].Operation)
			assert.Equal(t, "test-user", logs[0].User)
			assert.Equal(t, "CREATE", logs[1].Operation)
			assert.Equal(t, "test-user", logs[1].User)
		})
	}
}

func TestOutageManager_GetOutageAuditLogs(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(t *testing.T, tm *testManager) uint
		wantLogs []types.OutageAuditLog
	}{
		{
			name: "create only returns one CREATE log",
			setup: func(t *testing.T, tm *testManager) uint {
				outage := &types.Outage{
					Model:            gorm.Model{ID: 1},
					ComponentName:    "test-component",
					SubComponentName: "test-sub",
					Severity:         types.SeverityDown,
					StartTime:        time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
					Description:      "Automated test outage",
					CreatedBy:        "system",
					DiscoveredFrom:   "component-monitor",
				}
				config := &types.DashboardConfig{
					Components: []*types.Component{
						{
							Slug: "test-component",
							Name: "Test Component",
							Subcomponents: []types.SubComponent{
								{Slug: "test-sub", Name: "Test Sub"},
							},
						},
					},
				}
				tm2 := setupTestManager(t, config)
				*tm = *tm2
				err := tm.manager.CreateOutage(outage, nil, "test-user")
				require.NoError(t, err)
				return outage.ID
			},
			wantLogs: []types.OutageAuditLog{
				{Operation: "CREATE", User: "test-user"},
			},
		},
		{
			name: "create and update returns two logs newest first",
			setup: func(t *testing.T, tm *testManager) uint {
				config := &types.DashboardConfig{
					Components: []*types.Component{
						{
							Slug: "test-component",
							Name: "Test Component",
							Subcomponents: []types.SubComponent{
								{Slug: "test-sub", Name: "Test Sub"},
							},
						},
					},
				}
				tm2 := setupTestManager(t, config)
				*tm = *tm2
				outage := &types.Outage{
					Model:            gorm.Model{ID: 1},
					ComponentName:    "test-component",
					SubComponentName: "test-sub",
					Severity:         types.SeverityDown,
					StartTime:        time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
					Description:      "Automated test outage",
					CreatedBy:        "system",
					DiscoveredFrom:   "component-monitor",
				}
				ctx := context.WithValue(context.Background(), types.CurrentUserKey, "test-user")
				err := tm.db.WithContext(ctx).Create(outage).Error
				require.NoError(t, err)
				outage.Severity = types.SeverityDegraded
				err = tm.manager.UpdateOutage(outage, "test-user")
				require.NoError(t, err)
				return outage.ID
			},
			wantLogs: []types.OutageAuditLog{
				{Operation: "UPDATE", User: "test-user"},
				{Operation: "CREATE", User: "test-user"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var tm testManager
			outageID := tt.setup(t, &tm)
			defer tm.close()

			logs, err := tm.manager.GetOutageAuditLogs(outageID)
			require.NoError(t, err)
			require.Len(t, logs, len(tt.wantLogs))
			for i := range tt.wantLogs {
				assert.Equal(t, tt.wantLogs[i].Operation, logs[i].Operation)
				assert.Equal(t, tt.wantLogs[i].User, logs[i].User)
			}
		})
	}
}

func TestOutageManager_DeleteOutage(t *testing.T) {
	config := &types.DashboardConfig{
		Components: []*types.Component{
			{
				Slug: "test-component",
				Name: "Test Component",
				Subcomponents: []types.SubComponent{
					{Slug: "test-sub", Name: "Test Sub"},
				},
			},
		},
	}
	tm := setupTestManager(t, config)
	defer tm.close()

	outage := &types.Outage{
		Model:            gorm.Model{ID: 1},
		ComponentName:    "test-component",
		SubComponentName: "test-sub",
		Severity:         types.SeverityDown,
		StartTime:        time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		Description:      "Automated test outage",
		CreatedBy:        "system",
		DiscoveredFrom:   "component-monitor",
	}
	err := tm.manager.CreateOutage(outage, nil, "test-user")
	require.NoError(t, err)
	outageID := outage.ID

	err = tm.manager.DeleteOutage(outage, "test-user")
	require.NoError(t, err)

	var logs []types.OutageAuditLog
	err = tm.db.Where("outage_id = ?", outageID).Order("created_at DESC").Find(&logs).Error
	require.NoError(t, err)
	require.Len(t, logs, 2)
	assert.Equal(t, "DELETE", logs[0].Operation)
	assert.Equal(t, "test-user", logs[0].User)
	assert.Equal(t, "CREATE", logs[1].Operation)
	assert.Equal(t, "test-user", logs[1].User)
}
