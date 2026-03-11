# 🌌 ZENITH - Distributed Event Observer

[![Go Version](https://img.shields.io/badge/go-1.24+-blue.svg)](https://golang.org/doc/devel/release.html)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

> A high-performance, Cloud-Native backend platform designed to intercept, filter, and route massive event streams in real-time.

---

## 📖 Overview

**Zenith** acts as the central "brain" of your event-driven infrastructure. It receives incoming data streams, evaluates them against dynamic business rules, and triggers automated actions based on the results.

### Core Architecture

Zenith is composed of three primary, horizontally scalable layers:

1.  **Ingestor (Input):** High-throughput event reception supporting gRPC (for performance) and REST/Webhooks.
2.  **Rule Engine (Processing):** Leverages Go's powerful concurrency model (Goroutines/Channels) to filter and evaluate events against rules stored in a globally distributed CockroachDB.
3.  **Dispatcher (Output):** Forwards processed events to designated external sinks (e.g., Message Brokers, Cloud Storage, webhooks, or notification systems like Slack).

## 🚀 Status (Current Phase)

**Phase 1: Project Foundations & Architecture**
*Status: ✅ Completed.*

*   ✅ **Standard Go Layout:** Enforcing strict project organization.
*   ✅ **Consultant-Grade Tooling:** Configured with rigorous linting (`golangci-lint`) and structured logging (`slog`).
*   ✅ **gRPC Interfaces:** Defining the core `.proto` contracts for the Ingestor.
*   ✅ **Containerization:** Preparing Kubernetes (CKAD) ready deployments (`kind`).

**Phase 2: Persistence & Distributed Logic**
*Status: 🚧 In Progress (Sprint 2).*

*   ✅ **gRPC Server:** Building the core Ingestor logic with ConnectRPC.
*   ✅ **Graceful Shutdown:** Implemented OS signal handling for safe terminations.
*   ✅ **Unit Testing:** Initial coverage established using `testify`.
*   🚧 **Database Integration:** Preparing CockroachDB connections (Upcoming).

## 🛠 Tech Stack

*   **Language:** Go 1.24+
*   **Protocols:** gRPC & Protocol Buffers (ConnectRPC), HTTP/REST
*   **Database:** CockroachDB Serverless
*   **Observability:** OpenTelemetry, Prometheus, Grafana
*   **Infrastructure:** Terraform, Kubernetes (Kind for local dev), Docker

## 👨‍💻 Getting Started

### Prerequisites
*   Go 1.24+
*   `buf` & `protoc` plugins for code generation
*   `golangci-lint` for local development
*   Docker & `kind` for local Kubernetes

### Quickstart

Clone the repository:
```bash
git clone https://github.com/Grainbox/zenith.git
cd zenith
```

Run the Ingestor Server locally:
```bash
go run cmd/ingestor/main.go
```
*The server will start on port `:50051`. You can test it using `curl` or `grpcurl`.*


## 🤝 Contributing

We welcome contributions! Please see our [Contributing Guidelines](CONTRIBUTING.md) for details on code standards, local setup, and our development workflow.

---
*Built with ❤️ focusing on Cloud-Native best practices.*
