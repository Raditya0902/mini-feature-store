# Context: Phase 3 — Point-in-Time Join

## Module

`github.com/Raditya0902/mini-feature-store`

## Go Version

`go 1.26.1`

## No New Dependencies

All imports are internal packages or standard library (`path/filepath`, `time`).

## Package Dependency Graph

```
internal/historical
  → internal/offline  (FeatureRow, ParquetStore, Read)
  → internal/registry (FeatureView, Duration)
```

No circular dependencies.

## Key Design Decisions

### ParquetStore is a plain struct
Added to `internal/offline/parquet_store.go` with a single `BasePath string` field.
`GetHistoricalFeatures` constructs the full path as `filepath.Join(store.BasePath, featureView.Source)`.
This avoids adding methods to the offline package per YAGNI.

### joinRows is unexported
The core join logic is in an unexported `joinRows` helper so `GetHistoricalFeatures` stays
a thin error-handling wrapper. Easy to unit test via the public function.

### Algorithm: group-then-scan
Feature rows are grouped by EntityID once (O(M)), then each entity event scans only its own
group. This avoids O(N*M) full scans and is simple enough for the mini feature store scale.

### Correctness invariants (from CLAUDE.md)
- **No leakage**: `row.FeatureTimestamp.After(event.EventTimestamp)` rows are skipped
- **TTL**: `event.EventTimestamp.Sub(row.FeatureTimestamp) > ttl` rows are skipped
- **Latest-wins**: among qualifying rows, the one with the greatest `FeatureTimestamp` wins

### nil Features on no match
If no qualifying row exists for an EntityEvent, `TrainingRow.Features` is `nil` (not an empty map).
Tests check `row.Features == nil`.

### Timestamp precision in tests
All test timestamps use `time.Unix(epoch, 0).UTC()` (whole-second precision).
Feature rows are written via `offline.Write` (truncates to microseconds on Parquet storage) but
since all test timestamps are whole-second, the round-trip is lossless.
