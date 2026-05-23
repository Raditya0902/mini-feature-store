package server

import (
	"encoding/json"
	"net/http"

	"github.com/Raditya0902/mini-feature-store/internal/historical"
	"github.com/Raditya0902/mini-feature-store/internal/offline"
	"github.com/Raditya0902/mini-feature-store/internal/online"
	"github.com/Raditya0902/mini-feature-store/internal/registry"
)

type Server struct {
	online  *online.RedisStore
	offline *offline.ParquetStore
	reg     *registry.Registry
}

func NewServer(o *online.RedisStore, s *offline.ParquetStore, r *registry.Registry) *Server {
	return &Server{online: o, offline: s, reg: r}
}

func (s *Server) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/features/online", s.handleOnline)
	mux.HandleFunc("/features/historical", s.handleHistorical)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	data, err := json.Marshal(v)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(data)
}

func (s *Server) handleOnline(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	entityID := r.URL.Query().Get("entity_id")
	if entityID == "" {
		http.Error(w, "missing entity_id", http.StatusNotFound)
		return
	}

	features, err := s.online.Get(entityID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, OnlineResponse{EntityID: entityID, Features: features})
}

func (s *Server) handleHistorical(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req HistoricalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "malformed request body", http.StatusBadRequest)
		return
	}

	var view *registry.FeatureView
	for i := range s.reg.FeatureViews {
		if s.reg.FeatureViews[i].Name == "driver_stats" {
			view = &s.reg.FeatureViews[i]
			break
		}
	}
	if view == nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	entityEvents := make([]historical.EntityEvent, len(req.EntityEvents))
	for i, e := range req.EntityEvents {
		entityEvents[i] = historical.EntityEvent{
			EntityID:       e.EntityID,
			EventTimestamp: e.EventTimestamp,
		}
	}

	trainingRows, err := historical.GetHistoricalFeatures(s.offline, *view, entityEvents)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	rows := make([]TrainingRowResponse, len(trainingRows))
	for i, tr := range trainingRows {
		features := tr.Features
		if features == nil {
			features = map[string]any{}
		}
		rows[i] = TrainingRowResponse{
			EntityID:       tr.EntityID,
			EventTimestamp: tr.EventTimestamp,
			Features:       features,
		}
	}

	writeJSON(w, http.StatusOK, HistoricalResponse{TrainingRows: rows})
}
