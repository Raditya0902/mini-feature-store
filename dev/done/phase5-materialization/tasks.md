# Tasks: Phase 5 — Materialization Pipeline

## Status Legend
- [ ] Not started
- [~] In progress
- [x] Complete

---

## Implementation

- [x] `internal/materialization/taxi_features.go` — GenerateDriverStats: open CSV, parse header, accumulate per-driver stats, return sorted FeatureRows
- [x] `internal/materialization/runner.go` — Materialize: call GenerateDriverStats, write offline (Parquet), write online (Redis)
- [x] `cmd/materialize/main.go` — thin main: load registry, construct stores, call Materialize, print result

## Tests

- [x] `tests/consistency_test.go` — TestConsistency: write temp CSV, call Materialize, assert offline and online values match

## Verification

- [x] `go build ./internal/materialization/... ./cmd/materialize/...` passes
- [x] `go test ./tests/ -run TestConsistency -v` — skips correctly when Redis unavailable
- [x] `go test ./tests/ -run 'TestOffline|TestRegistry|TestPointInTime' -v` — all pass
- [ ] `go test ./tests/ -run TestConsistency -v` passes (needs Docker/Redis running)
