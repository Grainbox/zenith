package dispatcher

import (
	"context"
	"log/slog"
	"sync"

	"github.com/Grainbox/zenith/internal/domain"
)

// Dispatcher reads matched events from a channel and forwards them to sinks.
type Dispatcher struct {
	matchCh     <-chan *domain.MatchedEvent
	sinks       []Sink
	workerCount int
	logger      *slog.Logger
	wg          sync.WaitGroup
}

// New creates a Dispatcher that reads from matchCh and dispatches to sinks.
func New(matchCh <-chan *domain.MatchedEvent, workerCount int, sinks []Sink, logger *slog.Logger) *Dispatcher {
	return &Dispatcher{
		matchCh:     matchCh,
		sinks:       sinks,
		workerCount: workerCount,
		logger:      logger,
	}
}

// Start launches the dispatcher worker goroutines.
func (d *Dispatcher) Start(ctx context.Context) {
	for i := 0; i < d.workerCount; i++ {
		d.wg.Add(1)
		go d.runWorker(ctx, i)
	}
	d.logger.Info("Dispatcher started", "worker_count", d.workerCount, "sink_count", len(d.sinks))
}

// Stop waits for all in-flight dispatches to complete within the deadline.
func (d *Dispatcher) Stop(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		d.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		d.logger.Info("Dispatcher stopped cleanly")
		return nil
	case <-ctx.Done():
		d.logger.Warn("Dispatcher drain timed out; some in-flight events may be lost")
		return ctx.Err()
	}
}

func (d *Dispatcher) runWorker(ctx context.Context, id int) {
	defer d.wg.Done()
	for matched := range d.matchCh {
		d.dispatch(ctx, matched, id)
	}
}

func (d *Dispatcher) dispatch(ctx context.Context, matched *domain.MatchedEvent, workerID int) {
	for _, sink := range d.sinks {
		if err := sink.Send(ctx, matched); err != nil {
			d.logger.Error("Sink dispatch failed",
				"worker_id", workerID,
				"sink", sink.Name(),
				"event_id", matched.Event.ID,
				"rule_id", matched.Rule.ID,
				"error", err,
			)
		} else {
			d.logger.Info("Event dispatched",
				"worker_id", workerID,
				"sink", sink.Name(),
				"event_id", matched.Event.ID,
				"rule_id", matched.Rule.ID,
			)
		}
	}
}
