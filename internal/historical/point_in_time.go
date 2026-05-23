package historical

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/Raditya0902/mini-feature-store/internal/offline"
	"github.com/Raditya0902/mini-feature-store/internal/registry"
)

type EntityEvent struct {
	EntityID       string
	EventTimestamp time.Time
}

type TrainingRow struct {
	EntityID       string
	EventTimestamp time.Time
	Features       map[string]any
}

func GetHistoricalFeatures(
	store *offline.ParquetStore,
	featureView registry.FeatureView,
	entityRows []EntityEvent,
) ([]TrainingRow, error) {
	path := filepath.Join(store.BasePath, featureView.Source)
	rows, err := offline.Read(path)
	if err != nil {
		return nil, fmt.Errorf("reading feature data for %q: %w", featureView.Name, err)
	}
	return joinRows(rows, featureView.TTL.Duration, entityRows), nil
}

func joinRows(rows []offline.FeatureRow, ttl time.Duration, events []EntityEvent) []TrainingRow {
	byEntity := make(map[string][]offline.FeatureRow, len(rows))
	for _, r := range rows {
		byEntity[r.EntityID] = append(byEntity[r.EntityID], r)
	}

	result := make([]TrainingRow, len(events))
	for i, event := range events {
		result[i] = TrainingRow{
			EntityID:       event.EntityID,
			EventTimestamp: event.EventTimestamp,
		}

		var best *offline.FeatureRow
		for j := range byEntity[event.EntityID] {
			r := &byEntity[event.EntityID][j]
			if r.FeatureTimestamp.After(event.EventTimestamp) {
				continue
			}
			if event.EventTimestamp.Sub(r.FeatureTimestamp) > ttl {
				continue
			}
			if best == nil || r.FeatureTimestamp.After(best.FeatureTimestamp) {
				best = r
			}
		}

		if best != nil {
			result[i].Features = best.Values
		}
	}
	return result
}
