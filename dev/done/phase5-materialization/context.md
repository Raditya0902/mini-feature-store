# Context: Phase 5 — Materialization Pipeline

## Where We Are

Phases 1–4 are complete:
- **Phase 1**: Registry — `internal/registry` loads `configs/feature_registry.yaml` into typed structs
- **Phase 2**: Offline store — `internal/offline` reads/writes Parquet files; `FeatureRow` is the shared data type
- **Phase 3**: Point-in-time joins — `internal/historical` does leakage-safe, TTL-filtered joins
- **Phase 4**: Online store — `internal/online` wraps Redis with `Set`/`Get`/`MGet`; JSON-encoded `map[string]any` per entity

## What Phase 5 Adds

The materialization pipeline is the bridge between raw source data and both stores. After this phase:
- Raw CSV → computed `FeatureRow` slices (taxi_features.go)
- Computed rows → Parquet file (offline) + Redis keys (online)
- Both stores are consistent and queryable immediately after a single `Materialize` call

## Key Interface Contracts

### offline.FeatureRow (from parquet_store.go)
```go
type FeatureRow struct {
    EntityID         string
    FeatureTimestamp time.Time
    Values           map[string]any
}
```

### offline.Write (from parquet_store.go)
```go
func Write(path string, rows []FeatureRow) error
```
Takes an absolute-or-relative path directly. The ParquetStore.BasePath must be joined with the view.Source field before calling.

### offline.ParquetStore (from parquet_store.go)
```go
type ParquetStore struct { BasePath string }
```
Used by GetHistoricalFeatures as the root for path resolution:
```go
path := filepath.Join(store.BasePath, featureView.Source)
```

### online.RedisStore.Set (from redis_store.go)
```go
func (s *RedisStore) Set(entityID string, features map[string]any) error
```

### registry.Registry (from model.go)
```go
type FeatureView struct {
    Name     string
    Entity   string
    Source   string    // e.g. "data/driver_stats.parquet"
    TTL      Duration
    Features []Feature
}
```

## Feature Registry (configs/feature_registry.yaml)
```yaml
feature_views:
  - name: driver_stats
    entity: driver
    source: data/driver_stats.parquet
    ttl: 24h
    features:
      - name: trip_count
        dtype: int64
      - name: avg_fare
        dtype: float64
      - name: avg_trip_duration_minutes
        dtype: float64
```

Feature name keys in Values map must be exactly: `trip_count`, `avg_fare`, `avg_trip_duration_minutes`.

## Important: dtype mismatch
The registry declares `trip_count` as `int64`, but the Values map stores it as `float64` to be consistent with JSON round-trips (JSON numbers decode as float64 in Go's `map[string]any`). The consistency test must account for this: compare as float64.

## Module Path
`github.com/Raditya0902/mini-feature-store` (from go.mod)

## Redis Skip Pattern (from online_store_test.go)
```go
func newTestStore(t *testing.T) *online.RedisStore {
    t.Helper()
    c := redis.NewClient(&redis.Options{Addr: redisAddr})
    defer c.Close()
    if err := c.Ping(context.Background()).Err(); err != nil {
        t.Skipf("Redis unavailable at %s: %v", redisAddr, err)
    }
    return online.NewRedisStore(redisAddr)
}
```
`consistency_test.go` is in `package tests` and can reuse `newTestStore` directly.

## Files to Create (empty stubs already exist)
- `internal/materialization/runner.go` — empty
- `internal/materialization/taxi_features.go` — empty
- `cmd/materialize/main.go` — empty (in `cmd/materialize/` directory)
- `tests/consistency_test.go` — has `package tests` only

## Files NOT to Touch
- `internal/registry/` — complete, stable
- `internal/historical/` — complete, stable
- `internal/offline/` — complete, stable
- `internal/online/` — complete, stable
- `internal/server/` — complete, stable
- Any existing test files
