package server

import "time"

type EntityEventRequest struct {
	EntityID       string    `json:"entity_id"`
	EventTimestamp time.Time `json:"event_timestamp"`
}

type HistoricalRequest struct {
	EntityEvents []EntityEventRequest `json:"entity_events"`
}

type OnlineResponse struct {
	EntityID string         `json:"entity_id"`
	Features map[string]any `json:"features"`
}

type TrainingRowResponse struct {
	EntityID       string         `json:"entity_id"`
	EventTimestamp time.Time      `json:"event_timestamp"`
	Features       map[string]any `json:"features"`
}

type HistoricalResponse struct {
	TrainingRows []TrainingRowResponse `json:"training_rows"`
}
