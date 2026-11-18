package historical

import (
	"fmt"
	"path/filepath"
	"sort"
	"time"

	"github.com/Raditya0902/mini-feature-store/internal/offline"
	"github.com/Raditya0902/mini-feature-store/internal/registry"
)

type EntityEvent struct {
	EntityID       string
	EventTimestamp time.Time
}

type TrainingRow struct {
	EntityID         string
	EventTimestamp   time.Time
	FeatureTimestamp *time.Time
	Features         map[string]any
}

func GetHistoricalFeatures(
	store *offline.ParquetStore,
	featureView registry.FeatureView,
	entityRows []EntityEvent,
) ([]TrainingRow, error) {
	result := make([]TrainingRow, len(entityRows))
	byEntity := make(map[string][]int)
	for i, event := range entityRows {
		result[i] = TrainingRow{
			EntityID:       event.EntityID,
			EventTimestamp: event.EventTimestamp,
		}
		byEntity[event.EntityID] = append(byEntity[event.EntityID], i)
	}
	for _, indices := range byEntity {
		sort.Slice(indices, func(i, j int) bool {
			return entityRows[indices[i]].EventTimestamp.Before(entityRows[indices[j]].EventTimestamp)
		})
	}

	best := make([]*offline.FeatureRow, len(entityRows))
	ttl := featureView.TTL.Duration
	path := filepath.Join(store.BasePath, featureView.Source)
	err := offline.Scan(path, func(row offline.FeatureRow) {
		indices := byEntity[row.EntityID]
		first := sort.Search(len(indices), func(i int) bool {
			return !entityRows[indices[i]].EventTimestamp.Before(row.FeatureTimestamp)
		})
		for _, i := range indices[first:] {
			if entityRows[i].EventTimestamp.Sub(row.FeatureTimestamp) > ttl {
				break
			}
			if best[i] == nil || row.FeatureTimestamp.After(best[i].FeatureTimestamp) {
				best[i] = &row
			}
		}
	})
	if err != nil {
		return nil, fmt.Errorf("reading feature data for %q: %w", featureView.Name, err)
	}
	for i, row := range best {
		if row != nil {
			result[i].FeatureTimestamp = &row.FeatureTimestamp
			result[i].Features = row.Values
		}
	}
	return result, nil
}
