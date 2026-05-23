# Plan: Phase 4 — internal/online (Redis Store)

## Context

Phase 4 adds the online store: a Redis-backed key-value layer for low-latency feature serving
at inference time. Values are JSON-encoded `map[string]any` (same pattern as the offline store).
No TTL is set — the online store holds the latest materialized values until overwritten.

---

## `internal/online/redis_store.go`

```
type RedisStore struct { client *redis.Client }

func NewRedisStore(addr string) *RedisStore
func (s *RedisStore) Set(entityID string, features map[string]any) error
func (s *RedisStore) Get(entityID string) (map[string]any, error)
func (s *RedisStore) MGet(entityIDs []string) (map[string]map[string]any, error)
```

Key format: `"features:" + entityID`
Context: `context.Background()` internally
TTL: 0 (no expiry)

- Set: marshal → client.Set
- Get: client.Get → if redis.Nil → empty map; else unmarshal
- MGet: build keys → client.MGet → nil vals become nil entries in result map

---

## `tests/online_store_test.go`

Skip helper pings localhost:6379 and calls t.Skip if unreachable.

TestOnlineStore — 4 subtests:
1. set and get round-trip
2. get missing key → empty map
3. MGet mixed — some present, some nil
4. overwrite — second Set wins

---

## Verification

```bash
docker compose up -d
go build ./internal/online/...
go test ./tests/ -run TestOnline -v
go test ./tests/ -v
```
