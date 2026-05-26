# Mini Feature Store: Point-in-Time Correct ML Features in Go

A lightweight feature store in Go with Parquet-backed offline storage, Redis-backed online serving, a YAML feature registry, and point-in-time correct historical joins using NYC Taxi-style data.

This project focuses on the hardest correctness problem in feature stores: preventing training-time data leakage by ensuring every historical feature value existed before the prediction event.

## Highlights

- Built a Go-based feature store with Parquet offline storage, Redis online serving, and a PySpark materialization job for large-scale feature computation
- Implemented point-in-time correct joins to prevent training data leakage
- Added TTL filtering, latest-wins semantics, and offline/online consistency checks
- Covered core behavior with 37 test cases across unit and Redis-backed integration tests

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
        └──────────┬───────────┘                     │
                   │                                 │
          internal/materialization ─────────────────►┘
          cmd/materialize                 populates both stores
          (raw CSV → FeatureRows)
                   │
                   ▼
          internal/historical
          point-in-time join
                   │
                   ▼
          internal/server
          cmd/server :8080
          ┌────────────────────────────────┐
          │ GET  /features/online          │  Redis lookup
          │ POST /features/historical      │  Parquet join
          └────────────────────────────────┘
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

## Status
- Offline store: implemented
- Online store: implemented
- Feature registry: implemented
- Materialization: implemented
- Point-in-time joins: implemented
- REST API: implemented
- Redis integration tests: implemented

## How to Run

```bash
docker compose up -d
go run cmd/materialize/main.go
go run cmd/server/main.go
```

`cmd/materialize` reads `data/raw/taxi.csv`, groups by `driver_id`, computes per-driver stats, and writes to `data/driver_stats.parquet` (offline) and Redis (online). `cmd/server` starts the REST API on `:8080`.

## Spark Materialization

A PySpark job is provided as an alternative to the Go materializer for large-scale feature computation.

```bash
pip install -r spark/requirements.txt
python spark/materialize_features.py
```

Reads `data/raw/taxi.csv`, computes per-driver aggregates using a Spark groupBy, and writes partitioned Parquet to `data/offline_store/driver_stats_spark.parquet/`.

Supports two modes via the `MODE` environment variable:
- `MODE=csv` (default): reads `data/raw/taxi.csv` for local development
- `MODE=parquet`: reads a directory of Parquet files for large-scale runs

Benchmarked on 3 months of the NYC Taxi dataset (9.5M rows):
- Input rows: 9,554,778
- Rows after filtering: 9,407,110
- groupBy + write: 0.88s on a single-node local Spark session (Apple M4, 16GB RAM)

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

## Performance

Point-in-time join over 100,000 feature rows (10,000 entities) with 1,000 entity events:
- Join completed in: 63ms
- Heap delta: -0.4 MB
- Design note: current implementation loads all Parquet rows into memory. For production scale, partitioning by entity_id and predicate pushdown would reduce both latency and memory usage.

Spark materialization (groupBy + Parquet write): 0.88s over 9.5M rows on a single-node local Spark session.

## Intentional scope

S3, Kafka, and Airflow are deliberately excluded. The goal is to demonstrate the core correctness properties — point-in-time joins, TTL filtering, offline/online consistency — with the smallest possible stack. A PySpark materialization job is provided for large-scale feature computation, but streaming ingestion and orchestration are out of scope. Authentication is excluded for the same reason: the API is a local development server, not a production service.
