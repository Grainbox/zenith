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

*   **[Issue-602] External Sinks Integration (Slack & Webhooks)**
    *   **Description:** Implement at least two sink adapters in the Dispatcher: one for Slack (via Incoming Webhook URL) and one for a generic HTTP webhook. Each adapter reads its target URL from environment variables.
    *   **Deliverables:**
        *   `internal/dispatcher/sinks/slack.go` and `internal/dispatcher/sinks/webhook.go`.
        *   Unit tests for each sink using an `httptest.Server` as the mock target.
    *   **Status:** [ ] Pending

*   **[Issue-603] Audit Log Write-Back**
    *   **Description:** After the Dispatcher forwards a matched event to a sink, write a record to the `audit_logs` table (success/failure, sink target, timestamp). This closes the observability loop started with the schema in Issue-302.
    *   **Deliverables:**
        *   `AuditLogRepository` implementation in `/internal/repository/postgres/`.
        *   Dispatcher writes one `audit_log` row per dispatched event.
    *   **Status:** [ ] Pending

*   **[Issue-604] Multi-Binary Kubernetes Manifests**
    *   **Description:** Create separate `Deployment` and `Service` manifests for the Ingestor and Dispatcher so they are independently deployable and scalable within the `zenith-dev` namespace.
    *   **Deliverables:**
        *   `/deployments/k8s/local/dispatcher-deployment.yaml` and `dispatcher-service.yaml`.
        *   Both binaries deployable independently via `kubectl apply`.
    *   **Status:** [ ] Pending

*   **[Issue-605] [CKAD] Services & Liveness/Readiness Probes**
    *   **Description:** Expose the Ingestor and Dispatcher via Kubernetes `Service` resources (ClusterIP internal, LoadBalancer external). Add Liveness and Readiness probes to both Deployments to ensure zero-downtime and correct traffic routing during rolling updates.
    *   **Deliverables:**
        *   `livenessProbe` and `readinessProbe` configured in both Deployment manifests.
        *   LoadBalancer `Service` for the Ingestor exposing the gRPC + HTTP gateway ports.
    *   **Status:** [ ] Pending

*   **[Issue-606] Level 3 Final Validation (Milestone)**
    *   **Description:** End-to-end smoke test on the live cloud deployment. Push a commit to `main`, watch the CI/CD pipeline deploy automatically, send a webhook event to the public URL, verify the Rule Engine evaluates it, and confirm the Dispatcher forwards it to a Slack channel with a matching `audit_log` row written to CockroachDB.
    *   **Deliverables:**
        *   Public URL accessible and returning a valid response.
        *   Screenshot or log of: incoming event → rule match → Slack notification → audit_log row.
    *   **Status:** [ ] Pending
