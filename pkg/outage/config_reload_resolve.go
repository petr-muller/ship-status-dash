package outage

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"

	"ship-status-dash/pkg/repositories"
	"ship-status-dash/pkg/types"
)

// ConfigReloadResolverUser is recorded on audit logs when active outages are ended because
// their sub-component was removed from the dashboard configuration.
const ConfigReloadResolverUser = "dashboard-config-reload"

func configuredSubComponentKeys(cfg *types.DashboardConfig) map[string]struct{} {
	keys := make(map[string]struct{})
	if cfg == nil {
		return keys
	}
	for _, c := range cfg.Components {
		if c == nil {
			continue
		}
		for _, sub := range c.Subcomponents {
			keys[subComponentKey(c.Slug, sub.Slug)] = struct{}{}
		}
	}
	return keys
}

func subComponentKey(componentSlug, subComponentSlug string) string {
	return componentSlug + "/" + subComponentSlug
}

// ResolveActiveOutagesForMissingSubComponents sets end_time on active outages whose component/sub-component
// pair is not present in the given configuration (initial load or reload).
func (m *DBOutageManager) ResolveActiveOutagesForMissingSubComponents(cfg *types.DashboardConfig, resolverUser string) error {
	allowed := configuredSubComponentKeys(cfg)
	outageRepo := repositories.NewGORMOutageRepository(m.db)
	active, err := outageRepo.GetAllActiveOutages()
	if err != nil {
		return fmt.Errorf("listing active outages: %w", err)
	}

	now := time.Now().UTC()
	entry := m.logger.WithField("reason", "sub-component removed from configuration")
	var resolved int
	for i := range active {
		o := &active[i]
		if _, ok := allowed[subComponentKey(o.ComponentName, o.SubComponentName)]; ok {
			continue
		}
		o.EndTime = sql.NullTime{Time: now, Valid: true}
		if err := m.UpdateOutage(o, resolverUser); err != nil {
			entry.WithFields(logrus.Fields{
				"component_name":     o.ComponentName,
				"sub_component_name": o.SubComponentName,
				"outage_id":          o.ID,
				"error":              err,
			}).Error("Failed to resolve outage for removed sub-component")
			continue
		}
		resolved++
		entry.WithFields(logrus.Fields{
			"component_name":     o.ComponentName,
			"sub_component_name": o.SubComponentName,
			"outage_id":          o.ID,
		}).Info("Resolved active outage for sub-component no longer in configuration")
	}
	if resolved > 0 {
		entry.WithField("count", resolved).Info("Resolved active outages for sub-components removed from configuration")
	}
	return nil
}
