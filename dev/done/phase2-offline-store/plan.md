# Plan: Phase 2 — internal/offline (Parquet Store)

## Context

Phase 2 adds the offline store: reading and writing `FeatureRow` records to Parquet files.
Parquet is a typed columnar format; `map[string]any` has no native Parquet type, so the
values map is JSON-encoded into a `string` column. Timestamps are stored as Unix microseconds
(int64) — Parquet's standard TIMESTAMP_MICROS precision — and truncated accordingly.

Dependency: `github.com/parquet-go/parquet-go` (v0.30.1 already in module cache).

---

## Files to implement

### `internal/offline/parquet_store.go` (~80 lines)

**Public API:**
```go
type FeatureRow struct {
    EntityID         string
    FeatureTimestamp time.Time
    Values           map[string]any
}

func Write(path string, rows []FeatureRow) error
func Read(path string) ([]FeatureRow, error)
```

**Internal Parquet schema (unexported):**
```go
type parquetRow struct {
    EntityID         string `parquet:"entity_id"`
    FeatureTimestamp int64  `parquet:"feature_timestamp"` // Unix microseconds
    Values           string `parquet:"values"`            // JSON-encoded map[string]any
}
```

**Write:**
1. `os.MkdirAll(filepath.Dir(path), 0o755)` — create parent directories as needed
2. `os.Create(path)` — creates or truncates
3. `parquet.NewGenericWriter[parquetRow](f)`
4. For each row: `json.Marshal(row.Values)` → string; `row.FeatureTimestamp.UnixMicro()` → int64
5. `writer.Write(prows)` → `writer.Close()` → `f.Close()`

**Read:**
1. `os.Open(path)` + `f.Stat()` to get size
2. `parquet.OpenFile(f, stat.Size())` → `parquet.NewGenericReader[parquetRow](pf)`
3. `make([]parquetRow, reader.NumRows())` → `reader.Read(prows)` → `reader.Close()`
4. For each row: `json.Unmarshal` → `Values`; `time.UnixMicro(p.FeatureTimestamp).UTC()` → `FeatureTimestamp`

All errors wrapped with context.

---

### `tests/offline_store_test.go` (~80 lines)

**`TestOfflineRoundTrip`** — table-driven:

| case | description |
|------|-------------|
| single row | 1 row, float64 + string values |
| multiple rows | 3 rows, different entity IDs and timestamps |
| empty slice | 0 rows → write succeeds, read returns empty slice |

**Timestamp precision:** input timestamps are created with `time.Unix(ts, 500).UTC()` (has
nanoseconds), stored, then compared against `.Truncate(time.Microsecond)` of the original.

**Values:** use `float64` and `string` only — JSON round-trip is exact for these types.
(integer values normalise to `float64` via `json.Unmarshal`, so tests avoid raw `int64`.)

**Setup:** all paths via `filepath.Join(t.TempDir(), "test.parquet")` — no `data/` dir written.

---

## Verification

```bash
go get github.com/parquet-go/parquet-go
go build ./internal/offline/...
go test ./tests/ -run TestOffline -v
go test ./...
```
