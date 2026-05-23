# Tasks: Phase 3 — Point-in-Time Join

## Implementation
- [x] `internal/offline/parquet_store.go` — append `ParquetStore` struct
- [x] `internal/historical/point_in_time.go`
  - [x] `EntityEvent` struct
  - [x] `TrainingRow` struct
  - [x] `GetHistoricalFeatures` function
  - [x] `joinRows` helper (unexported)
- [x] `tests/point_in_time_test.go`
  - [x] `setupStore` helper
  - [x] `makeView` helper
  - [x] `TestPointInTime/basic_join`
  - [x] `TestPointInTime/no_leakage`
  - [x] `TestPointInTime/TTL_filter`
  - [x] `TestPointInTime/latest_wins`
  - [x] `TestPointInTime/no_match`
  - [x] `TestPointInTime/multiple_entities`

## Verification
- [x] `go build ./internal/offline/...`
- [x] `go build ./internal/historical/...`
- [x] `go test ./tests/ -run TestPointInTime -v` — 6/6 subtests pass
- [x] `go test ./tests/ -v` — all prior tests still green
