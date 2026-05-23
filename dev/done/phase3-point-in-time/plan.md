# Plan: Phase 3 — internal/historical (Point-in-Time Join)

## Context

Phase 3 implements the core correctness guarantee of the feature store: point-in-time correct
historical joins. Given a list of past events (entity + timestamp), it retrieves the feature
values that were known at each event's moment in time — no future data leakage, no stale data
beyond the TTL. This is the central correctness rule in CLAUDE.md.

---

## Step 0 — Minimal addition to `internal/offline/parquet_store.go`

Append `ParquetStore` struct (typed handle for the historical package):

```go
type ParquetStore struct {
    BasePath string
}
```

No methods, no constructor. `GetHistoricalFeatures` calls `offline.Read` directly via
`filepath.Join(store.BasePath, featureView.Source)`.

---

## `internal/historical/point_in_time.go` (~75 lines)

**Exported types:**
```go
type EntityEvent struct {
    EntityID       string
    EventTimestamp time.Time
}

type TrainingRow struct {
    EntityID       string
    EventTimestamp time.Time
    Features       map[string]any // nil when no qualifying row exists
}
```

**Public function:**
```go
func GetHistoricalFeatures(
    store *offline.ParquetStore,
    featureView registry.FeatureView,
    entityRows []EntityEvent,
) ([]TrainingRow, error)
```

**Algorithm:**
1. `path := filepath.Join(store.BasePath, featureView.Source)`
2. `rows, err := offline.Read(path)` — propagate error
3. Group rows by EntityID → `map[string][]offline.FeatureRow` (single pass, O(M))
4. For each EntityEvent, scan its group:
   - Skip if `row.FeatureTimestamp.After(event.EventTimestamp)` → leakage guard
   - Skip if `event.EventTimestamp.Sub(row.FeatureTimestamp) > featureView.TTL.Duration` → TTL guard
   - Track row with latest FeatureTimestamp (`best`)
5. Append TrainingRow with `Features = best.Values` or nil if no best

Unexported `joinRows(rows, ttl, events)` contains steps 3–5.

---

## `tests/point_in_time_test.go` (~130 lines)

**Helpers:**
- `setupStore(t, rows, source)` — writes to `<tempDir>/<source>`, returns `&ParquetStore{BasePath: tempDir}`
- `makeView(source, ttl)` — builds a `registry.FeatureView`

**`TestPointInTime`** — 6 table-driven subtests:

| # | name | setup | expected |
|---|------|-------|----------|
| 1 | basic join | e1@now-1h, avg_fare=10.0 | TrainingRow with avg_fare=10.0 |
| 2 | no leakage | e1@now+1h (future) | Features=nil |
| 3 | TTL filter | e1@now-25h (too old, TTL=24h) | Features=nil |
| 4 | latest-wins | e1@now-2h and e1@now-1h | Features from now-1h row |
| 5 | no match | no rows for e1 | Features=nil |
| 6 | multiple entities | e1,e2 have rows; e3 has none | e3 Features=nil |

All timestamps at whole-second precision. `reflect.DeepEqual` for Features.

---

## Verification

```bash
go build ./internal/offline/...
go build ./internal/historical/...
go test ./tests/ -run TestPointInTime -v
go test ./tests/ -v
```
