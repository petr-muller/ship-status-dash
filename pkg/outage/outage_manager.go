package outage

import (
	"fmt"
	"time"

	"ship-status-dash/pkg/config"
	"ship-status-dash/pkg/repositories"
	"ship-status-dash/pkg/types"

	"github.com/sirupsen/logrus"
	"github.com/slack-go/slack"
	"gorm.io/gorm"
)

// OutageManager defines the interface for outage management operations.
type OutageManager interface {
	CreateOutage(outage *types.Outage, reasons []types.Reason, user string) error
	UpdateOutage(outage *types.Outage, user string) error
	GetOutageByID(componentSlug, subComponentSlug string, outageID uint) (*types.Outage, error)
	GetOutagesForSubComponent(componentSlug, subComponentSlug string) ([]types.Outage, error)
	GetOutagesForComponent(componentSlug string, subComponentSlugs []string) ([]types.Outage, error)
	GetActiveOutagesForSubComponent(componentSlug, subComponentSlug string) ([]types.Outage, error)
	GetActiveOutagesForComponent(componentSlug string) ([]types.Outage, error)
	GetActiveOutagesCreatedBy(componentSlug, subComponentSlug, createdBy string) ([]types.Outage, error)
	GetActiveOutagesDiscoveredFrom(componentSlug, subComponentSlug, discoveredFrom string) ([]types.Outage, error)
	GetOutagesDuring(queryStart, queryEnd time.Time, refs []types.SubComponentRef) ([]types.Outage, error)
	GetOutageAuditLogs(outageID uint) ([]types.OutageAuditLog, error)
	DeleteOutage(outage *types.Outage, user string) error
}

// DBOutageManager handles outage creation and updates with Slack reporting using a database.
type DBOutageManager struct {
	slackThreadRepo repositories.SlackThreadRepository
	db              *gorm.DB
	slackReporter   *SlackReporter
	logger          *logrus.Logger
}

// NewDBOutageManager creates a new DBOutageManager instance.
func NewDBOutageManager(
	db *gorm.DB,
	slackClient *slack.Client,
	configManager *config.Manager[types.DashboardConfig],
	baseURL string,
	slackWorkspaceURL string,
	logger *logrus.Logger,
) OutageManager {
	slackThreadRepo := repositories.NewGORMSlackThreadRepository(db)
	var slackReporter *SlackReporter
	if slackClient != nil {
		slackReporter = NewSlackReporter(slackClient, slackThreadRepo, configManager, baseURL, slackWorkspaceURL, logger)
	}

	return &DBOutageManager{
		slackThreadRepo: slackThreadRepo,
		db:              db,
		slackReporter:   slackReporter,
		logger:          logger,
	}
}

// CreateOutage creates a new outage and posts to Slack channels if configured.
func (m *DBOutageManager) CreateOutage(outage *types.Outage, reasons []types.Reason, user string) error {
	if msg, ok := outage.Validate(); !ok {
		return fmt.Errorf("validation failed: %s", msg)
	}

	if err := m.db.Transaction(func(tx *gorm.DB) error {
		outageRepo := repositories.NewGORMOutageRepository(tx)
		if err := outageRepo.CreateOutage(outage, user); err != nil {
			return err
		}

		for _, reason := range reasons {
			reason.OutageID = outage.ID
			if err := outageRepo.CreateReason(&reason); err != nil {
				return err
			}
		}

		return nil
	}); err != nil {
		return err
	}

	// Slack reporting is done outside the transaction as we don't want to fail to create the outage due to slack reporting issues
	if m.slackReporter != nil {
		if err := m.slackReporter.ReportOutage(outage); err != nil {
			m.logger.WithFields(logrus.Fields{
				"outage_id": outage.ID,
				"error":     err,
			}).Error("Failed to report outage to Slack, but outage was created")
		}
	}

	return nil
}

// UpdateOutage updates an existing outage and posts thread replies to Slack.
func (m *DBOutageManager) UpdateOutage(outage *types.Outage, user string) error {
	if msg, ok := outage.Validate(); !ok {
		return fmt.Errorf("validation failed: %s", msg)
	}

	outageRepo := repositories.NewGORMOutageRepository(m.db)
	oldOutage, err := outageRepo.GetOutageByID(outage.ComponentName, outage.SubComponentName, outage.ID)
	if err != nil {
		return err
	}

	if err := outageRepo.SaveOutage(outage, user); err != nil {
		return err
	}

	if m.slackReporter != nil {
		if err := m.slackReporter.ReportOutageUpdate(outage, oldOutage); err != nil {
			m.logger.WithFields(logrus.Fields{
				"outage_id": outage.ID,
				"error":     err,
			}).Error("Failed to report outage update to Slack, but outage was updated")
		}
	}

	return nil
}

// GetOutageByID delegates read operations to the repository.
func (m *DBOutageManager) GetOutageByID(componentSlug, subComponentSlug string, outageID uint) (*types.Outage, error) {
	outageRepo := repositories.NewGORMOutageRepository(m.db)
	return outageRepo.GetOutageByID(componentSlug, subComponentSlug, outageID)
}

func (m *DBOutageManager) GetOutagesForSubComponent(componentSlug, subComponentSlug string) ([]types.Outage, error) {
	outageRepo := repositories.NewGORMOutageRepository(m.db)
	return outageRepo.GetOutagesForSubComponent(componentSlug, subComponentSlug)
}

func (m *DBOutageManager) GetOutagesForComponent(componentSlug string, subComponentSlugs []string) ([]types.Outage, error) {
	outageRepo := repositories.NewGORMOutageRepository(m.db)
	return outageRepo.GetOutagesForComponent(componentSlug, subComponentSlugs)
}

func (m *DBOutageManager) GetActiveOutagesForSubComponent(componentSlug, subComponentSlug string) ([]types.Outage, error) {
	outageRepo := repositories.NewGORMOutageRepository(m.db)
	return outageRepo.GetActiveOutagesForSubComponent(componentSlug, subComponentSlug)
}

func (m *DBOutageManager) GetActiveOutagesForComponent(componentSlug string) ([]types.Outage, error) {
	outageRepo := repositories.NewGORMOutageRepository(m.db)
	return outageRepo.GetActiveOutagesForComponent(componentSlug)
}

func (m *DBOutageManager) GetActiveOutagesCreatedBy(componentSlug, subComponentSlug, createdBy string) ([]types.Outage, error) {
	outageRepo := repositories.NewGORMOutageRepository(m.db)
	return outageRepo.GetActiveOutagesCreatedBy(componentSlug, subComponentSlug, createdBy)
}

func (m *DBOutageManager) GetActiveOutagesDiscoveredFrom(componentSlug, subComponentSlug, discoveredFrom string) ([]types.Outage, error) {
	outageRepo := repositories.NewGORMOutageRepository(m.db)
	return outageRepo.GetActiveOutagesDiscoveredFrom(componentSlug, subComponentSlug, discoveredFrom)
}

func (m *DBOutageManager) GetOutagesDuring(queryStart, queryEnd time.Time, refs []types.SubComponentRef) ([]types.Outage, error) {
	outageRepo := repositories.NewGORMOutageRepository(m.db)
	return outageRepo.GetOutagesDuring(queryStart, queryEnd, refs)
}

func (m *DBOutageManager) GetOutageAuditLogs(outageID uint) ([]types.OutageAuditLog, error) {
	outageRepo := repositories.NewGORMOutageRepository(m.db)
	return outageRepo.GetOutageAuditLogs(outageID)
}

func (m *DBOutageManager) DeleteOutage(outage *types.Outage, user string) error {
	outageRepo := repositories.NewGORMOutageRepository(m.db)
	return outageRepo.DeleteOutage(outage, user)
}
