package materialization

import (
	"fmt"
	"path/filepath"

	"github.com/Raditya0902/mini-feature-store/internal/offline"
	"github.com/Raditya0902/mini-feature-store/internal/online"
	"github.com/Raditya0902/mini-feature-store/internal/registry"
)

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

	for _, row := range rows {
		if err := onlineStore.Set(row.EntityID, row.Values); err != nil {
			return fmt.Errorf("writing online store for %q: %w", row.EntityID, err)
		}
	}

	return nil
}
