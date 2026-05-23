# Plan: Phase 6 — HTTP API

## Context

Phase 6 exposes the feature store over HTTP using `net/http` only. Two endpoints:
- Online lookup from Redis (low-latency)
- Historical point-in-time join from Parquet (batch training data)

All deps injected via a `Server` struct — no global state.

---

## `internal/server/schemas.go`

Request/response structs with `json` tags only.

```go
type EntityEventRequest struct {
    EntityID       string    `json:"entity_id"`
    EventTimestamp time.Time `json:"event_timestamp"`
}

type HistoricalRequest struct {
    EntityEvents []EntityEventRequest `json:"entity_events"`
}

type OnlineResponse struct {
    EntityID string         `json:"entity_id"`
    Features map[string]any `json:"features"`
}

type TrainingRowResponse struct {
    EntityID       string         `json:"entity_id"`
    EventTimestamp time.Time      `json:"event_timestamp"`
    Features       map[string]any `json:"features"`
}

type HistoricalResponse struct {
    TrainingRows []TrainingRowResponse `json:"training_rows"`
}
```

---

## `internal/server/handlers.go`

```go
type Server struct {
    online  *online.RedisStore
    offline *offline.ParquetStore
    reg     *registry.Registry
}

func NewServer(o, s, r) *Server
func (s *Server) RegisterRoutes(mux *http.ServeMux)   // binds both routes
func writeJSON(w, status, v)                           // Content-Type + marshal
```

### GET /features/online?entity_id=<id>
- Method guard → 405
- Missing entity_id → 404
- `online.Get(entityID)` → 500 on error
- 200 with `OnlineResponse` (empty features map if entity unknown)

### POST /features/historical
- Method guard → 405
- Bad JSON body → 400
- Find `driver_stats` view in registry → 500 if missing
- Convert request → `[]historical.EntityEvent`
- `historical.GetHistoricalFeatures` → 500 on error
- Convert result → `[]TrainingRowResponse` (nil Features → empty map)
- 200 with `HistoricalResponse`

---

## `cmd/server/main.go`

Constants: `registryPath`, `redisAddr`, `parquetBase`, `listenAddr = ":8080"`.

Load registry → construct stores → `NewServer` → `RegisterRoutes` → `ListenAndServe`.

---

## Verification

```bash
go build ./internal/server/... ./cmd/server/...
docker compose up -d
go run cmd/server/main.go

curl "http://localhost:8080/features/online?entity_id=d1"
curl "http://localhost:8080/features/online"                     # 404
curl -X POST http://localhost:8080/features/historical \
  -H "Content-Type: application/json" \
  -d '{"entity_events":[{"entity_id":"d1","event_timestamp":"2099-01-01T00:00:00Z"}]}'
curl -X POST http://localhost:8080/features/historical \
  -d 'bad json'                                                  # 400
```
