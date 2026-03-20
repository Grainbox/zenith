# 🚀 Detailed Roadmap: Phase 4 — ZENITH

**Global Objective:** Polish Zenith into a "Production-Ready" portfolio project. Integrate full observability (OpenTelemetry traces + Prometheus metrics), write professional documentation with architecture diagrams and a performance benchmark, clean up the GitHub repository to consultant-grade standards, and sit the CKAD certification exam.

---

## 🏃 Sprint 7: Observability & Telemetry

**Sprint Goal:** Instrument every layer of the pipeline with distributed tracing (OpenTelemetry) and expose real-time metrics (Prometheus). By the end of the sprint, a single event should be traceable from ingestion through rule evaluation to dispatch — all visible in a live dashboard.

### 📝 Sprint 7 Backlog

---

### [Issue-701] OpenTelemetry — Distributed Tracing

**Description:**
Instrument the three pipeline stages (Gateway → Rule Engine → Dispatcher workers) with OpenTelemetry spans so that a single event can be traced end-to-end. Use the OTEL SDK with the `otlptracehttp` exporter targeting GCP Cloud Trace (via the OpenTelemetry Collector sidecar or direct HTTP export). Each span must carry key attributes (`event_id`, `source`, `rule_id`, `sink_type`) to make traces actionable.

**Scope:**
- `cmd/ingestor/main.go` — initialize the global `TracerProvider` at startup; shut it down gracefully.
- `internal/ingestor/gateway.go` — create a root span `"ingestor.gateway.handle"` per incoming HTTP request; propagate context via W3C `traceparent` header.
- `internal/ingestor/engine.go` (or equivalent rule evaluation path) — child span `"engine.evaluate_rules"` with attributes `total_rules`, `matched_count`.
- `internal/dispatcher/worker.go` — child span `"dispatcher.dispatch"` with attributes `rule_id`, `sink_type`; mark span as error on dispatch failure.

**Deliverables:**
- `go.mod` updated with `go.opentelemetry.io/otel`, `go.opentelemetry.io/otel/sdk/trace`, `go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp`.
- `internal/telemetry/tracer.go` — factory function `NewTracerProvider(cfg Config) (*sdktrace.TracerProvider, error)` (service name, exporter endpoint, sampling ratio all from env vars).
- Spans visible in GCP Cloud Trace for a smoke-test event (screenshot as deliverable).
- Unit test: verify that `gateway.go` creates a span and that the span name is `"ingestor.gateway.handle"` using the OTEL in-memory exporter (`sdktrace.NewSimpleSpanProcessor` + `tracetest.NewInMemoryExporter`).

**Env vars added:**
| Variable | Default | Purpose |
|---|---|---|
| `OTEL_EXPORTER_OTLP_ENDPOINT` | `""` (disables tracing) | OTLP HTTP endpoint |
| `OTEL_SERVICE_NAME` | `zenith-ingestor` | Service name in traces |
| `OTEL_SAMPLING_RATIO` | `1.0` | Head-based sampling ratio |

**Status:** [x] Completed

---

### [Issue-702] Prometheus Metrics Endpoint

**Description:**
Add a `/metrics` endpoint exposing real-time processing statistics in Prometheus format. Metrics must cover the three critical dimensions of the pipeline: ingestion throughput, rule evaluation performance, and dispatch reliability. The endpoint must be accessible to Prometheus scraping without authentication (internal-only in production; the Dispatcher Cloud Run service already has `INGRESS_TRAFFIC_INTERNAL_ONLY`).

**Metrics to implement:**

| Metric name | Type | Labels | Description |
|---|---|---|---|
| `zenith_events_received_total` | Counter | `source`, `event_type` | Total events received by the Gateway |
| `zenith_events_accepted_total` | Counter | `source` | Events accepted (auth passed, payload valid) |
| `zenith_events_rejected_total` | Counter | `source`, `reason` | Events rejected (auth fail, invalid payload) |
| `zenith_rules_evaluated_total` | Counter | `source` | Rule evaluations performed |
| `zenith_rules_matched_total` | Counter | `source`, `rule_id` | Rules matched per evaluation |
| `zenith_dispatch_total` | Counter | `sink_type`, `status` | Dispatch attempts (`status`: `success`, `failed`) |
| `zenith_dispatch_duration_seconds` | Histogram | `sink_type` | Dispatch latency distribution |
| `zenith_rule_evaluation_duration_seconds` | Histogram | — | Rule engine evaluation latency |
| `zenith_worker_queue_depth` | Gauge | — | Current depth of the matched-event channel |

**Scope:**
- Add `github.com/prometheus/client_golang` to `go.mod`.
- `internal/telemetry/metrics.go` — declare all metric variables as package-level `prometheus.MustRegister(...)` calls.
- Wire `/metrics` handler in `cmd/ingestor/main.go` using `promhttp.Handler()` on a dedicated internal port (`8082` by default, configurable via `METRICS_PORT`).
- Update Kubernetes manifests: add `prometheus.io/scrape: "true"` and `prometheus.io/port: "8082"` annotations to the Ingestor `Deployment`.
- Update Terraform: expose port `8082` on the Cloud Run service for internal scraping (or document that GCP Managed Prometheus can scrape Cloud Run directly via OTEL).

**Deliverables:**
- `GET /metrics` returns a valid Prometheus exposition format response.
- All 9 metrics listed above are present in the response (even if zero-valued at startup).
- Unit test: send a mock event through the gateway and assert that `zenith_events_received_total` incremented using `testutil.ToFloat64(counter)`.

**Status:** [x] Completed

---

### [Issue-703] Grafana Dashboard & GCP Managed Prometheus

**Description:**
Configure GCP Managed Prometheus (GMP) to scrape the Ingestor's `/metrics` endpoint, and provision a Grafana dashboard (via `deployments/grafana/`) that visualizes the full pipeline in one view. The dashboard must be exportable as a JSON file so it can be committed to the repo and re-imported in any environment — fulfilling the "Infra-in-a-Box" portfolio deliverable.

**Scope:**
- `deployments/terraform/monitoring.tf` — enable `google_monitoring_monitored_project` and configure a GMP `PodMonitoring` resource (or equivalent for Cloud Run) that scrapes `/metrics` on port `8082`.
- `deployments/grafana/zenith-dashboard.json` — Grafana dashboard JSON with the following panels:

| Panel | Query | Visualization |
|---|---|---|
| Events / second | `rate(zenith_events_received_total[1m])` | Time series |
| Acceptance rate | `rate(zenith_events_accepted_total[1m]) / rate(zenith_events_received_total[1m])` | Stat (%) |
| Rules matched / min | `rate(zenith_rules_matched_total[1m])` | Time series |
| Dispatch success rate | `rate(zenith_dispatch_total{status="success"}[1m]) / rate(zenith_dispatch_total[1m])` | Stat (%) |
| Dispatch latency p50/p95/p99 | `histogram_quantile(0.95, zenith_dispatch_duration_seconds_bucket)` | Time series |
| Worker queue depth | `zenith_worker_queue_depth` | Gauge |
| Recent audit log failures | Direct CockroachDB query or log-based metric | Table |

**Deliverables:**
- `deployments/grafana/zenith-dashboard.json` committed to the repo. ✓
- `deployments/terraform/monitoring.tf` with GCP monitoring APIs enabled. ✓
- `deployments/k8s/local/prom-pod-monitoring.yaml` with GMP PodMonitoring CRD. ✓
- `deployments/monitoring/docker-compose.yml` for local monitoring stack. ✓
- Screenshot of the live dashboard with real data (from a smoke-test run) committed to `docs/assets/grafana-dashboard.png`.
- `README.md` updated to link to the screenshot and explain how to import the dashboard.

**Status:** [x] Completed

---

### [Issue-704] [CKAD] Resource Management — Requests, Limits & HPA

**Description:**
Add CPU/memory resource requests and limits to all Kubernetes Deployments, and configure a `HorizontalPodAutoscaler` (HPA) for the Ingestor. This is a critical CKAD exam topic and a production-readiness requirement — without resource limits, a traffic spike can OOM-kill the node; without HPA, the system cannot auto-scale under load.

**Scope:**
- Update `deployments/k8s/local/ingestor-deployment.yaml`: add `resources.requests` and `resources.limits` to the container spec.
- Update `deployments/k8s/local/dispatcher-deployment.yaml`: same pattern.
- Create `deployments/k8s/local/ingestor-hpa.yaml`: `HorizontalPodAutoscaler` targeting the Ingestor Deployment, scaling on CPU utilization.

**Suggested resource values (local Kind cluster):**

| Binary | CPU request | CPU limit | Memory request | Memory limit |
|---|---|---|---|---|
| Ingestor | `100m` | `500m` | `64Mi` | `256Mi` |
| Dispatcher | `50m` | `250m` | `32Mi` | `128Mi` |

**HPA spec:**
```yaml
minReplicas: 1
maxReplicas: 5
metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 60
```

**Deliverables:**
- `resources` block in both Deployment manifests.
- `deployments/k8s/local/ingestor-hpa.yaml` applied to the `zenith-dev` namespace.
- Demonstration: run a load test (`hey` or `k6`) and observe `kubectl get hpa -n zenith-dev` scaling the Ingestor from 1 → N replicas.
- Document the scale-out behavior in `BENCHMARK.md` (Issue-802).

**Status:** [x] Completed

---

## 🏃 Sprint 8: Portfolio Polish & CKAD Certification

**Sprint Goal:** Transform the GitHub repository into a professional, self-contained portfolio artifact. Write comprehensive documentation (README, architecture diagrams, benchmark report). Pass the CKAD certification exam.

### 📝 Sprint 8 Backlog

---

### [Issue-801] README.md — Professional Documentation

**Description:**
Write a comprehensive `README.md` at the repository root that serves as both a technical reference and a portfolio showcase. A recruiter or tech lead should be able to understand the architecture, deploy the system locally, and appreciate the engineering decisions — all from the README.

**Required sections:**

1. **Header** — Project name, tagline, badges (build status, Go version, license, CKAD badge once obtained).
2. **Architecture Overview** — Mermaid diagram showing the full pipeline:
   ```
   Client → Ingestor (gRPC + REST) → Rule Engine → Dispatcher → [Discord / Webhook / S3]
                                                              → audit_logs (CockroachDB)
   ```
   Include a second diagram for the Cloud deployment topology (Cloud Run + CockroachDB Serverless + GCP Secret Manager + Artifact Registry).
3. **Tech Stack** — Table listing Go version, CockroachDB, ConnectRPC/gRPC, Terraform, GitHub Actions, OpenTelemetry, Prometheus, Kubernetes (CKAD).
4. **Quickstart (Local)** — Step-by-step: clone → set env vars → `go run cmd/ingestor/main.go` → `curl POST /v1/events`.
5. **Quickstart (Kubernetes / Kind)** — `make build-kind` → `kubectl apply` → port-forward → smoke test.
6. **Configuration Reference** — All env vars in a table (name, default, description).
7. **API Reference** — `POST /v1/events` (headers, body schema, response codes), `GET /healthz`, `GET /metrics`.
8. **Project Structure** — Annotated directory tree explaining each top-level folder.
9. **Testing** — How to run unit tests, integration tests (requires Docker), and linting.
10. **Observability** — Link to Grafana dashboard screenshot; explain what each metric means.
11. **Roadmap** — Link to `docs/organization/OVERVIEW_ROADMAP.md`; summarize Phase 5 (message broker) as "future work".

**Deliverables:**
- `README.md` at repo root, fully written.
- `docs/assets/architecture-diagram.png` (exported from Mermaid or drawn with Excalidraw).
- All Mermaid diagrams render correctly on GitHub (test in a branch preview).

**Status:** [x] Completed

---

### [Issue-802] BENCHMARK.md — Performance Report

**Description:**
Write a `BENCHMARK.md` at the repository root documenting measured performance characteristics of the pipeline. This fulfills the "Performance Report" portfolio deliverable from `PROJECT.md` and provides evidence of engineering rigor. Use `k6` or `hey` for HTTP load testing and Go's built-in `testing.B` for micro-benchmarks.

**Sections to include:**

**1. Methodology**
- Tools: `hey` for HTTP, `go test -bench` for micro-benchmarks, `pprof` for profiling.
- Environment: local (Kind, M-series or x86-64 spec) + cloud (Cloud Run, 1vCPU / 512Mi).
- Warm-up: 10s ramp-up before measurement.

**2. gRPC vs REST Gateway Comparison**
Compare throughput (RPS), latency (p50/p95/p99), and error rate for the same `IngestEvent` payload sent via:
- Native gRPC (via `grpcurl` or a Go benchmark client)
- REST gateway (`POST /v1/events`)

Present results in a markdown table:

| Protocol | RPS | p50 latency | p95 latency | p99 latency | Error rate |
|---|---|---|---|---|---|
| gRPC | | | | | |
| REST | | | | | |

**3. Rule Engine Micro-Benchmark**
Go benchmark (`go test -bench=BenchmarkEvaluateRules`) measuring evaluation time per event for:
- 1 rule, simple `==` condition
- 10 rules, mixed operators
- 100 rules, mixed operators

**4. Dispatcher Throughput**
Measure how many events/second the Dispatcher can forward to a `httptest.Server` sink before the worker queue backs up. Show queue depth vs RPS curve.

**5. HPA Scale-Out (from Issue-704)**
Chart showing replica count vs incoming RPS during a `k6` ramp test (1→5 replicas). Document time-to-scale.

**6. Bottleneck Analysis**
Identify the limiting factor (DB read for rules, network to sink, CPU in evaluator) and propose Phase 5 mitigations (message broker to decouple ingestion from evaluation).

**Deliverables:**
- `BENCHMARK.md` at repo root with real numbers (not placeholders).
- Raw `hey` / `k6` output committed in `docs/assets/benchmarks/`.
- At least one `go test -bench` result block embedded in the doc.

**Status:** [x] Completed

---

### [Issue-803] GitHub Repository Polish

**Description:**
Finalize the repository to look like a professional open-source project. This is the "Portfolio: Clean up the GitHub repo" deliverable from the overview roadmap. The goal is that any tech lead landing on the repo from a job application should immediately understand the project and trust the code quality.

**Tasks:**

- **GitHub repository settings:**
  - Add description: `"Distributed Event Observer — high-throughput event ingestion, dynamic rule evaluation, and multi-sink dispatch. Built with Go, CockroachDB, and Kubernetes."`
  - Add topics: `go`, `grpc`, `kubernetes`, `cockroachdb`, `opentelemetry`, `terraform`, `cloud-run`, `distributed-systems`.
  - Enable "Discussions" (for portfolio engagement).

- **Labels & Issues cleanup:**
  - Create GitHub labels: `bug`, `enhancement`, `documentation`, `observability`, `infrastructure`, `phase-4`.
  - Close or archive all completed issue stubs (if any exist).
  - Pin Issue-702 (Prometheus metrics) as a showcase of the observability work.

- **`CONTRIBUTING.md`:**
  - How to set up the dev environment (CockroachDB local, env vars).
  - Code style: `golangci-lint`, `slog`, no ORM.
  - PR checklist: tests pass, lint passes, `CLAUDE.md` respected.

- **`CHANGELOG.md`:**
  - Summarize each phase as a release: `v0.1.0` (Phase 1), `v0.2.0` (Phase 2), `v0.3.0` (Phase 3), `v0.4.0` (Phase 4).
  - Follow [Keep a Changelog](https://keepachangelog.com/) format.

- **GitHub Actions badge in README:**
  - Build status badge from `.github/workflows/deploy.yml`.
  - Go version badge from `go.mod`.

- **`docs/assets/`:**
  - Architecture diagram (`architecture-diagram.png`).
  - Grafana dashboard screenshot (`grafana-dashboard.png`).
  - Benchmark charts (from Issue-802).

**Deliverables:**
- Repository description and topics set.
- `CONTRIBUTING.md` and `CHANGELOG.md` committed.
- All `docs/assets/` images in place and linked from `README.md`.

**Status:** [ ] Pending

---

### [Issue-804] [CKAD] Exam Preparation & Certification

**Description:**
Structured preparation sprint for the CKAD (Certified Kubernetes Application Developer) certification. The exam is 2 hours, 100% hands-on in a live cluster via `kubectl`. Speed and muscle memory are the primary differentiators. The target score is 90%+ on the Killer.sh mock exam before sitting the real exam.

**Preparation checklist by domain:**

**Application Design & Build (20%)**
- [ ] Multi-container pods: init containers, sidecars (log shipper, OTEL collector).
- [ ] Build images with Dockerfile best practices (multi-stage builds, non-root user).
- [ ] Jobs and CronJobs: create, inspect failures, set `restartPolicy`.

**Application Deployment (20%)**
- [ ] Deployments with `RollingUpdate` and `Recreate` strategies.
- [ ] `kubectl rollout status`, `rollout undo`, `rollout history`.
- [ ] Helm basics: install a chart, override values, inspect rendered manifests.

**Application Observability & Maintenance (15%)**
- [ ] Liveness, Readiness, and Startup probes (HTTP, TCP, exec).
- [ ] `kubectl logs`, `kubectl exec`, `kubectl describe` for debugging.
- [ ] `kubectl top pods/nodes` (requires Metrics Server).

**Application Environment, Configuration & Security (25%)**
- [ ] ConfigMaps and Secrets (create, mount as volume, inject as env var).
- [ ] SecurityContext: `runAsNonRoot`, `readOnlyRootFilesystem`, `capabilities.drop`.
- [ ] ServiceAccounts and RBAC: Role, RoleBinding, ClusterRole.
- [ ] Resource requests and limits, LimitRange, ResourceQuota.

**Services & Networking (20%)**
- [ ] ClusterIP, NodePort, LoadBalancer Services.
- [ ] Ingress with path-based routing and TLS termination.
- [ ] NetworkPolicy: restrict inter-pod traffic to `zenith-dev` namespace only.

**Killer.sh plan:**
- Attempt #1: baseline score, identify weak domains.
- Targeted practice (Days 2–4): drill weak domains using `kubectl` imperatively.
- Attempt #2 (Day 5): target 90%+.
- **Exam (Day 6 or 7):** Book via Linux Foundation portal.

**Deliverables:**
- Killer.sh score ≥ 90% (screenshot).
- CKAD badge obtained and added to LinkedIn + README.
- `docs/ckad-notes.md` — personal cheat sheet of imperative `kubectl` commands (committed as reference).

**Status:** [ ] Pending

---

### [Issue-805] Phase 4 Final Validation (Milestone)

**Description:**
End-to-end validation of all Phase 4 deliverables. Push a commit to `main`, confirm the CI/CD pipeline deploys successfully, send a load of test events, and verify that traces appear in GCP Cloud Trace, metrics are visible in Grafana, and the audit log reflects all dispatch outcomes. This milestone closes Phase 4 and marks Zenith as a production-ready portfolio project.

**Validation checklist:**

| Check | Expected result |
|---|---|
| `GET /metrics` | HTTP 200, all 9 metric families present |
| `GET /healthz` | HTTP 200 `OK` |
| `POST /v1/events` (x10) | HTTP 202, all events processed |
| GCP Cloud Trace | Root span `"ingestor.gateway.handle"` + child spans visible per event |
| Grafana dashboard | Events/s, dispatch success rate, latency panels showing live data |
| `audit_logs` table | 10 rows with `status=SUCCESS` |
| HPA (local Kind) | At least 1 scale-out event observed during load test |
| `README.md` | Renders correctly on GitHub, all diagrams display |
| `BENCHMARK.md` | Contains real numbers (no `TBD` placeholders) |
| CKAD badge | Published on LinkedIn profile |

**Evidence to capture:**
1. Grafana dashboard screenshot with real pipeline data.
2. GCP Cloud Trace screenshot showing a complete trace with all spans.
3. `kubectl get hpa -n zenith-dev` output during scale-out.
4. CKAD certificate / Credly badge link.
5. GitHub repository URL — clean, professional, complete.

**Status:** [ ] Pending

---

## 🎯 Phase 4 Success Criteria

| Criterion | Evidence |
|---|---|
| Distributed tracing operational | GCP Cloud Trace — root-to-leaf span for a real event |
| Prometheus metrics live | `GET /metrics` on the deployed service |
| Grafana dashboard committed | `deployments/grafana/zenith-dashboard.json` in repo |
| README with Mermaid diagrams | Renders on github.com — architecture + cloud topology |
| Benchmark report with real data | `BENCHMARK.md` — gRPC vs REST table, rule engine benchmark |
| HPA configured and tested | `ingestor-hpa.yaml` + scale-out log from load test |
| Repository is portfolio-ready | Description, topics, CONTRIBUTING.md, CHANGELOG.md, badges |
| CKAD certified | Badge on LinkedIn and Credly |
