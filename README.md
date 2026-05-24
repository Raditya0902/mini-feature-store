# mini-feature-store

A feature store in Go that demonstrates the core mechanics of offline/online serving and point-in-time correct historical joins, using the NYC Taxi dataset. Point-in-time correctness is the central challenge in ML feature stores: when building a training dataset, you must only use feature values that were available *at the moment the event occurred* — using data from after the event leaks future information into your model and causes it to overperform in training while failing in production. This store enforces that guarantee by filtering out any feature row whose `feature_timestamp` exceeds the `event_timestamp` of the request, and discarding rows older than the configured TTL.

## Architecture

```
configs/feature_registry.yaml
        │  entities, feature views, TTL
        │
        ├──────────────────────┬─────────────────────┐
        │                      │                     │
        ▼                      ▼                     ▼
internal/registry     internal/offline       internal/online
  Load + Validate      Parquet read/write     Redis get/set/bulk
        │                      │                     │
        └──────────┬─────────��─┘                     │
                   │                                 │
          internal/materialization ─────────────────►┘
          cmd/materialize                 populates both stores
          (raw CSV → FeatureRows)
                   │
                   ▼
          internal/historical
          point-in-time join
                   ��
                   ▼
          internal/server
          cmd/server :8080
          ┌────────────────────────────────┐
          │ GET  /features/online          │  Redis lookup
          │ POST /features/historical      │  Parquet join
          └────────────────���───────────────┘
```

| Package | Responsibility |
|---|---|
| `internal/registry` | Load and validate `configs/feature_registry.yaml` |
| `internal/offline` | Read and write Parquet files (`FeatureRow`) |
| `internal/online` | Redis `Set` / `Get` / `MGet` |
| `internal/historical` | Point-in-time join: leakage guard + TTL filter |
| `internal/materialization` | CSV → `[]FeatureRow`, writes both stores |
| `internal/server` | HTTP handlers, request/response schemas |
| `cmd/materialize` | CLI: compute features and populate stores |
| `cmd/server` | HTTP server on `:8080` |

## How to Run

```bash
docker compose up -d
go run cmd/materialize/main.go
go run cmd/server/main.go
```

`cmd/materialize` reads `data/raw/taxi.csv`, groups by `driver_id`, computes per-driver stats, and writes to `data/driver_stats.parquet` (offline) and Redis (online). `cmd/server` starts the REST API on `:8080`.

## API

### GET /features/online

Online lookup from Redis — use this at inference time.

```bash
curl "http://localhost:8080/features/online?entity_id=d1"
```

```json
{
  "entity_id": "d1",
  "features": {
    "avg_fare": 10.75,
    "avg_trip_duration_minutes": 15,
    "trip_count": 2
  }
}
```

```bash
curl "http://localhost:8080/features/online?entity_id=d2"
```

```json
{
  "entity_id": "d2",
  "features": {
    "avg_fare": 18.875,
    "avg_trip_duration_minutes": 26,
    "trip_count": 2
  }
}
```

```bash
curl "http://localhost:8080/features/online?entity_id=d3"
```

```json
{
  "entity_id": "d3",
  "features": {
    "avg_fare": 8.25,
    "avg_trip_duration_minutes": 10,
    "trip_count": 1
  }
}
```

Returns `404` if `entity_id` is missing from the query string. Returns `200` with `"features": {}` if the entity has no data in Redis.

---

### POST /features/historical

Point-in-time correct feature retrieval from Parquet — use this to build training datasets. The `event_timestamp` must be within 24h of when `cmd/materialize` was run (the feature view TTL).

```bash
curl -X POST http://localhost:8080/features/historical \
  -H "Content-Type: application/json" \
  -d '{
    "entity_events": [
      {"entity_id": "d1", "event_timestamp": "2026-05-23T15:00:00Z"},
      {"entity_id": "d2", "event_timestamp": "2026-05-23T15:00:00Z"},
      {"entity_id": "d3", "event_timestamp": "2026-05-23T15:00:00Z"}
    ]
  }'
```

```json
{
  "training_rows": [
    {
      "entity_id": "d1",
      "event_timestamp": "2026-05-23T15:00:00Z",
      "features": {
        "avg_fare": 10.75,
        "avg_trip_duration_minutes": 15,
        "trip_count": 2
      }
    },
    {
      "entity_id": "d2",
      "event_timestamp": "2026-05-23T15:00:00Z",
      "features": {
        "avg_fare": 18.875,
        "avg_trip_duration_minutes": 26,
        "trip_count": 2
      }
    },
    {
      "entity_id": "d3",
      "event_timestamp": "2026-05-23T15:00:00Z",
      "features": {
        "avg_fare": 8.25,
        "avg_trip_duration_minutes": 10,
        "trip_count": 1
      }
    }
  ]
}
```

Returns `400` on malformed JSON. Returns `"features": {}` for events where no valid feature row exists (leakage guard or TTL exceeded).

---

### Example: generate a training set

```bash
go run examples/generate_training_set.go
```

Loads the registry and offline Parquet store, calls `GetHistoricalFeatures` for d1, d2, d3 with `EventTimestamp = time.Now()`, and prints each `TrainingRow` as formatted JSON.

## Tests

```bash
go test ./... -v
```

28 tests pass without Redis (4 top-level test functions, 24 table-driven cases):

| Test function | Cases | What it covers |
|---|---|---|
| `TestLoad` | 4 | Registry loading: valid config, missing file, malformed YAML, invalid TTL duration |
| `TestValidate` | 11 | Registry validation: entity name, join key, duplicates, feature view fields, unknown entity ref |
| `TestOfflineRoundTrip` | 3 | Parquet write → read: single row, multiple rows with mixed types, empty slice |
| `TestPointInTime` | 6 | Point-in-time join: basic join, no leakage, TTL filter, latest-wins, no match, multiple entities |

With Redis running (`docker compose up -d`), 9 additional tests run:

| Test function | Cases | What it covers |
|---|---|---|
| `TestOnlineStore` | 4 | Redis Set/Get round-trip, missing key returns empty map, MGet mixed, overwrite |
| `TestConsistency` | 3 | After `Materialize`, offline (Parquet) and online (Redis) values match for d1, d2, d3 |

## Intentional scope

S3, Kafka, Spark, and Airflow are deliberately excluded. The goal is to demonstrate the core correctness properties — point-in-time joins, TTL filtering, offline/online consistency — with the smallest possible stack. Adding a distributed compute layer or a message broker would obscure those mechanics without changing what the code proves. Authentication is excluded for the same reason: the API is a local development server, not a production service.
