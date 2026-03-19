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
