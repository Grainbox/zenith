# Implementation Plan: Issue-602 — Platform Sinks (Discord + extensible architecture)

**Sprint:** 6 — The Dispatcher & Cloud Deployment
**Goal:** Replace `NoopSink` with real sink implementations. Implement `DiscordSink` for the portfolio demo and `HttpSink` as a generic fallback. The architecture must allow adding new platforms (Slack, Teams, etc.) as a one-file addition without touching the Dispatcher core.

---

## Context & Architecture Overview

The `Sink` interface and `Dispatcher` already exist from Issue-601. The Dispatcher currently fans out every `MatchedEvent` to **all** registered sinks — this must change.

**New routing model:** each `Rule` declares which sink type to use (`sink_type` column). The Dispatcher resolves the correct `Sink` from a registry at dispatch time and calls only that one.

```
MatchedEvent{Event, Rule{SinkType: "discord", TargetAction: "https://discord.com/api/webhooks/..."}}
    ↓
Dispatcher.dispatch()
    ↓
registry.Resolve("discord") → DiscordSink
    ↓
DiscordSink.Send(ctx, matched)
    ↓
POST {"embeds": [...]} to rule.TargetAction
```

**Key invariant:** `rule.TargetAction` is always the destination URL, regardless of sink type. No URL is stored in config or env vars.

---

## Files to Create or Modify

### New files

| File | Purpose |
|---|---|
| `deployments/db/migrations/000002_add_sink_type.up.sql` | Add `sink_type` column to `rules` table |
| `deployments/db/migrations/000002_add_sink_type.down.sql` | Rollback migration |
| `internal/dispatcher/registry.go` | `Registry` — maps sink type strings to `Sink` implementations |
| `internal/dispatcher/sinks/discord.go` | `DiscordSink` — formats payload as Discord embed and POSTs |
| `internal/dispatcher/sinks/http.go` | `HttpSink` — generic fallback, POSTs raw matched event JSON |
| `internal/dispatcher/sinks/discord_test.go` | Unit tests for DiscordSink |
| `internal/dispatcher/sinks/http_test.go` | Unit tests for HttpSink |

### Modified files

| File | Change |
|---|---|
| `internal/domain/models.go` | Add `SinkType string` field to `Rule` |
| `internal/repository/postgres/rule_repo.go` | Include `sink_type` in all SELECT/INSERT/UPDATE queries |
| `internal/dispatcher/dispatcher.go` | Replace `[]Sink` with `*Registry`; update `dispatch()` to route by sink type |
| `internal/dispatcher/sink.go` | Remove `NoopSink` (replaced by real sinks) |
| `cmd/ingestor/main.go` | Wire `DiscordSink` and `HttpSink` into the registry at startup |

---

## Step-by-Step Implementation

### Step 1 — Database migration

**File:** `deployments/db/migrations/000002_add_sink_type.up.sql`

```sql
ALTER TABLE rules ADD COLUMN IF NOT EXISTS sink_type TEXT NOT NULL DEFAULT 'http';
```

**File:** `deployments/db/migrations/000002_add_sink_type.down.sql`

```sql
ALTER TABLE rules DROP COLUMN IF EXISTS sink_type;
```

**Why `DEFAULT 'http'`:** All existing rules get the generic HTTP sink automatically. No data migration needed, no breaking change.

---

### Step 2 — Add `SinkType` to the domain model

**File:** `internal/domain/models.go`

```go
type Rule struct {
    ID           uuid.UUID       `json:"id"`
    SourceID     uuid.UUID       `json:"source_id"`
    Name         string          `json:"name"`
    Condition    json.RawMessage `json:"condition"`
    TargetAction string          `json:"target_action"`
    SinkType     string          `json:"sink_type"` // e.g. "discord", "http"
    IsActive     bool            `json:"is_active"`
    CreatedAt    time.Time       `json:"created_at"`
    UpdatedAt    time.Time       `json:"updated_at"`
}
```

---

### Step 3 — Update the repository

**File:** `internal/repository/postgres/rule_repo.go`

All queries that read or write rules must include `sink_type`. The three affected methods are `Create`, `GetByID`, and `ListBySourceID`.

`Create` — add `sink_type` to INSERT:
```go
query := `
    INSERT INTO rules (source_id, name, condition, target_action, sink_type, is_active)
    VALUES ($1, $2, $3, $4, $5, $6)
    RETURNING id, created_at, updated_at
`
err := r.db.QueryRowContext(ctx, query,
    ru.SourceID, ru.Name, ru.Condition, ru.TargetAction, ru.SinkType, ru.IsActive,
).Scan(&ru.ID, &ru.CreatedAt, &ru.UpdatedAt)
```

`GetByID` and `ListBySourceID` — add `sink_type` to SELECT and `.Scan()`:
```go
SELECT id, source_id, name, condition, target_action, sink_type, is_active, created_at, updated_at
FROM rules WHERE ...
```
```go
rows.Scan(&ru.ID, &ru.SourceID, &ru.Name, &ru.Condition, &ru.TargetAction, &ru.SinkType, &ru.IsActive, &ru.CreatedAt, &ru.UpdatedAt)
```

`Update` — add `sink_type` to the SET clause:
```go
query := `
    UPDATE rules
    SET name = $1, condition = $2, target_action = $3, sink_type = $4, is_active = $5, updated_at = now()
    WHERE id = $6
    RETURNING updated_at
`
```

---

### Step 4 — Implement the Registry

**File:** `internal/dispatcher/registry.go`

```go
package dispatcher

import "fmt"

// Registry maps sink type identifiers to Sink implementations.
// It is built once at startup and is read-only during operation.
type Registry struct {
    sinks map[string]Sink
}

// NewRegistry creates an empty Registry.
func NewRegistry() *Registry {
    return &Registry{sinks: make(map[string]Sink)}
}

// Register adds a Sink to the registry under the given type key.
// Panics if the type key is already registered — wiring errors must be caught at startup.
func (r *Registry) Register(sinkType string, sink Sink) {
    if _, exists := r.sinks[sinkType]; exists {
        panic(fmt.Sprintf("dispatcher: sink type %q already registered", sinkType))
    }
    r.sinks[sinkType] = sink
}

// Resolve returns the Sink registered for the given type, or false if not found.
func (r *Registry) Resolve(sinkType string) (Sink, bool) {
    s, ok := r.sinks[sinkType]
    return s, ok
}
```

**Why panic on duplicate registration:** wiring errors (two sinks registered under the same key) are programming errors, not runtime errors. Panicking at startup surfaces them immediately, unlike a silent overwrite.

---

### Step 5 — Implement `HttpSink`

**File:** `internal/dispatcher/sinks/http.go`

```go
package sinks

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "net/http"

    "github.com/Grainbox/zenith/internal/domain"
)

// HttpSink POSTs matched events as JSON to the URL in rule.TargetAction.
// It is the generic fallback sink for any HTTP endpoint that speaks Zenith's event format.
type HttpSink struct {
    client *http.Client
}

// NewHttpSink creates an HttpSink with the given HTTP client.
func NewHttpSink(client *http.Client) *HttpSink {
    return &HttpSink{client: client}
}

func (s *HttpSink) Name() string { return "http" }

func (s *HttpSink) Send(ctx context.Context, matched *domain.MatchedEvent) error {
    body, err := json.Marshal(matched)
    if err != nil {
        return fmt.Errorf("http sink: failed to marshal payload: %w", err)
    }

    req, err := http.NewRequestWithContext(ctx, http.MethodPost, matched.Rule.TargetAction, bytes.NewReader(body))
    if err != nil {
        return fmt.Errorf("http sink: failed to build request: %w", err)
    }
    req.Header.Set("Content-Type", "application/json")

    resp, err := s.client.Do(req)
    if err != nil {
        return fmt.Errorf("http sink: request failed: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode < 200 || resp.StatusCode >= 300 {
        return fmt.Errorf("http sink: unexpected status %d from %s", resp.StatusCode, matched.Rule.TargetAction)
    }
    return nil
}
```

---

### Step 6 — Implement `DiscordSink`

**File:** `internal/dispatcher/sinks/discord.go`

Discord Incoming Webhooks accept a JSON body with an `embeds` array. This produces a structured, readable message in the channel — better for a portfolio demo than raw JSON.

```go
package sinks

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "net/http"

    "github.com/Grainbox/zenith/internal/domain"
)

// discordPayload is the JSON body expected by Discord Incoming Webhooks.
type discordPayload struct {
    Embeds []discordEmbed `json:"embeds"`
}

type discordEmbed struct {
    Title  string         `json:"title"`
    Color  int            `json:"color"`  // decimal RGB
    Fields []discordField `json:"fields"`
}

type discordField struct {
    Name   string `json:"name"`
    Value  string `json:"value"`
    Inline bool   `json:"inline"`
}

// DiscordSink sends matched events as Discord embeds to the webhook URL in rule.TargetAction.
type DiscordSink struct {
    client *http.Client
}

// NewDiscordSink creates a DiscordSink with the given HTTP client.
func NewDiscordSink(client *http.Client) *DiscordSink {
    return &DiscordSink{client: client}
}

func (s *DiscordSink) Name() string { return "discord" }

func (s *DiscordSink) Send(ctx context.Context, matched *domain.MatchedEvent) error {
    payload := discordPayload{
        Embeds: []discordEmbed{
            {
                Title: fmt.Sprintf("Rule matched: %s", matched.Rule.Name),
                Color: 0xE74C3C, // red — signals an alert
                Fields: []discordField{
                    {Name: "Event ID", Value: matched.Event.ID, Inline: true},
                    {Name: "Source", Value: matched.Event.Source, Inline: true},
                    {Name: "Rule", Value: matched.Rule.Name, Inline: false},
                },
            },
        },
    }

    body, err := json.Marshal(payload)
    if err != nil {
        return fmt.Errorf("discord sink: failed to marshal payload: %w", err)
    }

    req, err := http.NewRequestWithContext(ctx, http.MethodPost, matched.Rule.TargetAction, bytes.NewReader(body))
    if err != nil {
        return fmt.Errorf("discord sink: failed to build request: %w", err)
    }
    req.Header.Set("Content-Type", "application/json")

    resp, err := s.client.Do(req)
    if err != nil {
        return fmt.Errorf("discord sink: request failed: %w", err)
    }
    defer resp.Body.Close()

    // Discord returns 204 No Content on success
    if resp.StatusCode != http.StatusNoContent {
        return fmt.Errorf("discord sink: unexpected status %d", resp.StatusCode)
    }
    return nil
}
```

**Why embeds:** Discord embeds render as a structured card with a colored sidebar — visually distinguishable from a bot message and much clearer in a portfolio demo screenshot than raw JSON.

---

### Step 7 — Refactor the Dispatcher

**File:** `internal/dispatcher/dispatcher.go`

Replace `[]Sink` with `*Registry` and update `dispatch()` to route by `rule.SinkType` instead of broadcasting to all sinks.

```go
type Dispatcher struct {
    matchCh     <-chan *domain.MatchedEvent
    registry    *Registry
    workerCount int
    logger      *slog.Logger
    wg          sync.WaitGroup
}

func New(matchCh <-chan *domain.MatchedEvent, workerCount int, registry *Registry, logger *slog.Logger) *Dispatcher {
    return &Dispatcher{
        matchCh:     matchCh,
        registry:    registry,
        workerCount: workerCount,
        logger:      logger,
    }
}
```

`dispatch()` routes to a single sink instead of broadcasting:

```go
func (d *Dispatcher) dispatch(ctx context.Context, matched *domain.MatchedEvent, workerID int) {
    sink, ok := d.registry.Resolve(matched.Rule.SinkType)
    if !ok {
        d.logger.Warn("No sink registered for type",
            "sink_type", matched.Rule.SinkType,
            "rule_id", matched.Rule.ID,
            "event_id", matched.Event.ID,
        )
        return
    }

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
```

**Why warn instead of error on unknown type:** an unknown `sink_type` is a data configuration issue (bad row in DB), not a system failure. The event is dropped with a warning and a traceable `rule_id` for investigation.

---

### Step 8 — Wire in `cmd/ingestor/main.go`

**File:** `cmd/ingestor/main.go` — update `setupPipeline`:

```go
func setupPipeline(cfg *config.Config, db *sql.DB, logger *slog.Logger) (*engine.Pipeline, *dispatcher.Dispatcher) {
    ruleRepo := postgres.NewRuleRepo(db)
    sourceRepo := postgres.NewSourceRepo(db)
    evaluator := engine.NewEvaluator(ruleRepo, sourceRepo, logger)
    pipeline := engine.New(cfg.Engine.WorkerCount, cfg.Engine.EventBufferSize, evaluator, logger)

    matchCh := make(chan *domain.MatchedEvent, 256)
    pipeline.SetDispatcher(matchCh)

    httpClient := &http.Client{Timeout: 10 * time.Second}

    registry := dispatcher.NewRegistry()
    registry.Register("http", sinks.NewHttpSink(httpClient))
    registry.Register("discord", sinks.NewDiscordSink(httpClient))

    disp := dispatcher.New(matchCh, 4, registry, logger)
    return pipeline, disp
}
```

**Why a shared `http.Client`:** a single client with connection pooling is more efficient than one client per sink. The 10s timeout prevents a slow target from blocking a dispatcher worker indefinitely.

---

## Testing Strategy

| Test | File | Approach |
|---|---|---|
| `HttpSink.Send` — success | `sinks/http_test.go` | `httptest.Server` returns 200; verify correct JSON body and `Content-Type` header |
| `HttpSink.Send` — non-2xx | `sinks/http_test.go` | `httptest.Server` returns 500; verify error returned |
| `HttpSink.Send` — timeout | `sinks/http_test.go` | `httptest.Server` sleeps > client timeout; verify error returned |
| `DiscordSink.Send` — success | `sinks/discord_test.go` | `httptest.Server` returns 204; verify body deserializes to `discordPayload` with correct `title` and `fields` |
| `DiscordSink.Send` — non-204 | `sinks/discord_test.go` | `httptest.Server` returns 200; verify error returned (Discord 200 = config error) |
| `Registry.Resolve` — known type | `dispatcher_test.go` | Register a sink, resolve it, verify same instance |
| `Registry.Resolve` — unknown type | `dispatcher_test.go` | Resolve unregistered type, verify `ok == false` |
| `Registry.Register` — duplicate | `dispatcher_test.go` | Register same type twice, verify panic |
| `Dispatcher` — routes to correct sink | `dispatcher_test.go` | Two sinks in registry, `MatchedEvent` with `SinkType="discord"`, verify only DiscordSink called |
| `Dispatcher` — unknown sink type | `dispatcher_test.go` | `MatchedEvent` with unregistered `SinkType`, verify warn logged, no error, no crash |

---

## Seed SQL for manual testing

```sql
-- Discord sink rule
INSERT INTO rules (source_id, name, condition, target_action, sink_type, is_active)
SELECT id,
       'high-value-discord-alert',
       '{"field":"amount","operator":">","value":100}'::jsonb,
       'https://discord.com/api/webhooks/<id>/<token>',
       'discord',
       true
FROM sources WHERE name = 'my-service';

-- Generic HTTP sink rule
INSERT INTO rules (source_id, name, condition, target_action, sink_type, is_active)
SELECT id,
       'http-forward',
       '{"field":"event_type","operator":"==","value":"payment.completed"}'::jsonb,
       'https://my-service.internal/zenith-hook',
       'http',
       true
FROM sources WHERE name = 'my-service';
```

---

## Definition of Done

- [ ] Migration `000002` applies cleanly with `make migrate-up`
- [ ] `go build ./...` passes with no errors
- [ ] `DiscordSink` and `HttpSink` tested with `httptest.Server`
- [ ] Dispatcher routes to the correct sink by `rule.SinkType`
- [ ] Unknown `sink_type` logs a warning and drops the event without crashing
- [ ] `NoopSink` removed from `sink.go`
- [ ] `golangci-lint run` passes with no new warnings
- [ ] All existing tests pass: `go test ./...`
- [ ] Manual end-to-end: send an event matching a `discord` rule → Discord embed appears in the target channel
