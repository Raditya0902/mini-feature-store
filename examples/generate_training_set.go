package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/Raditya0902/mini-feature-store/internal/historical"
	"github.com/Raditya0902/mini-feature-store/internal/offline"
	"github.com/Raditya0902/mini-feature-store/internal/registry"
)

type trainingRowOutput struct {
	EntityID       string         `json:"entity_id"`
	EventTimestamp time.Time      `json:"event_timestamp"`
	Features       map[string]any `json:"features"`
}

func main() {
	reg, err := registry.Load("configs/feature_registry.yaml")
	if err != nil {
		log.Fatalf("loading registry: %v", err)
	}

	var view *registry.FeatureView
	for i := range reg.FeatureViews {
		if reg.FeatureViews[i].Name == "driver_stats" {
			view = &reg.FeatureViews[i]
			break
		}
	}
	if view == nil {
		log.Fatal("driver_stats feature view not found in registry")
	}

	store := &offline.ParquetStore{BasePath: "."}

	jan1 := time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC)
	entityEvents := []historical.EntityEvent{
		{EntityID: "d1", EventTimestamp: jan1.Add(10*time.Hour + 59*time.Minute)},
		{EntityID: "d1", EventTimestamp: jan1.Add(11 * time.Hour)},
		{EntityID: "d1", EventTimestamp: jan1.Add(12 * time.Hour)},
		{EntityID: "d1", EventTimestamp: jan1.Add(36*time.Hour + time.Second)},
		{EntityID: "d1", EventTimestamp: jan1.Add(60 * time.Hour)},
	}

	rows, err := historical.GetHistoricalFeatures(store, *view, entityEvents)
	if err != nil {
		log.Fatalf("getting historical features: %v", err)
	}

	for _, row := range rows {
		features := row.Features
		if features == nil {
			features = map[string]any{}
		}
		out, err := json.MarshalIndent(trainingRowOutput{
			EntityID:       row.EntityID,
			EventTimestamp: row.EventTimestamp,
			Features:       features,
		}, "", "  ")
		if err != nil {
			log.Fatalf("marshaling row for %q: %v", row.EntityID, err)
		}
		fmt.Println(string(out))
	}
}
