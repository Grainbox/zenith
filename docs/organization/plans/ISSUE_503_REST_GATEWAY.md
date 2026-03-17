# Implementation Plan: Issue-503 — REST Gateway for Webhook Ingestion

## Objective

Add an `HTTP/JSON` gateway endpoint `POST /v1/events` alongside the existing `ConnectRPC` interface so that external services can push events via standard webhooks **without** a gRPC client.

---

## Architectural Decisions

### 1. No new router dependency (no Gin, no grpc-gateway)

The codebase uses stdlib `net/http` with `http.NewServeMux()`. Adding Gin or `grpc-gateway` for a single endpoint would violate YAGNI and introduce unnecessary complexity:

- `grpc-gateway` requires `.proto` annotations and a separate code-generation step — overkill for one REST endpoint.
- `Gin` is a full framework with its own `Context` type that conflicts with existing `connectrpc` middleware patterns.
- Stdlib `net/http` is already sufficient: method-scoped routing is natively supported since Go 1.22 (`"POST /v1/events"`).

**Decision: use stdlib `net/http`**, consistent with the existing `healthz` and `status` handler patterns.

### 2. New package `internal/gateway/`

Mirrors the `internal/ingestor/` package structure to keep HTTP concerns isolated. The gateway is a thin translation layer — it parses JSON, constructs a `domain.Event`, and enqueues it into the existing `engine.Pipeline`.

### 3. Direct pipeline injection (not via `ingestor.Server`)

The gateway calls `pipeline.Enqueue()` directly rather than calling `server.IngestEvent()`. This avoids a double-serialization cycle (JSON → proto → domain) and keeps the gateway independent of the protobuf layer. Validation logic specific to the gateway lives in the gateway package.

### 4. Source authentication via `X-Api-Key` header

The `SourceRepository.GetByAPIKey()` method already exists. The gateway uses the `X-Api-Key` request header to authenticate the caller and resolve the source. This:
- Closes the auth gap (the existing gRPC server has no auth — this will be noted as a future improvement).
- Validates that the `source` field in the payload matches a known, authenticated `Source`.

---

## Files to Create / Modify

```
internal/
  gateway/
    handler.go          ← NEW: HTTP JSON handler and response helpers
    handler_test.go     ← NEW: Unit tests using httptest

cmd/ingestor/
  main.go               ← MODIFY: register gateway routes on the existing mux
```

---

## Step-by-Step Implementation

### Step 1 — Define the JSON request/response schema in `internal/gateway/handler.go`

```go
// IngestEventRequest is the JSON body for POST /v1/events.
type IngestEventRequest struct {
    EventID   string          `json:"event_id"`
    EventType string          `json:"event_type"`
    Source    string          `json:"source"`
    Payload   json.RawMessage `json:"payload"`
}
```

- `event_id`, `event_type`, `source` are required strings → validate not empty.
- `payload` is arbitrary JSON — stored as-is in `domain.Event.Payload` (marshalled back to `[]byte`).
- Timestamp is server-assigned on reception (`time.Now().UTC()`), consistent with the gRPC handler behaviour.

Error response:

```go
type errorResponse struct {
    Code    string `json:"code"`
    Message string `json:"message"`
}
```

Helper `writeJSON(w, status, v)` and `writeError(w, status, code, msg)` — private, non-exported.

---

### Step 2 — Implement `Gateway` struct

```go
type Gateway struct {
    logger     *slog.Logger
    pipeline   *engine.Pipeline
    sourceRepo repository.SourceRepository
}

func NewGateway(
    logger *slog.Logger,
    pipeline *engine.Pipeline,
    sourceRepo repository.SourceRepository,
) *Gateway
```

Constructor follows the same dependency-injection pattern as `ingestor.NewServer`.

---

### Step 3 — Implement `HandleIngestEvent` handler

```go
func (g *Gateway) HandleIngestEvent(w http.ResponseWriter, r *http.Request)
```

**Request processing pipeline:**

```
1. Limit request body (http.MaxBytesReader — 1 MB max)
2. Decode JSON body → IngestEventRequest
3. Validate required fields (event_id, event_type, source)
4. Read X-Api-Key header → validate not empty
5. sourceRepo.GetByAPIKey(ctx, apiKey) → resolve Source
6. Verify req.Source == source.Name (prevents spoofing)
7. Marshal payload back to []byte
8. pipeline.Enqueue(ctx, domainEvent)
9. Write 202 Accepted + JSON body {"success": true, "message": "Event accepted"}
```

**Error mapping:**

| Condition | HTTP Status | `code` field |
|---|---|---|
| Body too large | 413 | `PAYLOAD_TOO_LARGE` |
| JSON decode error | 400 | `INVALID_JSON` |
| Missing required field | 400 | `INVALID_ARGUMENT` |
| Missing `X-Api-Key` header | 401 | `UNAUTHENTICATED` |
| Unknown API key | 401 | `UNAUTHENTICATED` |
| Source name mismatch | 403 | `PERMISSION_DENIED` |
| Pipeline full | 503 | `RESOURCE_EXHAUSTED` |
| Internal error | 500 | `INTERNAL` |

> **Note:** Returning `401` (not `404`) for unknown API keys to avoid leaking existence of source names.

---

### Step 4 — Register routes in `cmd/ingestor/main.go`

```go
gw := gateway.NewGateway(logger, pipeline, sourceRepo)
mux.HandleFunc("POST /v1/events", gw.HandleIngestEvent)
```

This single line wires the gateway into the existing `http.ServeMux` without any changes to the h2c or ConnectRPC setup. The `POST /v1/events` route is completely isolated from the existing ConnectRPC routes (which are all under `/proto.v1.IngestorService/`).

The `sourceRepo` is already instantiated in `main.go` — it is passed down to the gateway constructor, consistent with how it's already passed to `engine.NewEvaluator`.

---

### Step 5 — Unit tests in `internal/gateway/handler_test.go`

Use `httptest.NewRecorder()` and `httptest.NewServer()` — no real database required.

**Test cases:**

| Test name | Scenario | Expected HTTP |
|---|---|---|
| `success_valid_event` | Valid JSON body, valid API key | 202 |
| `error_missing_body_fields` | Missing `event_type` | 400 |
| `error_invalid_json` | Malformed JSON | 400 |
| `error_missing_api_key` | No `X-Api-Key` header | 401 |
| `error_unknown_api_key` | Source not found for key | 401 |
| `error_source_mismatch` | `source` field ≠ source.Name | 403 |
| `error_pipeline_full` | Pipeline returns enqueue error | 503 |
| `error_body_too_large` | Body > 1 MB | 413 |

**Mock pattern** (consistent with `internal/ingestor/server_test.go`):

```go
type mockSourceRepo struct {
    source *domain.Source
    err    error
}

func (m *mockSourceRepo) GetByAPIKey(_ context.Context, _ string) (*domain.Source, error) {
    return m.source, m.err
}
// ... implement remaining interface methods as stubs returning nil
```

---

## Config Changes

None required. The gateway reuses existing config values (`PORT`, `ENGINE_*`, `DATABASE_URL`). If a body size limit needs to be configurable in the future, add `GATEWAY_MAX_BODY_BYTES` to `config.go` — but hard-coding 1 MB is acceptable for MVP.

---

## Linting Checklist

Before committing, verify all rules from `.golangci.yml` pass:

- [ ] All errors checked (`errcheck`) — especially `json.NewDecoder.Decode`, `io.Copy`, `w.Write`
- [ ] No `fmt.Print*` — use `slog` with level `Debug`/`Info`/`Warn`/`Error`
- [ ] `gosec`: use `http.MaxBytesReader` to prevent unbounded reads (G112)
- [ ] `noctx`: all repo calls pass the request context `r.Context()`
- [ ] `revive`: exported types/funcs have godoc comments
- [ ] `nilnil`: `writeError` returns nothing (void) — not a `(T, error)` tuple

---

## Acceptance Criteria

- [ ] `curl -X POST http://localhost:8080/v1/events -H "X-Api-Key: <key>" -H "Content-Type: application/json" -d '{"event_id":"e1","event_type":"purchase","source":"my-shop","payload":{"price":120}}' ` returns HTTP 202.
- [ ] Invalid/missing API key returns HTTP 401.
- [ ] `go test ./internal/gateway/...` passes.
- [ ] `golangci-lint run` produces zero warnings.
- [ ] `go test ./...` (existing tests) still pass — no regression.

---

## Non-Goals (Out of Scope for This Issue)

- Retroactively adding API key auth to the existing gRPC/ConnectRPC handler (tracked separately).
- Rate limiting (future, post-Phase 3).
- OpenTelemetry tracing on the gateway (Issue-Phase 4).
- Batch event ingestion (`POST /v1/events/batch`).
