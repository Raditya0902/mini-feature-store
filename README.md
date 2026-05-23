# mini-feature-store

A lightweight feature store in Go. Supports offline storage (Parquet), online serving (Redis), point-in-time correct historical joins, and a REST API — demonstrated with the NYC Taxi dataset.

## Architecture

```
configs/
  feature_registry.yaml     — entity and feature view definitions

internal/
  registry/                 — loads and validates the YAML registry
  offline/                  — Parquet read/write (FeatureRow)
  online/                   — Redis get/set/bulk (latest features)
  historical/               — point-in-time join (leakage-safe, TTL-filtered)
  materialization/          — CSV → FeatureRow computation + writes both stores
  server/                   — HTTP handlers and request/response schemas

cmd/
  materialize/main.go       — CLI: compute features from raw CSV, populate stores
  server/main.go            — HTTP server on :8080

data/
  raw/taxi.csv              — source data (driver_id, fare_amount, trip_duration_minutes)
  driver_stats.parquet      — written by materialization
```

**Data flow:**

```
data/raw/taxi.csv
      │
      ▼
cmd/materialize  ──► data/driver_stats.parquet  (offline store)
                 ──► Redis keys                 (online store)
                              │
                              ▼
                      cmd/server  ──► GET /features/online
                                  ──► POST /features/historical
```

## Requirements

- Go 1.21+
- Docker (for Redis)

## How to Run

**1. Start Redis**

```bash
docker compose up -d
```

**2. Materialize features from raw data**

Reads `data/raw/taxi.csv`, computes per-driver stats, and writes to both the Parquet offline store and Redis.

```bash
go run cmd/materialize/main.go
# materialization complete
```

**3. Start the HTTP server**

```bash
go run cmd/server/main.go
# listening on :8080
```

## API

### GET /features/online

Fetch the latest features for a driver from Redis.

```bash
curl "http://localhost:8080/features/online?entity_id=driver_42"
```

```json
{
  "entity_id": "driver_42",
  "features": {
    "avg_fare": 14.75,
    "avg_trip_duration_minutes": 22.3,
    "trip_count": 8
  }
}
```

Returns `404` if `entity_id` is missing from the query string.
Returns `200` with an empty `features` map if the entity has no data in Redis.

---

### POST /features/historical

Fetch point-in-time correct features for a list of entity events. Feature rows with a `feature_timestamp` after the `event_timestamp` are excluded (no leakage). Rows older than the feature view's TTL (24h) are also excluded.

```bash
curl -X POST http://localhost:8080/features/historical \
  -H "Content-Type: application/json" \
  -d '{
    "entity_events": [
      {"entity_id": "driver_42", "event_timestamp": "2024-06-01T12:00:00Z"},
      {"entity_id": "driver_7",  "event_timestamp": "2024-06-01T09:00:00Z"}
    ]
  }'
```

```json
{
  "training_rows": [
    {
      "entity_id": "driver_42",
      "event_timestamp": "2024-06-01T12:00:00Z",
      "features": {
        "avg_fare": 14.75,
        "avg_trip_duration_minutes": 22.3,
        "trip_count": 8
      }
    },
    {
      "entity_id": "driver_7",
      "event_timestamp": "2024-06-01T09:00:00Z",
      "features": {}
    }
  ]
}
```

Returns `400` on malformed JSON. Returns `features: {}` for events where no valid feature row exists.

## Development

```bash
# Build everything
go build ./...

# Run tests
go test ./... -v

# Run only non-Redis tests
go test ./tests/ -run 'TestOffline|TestRegistry|TestPointInTime' -v

# Run consistency test (requires Redis)
docker compose up -d
go test ./tests/ -run TestConsistency -v
```
