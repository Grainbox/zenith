# Contributing to ZENITH

Thank you for your interest in contributing to **ZENITH**, the Distributed Event Observer. To maintain a high standard of quality ("Consultant-Grade"), please follow these guidelines.

## 🛠 Prerequisites

- **Go:** 1.26+ (must match `go.mod` version)
- **Linter:** [golangci-lint](https://golangci-lint.run/usage/install/) v1.62.0+
- **Protocol Buffers:** `buf` CLI (for code generation)
- **Docker:** Required for integration tests (`testcontainers-go`)

## 📁 Project Structure

We follow the [Standard Go Project Layout](https://github.com/golang-standards/project-layout):
- `/cmd/`: Main applications (entry points).
- `/internal/`: Private library code (packages not for external use).
- `/pkg/`: Public library code (can be used by other projects).
- `/api/`: API definitions (Protobuf files).
- `/docs/`: Documentation and roadmaps.

## 🛡 Quality Standards

### Linting
Before submitting a PR, you **must** ensure your code passes the linter. We use strict rules defined in `.golangci.yml`.

To run the linter locally:
```powershell
golangci-lint run
```

### Logging
Do **not** use `fmt.Print` or `fmt.Println` for logging. Use the structured logger `log/slog`.
```go
slog.Info("Something happened", "key", value)
```

## 🧪 Testing

Run all tests before committing:
```powershell
go test ./...
```

## 📜 Development Workflow

1.  Pick an issue from the [Roadmap](docs/organization/PHASE3_ROADMAP.md).
2.  Follow the 12-Factor App principles.
3.  Ensure your code is documented with proper Go comments.
4.  Run tests locally before pushing:
    ```bash
    go test ./...
    golangci-lint run
    buf lint
    ```

## 🔄 Continuous Integration

All pushes to `main` trigger GitHub Actions (`.github/workflows/deploy.yml`):

- **Pull Requests:** Lint + Test only (no deployment)
- **Push to main:** Full pipeline (lint → test → build → push → deploy)

The pipeline uses **Workload Identity Federation** for secure GCP authentication (no long-lived secrets stored).

See [Issue-502 plan](docs/organization/plans/ISSUE_502_CICD.md) for setup details.
