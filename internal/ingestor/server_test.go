package ingestor

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"connectrpc.com/connect"
	"github.com/Grainbox/zenith/internal/engine"
	v1 "github.com/Grainbox/zenith/pkg/pb/proto/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIngestEvent(t *testing.T) {
	// Setup logger to discard output during tests
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	pipeline := engine.New(2, 10, logger)
	server := NewServer(logger, pipeline)

	t.Run("success - valid event", func(t *testing.T) {
		req := &v1.IngestEventRequest{
			Event: &v1.Event{
				EventId:   "test-id",
				EventType: "test.event",
				Source:    "test-source",
			},
		}

		resp, err := server.IngestEvent(context.Background(), req)

		require.NoError(t, err)
		assert.True(t, resp.Success)
		assert.Equal(t, "Event handled by Zenith", resp.Message)
	})

	t.Run("error - missing event", func(t *testing.T) {
		req := &v1.IngestEventRequest{
			Event: nil,
		}

		resp, err := server.IngestEvent(context.Background(), req)

		assert.Error(t, err)
		assert.Nil(t, resp)
		
		var connectErr *connect.Error
		require.ErrorAs(t, err, &connectErr)
		assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
		assert.Equal(t, "event is required", connectErr.Message())
	})
}
