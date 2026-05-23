# Tasks: Phase 2 — Offline Store

## Setup
- [x] `go get github.com/parquet-go/parquet-go`

## Implementation
- [x] `internal/offline/parquet_store.go`
  - [x] `FeatureRow` struct
  - [x] `parquetRow` internal struct with parquet tags
  - [x] `Write(path string, rows []FeatureRow) error`
  - [x] `Read(path string) ([]FeatureRow, error)`
- [x] `tests/offline_store_test.go`
  - [x] `TestOfflineRoundTrip` — single row
  - [x] `TestOfflineRoundTrip` — multiple rows
  - [x] `TestOfflineRoundTrip` — empty slice
  - [x] Timestamp truncation assertion

## Verification
- [x] `go build ./internal/offline/...`
- [x] `go test ./tests/ -run TestOffline -v` — 3/3 subtests pass
- [x] `go test ./...` — tests package green; other failures are pre-existing empty stubs (future phases)
