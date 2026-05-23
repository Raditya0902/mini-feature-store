# Plan: Phase 5 — Materialization Pipeline

## Context

Phase 5 closes the loop between raw data and both stores. It adds:
- A CSV-to-feature-row computation layer (`taxi_features.go`)
- An orchestration runner that writes offline + online in one call (`runner.go`)
- A thin CLI entry point (`cmd/materialize/main.go`)
- A consistency test that verifies offline and online match after materialization (`tests/consistency_test.go`)

No new dependencies. Uses `encoding/csv` from stdlib. Parquet path comes from the registry `source` field (e.g. `data/driver_stats.parquet`), written relative to `ParquetStore.BasePath`.

---

## `internal/materialization/taxi_features.go`

### Signature
```go
func GenerateDriverStats(rawPath string) ([]offline.FeatureRow, error)
```

### Logic

1. Open `rawPath` with `os.Open`; return wrapped error on failure.
2. Create `csv.NewReader(f)`.
3. Read header row; validate columns `driver_id`, `fare_amount`, `trip_duration_minutes` are present (any order). Return error if any is missing.
4. Accumulate per-driver stats in a local struct:
   ```go
   type driverAcc struct {
       tripCount           int
       totalFare           float64
       totalTripDuration   float64
   }
   ```
5. For each data row: parse `fare_amount` and `trip_duration_minutes` as `float64`; skip row with a logged warning if either fails to parse (never silently drop). Increment accumulator.
6. After all rows, compute per driver:
   - `trip_count` = `float64(acc.tripCount)` (dtype is float64-compatible at JSON/Parquet boundary; stored as `float64` for consistency)
   - `avg_fare` = `acc.totalFare / float64(acc.tripCount)`
   - `avg_trip_duration_minutes` = `acc.totalTripDuration / float64(acc.tripCount)`
7. Map key names: `"trip_count"`, `"avg_fare"`, `"avg_trip_duration_minutes"` — must match `configs/feature_registry.yaml` exactly.
8. `FeatureTimestamp` = `time.Now().UTC().Truncate(time.Second)` (same value for all rows in one run).
9. Return `[]offline.FeatureRow`, one per driver. Order is deterministic (sort by `driver_id` before building slice).

### Edge cases
- Empty CSV (header only): return empty slice, no error.
- Unparseable numeric field: skip that CSV row; do not return error.
- Zero-row driver (impossible by construction, but guard: skip if `tripCount == 0`).

---

## `internal/materialization/runner.go`

### Signature
```go
func Materialize(
    reg *registry.Registry,
    store *offline.ParquetStore,
    onlineStore *online.RedisStore,
    rawPath string,
) error
```

### Logic

1. Call `GenerateDriverStats(rawPath)`. Return `fmt.Errorf("generating driver stats: %w", err)` on failure.
2. Find the `driver_stats` feature view in `reg.FeatureViews`. Return error if not found.
3. Build Parquet path: `filepath.Join(store.BasePath, view.Source)`.
4. Call `offline.Write(parquetPath, rows)`. Return `fmt.Errorf("writing offline store: %w", err)` on failure.
5. For each row, call `onlineStore.Set(row.EntityID, row.Values)`. Return `fmt.Errorf("writing online store for %q: %w", row.EntityID, err)` on first error.
6. Return `nil` on success.

### Notes
- No partial rollback — if online Set fails midway, offline Parquet has already been written. This is acceptable for a non-transactional store; the caller can retry.
- `BasePath` is used as the working root for the Parquet path, matching how `GetHistoricalFeatures` resolves paths.

---

## `cmd/materialize/main.go`

```go
package main

const (
    registryPath = "configs/feature_registry.yaml"
    rawDataPath  = "data/raw/taxi.csv"
    redisAddr    = "localhost:6379"
    parquetBase  = "."
)
```

Steps:
1. Load registry via `registry.Load(registryPath)`.
2. Construct `offline.ParquetStore{BasePath: parquetBase}`.
3. Construct `online.NewRedisStore(redisAddr)`.
4. Call `materialization.Materialize(reg, store, onlineStore, rawDataPath)`.
5. On error: `fmt.Fprintf(os.Stderr, "materialize: %v\n", err); os.Exit(1)`.
6. On success: `fmt.Println("materialization complete")`.

---

## `tests/consistency_test.go`

### Skip guard
Reuse `newTestStore(t)` helper defined in `online_store_test.go` — it pings Redis and calls `t.Skip` if unavailable. `consistency_test.go` is in the same `package tests`, so the helper is visible.

### TestConsistency

**Arrange:**
1. Write a temp CSV with synthetic driver rows (3 drivers, deterministic values):
   ```
   driver_id,fare_amount,trip_duration_minutes
   d1,10.0,20.0
   d1,20.0,40.0
   d2,15.0,30.0
   d3,5.0,10.0
   ```
2. Build expected values map per driver (hand-computed):
   - `d1`: `trip_count=2`, `avg_fare=15.0`, `avg_trip_duration_minutes=30.0`
   - `d2`: `trip_count=1`, `avg_fare=15.0`, `avg_trip_duration_minutes=30.0`
   - `d3`: `trip_count=1`, `avg_fare=5.0`, `avg_trip_duration_minutes=10.0`
3. Write Parquet to a `t.TempDir()` path (avoids polluting `data/`).
4. Load real registry from `../configs/feature_registry.yaml` to get `source` field and TTL.
5. Construct `offline.ParquetStore{BasePath: tempDir}`.
6. Construct online store via `newTestStore(t)`.

**Act:**
Call `materialization.Materialize(reg, offlineStore, onlineStore, tempCSV)`.

**Assert — offline:**
For each driver, call `historical.GetHistoricalFeatures` with `EventTimestamp = time.Now().UTC()`. Verify returned `TrainingRow.Features["avg_fare"]` and `TrainingRow.Features["trip_count"]` match expected.

**Assert — online:**
For each driver, call `onlineStore.Get(driverID)`. Verify `avg_fare` and `trip_count` match expected.

**Assert — consistency:**
The offline and online values for `avg_fare` and `trip_count` must be identical (same source data, same run).

### Float comparison
Use `math.Abs(got - want) < 1e-9` for float64 comparisons, not `==`.

---

## Verification

```bash
go build ./...
go test ./tests/ -run TestConsistency -v
go test ./... -v
```
