package repositories

import (
	"context"
	"database/sql"
	"time"

	"gorm.io/gorm"

	"ship-status-dash/pkg/types"
)

// OutageRepository defines the interface for outage and reason database operations.
type OutageRepository interface {
	CreateOutage(outage *types.Outage, user string) error
	CreateReason(reason *types.Reason) error

	SaveOutage(outage *types.Outage, user string) error

	GetOutageByID(componentSlug, subComponentSlug string, outageID uint) (*types.Outage, error)
	GetOutagesForSubComponent(componentSlug, subComponentSlug string) ([]types.Outage, error)
	GetOutagesForComponent(componentSlug string, subComponentSlugs []string) ([]types.Outage, error)
	GetActiveOutagesForSubComponent(componentSlug, subComponentSlug string) ([]types.Outage, error)
	GetActiveOutagesForComponent(componentSlug string) ([]types.Outage, error)
	// GetAllActiveOutages returns active outages across all components (same active definition as GetActiveOutagesForComponent).
	GetAllActiveOutages() ([]types.Outage, error)
	GetActiveOutagesCreatedBy(componentSlug, subComponentSlug, createdBy string) ([]types.Outage, error)
	GetActiveOutagesDiscoveredFrom(componentSlug, subComponentSlug, discoveredFrom string) ([]types.Outage, error)

	GetOutageAuditLogs(outageID uint) ([]types.OutageAuditLog, error)

	DeleteOutage(outage *types.Outage, user string) error
}

// gormOutageRepository is a GORM implementation of OutageRepository.
type gormOutageRepository struct {
	db *gorm.DB
}

// NewGORMOutageRepository creates a new GORM-based OutageRepository.
func NewGORMOutageRepository(db *gorm.DB) OutageRepository {
	return &gormOutageRepository{db: db}
}

// roundOutageTimes truncates all time fields in an outage down to the nearest second.
func roundOutageTimes(outage *types.Outage) {
	outage.StartTime = outage.StartTime.Truncate(time.Second).UTC()
	if outage.EndTime.Valid {
		outage.EndTime = sql.NullTime{
			Time:  outage.EndTime.Time.Truncate(time.Second).UTC(),
			Valid: true,
		}
	}
	if outage.ConfirmedAt.Valid {
		outage.ConfirmedAt = sql.NullTime{
			Time:  outage.ConfirmedAt.Time.Truncate(time.Second).UTC(),
			Valid: true,
		}
	}
}

// CreateOutage creates a new outage record in the database.
func (r *gormOutageRepository) CreateOutage(outage *types.Outage, user string) error {
	roundOutageTimes(outage)
	return r.db.WithContext(context.WithValue(context.Background(), types.CurrentUserKey, user)).Create(outage).Error
}

// CreateReason creates a new reason record in the database.
func (r *gormOutageRepository) CreateReason(reason *types.Reason) error {
	return r.db.Create(reason).Error
}

// SaveOutage updates an existing outage record in the database.
// If the outage does not exist, it will be created.
func (r *gormOutageRepository) SaveOutage(outage *types.Outage, user string) error {
	roundOutageTimes(outage)
	return r.db.WithContext(context.WithValue(context.Background(), types.CurrentUserKey, user)).Save(outage).Error
}

// GetOutageByID retrieves a specific outage by ID for a component/sub-component combination.
// Returns gorm.ErrRecordNotFound if the outage is not found.
func (r *gormOutageRepository) GetOutageByID(componentSlug, subComponentSlug string, outageID uint) (*types.Outage, error) {
	var outage types.Outage
	err := r.db.Preload("Reasons").Preload("SlackThreads").
		Where("id = ? AND component_name = ? AND sub_component_name = ?", outageID, componentSlug, subComponentSlug).
		First(&outage).Error
	return &outage, err
}

// GetOutagesForSubComponent retrieves all outages for a specific sub-component.
// Reasons are preloaded.
func (r *gormOutageRepository) GetOutagesForSubComponent(componentSlug, subComponentSlug string) ([]types.Outage, error) {
	var outages []types.Outage
	err := r.db.Preload("Reasons").
		Where("component_name = ? AND sub_component_name = ?", componentSlug, subComponentSlug).
		Order("start_time DESC").
		Find(&outages).Error
	return outages, err
}

// GetOutagesForComponent retrieves all outages for a component across multiple sub-components.
// Reasons are preloaded.
func (r *gormOutageRepository) GetOutagesForComponent(componentSlug string, subComponentSlugs []string) ([]types.Outage, error) {
	var outages []types.Outage
	err := r.db.Preload("Reasons").
		Where("component_name = ? AND sub_component_name IN ?", componentSlug, subComponentSlugs).
		Order("start_time DESC").
		Find(&outages).Error
	return outages, err
}

// GetActiveOutagesForSubComponent retrieves active outages for a specific sub-component.
// An outage is considered active if end_time IS NULL OR end_time > now (UTC for consistent DB comparison).
func (r *gormOutageRepository) GetActiveOutagesForSubComponent(componentSlug, subComponentSlug string) ([]types.Outage, error) {
	var outages []types.Outage
	now := time.Now().UTC()
	err := r.db.Where("component_name = ? AND sub_component_name = ? AND (end_time IS NULL OR end_time > ?)", componentSlug, subComponentSlug, now).
		Order("start_time DESC").
		Find(&outages).Error
	return outages, err
}

// GetActiveOutagesForComponent retrieves active outages for a component across all sub-components.
// An outage is considered active if end_time IS NULL OR end_time > now (UTC for consistent DB comparison).
func (r *gormOutageRepository) GetActiveOutagesForComponent(componentSlug string) ([]types.Outage, error) {
	var outages []types.Outage
	now := time.Now().UTC()
	err := r.db.Where("component_name = ? AND (end_time IS NULL OR end_time > ?)", componentSlug, now).
		Order("start_time DESC").
		Find(&outages).Error
	return outages, err
}

// GetAllActiveOutages retrieves every outage that is still considered active (end_time IS NULL OR end_time > now UTC).
func (r *gormOutageRepository) GetAllActiveOutages() ([]types.Outage, error) {
	var outages []types.Outage
	now := time.Now().UTC()
	err := r.db.Where("end_time IS NULL OR end_time > ?", now).
		Order("component_name, sub_component_name, start_time DESC").
		Find(&outages).Error
	return outages, err
}

// GetActiveOutagesCreatedBy retrieves all active outages for a specific component and sub-component
// that were created by the given creator. Note that the reasons are not considered here.
// An outage is considered active if its end_time is NULL.
func (r *gormOutageRepository) GetActiveOutagesCreatedBy(componentSlug, subComponentSlug, createdBy string) ([]types.Outage, error) {
	var activeOutages []types.Outage
	err := r.db.
		Where("component_name = ? AND sub_component_name = ? AND end_time IS NULL AND created_by = ?",
			componentSlug, subComponentSlug, createdBy).
		Find(&activeOutages).Error
	return activeOutages, err
}

// GetActiveOutagesDiscoveredFrom retrieves all active outages for a specific component and sub-component
// that were discovered from the given source. Note that the reasons are not considered here.
// An outage is considered active if its end_time is NULL.
func (r *gormOutageRepository) GetActiveOutagesDiscoveredFrom(componentSlug, subComponentSlug, discoveredFrom string) ([]types.Outage, error) {
	var activeOutages []types.Outage
	err := r.db.
		Where("component_name = ? AND sub_component_name = ? AND end_time IS NULL AND discovered_from = ?",
			componentSlug, subComponentSlug, discoveredFrom).
		Find(&activeOutages).Error
	return activeOutages, err
}

func (r *gormOutageRepository) GetOutageAuditLogs(outageID uint) ([]types.OutageAuditLog, error) {
	var outageAuditLogs []types.OutageAuditLog
	err := r.db.Where("outage_id = ?", outageID).Order("created_at DESC").Find(&outageAuditLogs).Error
	return outageAuditLogs, err
}

// DeleteOutage deletes an outage from the database.
func (r *gormOutageRepository) DeleteOutage(outage *types.Outage, user string) error {
	return r.db.WithContext(context.WithValue(context.Background(), types.CurrentUserKey, user)).Delete(outage).Error
}
