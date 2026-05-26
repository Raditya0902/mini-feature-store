import os
from pyspark.sql import SparkSession
from pyspark.sql.functions import count, avg

INPUT_PATH = os.environ.get("INPUT_PATH", "data/raw/taxi.csv")
OUTPUT_PATH = os.environ.get("OUTPUT_PATH", "data/offline_store/driver_stats_spark.parquet")

spark = (
    SparkSession.builder
    .appName("mini-feature-store-materialize")
    .getOrCreate()
)
spark.sparkContext.setLogLevel("WARN")

df = spark.read.csv(INPUT_PATH, header=True, inferSchema=True)

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
