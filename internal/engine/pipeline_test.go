package engine

import (
	"context"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/Grainbox/zenith/internal/domain"
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
