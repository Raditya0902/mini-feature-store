# Spark Materialization

## Install

```bash
pip install -r spark/requirements.txt
```

## Run

```bash
python spark/materialize_features.py
```

Run from the project root so the default paths resolve correctly.

## What it does

Reads `data/raw/taxi.csv`, groups by `driver_id`, and computes three aggregates:
`trip_count`, `avg_fare`, and `avg_trip_duration_minutes`. Coalesces to one partition
and writes Parquet to `data/offline_store/driver_stats_spark.parquet/`.

## Scaling to full NYC Taxi dataset

Tested on the sample `taxi.csv`. The same job runs on the full NYC Taxi dataset
(~100M rows) by setting the input path:

```bash
INPUT_PATH=/path/to/full/taxi.csv python spark/materialize_features.py
```
