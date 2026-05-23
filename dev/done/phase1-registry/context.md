# Context: Phase 1 — Registry

## Module

`github.com/Raditya0902/mini-feature-store`

## Go Version

`go 1.26.1`

## Dependencies Added

| Package | Purpose |
|---------|---------|
| `gopkg.in/yaml.v3` | YAML parsing for feature registry config |

## Key Decisions

### Custom `Duration` type
`Duration struct{ time.Duration }` with `UnmarshalYAML` parses YAML strings like `"24h"` directly
into `time.Duration`. This makes TTL immediately usable in point-in-time filtering comparisons
without a separate parse step at the call site. Invalid duration strings fail at `Load` time, not
at validation time — which is an acceptable fail-fast tradeoff.

### Early-return validation
`Validate` returns on the first error rather than collecting all errors. Keeps the validator simple;
callers can fix-and-retry iteratively. Revisit if we need full error reporting later.

### TTL = 0 is invalid
A zero TTL would drop all features immediately (event_time − feature_timestamp > 0). The validator
rejects it. If "no TTL / infinite" semantics are needed in a future phase, introduce a sentinel or
optional field at that point (YAGNI).

### No interfaces in registry
Registry types are plain structs consumed by other packages. Interfaces are defined at the
consumer (offline, historical) per the project Go conventions.
