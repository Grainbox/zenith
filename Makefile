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
DOCKER_CMD_DISPATCHER = docker --context default build -t zenith-dispatcher:latest -f build/package/Dockerfile.dispatcher .
KIND_LOAD_DISPATCHER  = kind load docker-image zenith-dispatcher:latest --name zenith-lab

# GCloud Registry (update GCP_PROJECT_ID and GCLOUD_REGION as needed)
GCP_PROJECT_ID  ?= zenith-490409
GCLOUD_REGION   ?= europe-west1
GCLOUD_REPO     ?= zenith
GCLOUD_REGISTRY = $(GCLOUD_REGION)-docker.pkg.dev/$(GCP_PROJECT_ID)/$(GCLOUD_REPO)
GCLOUD_IMAGE    = $(GCLOUD_REGISTRY)/ingestor:latest

# Database Migration — uses cockroachdb:// driver to avoid pg_advisory_lock() incompatibility
MIGRATE_URL = $(subst postgresql://,cockroachdb://,$(subst postgres://,cockroachdb://,$(DATABASE_URL)))
MIGRATE_CMD = migrate -path deployments/db/migrations -database "$(MIGRATE_URL)"

.PHONY: all gen lint tidy migrate-up migrate-down build-kind build-kind-dispatcher build-kind-all build-docker push-gcloud build-push-gcloud install-metrics-server help

all: lint gen tidy ## Run lint, generate code and tidy modules

## Database migrations (requires .env.secrets)
migrate-up: ## Run database migrations up
	@if [ -z "$(DATABASE_URL)" ]; then echo "❌ Error: DATABASE_URL is not defined. Check your .env.secrets"; exit 1; fi
	$(MIGRATE_CMD) up

migrate-down: ## Run database migrations down (1 step)
	$(MIGRATE_CMD) down 1

## Kubernetes setup
install-metrics-server: ## Install metrics-server in Kind cluster (run once after cluster creation)
	kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml
	kubectl patch deployment metrics-server -n kube-system --type='json' -p='[{"op":"add","path":"/spec/template/spec/containers/0/args/-","value":"--kubelet-insecure-tls"}]'
	kubectl rollout status deployment/metrics-server -n kube-system --timeout=60s
	@echo "✅ metrics-server ready"

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
	@echo "✅ Image loaded into Kind cluster. Deploying to K8s..."
	kubectl apply -f deployments/k8s/local/namespace.yaml
	kubectl apply -f deployments/k8s/local/config.yaml -n zenith-dev
	kubectl apply -f deployments/k8s/local/secrets.yaml -n zenith-dev
	kubectl apply -f deployments/k8s/local/ingestor-deployment.yml -n zenith-dev
	kubectl apply -f deployments/k8s/local/ingestor-service.yaml -n zenith-dev
	kubectl apply -f deployments/k8s/local/ingestor-hpa.yaml -n zenith-dev
	@echo "✅ Deployment and HPA applied to zenith-dev namespace"
	@echo "🕐 Waiting for deployment to be ready..."
	kubectl rollout status deployment/zenith-ingestor-deployment -n zenith-dev --timeout=120s

build-kind-dispatcher: ## Build Dispatcher image and load into local Kind cluster
	@echo "🔨 Building Dispatcher Docker image..."
	$(DOCKER_CMD_DISPATCHER)
	@docker inspect zenith-dispatcher:latest > /dev/null || (echo "❌ Failed to build dispatcher image"; exit 1)
	@echo "✅ Dispatcher image built. Loading into Kind cluster..."
	$(KIND_LOAD_DISPATCHER) || true
	@echo "✅ Dispatcher image loaded. Deploying to K8s..."
	kubectl apply -f deployments/k8s/local/dispatcher-deployment.yaml -n zenith-dev
	kubectl apply -f deployments/k8s/local/dispatcher-service.yaml -n zenith-dev
	@echo "✅ Dispatcher deployment applied to zenith-dev namespace"
	kubectl rollout status deployment/zenith-dispatcher-deployment -n zenith-dev --timeout=120s

build-kind-all: build-kind build-kind-dispatcher ## Build and deploy all binaries to local Kind cluster
	@echo "✅ Full stack deployed to zenith-dev namespace"

## Tools: Code generation and Linting
gen: ## Generate code from Protobuf files
	$(BUF_GEN)

lint: ## Lint Protobuf files
	$(BUF_LINT)

tidy: ## Tidy Go modules
	$(GO_TIDY)

help: ## Show this help message
	@grep -hE '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'
