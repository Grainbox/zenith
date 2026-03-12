# Load environment variables
ifneq (,$(wildcard ./.env.secrets))
	include .env.secrets
	export $(shell sed 's/=.*//' .env.secrets)
endif

# Command variables
BUF_GEN     = buf generate
BUF_LINT    = buf lint
GO_TIDY     = go mod tidy
DOCKER_CMD  = docker build -t zenith-ingestor:latest -f build/package/Dockerfile .
KIND_LOAD   = kind load docker-image zenith-ingestor:latest --name zenith-lab

# Database Migration string (Security check)
MIGRATE_CMD = migrate -path deployments/db/migrations -database "$(DATABASE_URL)"

.PHONY: all gen lint tidy migrate-up migrate-down build-kind help

all: lint gen tidy ## Run lint, generate code and tidy modules

## Database migrations (requires .env.secrets)
migrate-up: ## Run database migrations up
	@if [ -z "$(DATABASE_URL)" ]; then echo "❌ Error: DATABASE_URL is not defined. Check your .env.secrets"; exit 1; fi
	$(MIGRATE_CMD) up

migrate-down: ## Run database migrations down (1 step)
	$(MIGRATE_CMD) down 1

## Local development (Kind)
build-kind: ## Build Docker image and load into local Kind cluster
	$(DOCKER_CMD)
	$(KIND_LOAD)
	kubectl delete pod zenith-ingestor -n zenith-dev --ignore-not-found
	kubectl apply -f deployments/k8s/local/pod.yaml -n zenith-dev

## Tools: Code generation and Linting
gen: ## Generate code from Protobuf files
	$(BUF_GEN)

lint: ## Lint Protobuf files
	$(BUF_LINT)

tidy: ## Tidy Go modules
	$(GO_TIDY)

help: ## Show this help message
	@grep -hE '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'
