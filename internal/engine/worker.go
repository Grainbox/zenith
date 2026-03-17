package engine

import (
	"context"

	"github.com/Grainbox/zenith/internal/domain"
)

// runWorker processes events from the pipeline's event channel.
func (p *Pipeline) runWorker(ctx context.Context, id int) {
	defer p.wg.Done()
	for event := range p.eventCh {
		p.processEvent(ctx, event, id)
	}
}

// processEvent handles a single event by evaluating it against rules and dispatching matches.
func (p *Pipeline) processEvent(ctx context.Context, event *domain.Event, workerID int) {
	matched, err := p.evaluator.Evaluate(ctx, event)
	if err != nil {
		p.logger.Error("Failed to evaluate event",
			"worker_id", workerID,
			"event_id", event.ID,
			"error", err,
		)
		return
	}

	// Forward matched events to dispatcher if wired
	for _, rule := range matched {
		if p.dispatchCh == nil {
			continue
		}
		me := &domain.MatchedEvent{Event: event, Rule: rule}
		select {
		case p.dispatchCh <- me:
		default:
			p.logger.Warn("Dispatch channel full, dropping matched event",
				"event_id", event.ID,
				"rule_id", rule.ID,
			)
		}
	}

	if len(matched) > 0 {
		p.logger.Info("Event matched rules",
			"worker_id", workerID,
			"event_id", event.ID,
			"matched_count", len(matched),
		)
	}
}
