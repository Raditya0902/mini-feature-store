# Tasks: Phase 6 — HTTP API

## Status Legend
- [ ] Not started
- [~] In progress
- [x] Complete

---

## Implementation

- [ ] `internal/server/schemas.go` — request/response structs with json tags
- [ ] `internal/server/handlers.go` — Server struct, writeJSON, handleOnline, handleHistorical, RegisterRoutes
- [ ] `cmd/server/main.go` — thin main: load registry, construct stores, start server on :8080

## Verification

- [ ] `go build ./internal/server/... ./cmd/server/...` passes
- [ ] `go build ./...` passes for all non-stub packages
- [ ] Smoke test: curl online endpoint returns features
- [ ] Smoke test: curl historical endpoint returns training rows
- [ ] Smoke test: missing entity_id returns 404
- [ ] Smoke test: malformed JSON body returns 400
