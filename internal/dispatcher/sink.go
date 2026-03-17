// Package dispatcher implements the event dispatching service.
package dispatcher

import (
	"context"

	"github.com/Grainbox/zenith/internal/domain"
)

// Sink sends a matched event to an external target.
type Sink interface {
	Name() string
	Send(ctx context.Context, event *domain.MatchedEvent) error
}

// NoopSink is a placeholder sink that logs and discards matched events.
// It is replaced by real sinks in Issue-602.
type NoopSink struct{}

// Name returns the sink name.
func (NoopSink) Name() string { return "noop" }

// Send discards the matched event without error.
func (NoopSink) Send(_ context.Context, _ *domain.MatchedEvent) error {
	return nil
}
