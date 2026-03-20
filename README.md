# Zenith — Event Routing Middleware

[![Build Status](https://github.com/Grainbox/zenith/actions/workflows/deploy.yml/badge.svg)](https://github.com/Grainbox/zenith/actions/workflows/deploy.yml)
[![Go Version](https://img.shields.io/badge/go-1.26.1-blue.svg)](https://golang.org/doc/devel/release.html)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

> **Route events from any source to any target based on business rules stored in the database — without touching your application code.**

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

---

## Architecture

### Local Pipeline

```mermaid
flowchart LR
    Client["Client<br/>(HTTP/gRPC)"]
    GW["Gateway<br/>POST /v1/events"]
    Engine["Rule Engine<br/>(goroutine pool)"]
    Dispatcher["Dispatcher<br/>(worker pool)"]
    DB[("CockroachDB<br/>rules + audit_logs")]
    Sinks["Sinks<br/>(Discord / Webhook / S3)"]

    Client -->|"X-Api-Key + JSON"| GW
    GW -->|"domain.Event (channel)"| Engine
    Engine -->|"reads rules"| DB
    Engine -->|"matched events (channel)"| Dispatcher
    Dispatcher -->|"dispatch"| Sinks
    Dispatcher -->|"write audit_log"| DB
```

### Cloud Deployment (GCP)

```mermaid
flowchart LR
    Internet["Internet<br/>Clients"]
    CR_I["Cloud Run<br/>Ingestor"]
    CR_D["Cloud Run<br/>Dispatcher"]
    CRDB["CockroachDB<br/>Serverless"]
    Sinks["External Sinks<br/>(Discord, Webhooks)"]

    Internet -->|"REST/gRPC"| CR_I
    CR_I -->|"matched events"| CR_D
    CR_I -->|"read rules"| CRDB
    CR_D -->|"write audit logs"| CRDB
    CR_D --> Sinks
```

**Supporting GCP Services:** Artifact Registry (images), Secret Manager (credentials), Cloud Trace (tracing), Managed Prometheus (metrics), Workload Identity (auth).

See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for domain model and design decisions.

---

## Project Status

| Phase | Goal | Status |
|---|---|---|
| **Phase 1** | Foundations (gRPC, proto, linting) | ✅ Complete |
| **Phase 2** | Persistence (CockroachDB, Rule Engine) | ✅ Complete |
| **Phase 3** | IaC & Cloud (Terraform, CI/CD, Dispatcher) | ✅ Complete |
| **Phase 4** | Observability (OTel, Prometheus, Grafana) | ✅ Complete |
| **Phase 5** | Message Broker (independent services) | 🔮 Planned |

See [docs/ROADMAP.md](docs/ROADMAP.md) for detailed plans.

---

## Tech Stack

| Concern | Technology |
|---|---|
| **Language** | Go 1.26.1 |
| **RPC** | ConnectRPC (gRPC + HTTP/2 via h2c) + REST gateway |
| **Database** | CockroachDB Serverless (pgx/v5, no ORM) |
| **Migrations** | `golang-migrate` |
| **Tracing** | OpenTelemetry SDK + GCP Cloud Trace |
| **Metrics** | Prometheus + GCP Managed Prometheus |
| **Dashboard** | Grafana |
| **Infrastructure** | Terraform (GCP Cloud Run, AR, Secret Manager) |
| **CI/CD** | GitHub Actions (lint → test → build → deploy) |
| **Kubernetes** | `kind` (local), HPA, probes, resource limits |
| **Testing** | `testify` + `testcontainers-go` (real CockroachDB) |
| **Linting** | `golangci-lint` v1.62 + `buf lint` |

---

## Key Features

| Feature | Benefit |
|---|---|
| **Dynamic Rules** | Change routing logic without code redeploy (SQL INSERT) |
| **High Throughput** | Handles millions of events/hour via goroutine worker pools |
| **Distributed Tracing** | Every event traceable end-to-end (OTel → Cloud Trace) |
| **Real-Time Metrics** | 9 Prometheus metrics for ingestion, evaluation, dispatch |
| **Extensible Sinks** | Discord, webhooks, S3; easy to add more |
| **Zero Event Loss** | Graceful shutdown drains in-flight events |
| **Production Ready** | Kubernetes-native, CKAD-compliant, Cloud Run-ready |

---

## Quick Links

| Topic | Link |
|---|---|
| **Quick Start** | [docs/GETTING_STARTED.md](docs/GETTING_STARTED.md) |
| **API Reference** | [docs/API_REFERENCE.md](docs/API_REFERENCE.md) |
| **Configuration** | [docs/CONFIGURATION.md](docs/CONFIGURATION.md) |
| **Architecture** | [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) |
| **Observability** | [docs/OBSERVABILITY.md](docs/OBSERVABILITY.md) |
| **Development** | [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md) |
| **Roadmap** | [docs/ROADMAP.md](docs/ROADMAP.md) |

---

## Observability

Zenith is fully instrumented for production visibility:

- **Distributed Tracing:** Every event traced from REST request → rule evaluation → dispatch
- **Prometheus Metrics:** 9 metric families covering ingestion, evaluation, dispatch, queue depth
- **Grafana Dashboard:** Pre-built dashboard with live pipeline visibility

![Grafana Dashboard](docs/assets/grafana-dashboard.png)

See [docs/OBSERVABILITY.md](docs/OBSERVABILITY.md) for setup and example queries.

---

## Getting Started

### Local Development

```bash
# 1. Clone and set environment
git clone https://github.com/Grainbox/zenith.git
cd zenith
export DATABASE_URL=postgresql://root:password@localhost:26257/zenith?sslmode=require

# 2. Run migrations
make migrate-up

# 3. Start ingestor
go run cmd/ingestor/main.go

# 4. Send event
curl -X POST http://localhost:8080/v1/events \
  -H "Content-Type: application/json" \
  -H "X-Api-Key: my-api-key" \
  -d '{"event_id":"evt-001","event_type":"test","source":"my-service","payload":{"amount":250}}'
```

**Full guide:** [docs/GETTING_STARTED.md](docs/GETTING_STARTED.md)

### Kubernetes (Kind)

```bash
make build-kind
kubectl apply -f deployments/k8s/local/
kubectl rollout status deployment/zenith-ingestor -n zenith-dev
```

---

## Continuous Deployment

Every push to `main` triggers GitHub Actions:

```
Lint → Test → Build Docker Image → Push to Artifact Registry → Terraform Apply to Cloud Run
```

Pull requests run lint & test only (no deployment). See `.github/workflows/deploy.yml` for details.

---

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for:
- Development environment setup
- Code standards (linting, testing, error handling)
- PR checklist

---

## License

MIT — See [LICENSE](LICENSE) for details.

---

**Questions?** Check the [docs/](docs/) directory or open an [issue](https://github.com/Grainbox/zenith/issues).
