# Configuration Reference

All settings are loaded from environment variables (12-Factor app). The app auto-loads `.env.config` and `.env.secrets` if present.

## Environment Variables

| Variable | Type | Default | Required | Description |
|---|---|---|---|---|
| `DATABASE_URL` | string | — | **Yes** | CockroachDB connection string (pgx-compatible); includes SSL settings and pool config |
| `PORT` | int | `8080` | No | HTTP/gRPC server port (used if component-specific port not set) |
| `INGESTOR_PORT` | int | `8080` | No | Ingestor HTTP/gRPC port (overrides `PORT`) |
| `DISPATCHER_PORT` | int | `8081` | No | Dispatcher HTTP port (overrides `PORT`) |
| `METRICS_PORT` | int | `8082` | No | Prometheus `/metrics` port |
| `DB_MAX_OPEN_CONNS` | int | `25` | No | Max open database connections in the pool |
| `DB_MAX_IDLE_CONNS` | int | `25` | No | Max idle connections to keep in the pool |
| `API_KEY_SALT` | string | `""` | No | Salt for hashing API keys (optional; set for production) |
| `ENGINE_WORKER_COUNT` | int | `10` | No | Number of goroutines for rule evaluation |
| `ENGINE_BUFFER_SIZE` | int | `1024` | No | Event channel buffer capacity |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | string | `""` (disabled) | No | OpenTelemetry OTLP HTTP endpoint (e.g., `http://localhost:4318`); if empty, tracing is disabled |
| `OTEL_SERVICE_NAME` | string | `ingestor` / `dispatcher` | No | Service name in distributed traces |

## Loading Behavior

The app loads configuration in this order (later values override earlier ones):

1. System environment variables
2. `.env.config` (if exists)
3. `.env.secrets` (if exists)

### Port Precedence

Component-specific ports take precedence over the generic `PORT`:

```bash
# This will use 9000 for the ingestor
INGESTOR_PORT=9000 PORT=8080

# This will use 8080 (no component-specific var set)
PORT=8080
```

## Example Configurations

### Local Development

**.env.secrets** (CockroachDB local, no external services):
```bash
DATABASE_URL=postgresql://root:password@localhost:26257/zenith?sslmode=require&x-migrations-lock=false
API_KEY_SALT=dev-salt-not-secure
OTEL_EXPORTER_OTLP_ENDPOINT=
```

### Local Development with Observability

**.env.config** (with local Prometheus + Grafana):
```bash
OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318
OTEL_SERVICE_NAME=zenith-ingestor
```

**.env.secrets**:
```bash
DATABASE_URL=postgresql://root:password@localhost:26257/zenith?sslmode=require&x-migrations-lock=false
API_KEY_SALT=dev-salt
```

### Kubernetes (Kind)

Loaded via `ConfigMap` and `Secret`:

```yaml
# deployments/k8s/local/config.yaml (ConfigMap)
OTEL_SERVICE_NAME: zenith-ingestor
METRICS_PORT: "8082"

# deployments/k8s/local/secrets.yaml (Secret, base64-encoded)
DATABASE_URL: postgresql://user:password@cockroach:26257/zenith...
API_KEY_SALT: your-random-salt
```

### Cloud Run (GCP)

Sensitive vars from GCP Secret Manager; others via Cloud Run environment settings:

```bash
# Cloud Run environment variables (non-sensitive)
OTEL_EXPORTER_OTLP_ENDPOINT=https://cloudtrace.googleapis.com/...
OTEL_SERVICE_NAME=zenith-ingestor-dev
METRICS_PORT=8082

# DATABASE_URL and API_KEY_SALT injected via Secret Manager secret_key_ref
```

## SSL/TLS for CockroachDB

The `sslrootcert` path in `DATABASE_URL` varies by environment:

### Local Development (Windows)

```bash
DATABASE_URL=postgresql://root:password@localhost:26257/zenith?sslmode=require&x-migrations-lock=false&sslrootcert=C:\Users\<username>\AppData\Roaming\postgresql\root.crt
```

### Docker / Linux / Kubernetes

```bash
DATABASE_URL=postgresql://root:password@cockroach:26257/zenith?sslmode=require&x-migrations-lock=false&sslrootcert=/root/.postgresql/root.crt
```

(The cert is mounted as a Kubernetes Secret volume)

## Connection Pool Tuning

For high-throughput environments, tune connection pool settings:

| Setting | Recommended | Rationale |
|---|---|---|
| `DB_MAX_OPEN_CONNS` | 50–100 | For high concurrency; adjust based on workload |
| `DB_MAX_IDLE_CONNS` | 10–25 | Keep warm connections ready; too high wastes resources |

Example:
```bash
DB_MAX_OPEN_CONNS=50
DB_MAX_IDLE_CONNS=10
```

## Rule Engine Tuning

For CPU-bound rule evaluation:

| Setting | Recommended | Rationale |
|---|---|---|
| `ENGINE_WORKER_COUNT` | num_cpu cores | Matches available CPU; increase for high-throughput |
| `ENGINE_BUFFER_SIZE` | 1024–4096 | Larger buffer absorbs traffic spikes; uses more memory |

Example:
```bash
ENGINE_WORKER_COUNT=16
ENGINE_BUFFER_SIZE=2048
```

## Observability Configuration

### OpenTelemetry

```bash
# Export to GCP Cloud Trace
OTEL_EXPORTER_OTLP_ENDPOINT=https://cloudtrace.googleapis.com/opentelemetry.proto.collector.trace.v1.TraceService

# Or local OTEL Collector
OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318

# Service name (appears in traces)
OTEL_SERVICE_NAME=zenith-ingestor
```

**Note:** Tracing is disabled if `OTEL_EXPORTER_OTLP_ENDPOINT` is empty. Set to a valid endpoint to enable.

### Prometheus Metrics

```bash
# Metrics server port
METRICS_PORT=8082

# Accessed at: http://localhost:8082/metrics
```

## Validation

The app validates required variables at startup. If `DATABASE_URL` is missing, it exits with:

```
panic: DATABASE_URL environment variable is required
```

All other variables have sensible defaults and are optional.
