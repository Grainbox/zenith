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
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
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
			g.writeError(w, http.StatusBadRequest, "INVALID_JSON", "request body is empty")
			return
		}
		g.writeError(w, http.StatusBadRequest, "INVALID_JSON", "failed to decode JSON: "+err.Error())
		return
	}

	// Validate required fields
	if req.EventID == "" {
		g.writeError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "event_id is required")
		return
	}
	if req.EventType == "" {
		g.writeError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "event_type is required")
		return
	}
	if req.Source == "" {
		g.writeError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "source is required")
		return
	}

	// Read API key from header
	apiKey := r.Header.Get("X-Api-Key")
	if apiKey == "" {
		g.writeError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "X-Api-Key header is required")
		return
	}

	// Resolve source by API key
	source, err := g.sourceRepo.GetByAPIKey(ctx, apiKey)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			g.writeError(w, http.StatusInternalServerError, "INTERNAL", "request context canceled")
			return
		}
		g.writeError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "invalid API key")
		return
	}

	// Verify source name matches
	if source.Name != req.Source {
		g.logger.Warn("Source name mismatch",
			"expected", source.Name,
			"got", req.Source,
			"api_key", apiKey,
		)
		g.writeError(w, http.StatusForbidden, "PERMISSION_DENIED", "source name does not match authenticated source")
		return
	}

	// Use payload as-is; default to empty object if missing
	payload := req.Payload
	if len(payload) == 0 {
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

	// Inject trace context into event for propagation to async workers
	carrier := propagation.MapCarrier{}
	otel.GetTextMapPropagator().Inject(ctx, carrier)
	domainEvent.TraceContext = map[string]string(carrier)

	// Enqueue to pipeline
	if err := g.pipeline.Enqueue(domainEvent); err != nil {
		if errors.Is(err, engine.ErrPipelineFull) {
			g.writeError(w, http.StatusServiceUnavailable, "RESOURCE_EXHAUSTED", "event pipeline queue is full")
			return
		}
		g.logger.Error("Failed to enqueue event",
			"event_id", req.EventID,
			"error", err,
		)
		g.writeError(w, http.StatusInternalServerError, "INTERNAL", "failed to enqueue event")
		return
	}

	// Log successful ingestion
	g.logger.Info("Event received via gateway",
		"event_id", req.EventID,
		"event_type", req.EventType,
		"source", req.Source,
	)

	// Return 202 Accepted
	g.writeJSON(w, http.StatusAccepted, successResponse{
		Success: true,
		Message: "Event accepted",
	})
}

// writeJSON writes a JSON response with structured logging context.
func (g *Gateway) writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		g.logger.Error("Failed to encode JSON response", "error", err)
	}
}

// writeError writes an error response with the given status, code, and message.
func (g *Gateway) writeError(w http.ResponseWriter, status int, code, message string) {
	g.writeJSON(w, status, errorResponse{
		Code:    code,
		Message: message,
	})
}
