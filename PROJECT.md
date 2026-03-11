# 📑 Project Specification: ZENITH (Distributed Event Observer)

**Zenith** is a high-performance backend platform designed to intercept, filter, and route massive event streams in real-time. It acts as the "brain" that evaluates incoming data and triggers automated actions based on dynamic business logic (e.g., "If purchase > $500 AND user_segment = 'VIP', then trigger a Priority Slack Alert").

## 1. High-Level Technical Architecture

The system is built as a **Cloud-Native** suite of components written in **Go**, optimized for horizontal scaling:

1. **The Ingestor (Input):** Handles high-throughput event reception via **gRPC** (performance) and **REST/Webhooks** (compatibility).
2. **The Rule Engine (Processing):** Evaluates incoming event payloads against business rules stored in a distributed **CockroachDB** instance.
3. **The Dispatcher (Output):** Forwards processed/transformed events to external sinks (Slack, S3, Webhook URLs, or Message Brokers).

---

## 2. Functional Requirements (MVP)

* **Multi-Protocol Ingestion:** Native support for JSON and Protocol Buffers.
* **Dynamic Rule Management:** Full CRUD for "Rule" objects (e.g., `{ "field": "price", "operator": ">", "value": 100 }`).
* **Real-Time Filtering:** Leverages Go’s **concurrency model** (Goroutines/Channels) to ensure filtering doesn't block ingestion.
* **Reliable Delivery:** Implementation of a **Retry Mechanism** with exponential backoff for failing downstream targets.
* **Built-in Telemetry:** A `/metrics` endpoint exporting real-time processing stats in Prometheus format.

---

## 3. Detailed Technical Stack

* **Runtime:** Go 1.24+ (utilizing Generics and Workspace mode).
* **Communication Layer:** * **Protocol Buffers (protobuf):** For strict schema definition and cross-service contract safety.
* **gRPC:** For low-latency, high-performance internal communication.


* **Storage (NewSQL):** **CockroachDB Serverless** (PostgreSQL-compatible, natively distributed, and resilient).
* **Observability (The "Pro" Edge):**
* **OpenTelemetry:** For distributed tracing (track an event from Ingestor to Dispatcher).
* **Prometheus / Grafana:** For infrastructure and application performance monitoring.


* **Infrastructure as Code (IaC):** **Terraform** to provision Google Cloud Run (or AWS Fargate) and managed DB instances.

---

## 4. Quality Standards (Consultant-Grade)

To stand out in the freelance market, the project must adhere to industry-best practices:

* **12-Factor App Compliance:** Zero hardcoded configuration; strictly environment-variable driven.
* **Graceful Shutdown:** Handlers to ensure in-flight events are processed before the container terminates.
* **Testing Excellence:** Unit tests for the engine and integration tests using `testcontainers-go` (spinning up real DBs for CI).
* **Advanced Linting:** Strict `golangci-lint` configuration to ensure clean, idiomatic Go code.

---

## 5. Portfolio Deliverables

1. **Source Code:** Organized following the [Standard Go Project Layout](https://github.com/golang-standards/project-layout).
2. **Infra-in-a-Box:** A `/terraform` directory to deploy the entire production environment in one command.
3. **Live Observability:** Screenshots or a public Grafana dashboard showcasing the system's throughput and latency.
4. **Performance Report:** A `BENCHMARK.md` comparing gRPC vs. REST overhead under various loads.

---

## 6. Sample Data Schema

Your CockroachDB instance will manage:

* `Sources`: ID, Name, API Key (for Auth/Rate-limiting).
* `Rules`: Source_ID, Condition (stored as JSONB for flexibility), Target_Action (URL/Type).
* `AuditLogs`: Routing status (Success/Fail), Processing Latency (ms), Error Codes.