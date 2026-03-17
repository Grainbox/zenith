# Implementation Plan: Issue-601 — Dispatcher Service

**Sprint:** 6 — The Dispatcher & Cloud Deployment
**Goal:** Create the `cmd/dispatcher/main.go` binary and the `internal/dispatcher/` package with graceful shutdown, mirroring the Ingestor pattern. Wire the Rule Engine to forward matched events to the Dispatcher via an internal channel.

---

## Context & Architecture Overview

Currently, `engine/worker.go` evaluates events against rules and **logs** the matches — but never acts on them. The Dispatcher closes this gap: it receives matched events from the Rule Engine and forwards them to external sinks.

**Coupling strategy:** For Sprint 6, the Rule Engine and Dispatcher communicate via a **shared Go channel** (`chan *domain.MatchedEvent`). This is the "internal channel" mentioned in the roadmap. The channel is the seam that will later be replaced by a message broker (Kafka/NATS) for cross-process communication in production.

**Standalone binary:** `cmd/dispatcher/main.go` is independently startable. In Sprint 6, the Ingestor and Dispatcher share a process space (same Kind cluster, different Deployments via shared channel wired at startup). The abstraction makes replacing the channel with a broker a one-line change.

---

## Files to Create or Modify

### New files

| File | Purpose |
|---|---|
| `internal/dispatcher/dispatcher.go` | `Dispatcher` struct: reads from `MatchedEventCh`, fans out to sinks |
| `internal/dispatcher/sink.go` | `Sink` interface + `NoopSink` placeholder |
| `cmd/dispatcher/main.go` | Standalone binary entry point with graceful shutdown |

### Modified files

| File | Change |
|---|---|
| `internal/domain/models.go` | Add `MatchedEvent` type |
| `internal/engine/pipeline.go` | Add `dispatchCh chan *domain.MatchedEvent` field; expose `SetDispatcher` wiring method |
| `internal/engine/worker.go` | Forward matched events to `dispatchCh` if set |
| `cmd/ingestor/main.go` | Wire `dispatchCh` between Pipeline and Dispatcher at startup |

---

## Step-by-Step Implementation

### Step 1 — Add `MatchedEvent` to domain models

**File:** `internal/domain/models.go`

Add at the end of the file:

```go
// MatchedEvent pairs an event with the rule it matched.
// It is produced by the Rule Engine and consumed by the Dispatcher.
type MatchedEvent struct {
    Event *Event
    Rule  *Rule
}
```

**Why:** A clean domain type decouples the engine output from the dispatcher input. The dispatcher doesn't need to know about evaluation internals.

---

### Step 2 — Define the `Sink` interface

**File:** `internal/dispatcher/sink.go`

```go
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

func (NoopSink) Name() string { return "noop" }

func (NoopSink) Send(_ context.Context, _ *domain.MatchedEvent) error {
    return nil
}
```

**Why:** The `Sink` interface enables Issue-602 (Slack + webhook adapters) to be added without touching the Dispatcher core. `NoopSink` makes the Dispatcher functional and testable in this sprint.

---

### Step 3 — Implement the `Dispatcher`

**File:** `internal/dispatcher/dispatcher.go`

```go
// Package dispatcher implements the event dispatching service.
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
```

**Why:** Mirrors the `Pipeline` struct exactly (channel in, workers, WaitGroup, `Start`/`Stop`). The worker fan-out over `sinks` makes adding new sinks in Issue-602 trivial.

---

### Step 4 — Wire the Engine to emit matched events

**File:** `internal/engine/pipeline.go`

Add a `dispatchCh` field to `Pipeline` and a method to set it:

```go
type Pipeline struct {
    eventCh     chan *domain.Event
    dispatchCh  chan<- *domain.MatchedEvent  // nil = no dispatcher wired
    workerCount int
    evaluator   *Evaluator
    logger      *slog.Logger
    wg          sync.WaitGroup
}

// SetDispatcher wires a dispatch channel. Must be called before Start.
func (p *Pipeline) SetDispatcher(ch chan<- *domain.MatchedEvent) {
    p.dispatchCh = ch
}
```

**File:** `internal/engine/worker.go`

Modify `processEvent` to forward matched events:

```go
func (p *Pipeline) processEvent(ctx context.Context, event *domain.Event, workerID int) {
    matched, err := p.evaluator.Evaluate(ctx, event)
    if err != nil {
        p.logger.Error("Failed to evaluate event", "worker_id", workerID, "event_id", event.ID, "error", err)
        return
    }

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
        p.logger.Info("Event matched rules", "worker_id", workerID, "event_id", event.ID, "matched_count", len(matched))
    }
}
```

**Why:** Non-blocking send (`select/default`) prevents a slow dispatcher from stalling the Rule Engine workers. The `nil` guard keeps the engine functional without a dispatcher (backward compatible).

---

### Step 5 — Create `cmd/dispatcher/main.go`

```go
// Package main implements the Dispatcher entry point.
package main

import (
    "context"
    "errors"
    "log/slog"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/Grainbox/zenith/internal/config"
    "github.com/Grainbox/zenith/internal/dispatcher"
    "github.com/Grainbox/zenith/internal/domain"
)

const (
    drainTimeout = 30 * time.Second
    matchBufSize = 256
)

func main() {
    if err := run(); err != nil {
        slog.Error("Dispatcher failure", "error", err)
        os.Exit(1)
    }
}

func run() error {
    logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
    slog.SetDefault(logger)

    cfg, err := config.Load()
    if err != nil {
        return err
    }
    _ = cfg // used by sinks in Issue-602

    matchCh := make(chan *domain.MatchedEvent, matchBufSize)

    sinks := []dispatcher.Sink{
        dispatcher.NoopSink{},
    }

    d := dispatcher.New(matchCh, 4, sinks, logger)
    d.Start(context.Background())

    stop := make(chan os.Signal, 1)
    signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

    sig := <-stop
    logger.Info("Shutting down dispatcher...", "signal", sig.String())

    close(matchCh)

    ctx, cancel := context.WithTimeout(context.Background(), drainTimeout)
    defer cancel()

    if err := d.Stop(ctx); err != nil && !errors.Is(err, context.DeadlineExceeded) {
        logger.Warn("Dispatcher drain incomplete", "error", err)
    }

    logger.Info("Dispatcher exited properly")
    return nil
}
```

**Note:** In this standalone mode, `matchCh` is never written to (the Ingestor is in another process). In Issue-604, this binary will be wired to a broker. For local integration testing, a combined launcher can create a shared channel and pass it to both the Pipeline and Dispatcher.

---

### Step 6 — Wire in `cmd/ingestor/main.go`

In `setupPipeline`, create the shared channel and wire the dispatcher:

```go
func setupPipeline(cfg *config.Config, db *sql.DB, logger *slog.Logger) (*engine.Pipeline, *dispatcher.Dispatcher) {
    ruleRepo := postgres.NewRuleRepo(db)
    sourceRepo := postgres.NewSourceRepo(db)
    evaluator := engine.NewEvaluator(ruleRepo, sourceRepo, logger)
    pipeline := engine.New(cfg.Engine.WorkerCount, cfg.Engine.EventBufferSize, evaluator, logger)

    matchCh := make(chan *domain.MatchedEvent, 256)
    pipeline.SetDispatcher(matchCh)

    sinks := []dispatcher.Sink{dispatcher.NoopSink{}}
    d := dispatcher.New(matchCh, 4, sinks, logger)

    return pipeline, d
}
```

Update `run()` to start/stop the dispatcher alongside the pipeline (close `matchCh` after pipeline stops, before draining the dispatcher).

---

## Config Env Vars (no new required vars for Issue-601)

Issue-601 uses only existing config. Issue-602 will add:
- `SLACK_WEBHOOK_URL` (already in `SecretsConfig`)
- `WEBHOOK_TARGET_URL` (new, to add in Issue-602)
- `DISPATCHER_WORKER_COUNT` (optional, can add in Issue-602)

---

## Testing Strategy

| Test | Location | Approach |
|---|---|---|
| `Dispatcher.Start/Stop` lifecycle | `internal/dispatcher/dispatcher_test.go` | Buffered channel, send N events, verify all dispatched before Stop returns |
| `NoopSink.Send` | same file | Trivial, just confirm no error |
| `Pipeline` → `Dispatcher` fan-out | `internal/engine/pipeline_test.go` | Inject `dispatchCh`, send event that matches a rule, verify message received on channel |
| Dispatcher full channel backpressure | `internal/dispatcher/dispatcher_test.go` | Full channel + `default` branch → verify warning logged, no deadlock |

No new integration tests needed for Issue-601 (Issue-603 will add audit log integration tests).

---

## Shutdown Sequence

The correct shutdown order prevents in-flight event loss:

```
SIGTERM received
    ↓
HTTP server.Shutdown()      # stop accepting new requests
    ↓
pipeline.Stop()             # drain eventCh, workers finish → matchCh writes stop
    ↓
close(matchCh)              # signal dispatcher workers EOF
    ↓
dispatcher.Stop()           # drain matchCh, all sinks called
    ↓
db.Close()
```

This mirrors the existing ingestor pattern and ensures no matched event is dropped on graceful shutdown.

---

## Definition of Done

- [ ] `cmd/dispatcher/main.go` compiles and starts standalone with `go run cmd/dispatcher/main.go`
- [ ] `internal/dispatcher/` package has `Dispatcher`, `Sink`, `NoopSink`
- [ ] `engine/worker.go` forwards matched events to `dispatchCh` when wired
- [ ] `cmd/ingestor/main.go` wires pipeline → dispatcher via shared channel
- [ ] All existing tests pass: `go test ./...`
- [ ] New dispatcher unit tests pass
- [ ] `golangci-lint run` passes with no new warnings
