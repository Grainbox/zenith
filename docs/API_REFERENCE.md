# API Reference

## POST /v1/events

Ingest an event via REST. Returns `202 Accepted` on success.

### Headers

| Header | Required | Description |
|---|---|---|
| `Content-Type` | Yes | Must be `application/json` |
| `X-Api-Key` | Yes | Authenticate as a registered source; maps to a `Source` in the database |

### Request Body

```json
{
  "event_id": "evt-001",
  "event_type": "payment.completed",
  "source": "my-service",
  "payload": {
    "amount": 250,
    "currency": "USD"
  }
}
```

**Fields:**
- `event_id` (string, required) — Unique event identifier
- `event_type` (string, required) — Event category (e.g., `payment.completed`, `user.signup`)
- `source` (string, required) — Event source name; must match the source authenticated by `X-Api-Key`
- `payload` (object, optional) — Arbitrary JSON object evaluated against rules; defaults to `{}`

### Response (202 Accepted)

```json
{
  "success": true,
  "message": "Event accepted"
}
```

The event has been enqueued to the pipeline. Processing happens asynchronously:
1. Rule Engine evaluates rules for this source
2. Dispatcher routes matched events to sinks
3. Audit log is written

### Error Responses

| Status | Code | Message | Reason |
|---|---|---|---|
| `400` | `INVALID_JSON` | `request body is empty` or `failed to decode JSON: ...` | Malformed JSON or empty body |
| `400` | `INVALID_ARGUMENT` | `event_id is required` | Missing required field |
| `401` | `UNAUTHENTICATED` | `X-Api-Key header is required` | Missing `X-Api-Key` header |
| `401` | `UNAUTHENTICATED` | `invalid API key` | API key does not match any source |
| `403` | `PERMISSION_DENIED` | `source name does not match authenticated source` | `source` field in body doesn't match the authenticated source |
| `503` | `RESOURCE_EXHAUSTED` | `event pipeline queue is full` | Event channel buffer is at capacity; backoff and retry |

### Examples

**REST (curl):**
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

**gRPC (grpcurl):**

The gRPC version requires base64-encoded JSON payload:

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

---

## GET /healthz

Simple health check endpoint. Returns `200 OK` if the service is running and connected to the database.

### Response (200 OK)

```
OK
```

### Example

```bash
curl http://localhost:8080/healthz
# Response: OK
```

---

## GET /metrics

Prometheus-format metrics endpoint. Access on the **dedicated metrics port** (default `8082`).

### Response (200 OK)

Prometheus exposition format with all active metrics:

```
# HELP zenith_events_received_total Total events received by the Gateway
# TYPE zenith_events_received_total counter
zenith_events_received_total{event_type="payment.completed",source="my-service"} 100

# HELP zenith_events_accepted_total Events accepted (auth passed, payload valid)
# TYPE zenith_events_accepted_total counter
zenith_events_accepted_total{source="my-service"} 100

...
```

### Metrics Reference

| Metric | Type | Labels | Description |
|---|---|---|---|
| `zenith_events_received_total` | Counter | `source`, `event_type` | Total events received by the Gateway (before validation) |
| `zenith_events_accepted_total` | Counter | `source` | Successfully accepted events (auth passed, payload valid) |
| `zenith_events_rejected_total` | Counter | `source`, `reason` | Rejected events; `reason`: `missing_api_key`, `invalid_api_key`, `source_mismatch`, `invalid_body`, `pipeline_full` |
| `zenith_rules_evaluated_total` | Counter | `source` | Rule evaluations performed |
| `zenith_rules_matched_total` | Counter | `source`, `rule_id` | Rules matched per evaluation |
| `zenith_dispatch_total` | Counter | `sink_type`, `status` | Dispatch attempts; `status`: `success`, `failed` |
| `zenith_dispatch_duration_seconds` | Histogram | `sink_type` | Dispatch latency distribution (buckets: 0.005, 0.01, 0.025, ..., 10.0) |
| `zenith_rule_evaluation_duration_seconds` | Histogram | — | Rule engine evaluation latency |
| `zenith_worker_queue_depth` | Gauge | — | Current event queue depth (backlog indicator) |

### Example Queries (PromQL)

```promql
# Events ingested per second (last 5 minutes)
rate(zenith_events_received_total[5m])

# Dispatch success rate
rate(zenith_dispatch_total{status="success"}[5m]) / rate(zenith_dispatch_total[5m])

# p95 dispatch latency
histogram_quantile(0.95, zenith_dispatch_duration_seconds_bucket)

# Current event queue depth
zenith_worker_queue_depth
```

### Example

```bash
curl http://localhost:8082/metrics
```

---

## gRPC Service Definition

For full protobuf service definition, see [`api/proto/v1/event.proto`](../api/proto/v1/event.proto).

Service: `proto.v1.IngestorService`

RPC: `IngestEvent(IngestEventRequest) -> IngestEventResponse`

- Request: Contains event details (id, type, source, base64-encoded payload)
- Response: Empty (success indicated by RPC code OK)
