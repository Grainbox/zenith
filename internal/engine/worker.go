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

// processEvent handles a single event. Currently a placeholder for rule evaluation (Issue-402).
func (p *Pipeline) processEvent(_ context.Context, event *domain.Event, workerID int) {
	p.logger.Info("processing event",
		"worker_id", workerID,
		"event_id", event.ID,
		"event_type", event.Type,
		"source", event.Source,
	)
}
