import os
import time

from pyspark.sql import SparkSession
from pyspark.sql.functions import avg, col, count, unix_timestamp

MODE = os.environ.get("MODE", "csv")
OUTPUT_PATH = os.environ.get("OUTPUT_PATH", "data/offline_store/driver_stats_spark.parquet")

CSV_INPUT_PATH = os.environ.get("INPUT_PATH", "data/raw/taxi.csv") if MODE == "csv" else None
PARQUET_INPUT_PATH = os.environ.get("INPUT_PATH", "data/raw/nyc_taxi/") if MODE == "parquet" else None

spark = (
    SparkSession.builder
    .appName("mini-feature-store-materialize")
    .getOrCreate()
)
spark.sparkContext.setLogLevel("WARN")

# Spark evaluates the aggregation during the write. Start before reading the
# input so metadata discovery and CSV schema inference are included too.
start = time.perf_counter()

if MODE == "parquet":
    df_raw = spark.read.parquet(PARQUET_INPUT_PATH)

    df = df_raw.withColumn(
        "trip_duration_minutes",
        (unix_timestamp("tpep_dropoff_datetime") - unix_timestamp("tpep_pickup_datetime")) / 60,
    ).withColumn("entity_id", col("VendorID").cast("string"))

    df_filtered = df.filter(
        (col("fare_amount") > 0)
        & (col("trip_duration_minutes") > 0)
        & (col("trip_duration_minutes") <= 300)
    )

    stats = (
        df_filtered.groupBy("entity_id")
        .agg(
            count("*").alias("trip_count"),
            avg("fare_amount").alias("avg_fare"),
            avg("trip_duration_minutes").alias("avg_trip_duration_minutes"),
        )
        .coalesce(1)
    )

else:
    df = spark.read.csv(CSV_INPUT_PATH, header=True, inferSchema=True)

    stats = (
        df.groupBy("driver_id")
        .agg(
            count("*").alias("trip_count"),
            avg("fare_amount").alias("avg_fare"),
            avg("trip_duration_minutes").alias("avg_trip_duration_minutes"),
        )
        .coalesce(1)
    )

stats.write.mode("overwrite").parquet(OUTPUT_PATH)
elapsed = time.perf_counter() - start
print(f"Read + aggregate + write: {elapsed:.2f}s")
print(f"Output: {OUTPUT_PATH}")

spark.stop()
