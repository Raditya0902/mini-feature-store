package tests

import (
	"fmt"
	"math/rand"
	"reflect"
	"testing"
	"time"

	"github.com/Raditya0902/mini-feature-store/internal/historical"
	"github.com/Raditya0902/mini-feature-store/internal/offline"
)

func TestPointInTimeAgainstOracle(t *testing.T) {
	const ttl = 24 * time.Hour
	base := time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC)
	for _, seed := range []int64{1, 42, 2026} {
		t.Run(fmt.Sprintf("seed_%d", seed), func(t *testing.T) {
			t.Logf("random seed: %d", seed)
			rng := rand.New(rand.NewSource(seed))
			var rows []offline.FeatureRow
			var events []historical.EntityEvent
			for i := 0; i < 2500; i++ {
				rows = append(rows, offline.FeatureRow{
					EntityID:         fmt.Sprintf("driver_%d", rng.Intn(20)),
					FeatureTimestamp: base.Add(time.Duration(rng.Intn(72)) * time.Hour),
					Values:           map[string]any{"row_id": float64(i)},
				})
			}
			for i := 0; i < 250; i++ {
				events = append(events, historical.EntityEvent{
					EntityID:       fmt.Sprintf("driver_%d", rng.Intn(22)),
					EventTimestamp: base.Add(time.Duration(rng.Intn(100)) * time.Hour),
				})
			}

			const boundaryDriver = "boundary_driver"
			next := base.Add(48 * time.Hour)
			rows = append(rows,
				offline.FeatureRow{EntityID: boundaryDriver, FeatureTimestamp: base, Values: map[string]any{"row_id": -1.0}},
				offline.FeatureRow{EntityID: boundaryDriver, FeatureTimestamp: base, Values: map[string]any{"row_id": -2.0}},
				offline.FeatureRow{EntityID: boundaryDriver, FeatureTimestamp: next, Values: map[string]any{"row_id": -3.0}},
			)
			boundaryTimes := []time.Time{
				base.Add(-time.Nanosecond),
				base,
				base,
				base.Add(ttl),
				base.Add(ttl + time.Nanosecond),
				next.Add(-time.Nanosecond),
				next,
			}
			for _, ts := range boundaryTimes {
				events = append(events, historical.EntityEvent{EntityID: boundaryDriver, EventTimestamp: ts})
			}
			rng.Shuffle(len(rows), func(i, j int) { rows[i], rows[j] = rows[j], rows[i] })
			rng.Shuffle(len(events), func(i, j int) { events[i], events[j] = events[j], events[i] })

			const src = "oracle.parquet"
			store := setupStore(t, rows, src)
			got, err := historical.GetHistoricalFeatures(store, makeView(src, ttl), events)
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != len(events) {
				t.Fatalf("got %d rows, want %d", len(got), len(events))
			}
			for i, event := range events {
				want := historical.TrainingRow{
					EntityID:       event.EntityID,
					EventTimestamp: event.EventTimestamp,
					Features:       referenceFeatures(rows, event, ttl),
				}
				if !reflect.DeepEqual(got[i], want) {
					t.Errorf("event %d: got %+v, want %+v", i, got[i], want)
				}
			}
		})
	}
}

func referenceFeatures(rows []offline.FeatureRow, event historical.EntityEvent, ttl time.Duration) map[string]any {
	earliest := event.EventTimestamp.Add(-ttl)
	var best *offline.FeatureRow
	for i := range rows {
		row := &rows[i]
		if row.EntityID != event.EntityID || row.FeatureTimestamp.Before(earliest) || row.FeatureTimestamp.After(event.EventTimestamp) {
			continue
		}
		// Equal timestamps keep the first matching row in file order.
		if best == nil || best.FeatureTimestamp.Before(row.FeatureTimestamp) {
			best = row
		}
	}
	if best == nil {
		return nil
	}
	return best.Values
}
