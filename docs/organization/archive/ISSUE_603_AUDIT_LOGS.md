# Plan: Issue-603 — Audit Log Write-Back

## Context

The `audit_logs` table was provisioned in Issue-302 but never written to. Issue-603 closes the observability loop: after every dispatch attempt (success or failure), the Dispatcher must persist one row to `audit_logs` capturing the outcome, latency, and error context. This enables end-to-end traceability for Issue-606's final validation.

---

## Files to Modify / Create

| Action | File |
|--------|------|
| Edit | `internal/domain/models.go` |
| Edit | `internal/repository/repository.go` |
| Create | `internal/repository/postgres/audit_log_repo.go` |
| Edit | `internal/dispatcher/dispatcher.go` |
| Edit | `cmd/ingestor/main.go` (setupPipeline) |
| Edit | `cmd/dispatcher/main.go` |
| Create | `internal/repository/postgres/audit_log_repo_test.go` |

---

## Step 1 — Add `AuditLog` domain model

In `internal/domain/models.go`, append:

```go
// AuditLog records the outcome of a single dispatch attempt.
type AuditLog struct {
    ID                  uuid.UUID  `json:"id"`
    EventID             string     `json:"event_id"`
    SourceID            *uuid.UUID `json:"source_id"` // nullable (ON DELETE SET NULL)
    Status              string     `json:"status"`     // "SUCCESS" | "FAILED"
    ProcessingLatencyMs int64      `json:"processing_latency_ms"`
    ErrorMessage        *string    `json:"error_message,omitempty"`
    CreatedAt           time.Time  `json:"created_at"`
}
```

`SourceID` is `*uuid.UUID` to match the nullable FK in the DB schema.

---

## Step 2 — Add `AuditLogRepository` interface

In `internal/repository/repository.go`, append:

```go
// AuditLogRepository defines the contract for audit log persistence.
type AuditLogRepository interface {
    Create(ctx context.Context, log *domain.AuditLog) error
}
```

Write-only for now — querying audit logs is out of scope for Issue-603.

---

## Step 3 — Implement `AuditLogRepo`

New file `internal/repository/postgres/audit_log_repo.go`:

```go
package postgres

import (
    "context"
    "database/sql"
    "fmt"

    "github.com/Grainbox/zenith/internal/domain"
)

type AuditLogRepo struct {
    db *sql.DB
}

func NewAuditLogRepo(db *sql.DB) *AuditLogRepo {
    return &AuditLogRepo{db: db}
}

func (r *AuditLogRepo) Create(ctx context.Context, l *domain.AuditLog) error {
    query := `
        INSERT INTO audit_logs (event_id, source_id, status, processing_latency_ms, error_message)
        VALUES ($1, $2, $3, $4, $5)
        RETURNING id, created_at
    `
    err := r.db.QueryRowContext(ctx, query,
        l.EventID, l.SourceID, l.Status, l.ProcessingLatencyMs, l.ErrorMessage,
    ).Scan(&l.ID, &l.CreatedAt)
    if err != nil {
        return fmt.Errorf("failed to create audit log: %w", err)
    }
    return nil
}
```

Follows the exact same pattern as `source_repo.go` and `rule_repo.go`: `*sql.DB`, parameterized `$N` placeholders, `RETURNING` to populate server-generated fields.

---

## Step 4 — Instrument the Dispatcher

### 4a. Update the `Dispatcher` struct and constructor

In `internal/dispatcher/dispatcher.go`:

```go
import "github.com/Grainbox/zenith/internal/repository"

type Dispatcher struct {
    matchCh     <-chan *domain.MatchedEvent
    registry    *Registry
    auditLog    repository.AuditLogRepository // NEW
    workerCount int
    logger      *slog.Logger
    wg          sync.WaitGroup
}

func New(
    matchCh <-chan *domain.MatchedEvent,
    workerCount int,
    registry *Registry,
    auditLog repository.AuditLogRepository, // NEW param
    logger *slog.Logger,
) *Dispatcher {
    return &Dispatcher{
        matchCh:     matchCh,
        registry:    registry,
        auditLog:    auditLog,
        workerCount: workerCount,
        logger:      logger,
    }
}
```

### 4b. Instrument `dispatch()` with latency measurement and write-back

```go
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
        d.logger.Error("Sink dispatch failed", ...)
    } else {
        d.logger.Info("Event dispatched", ...)
    }
    d.writeAuditLog(ctx, matched, time.Since(start), err)
}

func (d *Dispatcher) writeAuditLog(ctx context.Context, matched *domain.MatchedEvent, latency time.Duration, dispatchErr error) {
    sourceID := matched.Rule.SourceID // uuid.UUID (non-pointer in Rule)
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
```

Audit log failures are logged as errors but do **not** propagate — dispatch outcome is not affected by observability failures.

---

## Step 5 — Update call sites

### `cmd/ingestor/main.go` — `setupPipeline`

```go
auditLogRepo := postgres.NewAuditLogRepo(db)
disp := dispatcher.New(matchCh, 4, registry, auditLogRepo, logger)
```

### `cmd/dispatcher/main.go` — `run()`

The standalone dispatcher binary currently has no DB connection. Add DB init following the ingestor pattern (`storage.NewDB` / `initDatabase`):

```go
db, err := initDatabase(cfg.Database, logger)
if err != nil {
    return err
}
defer db.Close()

auditLogRepo := postgres.NewAuditLogRepo(db)
d := dispatcher.New(matchCh, 4, registry, auditLogRepo, logger)
```

This requires importing `internal/storage`, `internal/repository/postgres`, and adding a `DATABASE_URL` env var to the dispatcher's deployment manifests (same secret already used by the ingestor — no new secret needed).

---

## Step 6 — Integration Test for `AuditLogRepo`

New file `internal/repository/postgres/audit_log_repo_test.go`, following the `testcontainers-go` pattern already established in the package:

- Spin up CockroachDB container, apply migrations
- `Create()` a source (to satisfy nullable FK), then call `AuditLogRepo.Create()`
- Assert `ID` and `CreatedAt` are populated after insert
- Test both SUCCESS and FAILED status values
- Test with `SourceID = nil` (no source)

---

## Verification

```bash
# Unit + integration tests (requires Docker)
go test ./internal/repository/postgres/... -run TestAuditLog -v

# Full test suite
go test ./...

# Linter
golangci-lint run

# Manual end-to-end: push a matched event through the pipeline,
# then query CockroachDB to confirm the row:
# SELECT * FROM audit_logs ORDER BY created_at DESC LIMIT 5;
```
