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

	csvContent := "driver_id,fare_amount,trip_duration_minutes\n" +
		"d1,10.0,20.0\n" +
		"d1,20.0,40.0\n" +
		"d2,15.0,30.0\n" +
		"d3,5.0,10.0\n"

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

	now := time.Now().UTC()

	for driverID, want := range expected {
		t.Run(driverID, func(t *testing.T) {
			entityEvents := []historical.EntityEvent{
				{EntityID: driverID, EventTimestamp: now},
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

			assertFloatFeature(t, "online", "trip_count", onlineFeatures, want.tripCount)
			assertFloatFeature(t, "online", "avg_fare", onlineFeatures, want.avgFare)

			offlineFare := offlineFeatures["avg_fare"].(float64)
			onlineFare := onlineFeatures["avg_fare"].(float64)
			if math.Abs(offlineFare-onlineFare) > 1e-9 {
				t.Errorf("consistency: avg_fare offline=%v online=%v", offlineFare, onlineFare)
			}

			offlineCount := offlineFeatures["trip_count"].(float64)
			onlineCount := onlineFeatures["trip_count"].(float64)
			if math.Abs(offlineCount-onlineCount) > 1e-9 {
				t.Errorf("consistency: trip_count offline=%v online=%v", offlineCount, onlineCount)
			}
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
