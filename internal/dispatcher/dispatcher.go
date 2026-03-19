package dispatcher

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/Grainbox/zenith/internal/domain"
	"github.com/Grainbox/zenith/internal/repository"
)

// Dispatcher reads matched events from a channel and forwards them to sinks.
type Dispatcher struct {
	matchCh     <-chan *domain.MatchedEvent
	registry    *Registry
	auditLog    repository.AuditLogRepository
	workerCount int
	logger      *slog.Logger
	wg          sync.WaitGroup
}

// New creates a Dispatcher that reads from matchCh and dispatches to sinks via the registry.
func New(matchCh <-chan *domain.MatchedEvent, workerCount int, registry *Registry, auditLog repository.AuditLogRepository, logger *slog.Logger) *Dispatcher {
	return &Dispatcher{
		matchCh:     matchCh,
		registry:    registry,
		auditLog:    auditLog,
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
	d.logger.Info("Dispatcher started", "worker_count", d.workerCount)
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
	start := time.Now()

	sink, ok := d.registry.Resolve(matched.Rule.SinkType)
	if !ok {
		d.logger.Warn("No sink registered for type",
			"sink_type", matched.Rule.SinkType,
			"rule_id", matched.Rule.ID,
			"event_id", matched.Event.ID,
		)
		d.writeAuditLog(ctx, matched, time.Since(start), fmt.Errorf("unknown sink type: %s", matched.Rule.SinkType))
		return
	}

	err := sink.Send(ctx, matched)
	if err != nil {
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
	d.writeAuditLog(ctx, matched, time.Since(start), err)
}

func (d *Dispatcher) writeAuditLog(ctx context.Context, matched *domain.MatchedEvent, latency time.Duration, dispatchErr error) {
	sourceID := matched.Rule.SourceID
	log := &domain.AuditLog{
		EventID:             matched.Event.ID,
		SourceID:            &sourceID,
		ProcessingLatencyMs: latency.Milliseconds(),
	}
	if dispatchErr != nil {
		log.Status = "FAILED"
		msg := dispatchErr.Error()
		log.ErrorMessage = &msg
	} else {
		log.Status = "SUCCESS"
	}

	if err := d.auditLog.Create(ctx, log); err != nil {
		d.logger.Error("Failed to write audit log",
			"event_id", matched.Event.ID,
			"rule_id", matched.Rule.ID,
			"error", err,
		)
	}
}
