package materialization

import (
	"encoding/csv"
	"fmt"
	"io"
	"math"
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

type driverWindow struct {
	driverID string
	end      time.Time
}

// GenerateDriverStats emits one snapshot per nonempty UTC hour and driver.
// A snapshot becomes available at the end of that hour.
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

	for _, required := range []string{"driver_id", "fare_amount", "trip_duration_minutes", "completed_at"} {
		if _, ok := colIndex[required]; !ok {
			return nil, fmt.Errorf("missing required column %q", required)
		}
	}

	idxDriverID := colIndex["driver_id"]
	idxFare := colIndex["fare_amount"]
	idxDuration := colIndex["trip_duration_minutes"]
	idxCompletedAt := colIndex["completed_at"]

	accumulators := make(map[driverWindow]*driverAcc)

	for lineNum := 2; ; lineNum++ {
		row, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("reading row %d: %w", lineNum, err)
		}

		driverID := row[idxDriverID]
		completedAt, err := time.Parse(time.RFC3339Nano, row[idxCompletedAt])
		if err != nil {
			return nil, fmt.Errorf("row %d: invalid completed_at: %w", lineNum, err)
		}

		fare, err := strconv.ParseFloat(row[idxFare], 64)
		if err != nil || math.IsNaN(fare) || math.IsInf(fare, 0) {
			fmt.Fprintf(os.Stderr, "taxi_features: skipping row %d: invalid fare_amount %q\n", lineNum, row[idxFare])
			continue
		}

		duration, err := strconv.ParseFloat(row[idxDuration], 64)
		if err != nil || math.IsNaN(duration) || math.IsInf(duration, 0) {
			fmt.Fprintf(os.Stderr, "taxi_features: skipping row %d: invalid trip_duration_minutes %q\n", lineNum, row[idxDuration])
			continue
		}

		window := driverWindow{driverID: driverID, end: completedAt.UTC().Truncate(time.Hour).Add(time.Hour)}
		acc, ok := accumulators[window]
		if !ok {
			acc = &driverAcc{}
			accumulators[window] = acc
		}
		acc.tripCount++
		acc.totalFare += fare
		acc.totalTripDuration += duration
	}

	windows := make([]driverWindow, 0, len(accumulators))
	for window := range accumulators {
		windows = append(windows, window)
	}
	sort.Slice(windows, func(i, j int) bool {
		if windows[i].driverID == windows[j].driverID {
			return windows[i].end.Before(windows[j].end)
		}
		return windows[i].driverID < windows[j].driverID
	})

	rows := make([]offline.FeatureRow, len(windows))
	for i, window := range windows {
		acc := accumulators[window]
		count := float64(acc.tripCount)
		rows[i] = offline.FeatureRow{
			EntityID:         window.driverID,
			FeatureTimestamp: window.end,
			Values: map[string]any{
				"trip_count":                count,
				"avg_fare":                  acc.totalFare / count,
				"avg_trip_duration_minutes": acc.totalTripDuration / count,
			},
		}
	}

	return rows, nil
}
