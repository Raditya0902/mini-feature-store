# Tasks: Phase 4 — Redis Online Store

## Setup
- [x] `go get github.com/redis/go-redis/v9`

## Implementation
- [x] `internal/online/redis_store.go`
  - [x] `RedisStore` struct
  - [x] `NewRedisStore(addr string) *RedisStore`
  - [x] `Set(entityID string, features map[string]any) error`
  - [x] `Get(entityID string) (map[string]any, error)`
  - [x] `MGet(entityIDs []string) (map[string]map[string]any, error)`
- [x] `tests/online_store_test.go`
  - [x] `newTestStore` skip helper
  - [x] `TestOnlineStore/set_and_get_round_trip`
  - [x] `TestOnlineStore/get_missing_key`
  - [x] `TestOnlineStore/MGet_mixed`
  - [x] `TestOnlineStore/overwrite`

## Verification
- [x] `go build ./internal/online/...`
- [x] `docker compose up -d` (start Redis)
- [x] `go test ./tests/ -run TestOnline -v` — 4/4 subtests pass
- [x] `go test ./tests/ -v` — all prior tests still green
