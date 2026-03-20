# Roadmap

Zenith is developed in phases. See [docs/organization/OVERVIEW_ROADMAP.md](../docs/organization/OVERVIEW_ROADMAP.md) for detailed plans and sprint breakdowns.

## Completed Phases

### Phase 1: Foundations & Software Design ✅

**Goal:** Shift from scripting to software architecture. Master Go and Kubernetes basics.

**Deliverables:**
- Standard Go project layout with proper package structure
- Protobuf v3 service definitions (gRPC contract)
- `golangci-lint` configuration for code quality
- Initial unit tests with `testify`
- Local Kubernetes setup with `kind`

**Status:** Complete

---

### Phase 2: Persistence & Distributed Logic ✅

**Goal:** Connect the Rule Engine to a real database. Handle concurrency safely.

**Key accomplishments:**
- CockroachDB Serverless instance
- Rule Engine with goroutine worker pool and channels
- Graceful shutdown (zero event loss on redeploy)
- Repository layer (DB abstraction)
- Integration tests with `testcontainers-go`

**Highlights:**
- Non-blocking concurrency via Go channels
- No shared state between workers; no mutex locks
- Event pipeline tested under load

**Status:** Complete

---

### Phase 3: Infrastructure as Code & Cloud ✅

**Goal:** Become cloud-native. Automate infrastructure provisioning and CI/CD.

**Key accomplishments:**
- Terraform scripts for Google Cloud Run, Artifact Registry, Secret Manager
- GitHub Actions CI/CD pipeline (lint → test → build → deploy)
- REST Gateway using standard HTTP handlers (not an external framework)
- Dispatcher service with worker pool and extensible sink architecture
- Multi-binary Kubernetes manifests (Ingestor + Dispatcher)
- Health checks and liveness/readiness probes

**Highlights:**
- Auto-deploy on `git push` to main
- Docker images tagged with git SHA for reproducibility
- Secrets managed securely in GCP Secret Manager, not Terraform state
- Kubernetes manifests follow CKAD best practices

**Status:** Complete

---

### Phase 4: Observability & Polish ✅

**Goal:** Transform into a production-ready, professionally documented portfolio project.

**Key accomplishments:**
- **Distributed Tracing:** OpenTelemetry SDK integrated; traces flow from Ingestor through Dispatcher to Cloud Trace
- **Prometheus Metrics:** 9 metric families covering ingestion, evaluation, dispatch, and queue depth
- **Grafana Dashboard:** Pre-built, importable dashboard with 7 key panels
- **HPA (Horizontal Pod Autoscaler):** Auto-scales Ingestor based on CPU utilization
- **Resource Limits:** CPU/memory requests and limits on all Deployments (CKAD requirement)
- **Professional Documentation:** README with architecture diagrams (Mermaid), complete API reference, configuration guide, observability setup
- **CKAD Preparation:** Full exam readiness (deferred to post-Phase-4)

**Highlights:**
- Full trace visibility: see event flow from REST request through rule evaluation to dispatch in a single trace tree
- Live Grafana dashboard showing pipeline health: events/sec, acceptance rate, dispatch latency, success rate
- Every event is observable end-to-end

**Status:** Complete

---

## Planned Phases

### Phase 5: Message Broker Decomposition 🔮

**Goal:** Enable independent scaling of Ingestor, Rule Engine, and Dispatcher.

**Planned accomplishments:**

1. **Extract Rule Engine into standalone service**
   - New `cmd/engine/main.go` binary
   - Consumes events from message broker
   - Evaluates rules, publishes matches back to broker

2. **Introduce message broker**
   - Options: NATS, Kafka, or GCP Pub/Sub
   - Decouples Ingestor from Rule Engine evaluation latency
   - Enables async batching for higher throughput

3. **Three independently scalable services:**
   - **Ingestor** — Pure ingestion, authentication, light validation
   - **Rule Engine** — CPU-bound rule evaluation
   - **Dispatcher** — I/O-bound sink dispatch

4. **Architectural pattern:**
   ```
   [Ingestor] --REST/gRPC--> [Queue] --> [Rule Engine] --Queue--> [Dispatcher]
   ```

**Benefits:**
- Horizontal scaling: scale any layer independently
- Better resource utilization: CPU-heavy and I/O-heavy workloads decoupled
- Higher throughput: batching in message broker
- Multi-language rule engine (not just Go)

**Tradeoff:** Latency increases from ~1ms (in-process channels) to ~100ms (broker hops + async batching).

**Rationale:** Phase 5 is for ultra-high-throughput scenarios (millions of events/hour). Phase 4 handles typical use cases well.

---

### Phase 6+ (Future)

Potential enhancements:
- Multi-tenant support (multiple organizations, isolated rules)
- Rule versioning and rollback
- A/B testing rules
- Custom rule DSL (instead of JSON condition)
- UI for rule management

---

## Success Criteria

### Phase 4 ✅

| Criterion | Status |
|---|---|
| Distributed tracing operational | ✅ GCP Cloud Trace shows complete traces |
| Prometheus metrics live | ✅ `GET /metrics` on every deployment |
| Grafana dashboard committed | ✅ `deployments/grafana/zenith-dashboard.json` |
| Architecture diagrams in README | ✅ Mermaid (local + cloud topology) |
| Complete API documentation | ✅ All endpoints, error codes, examples |
| Professional GitHub repository | ✅ Description, topics, README, CONTRIBUTING |
| HPA configured and tested | ✅ Auto-scales Ingestor under load |
| CKAD-ready Kubernetes manifests | ✅ Probes, resource limits, HPA |

### Phase 5 (Future)

| Criterion | Target |
|---|---|
| Three independently deployable services | Ingestor, Engine, Dispatcher |
| Message broker integration | NATS or Kafka deployed |
| Scale-independent load tests | Show independent scaling effectiveness |
| Multi-language rule evaluation | At least one non-Go implementation |

---

## Links

- **Detailed Phase 1 plan:** [docs/organization/archive/PHASE1_ROADMAP.md](../docs/organization/archive/PHASE1_ROADMAP.md)
- **Detailed Phase 2 plan:** [docs/organization/archive/PHASE2_ROADMAP.md](../docs/organization/archive/PHASE2_ROADMAP.md)
- **Detailed Phase 3 plan:** [docs/organization/archive/PHASE3_ROADMAP.md](../docs/organization/archive/PHASE3_ROADMAP.md)
- **Detailed Phase 4 plan:** [docs/organization/PHASE4_ROADMAP.md](../docs/organization/PHASE4_ROADMAP.md)
- **Overview roadmap:** [docs/organization/OVERVIEW_ROADMAP.md](../docs/organization/OVERVIEW_ROADMAP.md)
