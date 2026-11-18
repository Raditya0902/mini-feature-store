package tests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Raditya0902/mini-feature-store/internal/materialization"
	"github.com/Raditya0902/mini-feature-store/internal/offline"
	"github.com/Raditya0902/mini-feature-store/internal/registry"
	"github.com/Raditya0902/mini-feature-store/internal/server"
)

func TestMaterializedHistoryThroughAPI(t *testing.T) {
	reg, err := registry.Load(registryPath)
	if err != nil {
		t.Fatal(err)
	}
	store := &offline.ParquetStore{BasePath: t.TempDir()}
	if err := materialization.Materialize(reg, store, nil, "../data/raw/taxi.csv"); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		driver, timestamp, snapshot string
		want                        driverExpected
	}{
		{"d1", "2025-01-03T12:00:00Z", "2025-01-03T12:00:00Z", driverExpected{1, 30, 24}},
		{"d1", "2025-01-01T10:59:59Z", "", driverExpected{}},
		{"d1", "2025-01-01T11:00:00Z", "2025-01-01T11:00:00Z", driverExpected{2, 10.75, 15}},
		{"d2", "2025-01-01T11:00:00Z", "2025-01-01T11:00:00Z", driverExpected{2, 18.875, 26}},
		{"d1", "2025-01-01T11:59:59Z", "2025-01-01T11:00:00Z", driverExpected{2, 10.75, 15}},
		{"d1", "2025-01-01T12:00:00Z", "2025-01-01T12:00:00Z", driverExpected{1, 24, 30}},
		{"d1", "2025-01-02T12:00:00Z", "2025-01-01T12:00:00Z", driverExpected{1, 24, 30}},
		{"d1", "2025-01-02T12:00:00.000000001Z", "", driverExpected{}},
	}
	var request server.HistoricalRequest
	for _, tc := range cases {
		ts, err := time.Parse(time.RFC3339Nano, tc.timestamp)
		if err != nil {
			t.Fatal(err)
		}
		request.EntityEvents = append(request.EntityEvents, server.EntityEventRequest{
			EntityID: tc.driver, EventTimestamp: ts,
		})
	}
	payload, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	server.NewServer(nil, store, reg).RegisterRoutes(mux)
	res := httptest.NewRecorder()
	mux.ServeHTTP(res, httptest.NewRequest(http.MethodPost, "/features/historical", strings.NewReader(string(payload))))
	if res.Code != http.StatusOK {
		t.Fatalf("historical request: status %d, body %s", res.Code, res.Body)
	}
	var response server.HistoricalResponse
	if err := json.Unmarshal(res.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.TrainingRows) != len(cases) {
		t.Fatalf("got %d training rows, want %d", len(response.TrainingRows), len(cases))
	}

	for i, tc := range cases {
		row := response.TrainingRows[i]
		if row.EntityID != tc.driver || !row.EventTimestamp.Equal(request.EntityEvents[i].EventTimestamp) {
			t.Errorf("row %d: got %s at %v, want %s at %s", i, row.EntityID, row.EventTimestamp, tc.driver, tc.timestamp)
		}
		assertHistoricalTimestamp(t, row.FeatureTimestamp, row.EventTimestamp, 24*time.Hour)
		if tc.snapshot == "" {
			if row.FeatureTimestamp != nil {
				t.Errorf("%s at %s: got timestamp %s for a missing snapshot", tc.driver, tc.timestamp, row.FeatureTimestamp)
			}
			if row.Features == nil || len(row.Features) != 0 {
				t.Errorf("%s at %s: got %v, want empty features", tc.driver, tc.timestamp, row.Features)
			}
			continue
		}
		if row.FeatureTimestamp == nil || row.FeatureTimestamp.Format(time.RFC3339Nano) != tc.snapshot {
			t.Errorf("%s at %s: got feature timestamp %v, want %s", tc.driver, tc.timestamp, row.FeatureTimestamp, tc.snapshot)
		}
		assertFloatFeature(t, tc.timestamp, "trip_count", row.Features, tc.want.tripCount)
		assertFloatFeature(t, tc.timestamp, "avg_fare", row.Features, tc.want.avgFare)
		assertFloatFeature(t, tc.timestamp, "avg_trip_duration_minutes", row.Features, tc.want.avgDur)
	}
}
