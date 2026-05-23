package tests

import (
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/Raditya0902/mini-feature-store/internal/historical"
	"github.com/Raditya0902/mini-feature-store/internal/offline"
	"github.com/Raditya0902/mini-feature-store/internal/registry"
)

func setupStore(t *testing.T, rows []offline.FeatureRow, source string) *offline.ParquetStore {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, source)
	if err := offline.Write(path, rows); err != nil {
		t.Fatalf("setupStore: Write: %v", err)
	}
	return &offline.ParquetStore{BasePath: dir}
}

func makeView(source string, ttl time.Duration) registry.FeatureView {
	return registry.FeatureView{
		Name:     "driver_stats",
		Entity:   "driver",
		Source:   source,
		TTL:      registry.Duration{Duration: ttl},
		Features: []registry.Feature{{Name: "avg_fare", Dtype: "float64"}},
	}
}

func TestPointInTime(t *testing.T) {
	now := time.Unix(1700000000, 0).UTC()
	const src = "driver_stats.parquet"
	const ttl24h = 24 * time.Hour

	cases := []struct {
		name         string
		featureRows  []offline.FeatureRow
		entityEvents []historical.EntityEvent
		ttl          time.Duration
		wantRows     []historical.TrainingRow
	}{
		{
			name: "basic join",
			featureRows: []offline.FeatureRow{
				{EntityID: "e1", FeatureTimestamp: now.Add(-1 * time.Hour), Values: map[string]any{"avg_fare": float64(10)}},
			},
			entityEvents: []historical.EntityEvent{
				{EntityID: "e1", EventTimestamp: now},
			},
			ttl: ttl24h,
			wantRows: []historical.TrainingRow{
				{EntityID: "e1", EventTimestamp: now, Features: map[string]any{"avg_fare": float64(10)}},
			},
		},
		{
			name: "no leakage",
			featureRows: []offline.FeatureRow{
				{EntityID: "e1", FeatureTimestamp: now.Add(1 * time.Hour), Values: map[string]any{"avg_fare": float64(10)}},
			},
			entityEvents: []historical.EntityEvent{
				{EntityID: "e1", EventTimestamp: now},
			},
			ttl: ttl24h,
			wantRows: []historical.TrainingRow{
				{EntityID: "e1", EventTimestamp: now, Features: nil},
			},
		},
		{
			name: "TTL filter",
			featureRows: []offline.FeatureRow{
				{EntityID: "e1", FeatureTimestamp: now.Add(-25 * time.Hour), Values: map[string]any{"avg_fare": float64(10)}},
			},
			entityEvents: []historical.EntityEvent{
				{EntityID: "e1", EventTimestamp: now},
			},
			ttl: ttl24h,
			wantRows: []historical.TrainingRow{
				{EntityID: "e1", EventTimestamp: now, Features: nil},
			},
		},
		{
			name: "latest wins",
			featureRows: []offline.FeatureRow{
				{EntityID: "e1", FeatureTimestamp: now.Add(-2 * time.Hour), Values: map[string]any{"avg_fare": float64(5)}},
				{EntityID: "e1", FeatureTimestamp: now.Add(-1 * time.Hour), Values: map[string]any{"avg_fare": float64(10)}},
			},
			entityEvents: []historical.EntityEvent{
				{EntityID: "e1", EventTimestamp: now},
			},
			ttl: ttl24h,
			wantRows: []historical.TrainingRow{
				{EntityID: "e1", EventTimestamp: now, Features: map[string]any{"avg_fare": float64(10)}},
			},
		},
		{
			name:        "no match",
			featureRows: []offline.FeatureRow{},
			entityEvents: []historical.EntityEvent{
				{EntityID: "e1", EventTimestamp: now},
			},
			ttl: ttl24h,
			wantRows: []historical.TrainingRow{
				{EntityID: "e1", EventTimestamp: now, Features: nil},
			},
		},
		{
			name: "multiple entities",
			featureRows: []offline.FeatureRow{
				{EntityID: "e1", FeatureTimestamp: now.Add(-1 * time.Hour), Values: map[string]any{"avg_fare": float64(10)}},
				{EntityID: "e2", FeatureTimestamp: now.Add(-1 * time.Hour), Values: map[string]any{"avg_fare": float64(20)}},
			},
			entityEvents: []historical.EntityEvent{
				{EntityID: "e1", EventTimestamp: now},
				{EntityID: "e2", EventTimestamp: now},
				{EntityID: "e3", EventTimestamp: now},
			},
			ttl: ttl24h,
			wantRows: []historical.TrainingRow{
				{EntityID: "e1", EventTimestamp: now, Features: map[string]any{"avg_fare": float64(10)}},
				{EntityID: "e2", EventTimestamp: now, Features: map[string]any{"avg_fare": float64(20)}},
				{EntityID: "e3", EventTimestamp: now, Features: nil},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := setupStore(t, tc.featureRows, src)
			view := makeView(src, tc.ttl)

			got, err := historical.GetHistoricalFeatures(store, view, tc.entityEvents)
			if err != nil {
				t.Fatalf("GetHistoricalFeatures: %v", err)
			}

			if len(got) != len(tc.wantRows) {
				t.Fatalf("row count: got %d, want %d", len(got), len(tc.wantRows))
			}

			for i, want := range tc.wantRows {
				if got[i].EntityID != want.EntityID {
					t.Errorf("row %d EntityID: got %q, want %q", i, got[i].EntityID, want.EntityID)
				}
				if !got[i].EventTimestamp.Equal(want.EventTimestamp) {
					t.Errorf("row %d EventTimestamp: got %v, want %v", i, got[i].EventTimestamp, want.EventTimestamp)
				}
				if !reflect.DeepEqual(got[i].Features, want.Features) {
					t.Errorf("row %d Features: got %v, want %v", i, got[i].Features, want.Features)
				}
			}
		})
	}
}
