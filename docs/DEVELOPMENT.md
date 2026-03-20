# Development

## Prerequisites

- Go 1.26+
- Docker (required for integration tests)
- `golangci-lint` (for linting)
- `buf` (for protobuf code generation)
- `golang-migrate` (for database migrations)

## Testing

### Unit & Integration Tests

Run all tests (requires Docker for integration tests):

```bash
go test ./...
```

Run with race detector (recommended before committing):

```bash
go test -race ./...
```

Run only short tests (used by CI; skips Docker-based integration tests):

```bash
go test -short ./...
```

Run a specific test:

```bash
go test -run TestGatewayHandleIngestEvent ./internal/gateway
```

### Test Coverage

Generate coverage report:

```bash
go test -cover ./...
```

Generate detailed HTML coverage:

```bash
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Integration Tests

Tests in `internal/repository/postgres/` use `testcontainers-go` to spin up a real CockroachDB container and apply migrations via `golang-migrate`. These require Docker to be running.

Example:
```bash
# Start Docker
docker daemon

# Run integration tests
go test ./internal/repository/postgres/...
```

## Linting & Code Quality

### Run All Linters

```bash
golangci-lint run
```

This runs all linters configured in `.golangci.yml`:
- `errcheck` — Unhandled errors
- `gosec` — Security issues (SQL injection, unsafe crypto, etc.)
- `staticcheck` — Static analysis
- `revive` — Go vet on steroids
- `vet` — Built-in Go vet

### Proto Linting

```bash
buf lint
```

Checks `.proto` files for style violations and compatibility.

### Regenerate Protobuf Code

After modifying `.proto` files:

```bash
make gen
```

This runs `buf generate` and produces code in `pkg/pb/` (do NOT edit manually).

## Code Standards

- **Logging:** Use `log/slog` for all logging; never `fmt.Print*`.
- **Error handling:** Check all errors; use `errcheck` to verify.
- **Concurrency:** Use channels and goroutines; avoid mutexes where possible.
- **Testing:** Write tests for public functions; integration tests for DB interactions.
- **Naming:** Follow Go conventions: `CamelCase` for exported, `camelCase` for private.

## Common Makefile Commands

```bash
# Generate protobuf code
make gen

# Lint proto files
make lint

# Tidy Go modules
make tidy

# Apply database migrations
make migrate-up

# Rollback migrations
make migrate-down

# Build Docker image and load into Kind
make build-kind
```

## Database Migrations

Migrations are managed with `golang-migrate` in `deployments/db/migrations/`.

### Run Migrations

```bash
make migrate-up
```

### Create a New Migration

```bash
migrate create -ext sql -dir deployments/db/migrations -seq <name>
```

This creates two files:
- `000X_<name>.up.sql` — Applied on migrate up
- `000X_<name>.down.sql` — Applied on migrate down

### Rollback Migrations

```bash
make migrate-down
```

## Load Testing

See [BENCHMARK.md](../BENCHMARK.md) for performance test results and bottleneck analysis.

### Using k6

See [docs/load-test/README.md](../docs/load-test/README.md) for k6 load testing scripts.

Example:
```bash
k6 run docs/load-test/ingestor-load-test.js
```

### Using hey (HTTP benchmark)

```bash
# Simple HTTP load test
hey -n 1000 -c 10 http://localhost:8080/v1/events
```

### Using Go Benchmarks

Benchmark the rule evaluator:

```bash
go test -bench=BenchmarkEvaluateRules -benchmem ./internal/engine
```

## Debugging

### Debug Logs

Set `LOG_LEVEL` environment variable:

```bash
LOG_LEVEL=DEBUG go run cmd/ingestor/main.go
```

### pprof Profiling

If pprof is registered (add `import _ "net/http/pprof"`):

```bash
curl http://localhost:6060/debug/pprof/heap > heap.prof
go tool pprof heap.prof
```

### Verbose Test Output

```bash
go test -v ./...
```

## Project Structure

```
cmd/
  ingestor/             # Ingestor binary
  dispatcher/           # Dispatcher binary
  load-generator/       # Load test utility

internal/
  config/               # Config loading
  domain/               # Core models
  engine/               # Rule Engine
  gateway/              # REST handler
  ingestor/             # gRPC handler
  dispatcher/           # Dispatch logic
  dispatcher/sinks/     # Sink implementations
  repository/           # Repository interfaces
  repository/postgres/  # CockroachDB implementations
  storage/              # DB connection pool
  telemetry/            # OTEL + Prometheus

api/
  proto/v1/             # .proto source files

pkg/
  pb/                   # Auto-generated protobuf code (do NOT edit)

deployments/
  terraform/            # GCP IaC
  k8s/local/            # Kubernetes manifests
  grafana/              # Grafana dashboard
  monitoring/           # Docker-compose stack
  db/migrations/        # SQL migrations
```

## Continuous Integration

See `.github/workflows/deploy.yml` for the CI/CD pipeline. The pipeline:

1. **Lint** — `golangci-lint` + `buf lint`
2. **Test** — `go test -short ./...` (unit tests only)
3. **Build** — Docker image to Artifact Registry
4. **Deploy** — `terraform apply` to Cloud Run (main branch only)

PRs run lint + test. Merges to main trigger full pipeline.

## Contributing

1. Create a branch
2. Make changes
3. Run `go test -race ./...` locally
4. Run `golangci-lint run`
5. Commit with clear message
6. Push and open PR
7. CI pipeline runs automatically
