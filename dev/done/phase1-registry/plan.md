# Plan: Phase 1 — internal/registry

## Context

The mini-feature-store project has placeholder (empty) files for all five targets.
Phase 1 establishes the registry foundation — typed structs, YAML loading, and validation —
before any other package is implemented. Nothing outside `internal/registry`, `configs/`,
and `tests/registry_test.go` is touched.

---

## Files to implement

### 1. `internal/registry/model.go`

Three domain structs plus a custom `Duration` type:

- `Duration struct{ time.Duration }` with `UnmarshalYAML` — parses strings like `"24h"` directly into `time.Duration`; invalid formats fail at parse time
- `Entity` — `Name`, `JoinKey`, `Description` (optional)
- `Feature` — `Name`, `Dtype`
- `FeatureView` — `Name`, `Entity` (ref), `Source` (Parquet path), `TTL Duration`, `Features []Feature`
- `Registry` — top-level container with `Entities []Entity` and `FeatureViews []FeatureView`

### 2. `internal/registry/loader.go`

Single exported function:
```go
func Load(path string) (*Registry, error)
```
Reads the file, unmarshals YAML into `Registry`. Returns wrapped errors for file-read and parse failures.

### 3. `internal/registry/validator.go`

Single exported function:
```go
func Validate(reg *Registry) error
```

Checks in order (early return on first error):
1. Each Entity: `name` non-empty, `join_key` non-empty
2. No duplicate Entity names
3. Each FeatureView: `name`, `entity`, `source` non-empty; `ttl > 0`; `features` non-empty
4. No duplicate FeatureView names
5. Each `FeatureView.Entity` resolves to a known `Entity.Name`

### 4. `configs/feature_registry.yaml`

NYC Taxi context — one entity (`driver`), one feature view (`driver_stats`) with three features:
`trip_count` (int64), `avg_fare` (float64), `avg_trip_duration_minutes` (float64).

### 5. `tests/registry_test.go`

Two table-driven test functions:

**`TestLoad`** — file → struct pipeline:
- Valid sample config (uses `configs/feature_registry.yaml`)
- File not found → error wrapping "reading registry"
- Malformed YAML → error wrapping "parsing registry"
- Invalid TTL string → error containing "invalid duration"

**`TestValidate`** — in-memory `Registry` structs via a `validRegistry()` helper + mutate function:
- Valid registry → nil
- Empty entity name → "entity name"
- Empty join_key → "join_key"
- Duplicate entity names → "duplicate entity"
- Empty feature view name → "feature view name"
- Empty entity ref → "entity is required"
- Empty source → "source"
- Zero TTL → "ttl"
- Empty features list → "features"
- Duplicate feature view names → "duplicate feature view"
- Unknown entity ref → "unknown entity"

---

## Dependency

```
go get gopkg.in/yaml.v3
```

---

## Verification

```bash
go get gopkg.in/yaml.v3
go build ./internal/registry/...
go test ./tests/ -run TestRegistry -v
go test ./...
```
