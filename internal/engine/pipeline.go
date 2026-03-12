// Package engine implements the event processing pipeline.
package engine

import (
	"context"
	"errors"
	"log/slog"
	"sync"

	"github.com/Grainbox/zenith/internal/domain"
)

// ErrPipelineFull is returned when the event buffer is at capacity.
var ErrPipelineFull = errors.New("event pipeline queue is full")

// Pipeline manages the event processing workflow with a configurable worker pool.
type Pipeline struct {
	eventCh     chan *domain.Event
	workerCount int
	logger      *slog.Logger
	wg          sync.WaitGroup
}

// New creates a new Pipeline with the given worker count and buffer size.
func New(workerCount, bufferSize int, logger *slog.Logger) *Pipeline {
	return &Pipeline{
		eventCh:     make(chan *domain.Event, bufferSize),
		workerCount: workerCount,
		logger:      logger,
	}
}

// Start launches the worker goroutines to process events from the channel.
func (p *Pipeline) Start(ctx context.Context) {
	for i := 0; i < p.workerCount; i++ {
		p.wg.Add(1)
		go p.runWorker(ctx, i)
	}
	p.logger.Info("Event pipeline started", "worker_count", p.workerCount)
}

// Enqueue submits an event to the pipeline without blocking.
// Returns ErrPipelineFull if the buffer is at capacity.
func (p *Pipeline) Enqueue(event *domain.Event) error {
	select {
	case p.eventCh <- event:
		return nil
	default:
		return ErrPipelineFull
	}
}

// Stop closes the event channel and waits for all workers to finish processing.
// Must be called after the gRPC server has stopped accepting requests.
func (p *Pipeline) Stop() {
	close(p.eventCh)
	p.wg.Wait()
	p.logger.Info("Event pipeline stopped")
}
