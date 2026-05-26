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

if MODE == "parquet":
    df_raw = spark.read.parquet(PARQUET_INPUT_PATH)

    df = df_raw.withColumn(
        "trip_duration_minutes",
        (unix_timestamp("tpep_dropoff_datetime") - unix_timestamp("tpep_pickup_datetime")) / 60,
    ).withColumn("entity_id", col("VendorID").cast("string"))

    total_count = df.count()
    print(f"Input rows: {total_count}")

    df_filtered = df.filter(
        (col("fare_amount") > 0)
        & (col("trip_duration_minutes") > 0)
        & (col("trip_duration_minutes") <= 300)
    )

    filtered_count = df_filtered.count()
    print(f"Rows after filtering: {filtered_count}")

    unique_vendors = df_filtered.select("entity_id").distinct().count()
    print(f"Unique VendorIDs: {unique_vendors}")

    start = time.time()

    stats = (
        df_filtered.groupBy("entity_id")
        .agg(
            count("*").alias("trip_count"),
            avg("fare_amount").alias("avg_fare"),
            avg("trip_duration_minutes").alias("avg_trip_duration_minutes"),
        )
        .coalesce(1)
    )

    stats.write.mode("overwrite").parquet(OUTPUT_PATH)

    elapsed = time.time() - start
    print(f"groupBy + write: {elapsed:.2f}s")
    print(f"Output: {OUTPUT_PATH}")

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

    row_count = stats.count()
    print(f"Wrote {row_count} driver rows to {OUTPUT_PATH}")

spark.stop()
