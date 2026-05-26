package tests

import (
	"fmt"
	"math/rand"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/Raditya0902/mini-feature-store/internal/historical"
	"github.com/Raditya0902/mini-feature-store/internal/offline"
	"github.com/Raditya0902/mini-feature-store/internal/registry"
)

const (
	NUM_ENTITIES      = 10_000
	ROWS_PER_ENTITY   = 10
	TOTAL_ROWS        = NUM_ENTITIES * ROWS_PER_ENTITY
	NUM_EVENTS        = 1_000
	WINDOW_HOURS      = 24
	SCALE_TTL         = 48 * time.Hour
	SCALE_SOURCE      = "scale_driver_stats.parquet"
)

func TestPointInTimeScale(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	base := time.Now().UTC().Add(-time.Duration(WINDOW_HOURS) * time.Hour)

	// Generate 100,000 feature rows: 10 per entity across 10,000 entities
	featureRows := make([]offline.FeatureRow, 0, TOTAL_ROWS)
	for entityIdx := 0; entityIdx < NUM_ENTITIES; entityIdx++ {
		entityID := fmt.Sprintf("driver_%05d", entityIdx)
		for j := 0; j < ROWS_PER_ENTITY; j++ {
			offsetSec := rng.Int63n(int64(WINDOW_HOURS * 3600))
			ts := base.Add(time.Duration(offsetSec) * time.Second)
			featureRows = append(featureRows, offline.FeatureRow{
				EntityID:         entityID,
				FeatureTimestamp: ts,
				Values:           map[string]any{"avg_fare": rng.Float64() * 100},
			})
		}
	}

	// Write to temp Parquet file
	dir := t.TempDir()
	path := filepath.Join(dir, SCALE_SOURCE)
	if err := offline.Write(path, featureRows); err != nil {
		t.Fatalf("offline.Write: %v", err)
	}
	store := &offline.ParquetStore{BasePath: dir}

	view := registry.FeatureView{
		Name:     "driver_stats_scale",
		Entity:   "driver",
		Source:   SCALE_SOURCE,
		TTL:      registry.Duration{Duration: SCALE_TTL},
		Features: []registry.Feature{{Name: "avg_fare", Dtype: "float64"}},
	}

	// Generate 1,000 entity events from the same pool
	events := make([]historical.EntityEvent, NUM_EVENTS)
	for i := 0; i < NUM_EVENTS; i++ {
		entityIdx := rng.Intn(NUM_ENTITIES)
		events[i] = historical.EntityEvent{
			EntityID:       fmt.Sprintf("driver_%05d", entityIdx),
			EventTimestamp: time.Now().UTC(),
		}
	}

	t.Logf("entities: %d, feature rows: %d, events: %d", NUM_ENTITIES, TOTAL_ROWS, NUM_EVENTS)

	// Capture heap before
	var memBefore, memAfter runtime.MemStats
	runtime.ReadMemStats(&memBefore)

	start := time.Now()
	results, err := historical.GetHistoricalFeatures(store, view, events)
	elapsed := time.Since(start)

	runtime.ReadMemStats(&memAfter)

	if err != nil {
		t.Fatalf("GetHistoricalFeatures: %v", err)
	}

	// Assert all 1,000 training rows are returned
	if len(results) != NUM_EVENTS {
		t.Fatalf("expected %d training rows, got %d", NUM_EVENTS, len(results))
	}

	heapDeltaBytes := int64(memAfter.HeapAlloc) - int64(memBefore.HeapAlloc)
	mbDelta := float64(heapDeltaBytes) / (1024 * 1024)

	t.Logf("elapsed: %s", elapsed)
	t.Logf("heap delta: %.1f MB", mbDelta)
	t.Logf("scale: 100k rows / 1k events — join completed in %s, heap delta %.1f MB", elapsed, mbDelta)
}
