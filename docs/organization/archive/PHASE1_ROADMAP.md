# 🚀 Detailed Roadmap: Phase 1 - ZENITH

**Global Objective:** Shift from "scripting" to "software architecture" and master the Go ecosystem. Lay the foundations for a "Consultant-Grade" Cloud-Native project.

---

## 🏃 Sprint 1: Project Foundations & Interface Contracts

**Sprint Goal:** Initialize the Go project correctly (Standard Layout), configure high-quality tooling, and define gRPC/Protobuf contracts.

### 📝 Sprint 1 Backlog

*   **[Issue-101] Go Module Initialization and Project Structure**
    *   **Description:** Create the git repository and initialize `go.mod`. Set up the "Standard Go Project Layout" directory structure (`/cmd`, `/internal`, `/pkg`, `/api`, `/build`, `/deployments`, etc.).
    *   **Deliverables:**
        *   `go.mod` file with Go 1.24+ configured.
        *   Standard directories created.
        *   A basic `main.go` in `/cmd/zenith/main.go` displaying "Zenith is starting".
    *   **Status:** ✅ Completed (as `zenith.go`)

*   **[Issue-102] Tooling and Quality Configuration (Linter)**
    *   **Description:** Install and configure `golangci-lint` to ensure idiomatic and clean Go code from the start. Configure strict (Consultant-Grade) rules.
    *   **Deliverables:**
        *   `.golangci.yml` file at the root of the project with enabled linters (e.g., errcheck, revive, govet, staticcheck, etc.).
        *   Documentation in `CONTRIBUTING.md` on how to run the linter locally (`golangci-lint run`).
    *   **Status:** ✅ Completed

*   **[Issue-103] Protocol Buffers Contract Definition (v1)**
    *   **Description:** Create `.proto` files that will define the data structures (Events) and the gRPC service for the Ingestor.
    *   **Deliverables:**
        *   `/api/proto/v1/event.proto` directory/file.
        *   `Event` message definition (ID, Source, Timestamp, JSON/Bytes Payload).
        *   `IngestorService` service definition with an `IngestEvent` method.
	*   **Status:** ✅ Completed

*   **[Issue-104] Go Code Generation from Protobuf**
    *   **Description:** Configure `protoc` (or `buf`) to automatically compile `.proto` files into Go code usable by the application.
    *   **Deliverables:**
        *   Bash script, `Makefile`, or `buf.yaml` configuration to automate generation.
        *   Generated Go code (e.g., under `/pkg/pb/v1/`).
	*   **Status:** ✅ Completed

*   **[Issue-105] [CKAD] Local Kubernetes Environment Setup**
    *   **Description:** Install and configure `minikube` or `kind` on the local development machine for future Kubernetes experiments.
    *   **Deliverables:**
        *   CLI tools (`kubectl`, `minikube` / `kind`) installed and functional.
        *   Ability to start and stop a local cluster.
	*   **Status:** ✅ Completed

---

## 🏃 Sprint 2: gRPC Skeleton & Basic K8s Deployment

**Sprint Goal:** Implement the gRPC server capable of receiving a "ping" and deploy this server on the local Kubernetes cluster.

### 📝 Sprint 2 Backlog

*   **[Issue-201] gRPC Server Implementation (Ingestor - Skeleton)**
    *   **Description:** Develop the gRPC server component using the code generated in [Issue-104]. The server should listen on a port (e.g., 8080) and implement the `IngestorService` interface.
    *   **Deliverables:**
        *   Code in `/internal/ingestor/server.go`.
        *   Update `/cmd/zenith/main.go` to start the gRPC server.
        *   Structured logs (e.g., via `slog`) confirming the server start.
	*   **Status:** ✅ Completed

*   **[Issue-202] Signal Handling (Graceful Shutdown - Basic)**
    *   **Description:** Implement OS signal listening (SIGINT, SIGTERM) to cleanly stop the gRPC server, allowing in-flight requests to complete (crucial for resilience).
    *   **Deliverables:**
        *   Code in `main.go` using `os/signal` to intercept stop signals and call `server.GracefulStop()`.
	*   **Status:** ✅ Completed

*   **[Issue-203] `IngestEvent` Handler Implementation (Ping)**
    *   **Description:** Code the basic logic in the handler to receive the event, log it (console/slog) as "ping received", and send back a success response (Ack).
    *   **Deliverables:**
        *   Functional logic for the `IngestEvent` method.
	*   **Status:** ✅ Completed

*   **[Issue-204] Initial Unit Tests (Ingestor)**
    *   **Description:** Set up the `stretchr/testify` library and write the first unit test to verify that the `IngestEvent` handler correctly processes a mocked request.
    *   **Deliverables:**
        *   `testify` imported in `go.mod`.
        *   `/internal/ingestor/server_test.go` file.
	*   **Status:** ✅ Completed

*   **[Issue-205] [CKAD] Kubernetes Manifest Creation (Pod & Namespace)**
    *   **Description:** Create the first YAML files to deploy the application on the local cluster. Practice creating manifests without a GUI (imperative or declarative).
    *   **Deliverables:**
        *   `/deployments/k8s/local/` directory.
        *   `namespace.yaml` (e.g., `zenith-dev`).
        *   `pod.yaml` (defining a simple Pod, potentially with a temporary Docker image before full dockerization). *Note: Will require a basic `Dockerfile` (Issue-206) if we want to deploy our own code now.*
	*   **Status:** ✅ Completed

*   **[Issue-206] Basic Containerization (Dockerfile)**
    *   **Description:** Create a multi-stage `Dockerfile` to compile the Go application and build a lightweight image (Alpine or Scratch) containing only the executable.
    *   **Deliverables:**
        *   `build/package/Dockerfile` file.
	*   **Status:** ✅ Completed

*   **[Issue-207] Level 1 Final Validation (Milestone)**
    *   **Description:** End-to-end integration test locally. Start the gRPC server and use a gRPC client (like `grpcurl` or `Postman`) to send a "ping" event and verify receipt log and Ack.
    *   **Deliverables:**
        *   Demonstration (or logs) proving the "ping" is received and acknowledged by the system.
	*   **Status:** ✅ Completed
