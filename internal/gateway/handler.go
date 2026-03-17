// Package gateway implements the HTTP/JSON webhook gateway for event ingestion.
package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/Grainbox/zenith/internal/domain"
	"github.com/Grainbox/zenith/internal/engine"
	"github.com/Grainbox/zenith/internal/repository"
)

// PipelineEnqueuer is the interface for enqueuing events to the pipeline.
type PipelineEnqueuer interface {
	Enqueue(event *domain.Event) error
}

const maxBodyBytes = 1 << 20 // 1 MB

// IngestEventRequest is the JSON request body for POST /v1/events.
type IngestEventRequest struct {
	EventID   string          `json:"event_id"`
	EventType string          `json:"event_type"`
	Source    string          `json:"source"`
	Payload   json.RawMessage `json:"payload"`
}

type successResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

type errorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Gateway handles HTTP/JSON webhook ingestion.
type Gateway struct {
	logger     *slog.Logger
	pipeline   PipelineEnqueuer
	sourceRepo repository.SourceRepository
}

// NewGateway creates a new Gateway.
func NewGateway(
	logger *slog.Logger,
	pipeline PipelineEnqueuer,
	sourceRepo repository.SourceRepository,
) *Gateway {
	return &Gateway{
		logger:     logger,
		pipeline:   pipeline,
		sourceRepo: sourceRepo,
	}
}

// HandleIngestEvent handles HTTP POST requests to /v1/events.
func (g *Gateway) HandleIngestEvent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Limit request body size
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	defer func() {
		if err := r.Body.Close(); err != nil {
			g.logger.Error("Failed to close request body", "error", err)
		}
	}()

	// Decode JSON request body
	var req IngestEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		if errors.Is(err, io.EOF) {
			writeError(w, http.StatusBadRequest, "INVALID_JSON", "request body is empty")
			return
		}
		writeError(w, http.StatusBadRequest, "INVALID_JSON", "failed to decode JSON: "+err.Error())
		return
	}

	// Validate required fields
	if req.EventID == "" {
		writeError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "event_id is required")
		return
	}
	if req.EventType == "" {
		writeError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "event_type is required")
		return
	}
	if req.Source == "" {
		writeError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "source is required")
		return
	}

	// Read API key from header
	apiKey := r.Header.Get("X-Api-Key")
	if apiKey == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "X-Api-Key header is required")
		return
	}

	// Resolve source by API key
	source, err := g.sourceRepo.GetByAPIKey(ctx, apiKey)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			writeError(w, http.StatusInternalServerError, "INTERNAL", "request context cancelled")
			return
		}
		writeError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "invalid API key")
		return
	}

	// Verify source name matches
	if source.Name != req.Source {
		g.logger.Warn("Source name mismatch",
			"expected", source.Name,
			"got", req.Source,
			"api_key", apiKey,
		)
		writeError(w, http.StatusForbidden, "PERMISSION_DENIED", "source name does not match authenticated source")
		return
	}

	// Ensure payload is not nil
	payload := []byte(req.Payload)
	if payload == nil {
		payload = []byte("{}")
	}

	// Create domain event
	domainEvent := &domain.Event{
		ID:        req.EventID,
		Type:      req.EventType,
		Source:    req.Source,
		Payload:   payload,
		Timestamp: time.Now().UTC(),
	}

	// Enqueue to pipeline
	if err := g.pipeline.Enqueue(domainEvent); err != nil {
		if errors.Is(err, engine.ErrPipelineFull) {
			writeError(w, http.StatusServiceUnavailable, "RESOURCE_EXHAUSTED", "event pipeline queue is full")
			return
		}
		g.logger.Error("Failed to enqueue event",
			"event_id", req.EventID,
			"error", err,
		)
		writeError(w, http.StatusInternalServerError, "INTERNAL", "failed to enqueue event")
		return
	}

	// Log successful ingestion
	g.logger.Info("Event received via gateway",
		"event_id", req.EventID,
		"event_type", req.EventType,
		"source", req.Source,
	)

	// Return 202 Accepted
	writeJSON(w, http.StatusAccepted, successResponse{
		Success: true,
		Message: "Event accepted",
	})
}

// writeJSON writes a JSON response.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("Failed to encode JSON response", "error", err)
	}
}

// writeError writes an error response.
func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorResponse{
		Code:    code,
		Message: message,
	})
}
