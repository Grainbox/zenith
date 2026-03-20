# Getting Started

## Prerequisites

- **Go 1.26+**
- **Docker** (required for integration tests and Kubernetes local development)
- **`kind`** (Kubernetes in Docker, for local cluster)
- Optional: `grpcurl` (for manual gRPC testing)
- Optional: `buf` CLI (for regenerating protobuf code)
- Optional: `golangci-lint` (for linting locally)

## Configuration

All settings are loaded from environment variables (12-Factor app). The app auto-loads `.env.config` and `.env.secrets` if present.

**Required:**
```bash
DATABASE_URL=postgresql://user:password@host/zenith?sslmode=require
```

See [CONFIGURATION.md](CONFIGURATION.md) for the complete list of optional environment variables with defaults.

### Example `.env.secrets` for Local Development

```bash
DATABASE_URL=postgresql://root:password@localhost:26257/zenith?sslmode=require&x-migrations-lock=false
API_KEY_SALT=your-random-salt-here
```

## Database Setup

Apply migrations:

```bash
make migrate-up
```

Seed a source and a rule for testing:

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

## Running Locally

### Start the Ingestor

```bash
go run cmd/ingestor/main.go
```

Expected startup output:

```json
{"level":"INFO","msg":"Database connected successfully"}
{"level":"INFO","msg":"Event pipeline started","worker_count":10}
{"level":"INFO","msg":"Starting Zenith Ingestor Server","addr":":8080"}
{"level":"INFO","msg":"Metrics server listening on","addr":":8082"}
```

### Send an Event (REST)

```bash
curl -X POST http://localhost:8080/v1/events \
  -H "Content-Type: application/json" \
  -H "X-Api-Key: my-api-key" \
  -d '{
    "event_id": "evt-001",
    "event_type": "payment.completed",
    "source": "my-service",
    "payload": {"amount": 250, "currency": "USD"}
  }'
```

**Response (202 Accepted):**
```json
{"success": true, "message": "Event accepted"}
```

### Send an Event (gRPC)

The gRPC `payload` field is `bytes` — it must be base64-encoded JSON:

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

### Expected Log Output

```json
{"level":"INFO","msg":"Event received via gateway","event_id":"evt-001","event_type":"payment.completed","source":"my-service"}
{"level":"INFO","msg":"Rules matched","event_id":"evt-001","matched_count":1,"total_rules":1}
{"level":"INFO","msg":"Event dispatched","event_id":"evt-001","sink_type":"discord","status":"success"}
```

### Graceful Shutdown

Press `Ctrl+C` (or send `SIGTERM`). The server stops accepting new connections, drains all in-flight events through the worker pool, then exits.

```json
{"level":"INFO","msg":"Shutting down server...","signal":"interrupt"}
{"level":"INFO","msg":"Event pipeline stopped cleanly"}
{"level":"INFO","msg":"Database connection closed"}
{"level":"INFO","msg":"Server exited properly"}
```

## Running on Kubernetes (Kind)

Deploy the full pipeline to a local Kind cluster:

```bash
# 1. Build Docker image and load into Kind
make build-kind

# 2. Apply all Kubernetes manifests
kubectl apply -f deployments/k8s/local/

# 3. Wait for rollout
kubectl rollout status deployment/zenith-ingestor -n zenith-dev
kubectl rollout status deployment/zenith-dispatcher -n zenith-dev

# 4. Port-forward the Ingestor service
kubectl port-forward svc/zenith-ingestor -n zenith-dev 8080:8080 &

# 5. Send a test event (REST)
curl -X POST http://localhost:8080/v1/events \
  -H "Content-Type: application/json" \
  -H "X-Api-Key: test-key" \
  -d '{
    "event_id": "evt-k8s-001",
    "event_type": "test.event",
    "source": "test-source",
    "payload": {"amount": 500}
  }'

# 6. View logs
kubectl logs -f deployment/zenith-ingestor -n zenith-dev

# 7. Check HPA scaling
kubectl get hpa -n zenith-dev

# 8. Load test (optional; watch pod scaling)
kubectl run load-test --image=loadimpact/k6:latest --rm -it --restart=Never -n zenith-dev -- run - <<'EOF'
import http from 'k6/http';
export default function() {
  http.post('http://zenith-ingestor:8080/v1/events', JSON.stringify({
    event_id: 'evt-' + Math.random(),
    event_type: 'load.test',
    source: 'k6',
    payload: {amount: Math.random() * 1000}
  }), {headers: {'Content-Type': 'application/json', 'X-Api-Key': 'test-key'}});
}
EOF
```

## Next Steps

- See [API_REFERENCE.md](API_REFERENCE.md) for complete endpoint documentation
- See [CONFIGURATION.md](CONFIGURATION.md) for all environment variables
- See [DEVELOPMENT.md](DEVELOPMENT.md) for testing and linting
- See [OBSERVABILITY.md](OBSERVABILITY.md) for tracing and metrics
