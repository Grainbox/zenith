# Observability

Zenith is fully instrumented for end-to-end visibility: distributed tracing (OpenTelemetry) and real-time metrics (Prometheus).

## Distributed Tracing (OpenTelemetry)

Every event generates a complete trace tree from ingestion through dispatch.

### Trace Structure

Each event creates a root span with child spans for each stage:

1. **Root span:** `ingestor.gateway.handle`
   - Attributes: `event_id`, `source`, `event_type`
   - Captures HTTP request handling time

2. **Child span:** `engine.evaluate_rules`
   - Attributes: `source`, `total_rules`, `matched_count`
   - Captures rule evaluation latency

3. **Child span:** `dispatcher.dispatch`
   - Attributes: `rule_id`, `sink_type`, `status` (success/failed)
   - Captures dispatch latency per sink

### Enable Tracing

Set the OTLP endpoint:

```bash
# Export to GCP Cloud Trace
export OTEL_EXPORTER_OTLP_ENDPOINT=https://cloudtrace.googleapis.com/...

# Or local OTEL Collector / Jaeger
export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318
```

**Note:** Tracing is disabled if `OTEL_EXPORTER_OTLP_ENDPOINT` is empty.

### View Traces in GCP Cloud Trace

1. Navigate to **GCP Console** → **Cloud Trace** → **Trace list**
2. Filter by `event_id` or `source`
3. Click a trace to see the full span tree with latencies

Example query:
```
span.display_name="ingestor.gateway.handle" AND labels.event_id="evt-001"
```

### View Traces Locally (Jaeger)

If running Jaeger locally:

```bash
# Start Jaeger (via docker-compose in deployments/monitoring/)
docker-compose up jaeger

# View at http://localhost:16686
```

---

## Prometheus Metrics

All metrics are exposed on the dedicated metrics port (default `8082`).

### Access Metrics

```bash
curl http://localhost:8082/metrics
```

### Metrics Breakdown

#### Ingestion Metrics

| Metric | Type | Labels | Purpose |
|---|---|---|---|
| `zenith_events_received_total` | Counter | `source`, `event_type` | Total events received (before validation) |
| `zenith_events_accepted_total` | Counter | `source` | Successfully validated events |
| `zenith_events_rejected_total` | Counter | `source`, `reason` | Rejected events and rejection reason |

**Rejection reasons:**
- `missing_api_key` — No `X-Api-Key` header
- `invalid_api_key` — API key doesn't match a source
- `source_mismatch` — Event `source` doesn't match authenticated source
- `invalid_body` — Malformed JSON or missing required field
- `pipeline_full` — Event channel buffer at capacity

#### Rule Evaluation Metrics

| Metric | Type | Labels | Purpose |
|---|---|---|---|
| `zenith_rules_evaluated_total` | Counter | `source` | Total rule evaluations |
| `zenith_rules_matched_total` | Counter | `source`, `rule_id` | Rules matched |
| `zenith_rule_evaluation_duration_seconds` | Histogram | — | Latency of rule evaluation (per event) |

#### Dispatch Metrics

| Metric | Type | Labels | Purpose |
|---|---|---|---|
| `zenith_dispatch_total` | Counter | `sink_type`, `status` | Dispatch attempts (success/failed) |
| `zenith_dispatch_duration_seconds` | Histogram | `sink_type` | Dispatch latency per sink type |

#### Queue Depth

| Metric | Type | Labels | Purpose |
|---|---|---|---|
| `zenith_worker_queue_depth` | Gauge | — | Current event queue depth; backlog indicator |

### Example Queries (PromQL)

**Events ingested per second (last 5 minutes):**
```promql
rate(zenith_events_received_total[5m])
```

**Acceptance rate (successful vs rejected):**
```promql
rate(zenith_events_accepted_total[5m]) / rate(zenith_events_received_total[5m])
```

**Rules matched per minute:**
```promql
rate(zenith_rules_matched_total[1m])
```

**Dispatch success rate:**
```promql
rate(zenith_dispatch_total{status="success"}[5m]) / rate(zenith_dispatch_total[5m])
```

**p95 dispatch latency (by sink type):**
```promql
histogram_quantile(0.95, zenith_dispatch_duration_seconds_bucket) by (sink_type)
```

**p99 dispatch latency:**
```promql
histogram_quantile(0.99, zenith_dispatch_duration_seconds_bucket)
```

**Current event queue depth:**
```promql
zenith_worker_queue_depth
```

---

## Grafana Dashboard

A pre-built Grafana dashboard visualizes the full pipeline in one view: ingestion rate, acceptance rate, rules matched, dispatch success rate, latency distribution, queue depth.

### Import Dashboard

1. **Open Grafana** (local: `http://localhost:3000`, Cloud: GCP managed)
2. **Dashboards** → **New** → **Import**
3. **Upload** `deployments/grafana/zenith-dashboard.json` or paste dashboard ID
4. **Select Prometheus data source**
5. **Import**

The dashboard will display:

| Panel | Query | Visualization |
|---|---|---|
| **Events / second** | `rate(zenith_events_received_total[1m])` | Time series |
| **Acceptance rate** | Ratio of accepted to received | Stat (%) |
| **Rules matched / minute** | `rate(zenith_rules_matched_total[1m])` | Time series |
| **Dispatch success rate** | Ratio of success to all dispatches | Stat (%) |
| **Dispatch latency** | p50, p95, p99 percentiles | Time series |
| **Worker queue depth** | `zenith_worker_queue_depth` | Gauge |
| **Recent audit failures** | Failed dispatch entries | Table |

### Dashboard Screenshot

![Grafana Dashboard](../docs/assets/grafana-dashboard.png)

---

## Local Observability Stack (Docker Compose)

Run the full observability stack locally:

```bash
cd deployments/monitoring
docker-compose up
```

Services:
- **Prometheus** — `http://localhost:9090` (scrapes Ingestor `/metrics`)
- **Grafana** — `http://localhost:3000` (username: `admin`, password: `admin`)
- **OTEL Collector** — `http://localhost:4317/4318` (receives traces)
- **Jaeger** — `http://localhost:16686` (views traces)

### Configure Prometheus Scraping

Prometheus scrapes metrics from `http://localhost:8082/metrics` (configured in `docker-compose.yml`).

### Send Test Event to Jaeger

```bash
# With OTEL Collector running
export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318

go run cmd/ingestor/main.go
```

Then send an event:

```bash
curl -X POST http://localhost:8080/v1/events \
  -H "Content-Type: application/json" \
  -H "X-Api-Key: test-key" \
  -d '{"event_id":"evt-trace-001","event_type":"test","source":"test-source","payload":{}}'
```

View the trace in Jaeger at `http://localhost:16686`.

---

## Kubernetes Observability

### GCP Cloud Run + Managed Prometheus + Cloud Trace

When deployed to Cloud Run, Zenith automatically exports traces and metrics to GCP services:

1. **OpenTelemetry SDK** sends traces to Cloud Trace
2. **Prometheus `/metrics`** is scraped by GCP Managed Prometheus
3. **Grafana Dashboard** queries Managed Prometheus

No additional configuration needed — credentials come from Workload Identity.

### Local Kind Cluster

A `PodMonitoring` resource scrapes the Ingestor:

```yaml
# deployments/k8s/local/prom-pod-monitoring.yaml
apiVersion: monitoring.coreos.com/v1
kind: PodMonitoring
metadata:
  name: zenith-ingestor
  namespace: zenith-dev
spec:
  selector:
    matchLabels:
      app: zenith-ingestor
  endpoints:
  - port: metrics
    interval: 30s
```

---

## Troubleshooting

### Metrics Not Appearing

1. Check that `/metrics` endpoint is reachable:
   ```bash
   curl http://localhost:8082/metrics | head -20
   ```

2. Verify Prometheus is configured to scrape the endpoint:
   - In Prometheus UI: **Status** → **Targets**
   - Look for your service; should show "UP"

### Traces Not Appearing in Cloud Trace

1. Verify the endpoint is set:
   ```bash
   echo $OTEL_EXPORTER_OTLP_ENDPOINT
   ```

2. Check logs for OTEL errors:
   ```bash
   go run cmd/ingestor/main.go 2>&1 | grep -i otel
   ```

3. Ensure Workload Identity is configured (Cloud Run) or credentials are available

### Dashboard Not Showing Data

1. Check Prometheus data source in Grafana:
   - **Grafana** → **Data Sources** → **Prometheus**
   - Click **Test** to verify connectivity

2. Run a test query in Prometheus:
   - **Prometheus** → **Graph**
   - Enter query: `up{job="zenith"}` (should return metrics)

3. Re-import the dashboard and verify panels reference correct metrics
