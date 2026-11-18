# Mini Feature Store: Point-in-Time Correct ML Features in Go

A lightweight feature store in Go with Parquet-backed offline storage, Redis-backed online serving, a YAML feature registry, and point-in-time correct historical joins using NYC Taxi-style data.

The Go pipeline builds a history of hourly driver snapshots from completed trips. Historical lookups use the latest snapshot available at each prediction timestamp, subject to the feature view's TTL.

## Highlights

- Event-time materialization keeps multiple snapshots per driver in Parquet and publishes the latest closed snapshot per driver to Redis.
- Historical joins enforce snapshot availability and TTL while retaining at most one matching row per request event.
- A fixed, timestamped taxi fixture demonstrates historical backfills through the same materializer and join used by the API.

## Architecture

```
configs/feature_registry.yaml
        │  entities, feature views, TTL
        │
        ├──────────────────────┬─────────────────────┐
        │                      │                     │
        ▼                      ▼                     ▼
internal/registry     internal/offline       internal/online
  Load + Validate      Parquet scan/write     Redis get/set/bulk
        │                      │                     │
        └──────────┬───────────┘                     │
                   │                                 │
          internal/materialization ─────────────────►┘
          cmd/materialize                 offline history + latest in Redis
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
| `internal/materialization` | Completed trips → hourly snapshots in Parquet and latest snapshots in Redis |
| `internal/server` | HTTP handlers, request/response schemas |
| `cmd/materialize` | CLI: compute features and populate stores |
| `cmd/server` | HTTP server on `:8080` |

## Run the historical demo

```bash
go run ./cmd/materialize -offline-only
go run ./examples/generate_training_set.go
```

These commands use the checked-in `data/raw/taxi.csv` and work without Redis. The sample includes multiple hours for the same driver, with input deliberately out of chronological order.

| Prediction timestamp (UTC) | Driver | Trips | Average fare | Average duration (minutes) |
|---|---|---:|---:|---:|
| 2025-01-01 10:59 | d1 | — | — | — |
| 2025-01-01 11:00 | d1 | 2 | 10.75 | 15 |
| 2025-01-01 12:00 | d1 | 1 | 24 | 30 |
| 2025-01-02 12:00:01 | d1 | — | — | — |
| 2025-01-03 12:00 | d1 | 1 | 30 | 24 |

The CSV requires `driver_id`, `fare_amount`, `trip_duration_minutes`, and `completed_at` (RFC 3339 with a timezone). A trip belongs to the UTC hour in which it completed. Each nonempty interval `[start, end)` produces one snapshot per driver at `end`. A trip completed at 11:00 contributes to the 12:00 snapshot, since it falls in the 11:00–12:00 window. The snapshot time assumes completed trips are available by the end of their hour.

Materialization replaces `data/driver_stats.parquet` with snapshots rebuilt from the supplied CSV. Use `-input /path/to/taxi.csv` to choose a different source and include every period you want to keep in the offline store. Re-run materialization to regenerate Parquet files from the earlier single-snapshot format.

## Run the API with Redis

```bash
docker compose up -d
go run ./cmd/materialize
go run ./cmd/server
```

The normal materialization command publishes each driver's most recent closed snapshot from the input to Redis. For a historical rebuild that should leave Redis alone, use `-offline-only`. You can also start the server after offline-only materialization to use the historical endpoint without Redis.

## Spark Materialization

The PySpark job demonstrates batch aggregation on larger inputs. Its aggregate output uses a separate Parquet schema and is outside the Go historical serving path.

```bash
pip install -r spark/requirements.txt
python spark/materialize_features.py
```

Reads `data/raw/taxi.csv`, computes per-driver aggregates using a Spark groupBy, and writes a Parquet dataset to `data/offline_store/driver_stats_spark.parquet/`.

Supports two modes via the `MODE` environment variable:
- `MODE=csv` (default): reads `data/raw/taxi.csv` for local development
- `MODE=parquet`: reads a directory of Parquet files for large-scale runs

Read, aggregate, and write took 2.29–2.43 s across three runs on the January–March 2024 NYC Taxi yellow-trip Parquet files (9,554,778 input rows). Each run used a fresh local Spark 3.5.3 session with `local[*]`, Java 17, and a new output path on an Apple M4 with 16 GB RAM. Timing starts before the input read and ends after the output write. Spark startup is outside the timed interval. See the [Spark run instructions](spark/README.md).

## API

### GET /features/online

Returns the latest snapshot published to Redis for the requested driver.

```bash
curl "http://localhost:8080/features/online?entity_id=d1"
```

```json
{
  "entity_id": "d1",
  "features": {
    "avg_fare": 30,
    "avg_trip_duration_minutes": 24,
    "trip_count": 1
  }
}
```

Returns `404` when the query omits `entity_id` and `200` with `"features": {}` when Redis has no snapshot for the driver. The historical TTL applies to as-of joins. Redis serves the last value published for each driver until the next update.

### POST /features/historical

For each event, the API selects the newest snapshot with a timestamp at or before the event and an age of at most 24 hours. The 24-hour TTL comes from the registry and is relative to the event timestamp. Historical queries can use any date covered by the input data.

```bash
curl -X POST http://localhost:8080/features/historical \
  -H "Content-Type: application/json" \
  -d '{
    "entity_events": [
      {"entity_id": "d1", "event_timestamp": "2025-01-01T11:00:00Z"},
      {"entity_id": "d1", "event_timestamp": "2025-01-01T12:00:00Z"}
    ]
  }'
```

```json
{
  "training_rows": [
    {
      "entity_id": "d1",
      "event_timestamp": "2025-01-01T11:00:00Z",
      "features": {"avg_fare": 10.75, "avg_trip_duration_minutes": 15, "trip_count": 2}
    },
    {
      "entity_id": "d1",
      "event_timestamp": "2025-01-01T12:00:00Z",
      "features": {"avg_fare": 24, "avg_trip_duration_minutes": 30, "trip_count": 1}
    }
  ]
}
```

Returns `400` on malformed JSON. Events without an eligible snapshot return `"features": {}`. The response follows request order.

## Tests and scaling

```bash
go test ./... -v
```

The suite covers registry validation, Parquet round trips, point-in-time join rules, and a CSV-to-Parquet-to-HTTP regression test. A seeded randomized test compares the batched join with a naive reference using shuffled data, equal timestamps, gaps, and exact TTL boundaries.

[GitHub Actions](.github/workflows/test.yml) runs the Go suite with a Redis 7 service on pushes and pull requests. To require Redis-backed storage and consistency checks locally, run `docker compose up -d --wait redis` followed by `REQUIRE_REDIS=1 go test ./... -v -count=1`.

The scale test exercises 100,000 feature rows and 1,000 request events. Each request still scans the whole Parquet file in batches. The join retains request events and their current best matches as it scans, so it can answer from histories larger than the set of rows it needs to keep in memory.

## Intentional scope

S3, Kafka, and Airflow are deliberately excluded. The goal is to demonstrate the core correctness properties — point-in-time joins, TTL filtering, offline/online consistency — with the smallest possible stack. A PySpark materialization job is provided for large-scale feature computation, but streaming ingestion and orchestration are out of scope. Authentication is excluded for the same reason: the API is a local development server, not a production service.
