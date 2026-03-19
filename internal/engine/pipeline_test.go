package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Grainbox/zenith/internal/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestPipelineStopRespectsDeadline(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	pipeline := &Pipeline{
		eventCh:   make(chan *domain.Event, 1),
		wg:        sync.WaitGroup{},
		logger:    logger,
	}

	// Add a worker that will take time to finish
	pipeline.wg.Add(1)
	go func() {
		defer pipeline.wg.Done()
		time.Sleep(2 * time.Second)
	}()

	// Stop with very short timeout (10 millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	// Stop should return immediately with context error, not hang
	start := time.Now()
	err := pipeline.Stop(ctx)
	duration := time.Since(start)

	// Should return with deadline exceeded error
	if err == nil {
		t.Error("Expected Stop() to return context.DeadlineExceeded, but got nil")
	}

	// Should return within reasonable time (not wait for worker)
	if duration > 500*time.Millisecond {
		t.Errorf("Stop() took too long: %v", duration)
	}
}

func TestPipelineStopCompletesCleanly(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	pipeline := &Pipeline{
		eventCh:   make(chan *domain.Event, 1),
		wg:        sync.WaitGroup{},
		logger:    logger,
	}

	// Add a worker that finishes quickly
	pipeline.wg.Add(1)
	go func() {
		defer pipeline.wg.Done()
		time.Sleep(5 * time.Millisecond)
	}()

	// Stop with generous timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := pipeline.Stop(ctx)
	if err != nil {
		t.Errorf("Expected Stop() to complete cleanly, but got error: %v", err)
	}
}

// newStressEvaluator creates an Evaluator with a mock source and one active rule,
// ready for stress testing. It discards logs to avoid noise.
func newStressEvaluator(t *testing.T) *Evaluator {
	t.Helper()
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	sourceID := uuid.New()
	source := &domain.Source{
		ID:   sourceID,
		Name: "stress_source",
	}

	condJSON, err := json.Marshal(domain.Condition{
		Field:    "value",
		Operator: ">",
		Value:    0.0,
	})
	require.NoError(t, err)

	rule := &domain.Rule{
		ID:           uuid.New(),
		SourceID:     sourceID,
		Name:         "catch_all",
		Condition:    condJSON,
		TargetAction: "log",
		IsActive:     true,
	}

	srcRepo := NewMockSourceRepository()
	srcRepo.AddSource("stress_source", source)

	ruleRepo := NewMockRuleRepository()
	ruleRepo.AddRules(sourceID, []*domain.Rule{rule})

	return NewEvaluator(ruleRepo, srcRepo, logger, nil)
}

// TestPipelineStress_BurstConcurrency validates that the pipeline handles
// thousands of concurrent Enqueue() calls without data races or deadlocks.
// All 5000 events fit in the buffer, so accepted count should equal 5000.
func TestPipelineStress_BurstConcurrency(t *testing.T) {
	const numEvents = 5000
	const workerCount = 8
	const bufferSize = 10000

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	evaluator := newStressEvaluator(t)
	pipeline := New(workerCount, bufferSize, evaluator, logger, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pipeline.Start(ctx)

	var accepted atomic.Int64
	var wg sync.WaitGroup
	var startGate sync.WaitGroup
	startGate.Add(1)

	// Launch 5000 goroutines that all try to enqueue simultaneously
	for i := 0; i < numEvents; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			startGate.Wait() // Wait for all goroutines to be ready
			event := &domain.Event{
				ID:     fmt.Sprintf("evt_%d", idx),
				Type:   "stress_test",
				Source: "stress_source",
				Payload: func() []byte {
					b, _ := json.Marshal(map[string]interface{}{"value": 1.0})
					return b
				}(),
			}
			if err := pipeline.Enqueue(event); err == nil {
				accepted.Add(1)
			}
		}(i)
	}

	startGate.Done() // Unblock all goroutines simultaneously
	wg.Wait()       // Wait for all enqueues to complete

	// All events should have been accepted (buffer is large enough)
	require.Equal(t, int64(numEvents), accepted.Load(), "Expected all events to be accepted")

	// Stop with generous timeout; should complete cleanly
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer stopCancel()

	err := pipeline.Stop(stopCtx)
	require.NoError(t, err, "Pipeline should stop cleanly without deadlock")
}

// TestPipelineStress_PipelineFullBackpressure validates that the pipeline
// correctly rejects events when the buffer is full, and that no events are
// lost beyond the intended drops due to ErrPipelineFull.
func TestPipelineStress_PipelineFullBackpressure(t *testing.T) {
	const numEvents = 5000
	const numGoroutines = 50
	const workerCount = 8
	const bufferSize = 100 // Small buffer to trigger backpressure

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	evaluator := newStressEvaluator(t)
	pipeline := New(workerCount, bufferSize, evaluator, logger, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pipeline.Start(ctx)

	var accepted, dropped atomic.Int64
	var wg sync.WaitGroup
	var startGate sync.WaitGroup
	startGate.Add(1)

	// Distribute 5000 events across 50 goroutines
	eventsPerGoroutine := numEvents / numGoroutines

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			startGate.Wait() // Wait for all goroutines to be ready

			for j := 0; j < eventsPerGoroutine; j++ {
				eventIdx := goroutineID*eventsPerGoroutine + j
				event := &domain.Event{
					ID:     fmt.Sprintf("evt_%d", eventIdx),
					Type:   "stress_test",
					Source: "stress_source",
					Payload: func() []byte {
						b, _ := json.Marshal(map[string]interface{}{"value": 1.0})
						return b
					}(),
				}
				if err := pipeline.Enqueue(event); err == ErrPipelineFull {
					dropped.Add(1)
				} else if err == nil {
					accepted.Add(1)
				}
			}
		}(i)
	}

	startGate.Done() // Unblock all goroutines simultaneously
	wg.Wait()       // Wait for all enqueues to complete

	totalSent := accepted.Load() + dropped.Load()
	require.Equal(t, int64(numEvents), totalSent,
		"Expected accepted + dropped to equal total events sent")

	// Stop with generous timeout; should complete cleanly even under backpressure
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer stopCancel()

	err := pipeline.Stop(stopCtx)
	require.NoError(t, err, "Pipeline should stop cleanly without deadlock, even under backpressure")
}
