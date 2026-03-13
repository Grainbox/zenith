# 🚀 Detailed Roadmap: Phase 2 - ZENITH (Weeks 3 & 4)

**Global Objective:** Connect the "Brain" (Rule Engine) to "Memory" (CockroachDB) while handling high concurrency efficiently. Master Kubernetes storage and configuration.

---

## 🏃 Sprint 3: Persistence & Rule Management (Week 3)

**Sprint Goal:** Provision CockroachDB, establish a connection from the Go application, and implement CRUD operations for the dynamic rules. Configure the Kubernetes environment for secrets and config injection.

### 📝 Sprint 3 Backlog

*   **[Issue-301] CockroachDB Serverless Provisioning & Connection**
    *   **Description:** Create a free CockroachDB Serverless cluster. Set up the connection in Go using `pgx` or an ORM like `gorm` (or standard `database/sql` for better performance control).
    *   **Deliverables:**
        *   Database connection string secured.
        *   Go code in `/internal/storage/cockroach.go` to handle connection pooling.
    *   **Status:** [x] Completed

*   **[Issue-302] Database Schema & Migrations**
    *   **Description:** Define the schema for `Sources`, `Rules` (using `JSONB` for rule conditions), and `AuditLogs` as described in the architecture. Implement a tool like `golang-migrate` for versioned database schemas.
    *   **Deliverables:**
        *   Migration SQL scripts in `/deployments/db/migrations/`.
        *   Automation to run these migrations against the CockroachDB cluster.
    *   **Status:** [x] Completed

*   **[Issue-303] Dynamic Rule Management (CRUD)**
    *   **Description:** Implement the Go logic (Repository pattern) to create, read, update, and delete filtering rules from the database.
    *   **Deliverables:**
        *   Code in `/internal/repository/rule_repo.go`.
        *   Unit tests mocking the database connection to validate logic.
    *   **Status:** [x] Completed

*   **[Issue-304] Go Integration Tests with `testcontainers-go`**
    *   **Description:** Replace naive storage tests with robust integration tests that spin up an ephemeral CockroachDB Docker container during `go test`.
    *   **Deliverables:**
        *   A test suite in `/internal/repository/rule_repo_test.go` using `testcontainers-go`.
    *   **Status:** [x] Completed

*   **[Issue-305] [CKAD] Kubernetes Configuration (ConfigMaps & Secrets)**
    *   **Description:** Externalize all configuration (DB URL, Port, API Keys). Learn how to inject these values into the Pods using Kubernetes resources.
    *   **Deliverables:**
        *   `/deployments/k8s/local/config.yaml` for non-sensitive values.
        *   `/deployments/k8s/local/secrets.yaml` (Base64 encoded) for the CockroachDB connection string.
        *   Update `pod.yaml` to consume these variables.
    *   **Status:** [x] Completed

---

## 🏃 Sprint 4: The Rule Engine & Concurrency (Week 4)

**Sprint Goal:** Build the core logic that evaluates incoming events against the database rules without blocking ingestion. Master Go's concurrency model and Kubernetes persistence.

### 📝 Sprint 4 Backlog

*   **[Issue-401] The Event Processing Pipeline (Channels & Goroutines)**
    *   **Description:** Modify the Ingestor so that the `IngestEvent` gRPC method pushes events onto a Go channel instead of processing them synchronously. Create background "worker" goroutines that read from this channel.
    *   **Deliverables:**
        *   Code in `/internal/engine/dispatcher.go` or similar.
        *   A configurable worker pool (e.g., 10 concurrent workers).
    *   **Status:** [x] Completed

*   **[Issue-402] Rule Evaluation Logic**
    *   **Description:** Implement the actual filtering logic. When a worker picks up an event, it queries (or uses a cached version of) the active rules from the database to see if the event condition matches (e.g., parsing the JSON payload).
    *   **Deliverables:**
        *   Evaluation logic in `/internal/engine/evaluator.go`.
        *   Unit tests specifically checking complex JSON filtering conditions.
    *   **Status:** [x] Completed

*   **[Issue-403] Advanced Graceful Shutdown (Zero Event Loss)**
    *   **Description:** Ensure that when a SIGTERM is received, the Ingestor stops accepting new gRPC requests, but the worker goroutines finish processing all events currently waiting in the channels before the program exits fully.
    *   **Deliverables:**
        *   Use `sync.WaitGroup` to track active workers and ensure wait logic in `main.go`.
    *   **Status:** [x] Completed

*   **[Issue-404] Engine Stress Test & Concurrency Validation**
    *   **Description:** Write tests to fire thousands of mock events simultaneously into the Ingestor to guarantee there are no data races or deadlocks (using `go test -race`).
    *   **Deliverables:**
        *   Proof of successful race-detector tests.
        *   Log of the engine processing a burst of events smoothly.
    *   **Status:** [x] Completed

*   **[Issue-405] [CKAD] Kubernetes Persistence (PV & PVC)**
    *   **Description:** Statefulness in K8s. Although CockroachDB is serverless (managed externally) for Prod, we should practice running a local PostgreSQL pod with persistent storage to master PersistentVolumes.
    *   **Deliverables:**
        *   `/deployments/k8s/local/storage/pv.yaml` and `pvc.yaml`.
        *   A temporary pod mounting the volume to persist some mock data.
    *   **Status:** [ ] To Do

*   **[Issue-406] Level 2 Final Validation (Milestone)**
    *   **Description:** Start the entire system. Ingest an event via gRPC, watch the background worker pick it up, validate it against a rule in CockroachDB, and log the action successfully before a clean system shutdown.
    *   **Deliverables:**
        *   Complete end-to-end trace logged to console.
    *   **Status:** [ ] To Do
