# Zenith — Event Routing Middleware

[![Go Version](https://img.shields.io/badge/go-1.26+-blue.svg)](https://golang.org/doc/devel/release.html)
[![CI/CD](https://img.shields.io/badge/CI%2FCD-GitHub%20Actions-green.svg)](https://github.com/Grainbox/zenith/actions)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

> Route events from any source to any target based on business rules stored in the database — without touching your application code.

---

## The Problem

Your backend emits events. You want to react to certain ones: notify a team, trigger a downstream service, flag something for review. The naive approach is to hardcode these reactions directly in your service:

```go
// In your payment service
if payment.Amount > 2000 {
    notifyComplianceTeam(payment)
}
if payment.Country == "NG" {
    alertFraudTeam(payment)
}
```

This breaks down quickly:
- A threshold changes → code change + redeploy
- A new team needs a different rule → more hardcoded conditions
- 10 services each have their own notification logic → no central visibility

**Zenith extracts routing logic from your code and stores it in a database.** Your services just emit events. Zenith evaluates and routes.

### Concrete Example

A payment platform processes 50,000 transactions/hour. Compliance, fraud, and finance each have routing requirements that change weekly. Instead of redeploying the payment service every time, they insert rows into Zenith's `rules` table:

| Rule | Condition | Target |
|---|---|---|
| High-value alert | `amount > 2000` | `https://compliance.internal/webhook` |
| Restricted country | `country == "NG"` | `https://fraud-team.slack-webhook.com/...` |
| FX exposure | `currency != "USD"` | `https://fx-desk.internal/notify` |

Adding a new rule:
```sql
INSERT INTO rules (source_id, name, condition, target_action, is_active)
VALUES (..., 'vip-customer-alert', '{"field":"user_tier","operator":"==","value":"VIP"}',
        'https://crm.internal/vip-hook', true);
```

No code change. No deploy. The rule is active immediately.

### How It Differs from Zapier / n8n

Zapier and n8n are no-code SaaS tools for connecting consumer applications (Gmail → Notion, Stripe → Slack). They are designed for low-frequency, user-triggered workflows and cannot handle high-throughput backend event streams.

Zenith is backend infrastructure, embedded in your own architecture:
- Handles **millions of events/hour** via gRPC with a concurrent worker pool
- Rules are managed **programmatically** (SQL/API), not via a UI
- **Self-hosted** in your Kubernetes cluster — no per-action cost, no vendor lock-in
- **Sub-millisecond routing decisions** using in-process Go channels

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
| **Dispatcher** — Routes matched events to external sinks, writes audit logs | `cmd/dispatcher/` | Standalone binary | ✅ Sprint 6 |

> **Note:** In local development, the Ingestor and Dispatcher communicate via an in-memory Go channel (phase 2). For independent scaling in production, a message broker (Kafka/NATS) will replace the channel (Phase 3).

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
| **Phase 3** — IaC & Cloud | Terraform, GitHub Actions CI/CD, Dispatcher service, REST gateway | 🔄 In Progress |
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
PORT=8080
ENGINE_WORKER_COUNT=10
ENGINE_BUFFER_SIZE=1024
DB_MAX_OPEN_CONNS=25
DB_MAX_IDLE_CONNS=25
API_KEY_SALT=
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
{"level":"INFO","msg":"Starting Zenith Ingestor Server","addr":":8080"}
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
  localhost:8080 \
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

## Continuous Deployment

### GitHub Actions Pipeline

Every push to `main` triggers an automated CI/CD pipeline (`.github/workflows/deploy.yml`):

```
Push to main
    ↓
[Lint + Test] (pull requests run lint/test only)
    ↓
[Build & Push Docker Image] → Artifact Registry
    ↓
[Terraform Apply] → Deploy to Cloud Run
    ↓
[Output] → Service URL available
```

**Pipeline Stages:**

1. **Lint** — `golangci-lint` + `buf lint`
2. **Test** — Unit tests (`go test -short`)
3. **Build & Push** — Docker image to Google Artifact Registry (tag = git SHA)
4. **Deploy** — `terraform apply` with updated `image_tag`

**PR Behavior:** Pull requests only run lint & test (no deployment).

**Setup:** Configure 4 GitHub Secrets (see [Issue-502 plan](docs/organization/plans/ISSUE_502_CICD.md)):
- `GCP_PROJECT_ID`
- `GCP_WORKLOAD_IDENTITY_PROVIDER`
- `GCP_SERVICE_ACCOUNT`
- `TF_BACKEND_BUCKET`

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
