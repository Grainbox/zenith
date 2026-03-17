SHELL := /bin/bash
.SHELLFLAGS := -ec

# Load environment variables
ifneq (,$(wildcard ./.env.secrets))
	include .env.secrets
	export $(shell sed 's/=.*//' .env.secrets)
endif

# Command variables
BUF_GEN     = buf generate
BUF_LINT    = buf lint
GO_TIDY     = go mod tidy
DOCKER_CMD  = docker --context default build -t zenith-ingestor:latest -f build/package/Dockerfile .
KIND_LOAD   = kind load docker-image zenith-ingestor:latest --name zenith-lab

# GCloud Registry (update GCP_PROJECT_ID and GCLOUD_REGION as needed)
GCP_PROJECT_ID  ?= zenith-490409
GCLOUD_REGION   ?= europe-west1
GCLOUD_REPO     ?= zenith
GCLOUD_REGISTRY = $(GCLOUD_REGION)-docker.pkg.dev/$(GCP_PROJECT_ID)/$(GCLOUD_REPO)
GCLOUD_IMAGE    = $(GCLOUD_REGISTRY)/ingestor:latest

# Database Migration string (Security check)
MIGRATE_CMD = migrate -path deployments/db/migrations -database "$(DATABASE_URL)"

.PHONY: all gen lint tidy migrate-up migrate-down build-kind build-docker push-gcloud build-push-gcloud help

all: lint gen tidy ## Run lint, generate code and tidy modules

## Database migrations (requires .env.secrets)
migrate-up: ## Run database migrations up
	@if [ -z "$(DATABASE_URL)" ]; then echo "❌ Error: DATABASE_URL is not defined. Check your .env.secrets"; exit 1; fi
	$(MIGRATE_CMD) up

migrate-down: ## Run database migrations down (1 step)
	$(MIGRATE_CMD) down 1

## Docker builds
build-docker: ## Build Docker image locally
	$(DOCKER_CMD)
	@echo "✅ Image built: zenith-ingestor:latest"

push-gcloud: ## Tag and push image to Google Cloud Artifact Registry
	@if [ -z "$(GCP_PROJECT_ID)" ]; then echo "❌ Error: GCP_PROJECT_ID not set"; exit 1; fi
	docker tag zenith-ingestor:latest $(GCLOUD_IMAGE)
	docker push $(GCLOUD_IMAGE)
	@echo "✅ Image pushed to $(GCLOUD_IMAGE)"

build-push-gcloud: build-docker push-gcloud ## Build and push image to Google Cloud

## Local development (Kind)
build-kind: ## Build Docker image and load into local Kind cluster
	@echo "🔨 Building Docker image..."
	$(DOCKER_CMD)
	@docker inspect zenith-ingestor:latest > /dev/null || (echo "❌ Failed to build image"; exit 1)
	@echo "✅ Image built. Loading into Kind cluster..."
	$(KIND_LOAD) || true
	@echo "✅ Image loaded into Kind cluster"
	kubectl delete pod zenith-ingestor -n zenith-dev --ignore-not-found
	kubectl apply -f deployments/k8s/local/pod.yaml -n zenith-dev
	@echo "✅ Pod deployed to zenith-dev namespace"

## Tools: Code generation and Linting
gen: ## Generate code from Protobuf files
	$(BUF_GEN)

lint: ## Lint Protobuf files
	$(BUF_LINT)

tidy: ## Tidy Go modules
	$(GO_TIDY)

help: ## Show this help message
	@grep -hE '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'
