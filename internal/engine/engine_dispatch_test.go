package engine

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/Grainbox/zenith/internal/domain"
	"github.com/Grainbox/zenith/internal/repository"
	"github.com/google/uuid"
)

var errNotFound = errors.New("not found")

// TestPipelineDispatch verifies that matched events are forwarded to the dispatch channel.
func TestPipelineDispatch(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	sourceID := uuid.New()
	ruleID := uuid.New()

	// Mock repositories
	sourceRepo := &testSourceRepo{
		sources: map[string]*domain.Source{
			"test-source": {
				ID:   sourceID,
				Name: "test-source",
			},
		},
	}
	ruleRepo := &testRuleRepo{
		rules: map[uuid.UUID][]*domain.Rule{
			sourceID: {
				{
					ID:        ruleID,
					SourceID:  sourceID,
					Name:      "test-rule",
					IsActive:  true,
					Condition: json.RawMessage(`{"field":"level","operator":"==","value":"error"}`),
				},
			},
		},
	}

	evaluator := NewEvaluator(ruleRepo, sourceRepo, logger, nil)
	pipeline := New(1, 10, evaluator, logger, nil)

	// Wire dispatcher
	dispatchCh := make(chan *domain.MatchedEvent, 10)
	pipeline.SetDispatcher(dispatchCh)

	pipeline.Start(context.Background())
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = pipeline.Stop(ctx)
	}()

	// Send event that matches
	event := &domain.Event{
		ID:        "evt-1",
		Type:      "log",
		Source:    "test-source",
		Payload:   []byte(`{"level":"error","msg":"oops"}`),
		Timestamp: time.Now(),
	}

	if err := pipeline.Enqueue(event); err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}

	// Wait for dispatch
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	select {
	case matched := <-dispatchCh:
		if matched.Event.ID != "evt-1" {
			t.Errorf("expected event ID evt-1, got %s", matched.Event.ID)
		}
		if matched.Rule.ID != ruleID {
			t.Errorf("expected rule ID %s, got %s", ruleID, matched.Rule.ID)
		}
	case <-ctx.Done():
		t.Fatal("timeout waiting for matched event on dispatch channel")
	}
}

// TestPipelineDispatchChannelFull verifies that a full dispatch channel is handled gracefully.
func TestPipelineDispatchChannelFull(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	sourceID := uuid.New()
	ruleID := uuid.New()

	sourceRepo := &testSourceRepo{
		sources: map[string]*domain.Source{
			"test-source": {
				ID:   sourceID,
				Name: "test-source",
			},
		},
	}
	ruleRepo := &testRuleRepo{
		rules: map[uuid.UUID][]*domain.Rule{
			sourceID: {
				{
					ID:        ruleID,
					SourceID:  sourceID,
					Name:      "test-rule",
					IsActive:  true,
					Condition: json.RawMessage(`{"field":"x","operator":"==","value":"y"}`),
				},
			},
		},
	}

	evaluator := NewEvaluator(ruleRepo, sourceRepo, logger, nil)
	pipeline := New(1, 10, evaluator, logger, nil)

	// Wire a small dispatch channel to test backpressure
	dispatchCh := make(chan *domain.MatchedEvent, 1)
	pipeline.SetDispatcher(dispatchCh)

	pipeline.Start(context.Background())
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = pipeline.Stop(ctx)
	}()

	// Fill the dispatch channel
	event1 := &domain.Event{
		ID:      "evt-1",
		Type:    "test",
		Source:  "test-source",
		Payload: []byte(`{"x":"y"}`),
	}
	if err := pipeline.Enqueue(event1); err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}

	// Give time for first event to reach dispatch channel
	time.Sleep(100 * time.Millisecond)

	// Send a second event (should hit backpressure and log warning)
	event2 := &domain.Event{
		ID:      "evt-2",
		Type:    "test",
		Source:  "test-source",
		Payload: []byte(`{"x":"y"}`),
	}
	if err := pipeline.Enqueue(event2); err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}

	// Give time for processing
	time.Sleep(100 * time.Millisecond)

	// The first event should be in the channel, the second should have triggered a warning
	// (but not an error—the pipeline should handle backpressure gracefully)
	if len(dispatchCh) == 0 {
		t.Fatal("expected at least one matched event in dispatch channel")
	}
}

type testSourceRepo struct {
	sources map[string]*domain.Source
}

func (r *testSourceRepo) Create(ctx context.Context, source *domain.Source) error {
	return nil
}

func (r *testSourceRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Source, error) {
	return nil, errNotFound
}

func (r *testSourceRepo) GetByAPIKey(ctx context.Context, apiKey string) (*domain.Source, error) {
	return nil, errNotFound
}

func (r *testSourceRepo) GetByName(ctx context.Context, name string) (*domain.Source, error) {
	return r.sources[name], nil
}

type testRuleRepo struct {
	rules map[uuid.UUID][]*domain.Rule
}

func (r *testRuleRepo) Create(ctx context.Context, rule *domain.Rule) error {
	return nil
}

func (r *testRuleRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Rule, error) {
	return nil, errNotFound
}

func (r *testRuleRepo) ListBySourceID(ctx context.Context, sourceID uuid.UUID, opts repository.ListOptions) ([]*domain.Rule, error) {
	return r.rules[sourceID], nil
}

func (r *testRuleRepo) Update(ctx context.Context, rule *domain.Rule) error {
	return nil
}

func (r *testRuleRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return nil
}
