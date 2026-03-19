# Load Testing Guide

A production-ready load generator for generating synthetic event streams to the Zenith Ingestor. Used for performance testing, dashboard visualization, and system validation.

## Features

- **Rate limiting** via configurable RPS (requests per second)
- **Worker pool** pattern for efficient concurrent load generation
- **Retry logic** with exponential backoff
- **Graceful shutdown** via context and signal handling
- **Comprehensive metrics** (success rate, actual RPS, latency)
- **Clean logging** via slog (structured logging)

## Prerequisites

- Zenith ingestor running locally (`go run cmd/ingestor/main.go`)
- Monitoring stack running (`docker compose -f deployments/monitoring/docker-compose.yml up -d`)
- At least one `source` and associated `rule` in the database

### Database Setup

**Verify a source exists** with the API key you plan to use:

```sql
SELECT id, name FROM sources;
```

**Verify at least one rule** is configured and targets a reachable sink:

```sql
SELECT id, source_id, condition, sink_type, target_action FROM rules;
```

For load testing, use `sink_type = 'http'` with `target_action = 'https://httpbin.org/post'` — this endpoint accepts anything, always returns 200, and has no rate-limiting.

> **Important:** The load generator sends events with `source: "zenith-demo"`. That name must match the source registered for the API key you use (`-key` flag). A mismatch causes `Source name mismatch` warnings and all events are rejected.

## Quick Start

```powershell
# Run Zenith Ingestor first:
go run cmd/ingestor/main.go

# In another terminal, start the load generator:
cmd/load-generator/load-generator.exe -target http://localhost:8080 -key <your-api-key> -rps 50 -duration 5m -workers 10
```

## Configuration

| Flag | Default | Description |
|------|---------|-------------|
| `-target` | `http://localhost:8080` | Target Ingestor URL |
| `-key` | *(required)* | API key — must match a source in the DB |
| `-rps` | `10` | Target requests per second |
| `-duration` | `1m` | How long to generate events |
| `-workers` | `10` | Number of concurrent workers |
| `-verbose` | `false` | Enable verbose logging |

## Examples

### Light load (dashboard visualization)
```powershell
cmd/load-generator/load-generator.exe -key <your-api-key> -rps 20 -duration 2m -workers 5
```

### Heavy load (performance testing)
```powershell
cmd/load-generator/load-generator.exe -key <your-api-key> -rps 500 -duration 10m -workers 50 -verbose
```

## Output

The generator logs metrics in structured JSON format:

```json
{
  "level": "INFO",
  "msg": "Load generation complete",
  "sent": 15000,
  "failed": 0,
  "retried": 0,
  "success_rate": "100.0%",
  "actual_rps": "50.0",
  "duration": "5m0s"
}
```

## Synthetic Event Structure

Generated events follow this pattern:

```json
{
  "event_id": "evt_1711003425123456789_1",
  "event_type": "purchase|refund|error|audit|inventory",
  "source": "zenith-demo",
  "payload": {
    "severity": "info|warning|error|critical",
    "value": 0.0-100.0,
    "duration": 0-5000
  }
}
```

## Viewing Metrics

### Grafana

Open `http://localhost:3000` (credentials: `admin` / `admin`).

Navigate to **Dashboards → Zenith Pipeline**.

Grafana auto-refreshes every 30 seconds. Use the time picker (top-right) to zoom in on your test window (e.g. "Last 15 minutes").

### Prometheus

Open `http://localhost:9090` to query raw metrics directly. Useful for debugging panels that show "No data".

```promql
# Raw event counter
zenith_events_received_total

# Dispatch count by sink and status
zenith_dispatch_total

# Rule match rate
rate(zenith_rules_matched_total[1m])
```

## Dashboard Panels: Interpretation

### Events / second
**Query:** `rate(zenith_events_received_total[1m])`

Shows how many events/sec the Gateway is receiving. Should match your `-rps` value closely (within ~10%).

---

### Acceptance Rate
**Query:** `sum(rate(zenith_events_accepted_total[1m])) / sum(rate(zenith_events_received_total[1m]))`

Percentage of events that passed authentication and validation. Should be **100%** under normal conditions.

If below 100%:
- Wrong API key (`-key` flag)
- Source name mismatch (source in DB vs. `"zenith-demo"` sent by generator)
- Invalid payload structure

---

### Dispatch Success Rate
**Query:** `sum(rate(zenith_dispatch_total{status="success"}[1m])) / sum(rate(zenith_dispatch_total[1m]))`

Percentage of dispatched events that reached the sink. Should be **100%** with httpbin.org.

If below 100%:
- Sink URL unreachable
- Discord webhooks enforce strict rate limits (429) — use `sink_type = 'http'` for load tests
- `Dispatch channel full` warnings mean dispatcher workers are saturated (see Troubleshooting)

---

### Rules Matched / min
**Query:** `rate(zenith_rules_matched_total[1m]) * 60`

Number of rule matches per minute. With one catch-all rule and 50 RPS, expect ~3000 matches/min.

If 0: no rules match the event payload. The generator sends `{ "severity": "...", "value": ..., "duration": ... }`.

---

### Dispatch Latency (p50/p95/p99)

End-to-end time from dispatch attempt to sink response.

**Expected ranges (httpbin.org):**

| Percentile | Normal | Concern |
|---|---|---|
| p50 | 50–500ms | > 1s |
| p95 | 200–1500ms | > 3s |
| p99 | 500–3000ms | > 8s |

High latency: network issues, sink rate-limiting, or dispatcher saturation.

---

### Rule Evaluation Latency (p95)

Time to evaluate all rules for one event (includes DB query).

**Expected:** 10–100ms. Above 200ms suggests slow DB queries.

---

### Dispatch Failures / min

Should be **0** during normal operation. Common causes: sink unreachable, 429 rate-limiting (Discord).

## Architecture

```
Rate Limiter → Event Channel → Worker Pool (N workers)
                                      ↓
                                 HTTP Client
                                      ↓
                              Ingestor (:8080)
```

Each worker consumes events from the channel, retries failed requests (max 3 attempts, exponential backoff), and updates metrics. Workers drain cleanly on SIGINT/SIGTERM.

## Testing

```powershell
go test -v ./internal/load/...
```

Tests cover: config validation, event generation, load generation with mock server, early cancellation, and metrics accuracy.

## Performance Notes

- **Throughput**: Tested up to 10k RPS on modest hardware (8 core, 16GB RAM)
- **Memory**: ~50MB baseline + ~1MB per 1k pending requests
- **CPU**: Scales linearly with RPS

For higher throughput, increase `-workers`. Optimal value is typically 2× CPU core count.

## Troubleshooting

### "No data" on a Grafana panel
1. Check Prometheus has the metric at `http://localhost:9090`
2. Prometheus scrapes every 15 seconds — wait ~30s after starting the generator
3. Check the Grafana time range includes your test window ("Last 15 minutes")

### "Dispatch channel full" warnings
The matched-event channel (capacity 256) is saturated. Increase dispatcher workers in `cmd/ingestor/main.go → setupPipeline()`:
```go
disp := dispatcher.New(matchCh, 20, registry, ...) // was 4
```

### Discord 429 rate-limiting
Use `sink_type = 'http'` with `target_action = 'https://httpbin.org/post'` for load tests instead of a Discord webhook.

### Source name mismatch
The load generator sends `source: "zenith-demo"`. Make sure your DB source record has `name = 'zenith-demo'`, or update `sources` in `internal/load/generator.go`.

### High failure rate
- Check Ingestor is running: `curl http://localhost:8080/healthz`
- Reduce RPS or increase `-workers`
- Check rejected events: `curl http://localhost:8082/metrics | grep zenith_events_rejected`
