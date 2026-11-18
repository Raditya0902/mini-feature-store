package materialization

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/Raditya0902/mini-feature-store/internal/offline"
	"github.com/Raditya0902/mini-feature-store/internal/online"
	"github.com/Raditya0902/mini-feature-store/internal/registry"
)

// Materialize rebuilds the offline history from the input. A nil onlineStore
// skips publishing snapshots to Redis.
func Materialize(
	reg *registry.Registry,
	store *offline.ParquetStore,
	onlineStore *online.RedisStore,
	rawPath string,
) error {
	rows, err := GenerateDriverStats(rawPath)
	if err != nil {
		return fmt.Errorf("generating driver stats: %w", err)
	}

	var view *registry.FeatureView
	for i := range reg.FeatureViews {
		if reg.FeatureViews[i].Name == "driver_stats" {
			view = &reg.FeatureViews[i]
			break
		}
	}
	if view == nil {
		return fmt.Errorf("feature view %q not found in registry", "driver_stats")
	}

	parquetPath := filepath.Join(store.BasePath, view.Source)
	if err := offline.Write(parquetPath, rows); err != nil {
		return fmt.Errorf("writing offline store: %w", err)
	}

	if onlineStore == nil {
		return nil
	}

	now := time.Now()
	latest := make(map[string]offline.FeatureRow)
	for _, row := range rows {
		if row.FeatureTimestamp.After(now) {
			continue
		}
		previous, exists := latest[row.EntityID]
		if !exists || row.FeatureTimestamp.After(previous.FeatureTimestamp) {
			latest[row.EntityID] = row
		}
	}
	for _, row := range latest {
		if err := onlineStore.SetSnapshot(row.EntityID, row.FeatureTimestamp, row.Values); err != nil {
			return fmt.Errorf("writing online store for %q: %w", row.EntityID, err)
		}
	}

	return nil
}
