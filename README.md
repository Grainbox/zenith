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
*Currently under active development.*

*   ✅ **Standard Go Layout:** Enforcing strict project organization.
*   ✅ **Consultant-Grade Tooling:** Configured with rigorous linting (`golangci-lint`) and structured logging (`slog`).
*   ✅ **gRPC Interfaces:** Defining the core `.proto` contracts for the Ingestor.
*   🚧 **Containerization:** Preparing Kubernetes (CKAD) ready deployments.

## 🛠 Tech Stack

*   **Language:** Go 1.24+
*   **Protocols:** gRPC & Protocol Buffers, HTTP/REST
*   **Database:** CockroachDB Serverless
*   **Observability:** OpenTelemetry, Prometheus, Grafana
*   **Infrastructure:** Terraform, Kubernetes, Google Cloud Run / AWS Fargate

## 👨‍💻 Getting Started

### Prerequisites
*   Go 1.24 or higher
*   `golangci-lint` for local development

### Quickstart

Clone the repository:
```bash
git clone https://github.com/Grainbox/zenith.git
cd zenith
```

Run the application locally:
```bash
go run cmd/zenith/zenith.go
```
*(Note: As we are in Phase 1, the current output simply verifies the entry point is active).*

## 🤝 Contributing

We welcome contributions! Please see our [Contributing Guidelines](CONTRIBUTING.md) for details on code standards, local setup, and our development workflow.

---
*Built with ❤️ focusing on Cloud-Native best practices.*
