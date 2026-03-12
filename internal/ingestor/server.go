// Package ingestor implements the logic for handling event ingestion.
package ingestor

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"connectrpc.com/connect"
	"github.com/Grainbox/zenith/internal/domain"
	"github.com/Grainbox/zenith/internal/engine"
	v1 "github.com/Grainbox/zenith/pkg/pb/proto/v1"
)

// Server handles incoming event ingestion requests.
type Server struct {
	logger   *slog.Logger
	pipeline *engine.Pipeline
}

// NewServer creates a new Server.
func NewServer(logger *slog.Logger, pipeline *engine.Pipeline) *Server {
	return &Server{
		logger:   logger,
		pipeline: pipeline,
	}
}

// IngestEvent handles incoming event ingestion requests.
func (s *Server) IngestEvent(
	_ context.Context,
	req *v1.IngestEventRequest,
) (*v1.IngestEventResponse, error) {
	if req.GetEvent() == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("event is required"))
	}

	protoEvent := req.GetEvent()

	// Convert protobuf event to domain event
	timestamp := time.Now()
	if protoEvent.Timestamp != nil {
		timestamp = protoEvent.Timestamp.AsTime()
	}

	domainEvent := &domain.Event{
		ID:        protoEvent.GetEventId(),
		Type:      protoEvent.GetEventType(),
		Source:    protoEvent.GetSource(),
		Payload:   protoEvent.GetPayload(),
		Timestamp: timestamp,
	}

	// Enqueue to pipeline
	if err := s.pipeline.Enqueue(domainEvent); err != nil {
		if errors.Is(err, engine.ErrPipelineFull) {
			return nil, connect.NewError(connect.CodeResourceExhausted, errors.New("event pipeline queue is full"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	s.logger.Info("Event Received",
		"event_id", protoEvent.GetEventId(),
		"event_type", protoEvent.GetEventType(),
		"source", protoEvent.GetSource(),
	)

	return &v1.IngestEventResponse{
		Success: true,
		Message: "Event handled by Zenith",
	}, nil
}
