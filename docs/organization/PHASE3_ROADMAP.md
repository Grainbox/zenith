# 🚀 Detailed Roadmap: Phase 3 - ZENITH (Weeks 5 & 6)

**Global Objective:** Become "Cloud-Native" by automating the entire infrastructure lifecycle. Implement the Dispatcher service and expose Zenith on a public URL. Master Kubernetes workload management and zero-downtime deployments.

---

## 🏃 Sprint 5: Infrastructure as Code & CI/CD (Week 5)

**Sprint Goal:** Provision cloud infrastructure with Terraform and automate deployments via GitHub Actions. Add a REST gateway to accept standard webhooks alongside the existing gRPC interface.

### 📝 Sprint 5 Backlog

*   **[Issue-501] Terraform Infrastructure Provisioning**
    *   **Description:** Write Terraform scripts to provision the target cloud environment (Google Cloud Run or AWS Fargate) including networking, IAM roles, and service configuration. Follow the "Terraform Rule": no resource is created manually in the Cloud Console.
    *   **Deliverables:**
        *   Terraform config in `/deployments/terraform/` (main, variables, outputs).
        *   `terraform plan` and `terraform apply` produce a live, reachable endpoint.
        *   Plan disponible dans `docs/organization/plans/ISSUE_501_TERRAFORM.md`
    *   **Status:** [x] Completed

*   **[Issue-502] GitHub Actions CI/CD Pipeline**
    *   **Description:** Build a GitHub Actions workflow that automatically lints, tests, builds the Docker image, pushes it to a container registry, and deploys to the cloud environment on every push to `main`.
    *   **Deliverables:**
        *   `.github/workflows/deploy.yml` covering lint → test → build → push → deploy.
        *   Secrets (registry credentials, cloud credentials) injected via GitHub Secrets.
        *   Plan disponible dans `docs/organization/plans/ISSUE_502_CICD.md`
    *   **Status:** [x] Completed

*   **[Issue-503] REST Gateway for Webhook Ingestion**
    *   **Description:** Add an HTTP/JSON gateway alongside the existing ConnectRPC interface so that external services can push events via standard webhooks without a gRPC client. Use `grpc-gateway` or a lightweight `Gin` router to translate incoming POST requests into the existing `IngestEvent` pipeline.
    *   **Deliverables:**
        *   HTTP endpoint `POST /v1/events` translating the JSON body into an `IngestEvent` call.
        *   Gateway wired in `cmd/ingestor/main.go` alongside the existing h2c server.
    *   **Status:** [x] Completed

*   **[Issue-504] [CKAD] Kubernetes Deployments & Rolling Updates**
    *   **Description:** Replace the bare pod manifest from Phase 2 with a proper `Deployment` resource. Practice rolling updates and rollbacks using `kubectl rollout` to simulate zero-downtime deploys.
    *   **Deliverables:**
        *   `/deployments/k8s/local/ingestor-deployment.yaml` with configurable `replicas` and `strategy: RollingUpdate`.
        *   Successful `kubectl rollout undo` demonstration on a bad image tag.
    *   **Status:** [x] Completed

---

## 🏃 Sprint 6: The Dispatcher & Cloud Deployment (Week 6)

**Sprint Goal:** Implement the Dispatcher binary, integrate external sinks, and validate the full end-to-end pipeline running live on the cloud with automated deployments.

### 📝 Sprint 6 Backlog

*   **[Issue-601] Dispatcher Service Implementation**
    *   **Description:** Create the `cmd/dispatcher/main.go` binary. It consumes matched events produced by the Rule Engine (via an internal channel or future broker interface) and is independently startable and deployable from the Ingestor.
    *   **Deliverables:**
        *   `cmd/dispatcher/main.go` as a standalone entry point.
        *   `internal/dispatcher/` package with its own graceful shutdown logic (mirroring the Ingestor pattern).
    *   **Status:** [x] Completed

*   **[Issue-601B] Sink Config Cleanup**
    *   **Description:** Remove all sink-related environment variables (`DISPATCH_SINK_SLACK_URL`, `SLACK_WEBHOOK_URL`) from config, infrastructure files, and documentation. Establishes the architectural principle: sink target URLs are data-driven (from `rule.TargetAction`), not configuration.
    *   **Deliverables:**
        *   `DispatcherConfig` struct removed from `internal/config/config.go`.
        *   All infra files (K8s secrets, Terraform) updated — only `DATABASE_URL` and `API_KEY_SALT` remain as managed secrets.
        *   CLAUDE.md, README.md, and plan documents updated to reflect the principle.
    *   **Status:** [x] Completed

*   **[Issue-602] Platform Sinks (Discord + extensible architecture)**
    *   **Description:** Implement the Discord sink for the portfolio demo, and establish the architecture that makes adding future platforms (Slack, Teams, etc.) a one-file addition without touching the Dispatcher core. Each sink formats its payload differently but implements the same `Sink` interface. The target URL always comes from `rule.TargetAction` — never from config.
    *   **Sink selection:** Add a `sink_type` column (`text`, e.g. `"discord"`, `"slack"`, `"http"`) to the `rules` table. The Dispatcher resolves the correct `Sink` implementation from a registry keyed by `sink_type` at startup. Adding a new platform = implement `Sink`, register it — zero changes to the Dispatcher core.
    *   **Deliverables:**
        *   DB migration: `sink_type text NOT NULL DEFAULT 'http'` column on `rules`.
        *   `internal/dispatcher/sinks/discord.go` — formats payload as `{"content": "..."}` and POSTs to `rule.TargetAction`.
        *   `internal/dispatcher/sinks/http.go` — generic fallback, POSTs raw matched event JSON.
        *   Sink registry in `internal/dispatcher/registry.go` — maps `sink_type` string to `Sink` implementation.
        *   Unit tests for `DiscordSink` using `httptest.Server` (success, non-2xx, timeout).
    *   **Status:** [x] Completed

*   **[Issue-603] Audit Log Write-Back**
    *   **Description:** After the Dispatcher forwards a matched event to a sink, write a record to the `audit_logs` table (success/failure, sink target, timestamp). This closes the observability loop started with the schema in Issue-302.
    *   **Deliverables:**
        *   `AuditLogRepository` implementation in `/internal/repository/postgres/`.
        *   Dispatcher writes one `audit_log` row per dispatched event.
    *   **Status:** [x] Completed

*   **[Issue-604] Multi-Binary Kubernetes Manifests**
    *   **Description:** Create separate `Deployment` and `Service` manifests for the Ingestor and Dispatcher so they are independently deployable and scalable within the `zenith-dev` namespace.
    *   **Deliverables:**
        *   `/deployments/k8s/local/dispatcher-deployment.yaml` and `dispatcher-service.yaml`.
        *   Both binaries deployable independently via `kubectl apply`.
    *   **Status:** [x] Completed

*   **[Issue-605] [CKAD] Services & Liveness/Readiness Probes**
    *   **Description:** Expose the Ingestor and Dispatcher via Kubernetes `Service` resources (ClusterIP internal, LoadBalancer external). Add Liveness and Readiness probes to both Deployments to ensure zero-downtime and correct traffic routing during rolling updates.
    *   **Deliverables:**
        *   `livenessProbe` and `readinessProbe` configured in both Deployment manifests.
        *   LoadBalancer `Service` for the Ingestor exposing the gRPC + HTTP gateway ports.
    *   **Status:** [x] Completed

*   **[Issue-605B] Cloud Deployment — Dispatcher Binary**
    *   **Description:** The Dispatcher binary exists locally (Issue-604) but is invisible to the cloud pipeline. The CI/CD workflow only builds and pushes the `ingestor` image; Terraform only provisions one Cloud Run service. This issue closes that gap so the full pipeline runs on GCP, not just in the local Kind cluster.
    *   **Deliverables:**
        *   `.github/workflows/deploy.yml` — `build-push` job builds and pushes `Dockerfile.dispatcher` → `dispatcher:${SHA}` and `dispatcher:latest` to Artifact Registry, in parallel with the ingestor build.
        *   `deployments/terraform/cloud_run.tf` — `google_cloud_run_v2_service.dispatcher` resource: internal-only ingress (`INGRESS_TRAFFIC_INTERNAL_ONLY`), `DATABASE_URL` from Secret Manager, startup and liveness probes on `/healthz:8081`, shared `zenith_runner` service account (no new IAM needed).
        *   `deployments/terraform/outputs.tf` — `dispatcher_url` output for debugging and Issue-606 validation.
    *   **Note:** The Dispatcher must **not** be publicly invokable — unlike the Ingestor, it is a background worker. External traffic must be blocked at the IAM level (`allUsers` invoker role must NOT be added).
    *   **Status:** [x] Completed

*   **[Issue-606] Level 3 Final Validation (Milestone)**
    *   **Description:** End-to-end smoke test on the live cloud deployment. Push a commit to `main`, watch the CI/CD pipeline deploy automatically, send a webhook event to the public URL, verify the Rule Engine evaluates it, and confirm the Dispatcher forwards it to a Slack channel with a matching `audit_log` row written to CockroachDB.
    *   **Deliverables:**
        *   Public URL accessible and returning a valid response.
        *   Screenshot or log of: incoming event → rule match → Slack notification → audit_log row.
    *   **Status:** [ ] Pending
