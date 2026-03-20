# Contributing to Zenith

Thank you for your interest in contributing to **Zenith**, a distributed event observer for high-throughput event routing. To maintain production-quality code, please follow these guidelines.

## Prerequisites

- **Go 1.26+** (must match `go.mod`)
- **Docker** (required for integration tests with `testcontainers-go`)
- **golangci-lint** v1.62+ (linting)
- **buf** CLI (protobuf code generation)
- **golang-migrate** (database migrations)

## Project Structure

We follow the [Standard Go Project Layout](https://github.com/golang-standards/project-layout):

- `/cmd/` — Main applications: `ingestor`, `dispatcher`, `load-generator`
- `/internal/` — Private packages: config, domain, engine, gateway, ingestor, dispatcher, repository, storage, telemetry
- `/pkg/pb/` — Auto-generated protobuf code (do NOT edit manually)
- `/api/proto/` — Protobuf source files (`.proto`)
- `/deployments/` — Infrastructure: Terraform (GCP), Kubernetes manifests, Grafana, migrations
- `/docs/` — User documentation and roadmaps

See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for detailed architecture.

## Code Standards

### Linting (Required)

All code must pass linting before commit:

```bash
golangci-lint run  # All linters in .golangci.yml
buf lint           # Protocol buffers
```

Linters enforce:
- Error handling (`errcheck`) — all errors must be handled or explicitly ignored
- Security (`gosec`) — no SQL injection, unsafe crypto, etc.
- Static analysis (`staticcheck`, `revive`)
- Race conditions (`-race` detector)

### Logging

Use `log/slog` for all logging:

```go
slog.Info("Event processed", "event_id", eventID, "source", source)
slog.Warn("Rule evaluation slow", "duration_ms", latency)
slog.Error("Dispatch failed", "error", err, "sink", sinkType)
```

Never use `fmt.Print*` or `log.Println`.

### Testing

Run all tests before committing:

```bash
go test ./...                    # All tests
go test -race ./...             # Detect race conditions
go test -short ./...            # Quick tests (skips integration)
go test -v ./internal/gateway   # Specific package
```

Integration tests in `internal/repository/postgres/` require Docker. They use `testcontainers-go` to spin up a real CockroachDB instance.

### Concurrency

- Use **channels** and **goroutines** for concurrency (no shared state)
- Avoid **mutex locks** where possible
- All long-running goroutines must respect context cancellation

## Development Workflow

### Before Pushing

1. **Run all linters:**
   ```bash
   golangci-lint run
   buf lint
   ```

2. **Run all tests (including integration):**
   ```bash
   go test -race ./...
   ```

3. **Commit with a clear message:**
   ```bash
   git commit -m "feat: add tracing to dispatcher

   - Instrument dispatch with OTel spans
   - Add span attributes: sink_type, rule_id
   - Mark span as error on dispatch failure"
   ```

### Picking a Task

See [docs/ROADMAP.md](docs/ROADMAP.md) for current phase status. Open issues are tracked in the GitHub repo.

### 12-Factor App Principles

- All configuration from environment variables (see [docs/CONFIGURATION.md](docs/CONFIGURATION.md))
- No hardcoded secrets or config files
- Graceful shutdown on SIGTERM
- Stateless design (all state in CockroachDB)

### Documentation

- Write Go doc comments for all public functions
- Update [docs/](docs/) if you change architecture or add features
- Link to relevant issues/PRs in commit messages

## Continuous Integration

All pushes trigger GitHub Actions (`.github/workflows/deploy.yml`):

| Trigger | Stages | Notes |
|---|---|---|
| **Pull Request** | Lint → Test | No deployment |
| **Push to main** | Lint → Test → Build → Push → Deploy | Auto-deploy to Cloud Run |

The pipeline uses **Workload Identity Federation** for secure GCP auth (no long-lived secrets in repo).

## Need Help?

- **Getting started?** See [docs/GETTING_STARTED.md](docs/GETTING_STARTED.md)
- **API questions?** See [docs/API_REFERENCE.md](docs/API_REFERENCE.md)
- **Architecture?** See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)
- **Development tips?** See [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md)
