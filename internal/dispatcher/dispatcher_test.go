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

// noopAuditLog satisfies repository.AuditLogRepository without any side effects.
type noopAuditLog struct{}

func (n *noopAuditLog) Create(_ context.Context, _ *domain.AuditLog) error { return nil }

func TestDispatcherStartStop(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	matchCh := make(chan *domain.MatchedEvent, 10)

	registry := NewRegistry()
	registry.Register("http", &TestMockSink{name: "http"})

	d := New(matchCh, 2, registry, &noopAuditLog{}, logger, nil)
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
		SinkType:     "http",
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

	registry := NewRegistry()
	registry.Register("slow", slowSink)

	d := New(matchCh, 1, registry, &noopAuditLog{}, logger, nil)
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
		SinkType:     "slow",
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

// TestMockSink is a simple test sink that tracks if Send was called.
type TestMockSink struct {
	name       string
	sendCalled bool
}

func (s *TestMockSink) Name() string {
	return s.name
}

func (s *TestMockSink) Send(ctx context.Context, event *domain.MatchedEvent) error {
	s.sendCalled = true
	return nil
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

func TestRegistry_Register_Success(t *testing.T) {
	registry := NewRegistry()
	sink := &TestMockSink{name: "test"}

	registry.Register("test", sink)

	resolved, ok := registry.Resolve("test")
	if !ok {
		t.Fatal("expected sink to be resolved")
	}
	if resolved != sink {
		t.Fatal("expected same sink instance")
	}
}

func TestRegistry_Resolve_Unknown(t *testing.T) {
	registry := NewRegistry()

	_, ok := registry.Resolve("unknown")
	if ok {
		t.Fatal("expected unknown type to not be resolved")
	}
}

func TestRegistry_Register_Duplicate_Panics(t *testing.T) {
	registry := NewRegistry()
	sink := &TestMockSink{name: "test"}

	registry.Register("test", sink)

	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on duplicate registration")
		}
	}()

	registry.Register("test", sink)
}

func TestDispatcher_Routes_To_Correct_Sink(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	registry := NewRegistry()
	httpSink := &TestMockSink{name: "http"}
	discordSink := &TestMockSink{name: "discord"}

	registry.Register("http", httpSink)
	registry.Register("discord", discordSink)

	matchCh := make(chan *domain.MatchedEvent, 1)
	disp := New(matchCh, 1, registry, &noopAuditLog{}, logger, nil)

	matched := &domain.MatchedEvent{
		Event: &domain.Event{
			ID:     "test-event-1",
			Type:   "test",
			Source: "test-source",
		},
		Rule: &domain.Rule{
			ID:       uuid.New(),
			SinkType: "discord",
		},
	}

	disp.dispatch(context.Background(), matched, 0)

	if !discordSink.sendCalled {
		t.Fatal("expected discord sink to be called")
	}
	if httpSink.sendCalled {
		t.Fatal("expected http sink to not be called")
	}
}

func TestDispatcher_Unknown_Sink_Type_Warns(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	registry := NewRegistry()
	httpSink := &TestMockSink{name: "http"}
	registry.Register("http", httpSink)

	matchCh := make(chan *domain.MatchedEvent, 1)
	disp := New(matchCh, 1, registry, &noopAuditLog{}, logger, nil)

	matched := &domain.MatchedEvent{
		Event: &domain.Event{
			ID:     "test-event-1",
			Type:   "test",
			Source: "test-source",
		},
		Rule: &domain.Rule{
			ID:       uuid.New(),
			SinkType: "unknown",
		},
	}

	// Should not panic or error, just warn
	disp.dispatch(context.Background(), matched, 0)

	if httpSink.sendCalled {
		t.Fatal("expected no sink to be called")
	}
}
