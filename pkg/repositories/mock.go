package repositories

import (
	"time"

	"gorm.io/gorm"

	"ship-status-dash/pkg/types"
)

// MockOutageRepository is a mock implementation of OutageRepository for testing.
type MockOutageRepository struct {
	ActiveOutages      []types.Outage
	ActiveOutagesError error
	SaveOutageError    error
	CreateReasonError  error
	CreateOutageError  error
	TransactionError   error
	DeleteOutageError  error
	CreateReasonFn     func(*types.Reason)
	CreateOutageFn     func(*types.Outage)
	SaveOutageFn       func(*types.Outage)
	TransactionFn      func(func(OutageRepository) error) error
	// Captured data for assertions
	SavedOutages   []*types.Outage
	CreatedReasons []*types.Reason
	CreatedOutages []*types.Outage
	DeletedOutages []*types.Outage
	SaveCount      int
	// Mock data for queries
	OutagesForComponent       []types.Outage
	OutagesForSubComponent    []types.Outage
	OutageByID                *types.Outage
	OutageByIDFn              func(string, string, uint) (*types.Outage, error)
	OutageByIDError           error
	ActiveOutagesForSubComp   []types.Outage
	ActiveOutagesForComponent []types.Outage
	OutageAuditLogs           []types.OutageAuditLog
}

func (m *MockOutageRepository) GetOutageAuditLogs(outageID uint) ([]types.OutageAuditLog, error) {
	return m.OutageAuditLogs, nil
}

// MockComponentPingRepository is a mock implementation of ComponentPingRepository for testing.
type MockComponentPingRepository struct {
	UpsertError      error
	UpsertFn         func(string, string, time.Time)
	LastPingTimes    map[string]*time.Time // key format: "componentSlug/subComponentSlug"
	GetLastPingError error
	// Captured data for assertions
	UpsertedPings []struct {
		ComponentSlug    string
		SubComponentSlug string
		Timestamp        time.Time
	}
}

func (m *MockOutageRepository) GetActiveOutagesCreatedBy(componentSlug, subComponentSlug, createdBy string) ([]types.Outage, error) {
	if m.ActiveOutagesError != nil {
		return nil, m.ActiveOutagesError
	}
	return m.ActiveOutages, nil
}

func (m *MockOutageRepository) GetActiveOutagesDiscoveredFrom(componentSlug, subComponentSlug, discoveredFrom string) ([]types.Outage, error) {
	if m.ActiveOutagesError != nil {
		return nil, m.ActiveOutagesError
	}
	return m.ActiveOutages, nil
}

func (m *MockOutageRepository) SaveOutage(outage *types.Outage, user string) error {
	m.SaveCount++
	outageCopy := *outage
	m.SavedOutages = append(m.SavedOutages, &outageCopy)
	if m.SaveOutageFn != nil {
		m.SaveOutageFn(outage)
	}
	return m.SaveOutageError
}

func (m *MockOutageRepository) CreateReason(reason *types.Reason) error {
	reasonCopy := *reason
	m.CreatedReasons = append(m.CreatedReasons, &reasonCopy)
	if m.CreateReasonFn != nil {
		m.CreateReasonFn(reason)
	}
	return m.CreateReasonError
}

func (m *MockOutageRepository) CreateOutage(outage *types.Outage, user string) error {
	outageCopy := *outage
	m.CreatedOutages = append(m.CreatedOutages, &outageCopy)
	if m.CreateOutageFn != nil {
		m.CreateOutageFn(outage)
	}
	return m.CreateOutageError
}

func (m *MockOutageRepository) Transaction(fn func(OutageRepository) error) error {
	if m.TransactionError != nil {
		return m.TransactionError
	}
	if m.TransactionFn != nil {
		return m.TransactionFn(fn)
	}
	return fn(m)
}

func (m *MockOutageRepository) GetOutagesForComponent(componentSlug string, subComponentSlugs []string) ([]types.Outage, error) {
	return m.OutagesForComponent, nil
}

func (m *MockOutageRepository) GetOutagesForSubComponent(componentSlug, subComponentSlug string) ([]types.Outage, error) {
	return m.OutagesForSubComponent, nil
}

func (m *MockOutageRepository) GetOutageByID(componentSlug, subComponentSlug string, outageID uint) (*types.Outage, error) {
	if m.OutageByIDError != nil {
		return nil, m.OutageByIDError
	}
	if m.OutageByIDFn != nil {
		return m.OutageByIDFn(componentSlug, subComponentSlug, outageID)
	}
	if m.OutageByID == nil {
		return nil, gorm.ErrRecordNotFound
	}
	return m.OutageByID, nil
}

func (m *MockOutageRepository) GetActiveOutagesForSubComponent(componentSlug, subComponentSlug string) ([]types.Outage, error) {
	return m.ActiveOutagesForSubComp, nil
}

func (m *MockOutageRepository) GetActiveOutagesForComponent(componentSlug string) ([]types.Outage, error) {
	return m.ActiveOutagesForComponent, nil
}

func (m *MockOutageRepository) GetOutagesDuring(queryStart, queryEnd time.Time, refs []types.SubComponentRef) ([]types.Outage, error) {
	return []types.Outage{}, nil
}

func (m *MockOutageRepository) DeleteOutage(outage *types.Outage, user string) error {
	outageCopy := *outage
	m.DeletedOutages = append(m.DeletedOutages, &outageCopy)
	return m.DeleteOutageError
}

func (m *MockComponentPingRepository) UpsertComponentReportPing(componentSlug, subComponentSlug string, timestamp time.Time) error {
	m.UpsertedPings = append(m.UpsertedPings, struct {
		ComponentSlug    string
		SubComponentSlug string
		Timestamp        time.Time
	}{componentSlug, subComponentSlug, timestamp})
	if m.UpsertFn != nil {
		m.UpsertFn(componentSlug, subComponentSlug, timestamp)
	}
	return m.UpsertError
}

func (m *MockComponentPingRepository) GetLastPingTime(componentSlug, subComponentSlug string) (*time.Time, error) {
	if m.GetLastPingError != nil {
		return nil, m.GetLastPingError
	}
	if m.LastPingTimes == nil {
		return nil, nil
	}
	key := componentSlug + "/" + subComponentSlug
	return m.LastPingTimes[key], nil
}

func (m *MockComponentPingRepository) GetMostRecentPingTimeForAnySubComponent(componentSlug string) (*time.Time, error) {
	if m.GetLastPingError != nil {
		return nil, m.GetLastPingError
	}
	if m.LastPingTimes == nil {
		return nil, nil
	}
	var mostRecent *time.Time
	for _, pingTime := range m.LastPingTimes {
		if pingTime != nil && (mostRecent == nil || pingTime.After(*mostRecent)) {
			mostRecent = pingTime
		}
	}
	return mostRecent, nil
}

// MockSlackThreadRepository is a mock implementation of SlackThreadRepository for testing.
type MockSlackThreadRepository struct {
	CreateThreadError      error
	GetThreadsError        error
	GetThreadError         error
	UpdateThreadError      error
	CreateThreadFn         func(*types.SlackThread)
	UpdateThreadFn         func(*types.SlackThread)
	ThreadsForOutage       []types.SlackThread
	ThreadForOutageChannel *types.SlackThread
	CreatedThreads         []*types.SlackThread
	UpdatedThreads         []*types.SlackThread
}

func (m *MockSlackThreadRepository) CreateThread(thread *types.SlackThread) error {
	threadCopy := *thread
	m.CreatedThreads = append(m.CreatedThreads, &threadCopy)
	if m.CreateThreadFn != nil {
		m.CreateThreadFn(thread)
	}
	return m.CreateThreadError
}

func (m *MockSlackThreadRepository) GetThreadsForOutage(outageID uint) ([]types.SlackThread, error) {
	if m.GetThreadsError != nil {
		return nil, m.GetThreadsError
	}
	return m.ThreadsForOutage, nil
}

func (m *MockSlackThreadRepository) GetThreadForOutageAndChannel(outageID uint, channel string) (*types.SlackThread, error) {
	if m.GetThreadError != nil {
		return nil, m.GetThreadError
	}
	if m.ThreadForOutageChannel == nil {
		return nil, gorm.ErrRecordNotFound
	}
	return m.ThreadForOutageChannel, nil
}

func (m *MockSlackThreadRepository) UpdateThread(thread *types.SlackThread) error {
	threadCopy := *thread
	m.UpdatedThreads = append(m.UpdatedThreads, &threadCopy)
	if m.UpdateThreadFn != nil {
		m.UpdateThreadFn(thread)
	}
	return m.UpdateThreadError
}

// TestConfig creates a test DashboardConfig for testing.
func TestConfig(autoResolve, requiresConfirmation bool) *types.DashboardConfig {
	subComponent := types.SubComponent{
		Slug: "test-subcomponent",
		Monitoring: &types.Monitoring{
			AutoResolve:      autoResolve,
			ComponentMonitor: "test-monitor",
		},
		RequiresConfirmation: requiresConfirmation,
	}
	return &types.DashboardConfig{
		Components: []*types.Component{
			{
				Slug:          "test-component",
				Subcomponents: []types.SubComponent{subComponent},
				Owners: []types.Owner{
					{ServiceAccount: "test-sa"},
				},
			},
		},
	}
}
