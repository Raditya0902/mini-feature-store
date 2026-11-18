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

CSV mode groups the bundled taxi data by `driver_id`. Parquet mode reads NYC
Taxi yellow-trip data and groups by `VendorID`. Both modes compute
`trip_count`, `avg_fare`, and `avg_trip_duration_minutes`, then coalesce to one
partition and write a Parquet dataset. The Go historical API reads its own
hourly snapshots.

## Reproduce the NYC Taxi timing

Place the January, February, and March 2024 yellow-trip Parquet files in
`data/raw/nyc_taxi/`. Use Java 17 and a fresh Spark process and output path for
each run.

```bash
PYSPARK_SUBMIT_ARGS='--master local[*] pyspark-shell' \
  MODE=parquet INPUT_PATH=data/raw/nyc_taxi/ \
  OUTPUT_PATH="$(mktemp -d)/driver_stats" \
  python spark/materialize_features.py
```

The reported time covers reading the input, computing the aggregates, and
writing the output. The timer starts after Spark creates its session.
