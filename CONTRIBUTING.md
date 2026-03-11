# Contributing to ZENITH

Thank you for your interest in contributing to **ZENITH**, the Distributed Event Observer. To maintain a high standard of quality ("Consultant-Grade"), please follow these guidelines.

## 🛠 Prerequisites

- **Go:** 1.24+
- **Linter:** [golangci-lint](https://golangci-lint.run/usage/install/) (latest version)
- **Protocol Buffers:** `protoc` and `protoc-gen-go` (for future API changes)

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

1.  Pick an issue from the [Roadmap](docs/organization/PHASE1_ROADMAP.md).
2.  Follow the 12-Factor App principles.
3.  Ensure your code is documented with proper Go comments.
