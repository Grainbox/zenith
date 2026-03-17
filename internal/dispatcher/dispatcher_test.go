package dispatcher

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/Grainbox/zenith/internal/domain"
	"github.com/google/uuid"
)

func TestDispatcherStartStop(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	matchCh := make(chan *domain.MatchedEvent, 10)
	sinks := []Sink{NoopSink{}}

	d := New(matchCh, 2, sinks, logger)
	d.Start(context.Background())

	// Send an event
	event := &domain.Event{
		ID:        "test-1",
		Type:      "test",
		Source:    "test-source",
		Payload:   []byte(`{"foo":"bar"}`),
		Timestamp: time.Now(),
	}
	rule := &domain.Rule{
		ID:           uuid.New(),
		SourceID:     uuid.New(),
		Name:         "test-rule",
		Condition:    []byte(`{"field":"foo","operator":"==","value":"bar"}`),
		TargetAction: "noop",
		IsActive:     true,
	}
	matched := &domain.MatchedEvent{Event: event, Rule: rule}
	matchCh <- matched

	// Close and wait for drain
	close(matchCh)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := d.Stop(ctx)
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}

func TestDispatcherStopTimeout(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	matchCh := make(chan *domain.MatchedEvent, 10)

	// Sink that blocks indefinitely
	slowSink := &TestSlowSink{
		delay: 10 * time.Second,
	}
	sinks := []Sink{slowSink}

	d := New(matchCh, 1, sinks, logger)
	d.Start(context.Background())

	event := &domain.Event{
		ID:        "test-2",
		Type:      "test",
		Source:    "test-source",
		Payload:   []byte(`{}`),
		Timestamp: time.Now(),
	}
	rule := &domain.Rule{
		ID:           uuid.New(),
		SourceID:     uuid.New(),
		Name:         "test-rule",
		Condition:    []byte(`{}`),
		TargetAction: "noop",
		IsActive:     true,
	}
	matchCh <- &domain.MatchedEvent{Event: event, Rule: rule}
	close(matchCh)

	// Try to stop with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := d.Stop(ctx)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected DeadlineExceeded, got %v", err)
	}
}

func TestNoopSinkSend(t *testing.T) {
	sink := NoopSink{}
	if sink.Name() != "noop" {
		t.Fatalf("expected name noop, got %s", sink.Name())
	}

	event := &domain.Event{ID: "test"}
	rule := &domain.Rule{ID: uuid.New()}
	matched := &domain.MatchedEvent{Event: event, Rule: rule}

	err := sink.Send(context.Background(), matched)
	if err != nil {
		t.Fatalf("NoopSink.Send should not error: %v", err)
	}
}

// TestSlowSink is a test sink that delays before sending.
type TestSlowSink struct {
	delay time.Duration
}

func (s *TestSlowSink) Name() string {
	return "test-slow"
}

func (s *TestSlowSink) Send(ctx context.Context, event *domain.MatchedEvent) error {
	select {
	case <-time.After(s.delay):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
