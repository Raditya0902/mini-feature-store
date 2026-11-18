package tests

import (
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Raditya0902/mini-feature-store/internal/historical"
	"github.com/Raditya0902/mini-feature-store/internal/materialization"
	"github.com/Raditya0902/mini-feature-store/internal/offline"
	"github.com/Raditya0902/mini-feature-store/internal/registry"
)

const registryPath = "../configs/feature_registry.yaml"

type driverExpected struct {
	tripCount float64
	avgFare   float64
	avgDur    float64
}

func TestConsistency(t *testing.T) {
	onlineStore := newTestStore(t)

	csvContent := "driver_id,fare_amount,trip_duration_minutes,completed_at\n" +
		"d1,10.0,20.0,2025-01-01T11:00:00Z\n" +
		"d1,20.0,40.0,2025-01-01T11:30:00Z\n" +
		"d1,99.0,99.0,2025-01-01T10:15:00Z\n" +
		"d1,99.0,99.0,9999-01-01T10:15:00Z\n" +
		"d2,15.0,30.0,2025-01-01T11:15:00Z\n" +
		"d3,5.0,10.0,2025-01-01T11:15:00Z\n"

	tempDir := t.TempDir()
	csvPath := filepath.Join(tempDir, "taxi.csv")
	if err := os.WriteFile(csvPath, []byte(csvContent), 0o644); err != nil {
		t.Fatalf("writing temp CSV: %v", err)
	}

	reg, err := registry.Load(registryPath)
	if err != nil {
		t.Fatalf("loading registry: %v", err)
	}

	offlineStore := &offline.ParquetStore{BasePath: tempDir}

	if err := materialization.Materialize(reg, offlineStore, onlineStore, csvPath); err != nil {
		t.Fatalf("Materialize: %v", err)
	}

	expected := map[string]driverExpected{
		"d1": {tripCount: 2, avgFare: 15.0, avgDur: 30.0},
		"d2": {tripCount: 1, avgFare: 15.0, avgDur: 30.0},
		"d3": {tripCount: 1, avgFare: 5.0, avgDur: 10.0},
	}

	var view *registry.FeatureView
	for i := range reg.FeatureViews {
		if reg.FeatureViews[i].Name == "driver_stats" {
			view = &reg.FeatureViews[i]
			break
		}
	}
	if view == nil {
		t.Fatal("driver_stats feature view not found in registry")
	}

	asOf := time.Date(2025, time.January, 1, 12, 0, 0, 0, time.UTC)

	for driverID, want := range expected {
		t.Run(driverID, func(t *testing.T) {
			entityEvents := []historical.EntityEvent{
				{EntityID: driverID, EventTimestamp: asOf},
			}
			trainingRows, err := historical.GetHistoricalFeatures(offlineStore, *view, entityEvents)
			if err != nil {
				t.Fatalf("GetHistoricalFeatures: %v", err)
			}
			if len(trainingRows) != 1 {
				t.Fatalf("expected 1 training row, got %d", len(trainingRows))
			}
			offlineFeatures := trainingRows[0].Features
			if offlineFeatures == nil {
				t.Fatal("offline features are nil (TTL may have filtered the row)")
			}

			onlineFeatures, err := onlineStore.Get(driverID)
			if err != nil {
				t.Fatalf("online Get: %v", err)
			}

			assertFloatFeature(t, "offline", "trip_count", offlineFeatures, want.tripCount)
			assertFloatFeature(t, "offline", "avg_fare", offlineFeatures, want.avgFare)
			assertFloatFeature(t, "offline", "avg_trip_duration_minutes", offlineFeatures, want.avgDur)

			assertFloatFeature(t, "online", "trip_count", onlineFeatures, want.tripCount)
			assertFloatFeature(t, "online", "avg_fare", onlineFeatures, want.avgFare)
			assertFloatFeature(t, "online", "avg_trip_duration_minutes", onlineFeatures, want.avgDur)
		})
	}
}

func assertFloatFeature(t *testing.T, store, name string, features map[string]any, want float64) {
	t.Helper()
	raw, ok := features[name]
	if !ok {
		t.Errorf("%s: feature %q missing", store, name)
		return
	}
	got, ok := raw.(float64)
	if !ok {
		t.Errorf("%s: feature %q: expected float64, got %T (%v)", store, name, raw, raw)
		return
	}
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("%s: feature %q: got %v, want %v", store, name, got, want)
	}
}
