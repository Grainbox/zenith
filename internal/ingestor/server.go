// Package ingestor implements the logic for handling event ingestion.
package ingestor

import (
	"context"
	"log/slog"

	v1 "github.com/Grainbox/zenith/pkg/pb/proto/v1"
)

// Server handles incoming event ingestion requests.
type Server struct {
	logger *slog.Logger
}

// NewServer creates a new Server.
func NewServer(logger *slog.Logger) *Server {
	return &Server{
		logger: logger,
	}
}

// IngestEvent handles incoming event ingestion requests.
func (s *Server) IngestEvent(
	_ context.Context,
	req *v1.IngestEventRequest,
) (*v1.IngestEventResponse, error) {
	s.logger.Info("Event Received",
		"event_id", req.GetEvent().GetEventId(),
		"type", req.GetEvent().GetEventType(),
	)

	return &v1.IngestEventResponse{
		Success: true,
		Message: "Event handled by Zenith",
	}, nil
}
