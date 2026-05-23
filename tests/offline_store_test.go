package tests

import (
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/Raditya0902/mini-feature-store/internal/offline"
)

func TestOfflineRoundTrip(t *testing.T) {
	// Timestamps with sub-microsecond precision to verify truncation behaviour.
	ts1 := time.Unix(1700000000, 1999).UTC() // 1999 ns → truncates to 1 µs on store
	ts2 := time.Unix(1700001000, 0).UTC()
	ts3 := time.Unix(1700002000, 500000).UTC() // 500 µs — exact after truncation

	cases := []struct {
		name string
		rows []offline.FeatureRow
	}{
		{
			name: "single row with sub-microsecond timestamp",
			rows: []offline.FeatureRow{
				{
					EntityID:         "driver_1",
					FeatureTimestamp: ts1,
					Values: map[string]any{
						"avg_fare":   float64(12.5),
						"trip_count": float64(42),
					},
				},
			},
		},
		{
			name: "multiple rows with mixed value types",
			rows: []offline.FeatureRow{
				{
					EntityID:         "driver_1",
					FeatureTimestamp: ts1,
					Values:           map[string]any{"avg_fare": float64(12.5), "zone": "city"},
				},
				{
					EntityID:         "driver_2",
					FeatureTimestamp: ts2,
					Values:           map[string]any{"avg_fare": float64(18.0), "zone": "suburb"},
				},
				{
					EntityID:         "driver_3",
					FeatureTimestamp: ts3,
					Values:           map[string]any{"avg_fare": float64(9.99), "zone": "airport"},
				},
			},
		},
		{
			name: "empty slice",
			rows: []offline.FeatureRow{},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "test.parquet")

			if err := offline.Write(path, tc.rows); err != nil {
				t.Fatalf("Write: %v", err)
			}

			got, err := offline.Read(path)
			if err != nil {
				t.Fatalf("Read: %v", err)
			}

			if len(got) != len(tc.rows) {
				t.Fatalf("row count: got %d, want %d", len(got), len(tc.rows))
			}

			for i, want := range tc.rows {
				if got[i].EntityID != want.EntityID {
					t.Errorf("row %d EntityID: got %q, want %q", i, got[i].EntityID, want.EntityID)
				}

				wantTs := want.FeatureTimestamp.UTC().Truncate(time.Microsecond)
				if !got[i].FeatureTimestamp.Equal(wantTs) {
					t.Errorf("row %d FeatureTimestamp: got %v, want %v", i, got[i].FeatureTimestamp, wantTs)
				}

				if !reflect.DeepEqual(got[i].Values, want.Values) {
					t.Errorf("row %d Values: got %v, want %v", i, got[i].Values, want.Values)
				}
			}
		})
	}
}
