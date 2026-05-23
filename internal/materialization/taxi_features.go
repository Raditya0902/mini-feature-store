package materialization

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"time"

	"github.com/Raditya0902/mini-feature-store/internal/offline"
)

type driverAcc struct {
	tripCount         int
	totalFare         float64
	totalTripDuration float64
}

func GenerateDriverStats(rawPath string) ([]offline.FeatureRow, error) {
	f, err := os.Open(rawPath)
	if err != nil {
		return nil, fmt.Errorf("opening raw data: %w", err)
	}
	defer f.Close()

	r := csv.NewReader(f)

	header, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("reading header: %w", err)
	}

	colIndex := make(map[string]int, len(header))
	for i, col := range header {
		colIndex[col] = i
	}

	for _, required := range []string{"driver_id", "fare_amount", "trip_duration_minutes"} {
		if _, ok := colIndex[required]; !ok {
			return nil, fmt.Errorf("missing required column %q", required)
		}
	}

	idxDriverID := colIndex["driver_id"]
	idxFare := colIndex["fare_amount"]
	idxDuration := colIndex["trip_duration_minutes"]

	accumulators := make(map[string]*driverAcc)

	for lineNum := 2; ; lineNum++ {
		row, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("reading row %d: %w", lineNum, err)
		}

		driverID := row[idxDriverID]

		fare, err := strconv.ParseFloat(row[idxFare], 64)
		if err != nil {
			fmt.Fprintf(os.Stderr, "taxi_features: skipping row %d: invalid fare_amount %q: %v\n", lineNum, row[idxFare], err)
			continue
		}

		duration, err := strconv.ParseFloat(row[idxDuration], 64)
		if err != nil {
			fmt.Fprintf(os.Stderr, "taxi_features: skipping row %d: invalid trip_duration_minutes %q: %v\n", lineNum, row[idxDuration], err)
			continue
		}

		acc, ok := accumulators[driverID]
		if !ok {
			acc = &driverAcc{}
			accumulators[driverID] = acc
		}
		acc.tripCount++
		acc.totalFare += fare
		acc.totalTripDuration += duration
	}

	if len(accumulators) == 0 {
		return []offline.FeatureRow{}, nil
	}

	driverIDs := make([]string, 0, len(accumulators))
	for id := range accumulators {
		driverIDs = append(driverIDs, id)
	}
	sort.Strings(driverIDs)

	ts := time.Now().UTC().Truncate(time.Second)
	rows := make([]offline.FeatureRow, len(driverIDs))
	for i, id := range driverIDs {
		acc := accumulators[id]
		count := float64(acc.tripCount)
		rows[i] = offline.FeatureRow{
			EntityID:         id,
			FeatureTimestamp: ts,
			Values: map[string]any{
				"trip_count":              count,
				"avg_fare":                acc.totalFare / count,
				"avg_trip_duration_minutes": acc.totalTripDuration / count,
			},
		}
	}

	return rows, nil
}
