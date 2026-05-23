# Context: Phase 6 — HTTP API

## Where We Are

Phases 1–5 complete:
- **Phase 1**: Registry — `internal/registry` loads YAML into typed structs
- **Phase 2**: Offline store — `internal/offline` reads/writes Parquet
- **Phase 3**: Point-in-time joins — `internal/historical.GetHistoricalFeatures`
- **Phase 4**: Online store — `internal/online.RedisStore` (Set/Get/MGet)
- **Phase 5**: Materialization — `internal/materialization.Materialize` populates both stores from CSV

## Key Interface Contracts

### online.RedisStore.Get
```go
func (s *RedisStore) Get(entityID string) (map[string]any, error)
```
Returns **empty map** (not nil, not error) when the key is absent in Redis. So 200 with `{}` features is correct for unknown entities — no 404 needed.

### historical.GetHistoricalFeatures
```go
func GetHistoricalFeatures(
    store *offline.ParquetStore,
    featureView registry.FeatureView,
    entityRows []EntityEvent,
) ([]TrainingRow, error)
```
`TrainingRow.Features` is nil when no feature row matched (leakage/TTL filtered). Convert to empty map before serializing.

### registry.FeatureView lookup pattern
```go
for i := range reg.FeatureViews {
    if reg.FeatureViews[i].Name == "driver_stats" {
        view = &reg.FeatureViews[i]
        break
    }
}
```

## Feature Registry (configs/feature_registry.yaml)
Feature view name: `driver_stats`. Source: `data/driver_stats.parquet`. TTL: `24h`.

## Files to Implement (empty stubs)
- `internal/server/handlers.go`
- `internal/server/schemas.go`
- `cmd/server/main.go`

## Module Path
`github.com/Raditya0902/mini-feature-store`

## Do NOT Touch
- `internal/registry/`, `internal/offline/`, `internal/online/`, `internal/historical/`, `internal/materialization/`
- Any existing test files
