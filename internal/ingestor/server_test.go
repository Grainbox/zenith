package ingestor

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"connectrpc.com/connect"
	"github.com/Grainbox/zenith/internal/domain"
	"github.com/Grainbox/zenith/internal/engine"
	"github.com/Grainbox/zenith/internal/repository"
	v1 "github.com/Grainbox/zenith/pkg/pb/proto/v1"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockRuleRepo for testing
type MockRuleRepo struct{}

func (m *MockRuleRepo) Create(_ context.Context, _ *domain.Rule) error {
	return nil
}

func (m *MockRuleRepo) GetByID(_ context.Context, _ uuid.UUID) (*domain.Rule, error) {
	return nil, errors.New("not implemented")
}

func (m *MockRuleRepo) ListBySourceID(_ context.Context, _ uuid.UUID, _ repository.ListOptions) ([]*domain.Rule, error) {
	return []*domain.Rule{}, nil
}

func (m *MockRuleRepo) Update(_ context.Context, _ *domain.Rule) error {
	return nil
}

func (m *MockRuleRepo) Delete(_ context.Context, _ uuid.UUID) error {
	return nil
}

// MockSourceRepo for testing
type MockSourceRepo struct{}

func (m *MockSourceRepo) Create(_ context.Context, _ *domain.Source) error {
	return nil
}

func (m *MockSourceRepo) GetByID(_ context.Context, _ uuid.UUID) (*domain.Source, error) {
	return nil, errors.New("not implemented")
}

func (m *MockSourceRepo) GetByAPIKey(_ context.Context, _ string) (*domain.Source, error) {
	return nil, errors.New("not implemented")
}

func (m *MockSourceRepo) GetByName(_ context.Context, _ string) (*domain.Source, error) {
	return nil, errors.New("not implemented")
}

func TestIngestEvent(t *testing.T) {
	// Setup logger to discard output during tests
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))

	// Create mock repos and evaluator
	ruleRepo := &MockRuleRepo{}
	sourceRepo := &MockSourceRepo{}
	evaluator := engine.NewEvaluator(ruleRepo, sourceRepo, logger)

	// Create pipeline with evaluator
	pipeline := engine.New(2, 10, evaluator, logger)
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
