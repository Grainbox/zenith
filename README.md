# Zenith — Distributed Event Observer

[![Go Version](https://img.shields.io/badge/go-1.26+-blue.svg)](https://golang.org/doc/devel/release.html)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

> A high-performance, Cloud-Native backend platform that ingests event streams, evaluates them against dynamic business rules stored in CockroachDB, and dispatches actions to external sinks.

---

## Architecture

Zenith is a three-layer pipeline.

```
[Client] --gRPC--> [Ingestor] --channel--> [Rule Engine] --matched rules--> [Dispatcher]
                       |                        |                                  |
                  (ConnectRPC)           (CockroachDB)                   (Slack/webhooks/S3)
```

| Layer | Location | Deployment | Status |
|---|---|---|---|
| **Ingestor** — Receives events via gRPC, enqueues to worker pool | `cmd/ingestor/` | Standalone binary | ✅ Complete |
| **Rule Engine** — Evaluates events against rules from CockroachDB | `internal/engine/` | Embedded in Ingestor process (in-process channel) | ✅ Complete |
| **Dispatcher** — Routes matched events to external sinks, writes audit logs | `cmd/dispatcher/` | Standalone binary | Planned (Phase 3) |

> **Note:** The Rule Engine currently runs inside the Ingestor process and communicates via an in-memory Go channel. This is intentional for Phase 2. True independent scaling requires decoupling via a message broker (Kafka/NATS) — planned for a later phase.

### Domain Model

- **Source** — An event producer identified by name and API key. Rules are scoped to a source.
- **Rule** — A condition (`field`, `operator`, `value`) linked to a source and a `target_action`. Stored as JSONB in CockroachDB.
- **Event** — An inbound event with a `source` name, `event_type`, and a JSON `payload` evaluated against rules.

Supported rule operators: `==`, `!=`, `>`, `>=`, `<`, `<=` — works on both numeric and string payloads.

---

## Project Status

| Phase | Goal | Status |
|---|---|---|
| **Phase 1** — Foundations | gRPC skeleton, proto contracts, linting, CI | ✅ Complete |
| **Phase 2** — Persistence & Rule Engine | CockroachDB, rule evaluation, concurrency, graceful shutdown | ✅ Complete |
| **Phase 3** — IaC & Cloud | Terraform, GitHub Actions, Dispatcher service, REST gateway | Upcoming |
| **Phase 4** — Observability | OpenTelemetry, Prometheus, CKAD certification | Upcoming |

---

## Tech Stack

| Concern | Technology |
|---|---|
| Language | Go 1.26 |
| RPC | ConnectRPC (gRPC + HTTP/2 via h2c) |
| Protocol | Protocol Buffers v3 |
| Database | CockroachDB Serverless (pgx/v5 driver, no ORM) |
| Migrations | `golang-migrate` |
| Testing | `testify` + `testcontainers-go` (real CockroachDB container) |
| Logging | `log/slog` (structured JSON) |
| Config | 12-Factor (env vars, `godotenv`) |
| Local K8s | `kind` cluster |
| Linting | `golangci-lint` |

---

## Getting Started

### Prerequisites

- Go 1.26+
- Docker (required for integration tests)
- `grpcurl` (for manual testing)
- `buf` CLI (for regenerating protobuf code)
- `golangci-lint`

### Configuration

Create `.env.secrets` in the project root:

```bash
DATABASE_URL=postgresql://user:password@host/zenith?sslmode=require
```

Optional environment variables (with defaults):

```bash
ZENITH_PORT=50051
ENGINE_WORKER_COUNT=10
ENGINE_BUFFER_SIZE=1024
DB_MAX_OPEN_CONNS=25
DB_MAX_IDLE_CONNS=25
API_KEY_SALT=
SLACK_WEBHOOK_URL=
```

### Database Setup

Apply migrations:

```bash
make migrate-up
```

Seed a source and a rule to test evaluation:

```sql
BEGIN;

INSERT INTO sources (name, api_key)
VALUES ('my-service', 'my-api-key')
ON CONFLICT DO NOTHING;

INSERT INTO rules (source_id, name, condition, target_action, is_active)
SELECT id, 'high-value-alert',
       '{"field":"amount","operator":">","value":100}'::jsonb,
       'notify-finance', true
FROM sources WHERE name = 'my-service'
ON CONFLICT DO NOTHING;

COMMIT;
```

### Running the Ingestor

```bash
go run cmd/ingestor/main.go
```

Expected startup output:

```json
{"level":"INFO","msg":"Database connected successfully"}
{"level":"INFO","msg":"Event pipeline started","worker_count":10}
{"level":"INFO","msg":"Starting Zenith Ingestor Server","addr":":50051"}
```

### Sending an Event

The `payload` field is `bytes` in proto — it must be base64-encoded JSON.

```bash
grpcurl -plaintext \
  -d '{
    "event": {
      "event_id": "evt-001",
      "event_type": "payment.completed",
      "source": "my-service",
      "payload": "eyJhbW91bnQiOjI1MCwiY3VycmVuY3kiOiJVU0QifQ=="
    }
  }' \
  localhost:50051 \
  proto.v1.IngestorService/IngestEvent
```

The payload above decodes to `{"amount":250,"currency":"USD"}`, which matches the `amount > 100` rule.

Expected log trace:

```json
{"level":"INFO","msg":"Event Received","event_id":"evt-001","source":"my-service"}
{"level":"INFO","msg":"Rules matched","event_id":"evt-001","source":"my-service","matched_count":1,"total_rules":1}
{"level":"INFO","msg":"Event matched rules","worker_id":0,"event_id":"evt-001","matched_count":1}
```

### Graceful Shutdown

Press `Ctrl+C` (or send `SIGTERM`). The server stops accepting new connections, drains all in-flight events through the worker pool, then exits.

```json
{"level":"INFO","msg":"Shutting down server...","signal":"interrupt"}
{"level":"INFO","msg":"Event pipeline stopped cleanly"}
{"level":"INFO","msg":"Database connection closed"}
{"level":"INFO","msg":"Server exited properly"}
```

---

## Development

```bash
# Run all tests (requires Docker for integration tests)
go test ./...

# Run with race detector
go test -race ./...

# Lint
golangci-lint run

# Regenerate protobuf code
make gen

# Lint .proto files
make lint

# Tidy modules
make tidy

# Database migrations
make migrate-up
make migrate-down

# Build and deploy to local Kind cluster
make build-kind
```

---

## Project Structure

```
cmd/
  ingestor/         # Ingestor binary entry point
api/
  proto/v1/         # .proto source files (source of truth)
internal/
  config/           # Env-var config loading
  domain/           # Core models: Source, Rule, Event, Condition
  engine/           # Rule Engine: Pipeline, Worker, Evaluator
  ingestor/         # gRPC handler (IngestorService)
  repository/       # Repository interfaces
  repository/postgres/  # CockroachDB implementations
  storage/          # DB connection pool
pkg/
  pb/               # Auto-generated protobuf/ConnectRPC code (do not edit)
deployments/
  db/migrations/    # SQL migration files (golang-migrate)
  k8s/local/        # Kubernetes manifests for local Kind cluster
```
