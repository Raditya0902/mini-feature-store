# Context: Phase 4 — Redis Online Store

## Module

`github.com/Raditya0902/mini-feature-store`

## Go Version

`go 1.26.1`

## Dependencies Added

| Package | Version | Purpose |
|---------|---------|---------|
| `github.com/redis/go-redis/v9` | v9.19.0 | Redis client |

## Key Design Decisions

### JSON encoding (same as offline store)
Values are `map[string]any` serialized with `encoding/json`. Integer values normalise to
`float64` after JSON decode — acceptable for the NYC taxi feature types.

### context.Background() internally
go-redis/v9 requires `context.Context` in all calls. The API does not expose context
(the user didn't request it), so `context.Background()` is used throughout.

### Get missing key returns empty map (not nil)
`redis.Nil` → `map[string]any{}, nil`. Callers can check `len(features) == 0` without a nil guard.

### MGet missing keys → nil entries
`client.MGet` returns `[]interface{}` where absent keys are `nil`. These become
`result[entityID] = nil` in the returned map — distinguishable from an empty map.

### No TTL
Features are set with `expiration=0` (no expiry). The online store holds the latest
materialized snapshot indefinitely until overwritten.

### Test isolation
Tests use stable, unique entity ID prefixes (`"online-rt-e1"`, `"online-mget-e1"`, etc.)
so they are idempotent across repeated runs without explicit cleanup.
