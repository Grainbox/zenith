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
	dispatchCh  chan<- *domain.MatchedEvent
	workerCount int
	evaluator   *Evaluator
	logger      *slog.Logger
	wg          sync.WaitGroup
}

// New creates a new Pipeline with the given worker count, buffer size, and evaluator.
func New(workerCount, bufferSize int, evaluator *Evaluator, logger *slog.Logger) *Pipeline {
	return &Pipeline{
		eventCh:     make(chan *domain.Event, bufferSize),
		dispatchCh:  nil,
		workerCount: workerCount,
		evaluator:   evaluator,
		logger:      logger,
	}
}

// SetDispatcher wires a dispatch channel for matched events. Must be called before Start.
func (p *Pipeline) SetDispatcher(ch chan<- *domain.MatchedEvent) {
	p.dispatchCh = ch
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
// Returns context.DeadlineExceeded if workers do not drain within ctx's deadline.
func (p *Pipeline) Stop(ctx context.Context) error {
	close(p.eventCh)

	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		if p.dispatchCh != nil {
			close(p.dispatchCh)
		}
		close(done)
	}()

	select {
	case <-done:
		p.logger.Info("Event pipeline stopped cleanly")
		return nil
	case <-ctx.Done():
		p.logger.Warn("Pipeline drain timed out; some in-flight events may be lost")
		return ctx.Err()
	}
}
