# Context: Phase 2 — Offline Store

## Module

`github.com/Raditya0902/mini-feature-store`

## Go Version

`go 1.26.1`

## Dependencies Added

| Package | Version | Purpose |
|---------|---------|---------|
| `github.com/parquet-go/parquet-go` | v0.30.1 | Parquet file read/write |

## FeatureRow Schema Decisions

### Values encoding: JSON string column
`map[string]any` has no native Parquet type. Options considered:
- **Parquet VARIANT** — draft spec, not universally supported across readers
- **JSON string column** ✓ — portable, simple, works with standard library

Downside: integer values decode as `float64` (standard `json.Unmarshal` behaviour). Acceptable
for this project — NYC taxi features are float64 or int-as-float64.

### Timestamp storage: Unix microseconds (int64)
Parquet's standard TIMESTAMP precision is TIMESTAMP_MICROS (microseconds). Storing as
`FeatureTimestamp.UnixMicro()` and reconstructing with `time.UnixMicro(ts).UTC()` ensures
round-trip fidelity at microsecond resolution. Sub-microsecond precision is truncated — the
test accounts for this explicitly.

### Directory creation
`Write` calls `os.MkdirAll(filepath.Dir(path), 0o755)` so callers can pass a deep path like
`data/offline_store/driver_stats.parquet` without pre-creating directories.

## Key Design Constraints

- `FeatureRow` is in `package offline` — not imported from or dependent on `internal/registry`
- No Parquet schema type annotations beyond struct field tags (`parquet:"name"`)
- `parquetRow` is unexported — callers only see `FeatureRow`
