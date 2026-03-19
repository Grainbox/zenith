# Implementation Plan: Issue-604 — Multi-Binary Kubernetes Manifests

**Sprint:** 6 — The Dispatcher & Cloud Deployment
**Goal:** Create a Kubernetes Deployment and ClusterIP Service for the Dispatcher, add a ClusterIP Service for the Ingestor, and update the Makefile so both binaries are independently buildable and deployable in the local Kind cluster.

---

## Context & Architecture Overview

After Issue-601/603, the Dispatcher is a fully functional standalone binary. However, no Kubernetes manifests exist for it, and the Ingestor has a Deployment but no Service (it cannot be reached by other pods or the outside world within the cluster).

This issue fills that gap for local development:

```
zenith-dev namespace
┌──────────────────────────────────────────────────────────────┐
│                                                              │
│  ┌──────────────────────────────┐                           │
│  │  Deployment: zenith-ingestor │   ◄── already exists       │
│  │  (3 replicas, port 8080)    │                             │
│  └──────────────────────────────┘                           │
│  ┌──────────────────────────────┐                           │
│  │  Service: zenith-ingestor   │   ◄── NEW (ClusterIP)      │
│  │  ClusterIP :8080            │      (LoadBalancer in 605)  │
│  └──────────────────────────────┘                           │
│                                                              │
│  ┌──────────────────────────────┐                           │
│  │  Deployment: zenith-dispatcher │ ◄── NEW                  │
│  │  (2 replicas, port 8081)    │                             │
│  └──────────────────────────────┘                           │
│  ┌──────────────────────────────┐                           │
│  │  Service: zenith-dispatcher │   ◄── NEW (ClusterIP)      │
│  │  ClusterIP :8081            │                             │
│  └──────────────────────────────┘                           │
│                                                              │
│  ConfigMap: zenith-config        ◄── shared, unchanged       │
│  Secret: zenith-secrets          ◄── shared, unchanged       │
│  Secret: zenith-ca-cert          ◄── shared, unchanged       │
└──────────────────────────────────────────────────────────────┘
```

**Design principles:**
- Both binaries share the same `zenith-secrets` (DATABASE_URL, API_KEY_SALT) and `zenith-config` ConfigMap — no new secrets needed.
- The Dispatcher is internal-only: `ClusterIP` is sufficient; it does not expose a public port. The LoadBalancer for the Ingestor is Issue-605.
- Each binary gets its own `Dockerfile` to keep build contexts isolated and independent.
- The Makefile is updated so each binary can be built/loaded/deployed independently or together.

---

## Files to Create or Modify

### New files

| File | Purpose |
|---|---|
| `build/package/Dockerfile.dispatcher` | Builds the `zenith-dispatcher` binary |
| `deployments/k8s/local/dispatcher-deployment.yaml` | Dispatcher Kubernetes Deployment (2 replicas) |
| `deployments/k8s/local/dispatcher-service.yaml` | Dispatcher ClusterIP Service (port 8081) |
| `deployments/k8s/local/ingestor-service.yaml` | Ingestor ClusterIP Service (port 8080) — foundation for Issue-605 LoadBalancer |

### Modified files

| File | Change |
|---|---|
| `Makefile` | Add dispatcher build/load targets; update `build-kind` to deploy all manifests |

---

## Step-by-Step Implementation

### Step 1 — Dispatcher Dockerfile

**File:** `build/package/Dockerfile.dispatcher`

Mirrors the existing `Dockerfile` exactly, changing only the binary name and entry point.

```dockerfile
FROM golang:1.26-alpine AS builder

RUN apk --no-network add ca-certificates

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

RUN go install github.com/bufbuild/buf/cmd/buf@latest

COPY . .

RUN $GOPATH/bin/buf generate

ARG COMMIT_HASH=dev
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s -X main.commit=${COMMIT_HASH}" \
    -o /zenith-dispatcher cmd/dispatcher/main.go

FROM gcr.io/distroless/static-debian12
COPY --from=builder /zenith-dispatcher /zenith-dispatcher

EXPOSE 8081

CMD ["/zenith-dispatcher"]
```

**Why a separate Dockerfile?** Keeps build targets and image sizes independent. The Ingestor and Dispatcher have different entry points and exposed ports; a single parameterized Dockerfile would add complexity for minimal gain at this stage.

---

### Step 2 — Dispatcher Deployment

**File:** `deployments/k8s/local/dispatcher-deployment.yaml`

The Dispatcher is an internal worker (no external gRPC traffic), so:
- **2 replicas** instead of 3 (lighter load, still HA).
- **Smaller resource footprint** than the Ingestor.
- All probes hit `/healthz` on port 8081 (already implemented in `cmd/dispatcher/main.go`).
- Same `zenith-secrets` + `zenith-config` + `ca-cert` volume as the Ingestor.

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: zenith-dispatcher-deployment
  labels:
    app: zenith-dispatcher
spec:
  replicas: 2
  selector:
    matchLabels:
      app: zenith-dispatcher
  template:
    metadata:
      labels:
        app: zenith-dispatcher
    spec:
      containers:
      - name: zenith-dispatcher
        image: zenith-dispatcher:latest
        imagePullPolicy: IfNotPresent
        ports:
        - containerPort: 8081
        envFrom:
        - configMapRef:
            name: zenith-config
        - secretRef:
            name: zenith-secrets
        volumeMounts:
        - name: ca-cert
          mountPath: "/root/.postgresql"
          readOnly: true

        resources:
          requests:
            cpu: "50m"
            memory: "128Mi"
          limits:
            cpu: "500m"
            memory: "256Mi"

        # Startup probe: allow time for DB connection on cold start
        startupProbe:
          httpGet:
            path: /healthz
            port: 8081
          initialDelaySeconds: 5
          periodSeconds: 5
          timeoutSeconds: 5
          failureThreshold: 10

        # Readiness probe: only route traffic to ready pods
        readinessProbe:
          httpGet:
            path: /healthz
            port: 8081
          initialDelaySeconds: 3
          periodSeconds: 10
          timeoutSeconds: 5
          failureThreshold: 2

        # Liveness probe: restart unresponsive pods
        livenessProbe:
          httpGet:
            path: /healthz
            port: 8081
          initialDelaySeconds: 10
          periodSeconds: 30
          timeoutSeconds: 5
          failureThreshold: 3

      volumes:
      - name: ca-cert
        secret:
          secretName: zenith-ca-cert

  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxUnavailable: 1
```

**Note:** Probes are included here because the dispatcher is a new manifest being created from scratch — it would be inconsistent to create it without probes when the ingestor already has them. Issue-605 will verify and finalize probe tuning for both services.

---

### Step 3 — Dispatcher Service

**File:** `deployments/k8s/local/dispatcher-service.yaml`

ClusterIP only — the Dispatcher is not externally reachable. Other pods in `zenith-dev` can reach it via `zenith-dispatcher:8081`.

```yaml
apiVersion: v1
kind: Service
metadata:
  name: zenith-dispatcher
  labels:
    app: zenith-dispatcher
spec:
  type: ClusterIP
  selector:
    app: zenith-dispatcher
  ports:
  - name: http
    port: 8081
    targetPort: 8081
    protocol: TCP
```

---

### Step 4 — Ingestor Service

**File:** `deployments/k8s/local/ingestor-service.yaml`

The Ingestor Deployment has existed since Issue-504 but has no Service. Without a Service, the Ingestor pods cannot be reached by other pods via a stable DNS name, and there is no path to expose it externally. This ClusterIP Service is the foundation that Issue-605 will promote to LoadBalancer.

The Ingestor exposes two ports on 8080: the gRPC (h2c) server and the REST gateway — both served from the same listener.

```yaml
apiVersion: v1
kind: Service
metadata:
  name: zenith-ingestor
  labels:
    app: zenith-ingestor
spec:
  type: ClusterIP
  selector:
    app: zenith-ingestor
  ports:
  - name: grpc-http
    port: 8080
    targetPort: 8080
    protocol: TCP
```

---

### Step 5 — Makefile Updates

The Makefile currently only handles the Ingestor in `build-kind`. Add dispatcher-specific targets and update `build-kind` to orchestrate a full local deployment.

```makefile
# Dispatcher image variables (append below existing ingestor variables)
DOCKER_CMD_DISPATCHER = docker --context default build -t zenith-dispatcher:latest -f build/package/Dockerfile.dispatcher .
KIND_LOAD_DISPATCHER  = kind load docker-image zenith-dispatcher:latest --name zenith-lab

## Local development (Kind) — Dispatcher
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

## Local development (Kind) — Full stack
build-kind-all: build-kind build-kind-dispatcher ## Build and deploy all binaries to local Kind cluster
	kubectl apply -f deployments/k8s/local/ingestor-service.yaml -n zenith-dev
	@echo "✅ Full stack deployed to zenith-dev namespace"
```

Also update the existing `build-kind` target to apply the ingestor Service:

```makefile
# In the existing build-kind target, add after the ingestor-deployment line:
kubectl apply -f deployments/k8s/local/ingestor-service.yaml -n zenith-dev
```

The updated `build-kind` target in full:

```makefile
build-kind: ## Build Ingestor image and load into local Kind cluster
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
	@echo "✅ Deployment applied to zenith-dev namespace"
	@echo "🕐 Waiting for deployment to be ready..."
	kubectl rollout status deployment/zenith-ingestor-deployment -n zenith-dev --timeout=120s
```

Also update `.PHONY` to include the new targets:

```makefile
.PHONY: all gen lint tidy migrate-up migrate-down build-kind build-kind-dispatcher build-kind-all build-docker push-gcloud build-push-gcloud help
```

---

## Verification

```bash
# 1. Build and deploy Ingestor (existing target, now with Service)
make build-kind

# 2. Build and deploy Dispatcher
make build-kind-dispatcher

# 3. Confirm both Deployments are running
kubectl get deployments -n zenith-dev
# Expected:
# NAME                           READY   UP-TO-DATE   AVAILABLE
# zenith-ingestor-deployment     3/3     3            3
# zenith-dispatcher-deployment   2/2     2            2

# 4. Confirm Services are created
kubectl get services -n zenith-dev
# Expected:
# NAME               TYPE        CLUSTER-IP      PORT(S)
# zenith-ingestor    ClusterIP   10.96.x.x       8080/TCP
# zenith-dispatcher  ClusterIP   10.96.x.x       8081/TCP

# 5. Verify Ingestor healthz endpoint (via port-forward)
kubectl port-forward svc/zenith-ingestor 8080:8080 -n zenith-dev
curl http://localhost:8080/healthz
# Expected: 200 OK

# 6. Verify Dispatcher healthz endpoint (via port-forward)
kubectl port-forward svc/zenith-dispatcher 8081:8081 -n zenith-dev
curl http://localhost:8081/healthz
# Expected: 200 OK

# 7. Verify Dispatcher status endpoint
curl http://localhost:8081/status
# Expected: {"status":"online","component":"dispatcher","commit":"dev"}

# 8. Verify Dispatcher pod logs (confirm DB connection and startup)
kubectl logs -l app=zenith-dispatcher -n zenith-dev --tail=20

# 9. Independent rollout test (proves independent deployability)
kubectl rollout restart deployment/zenith-dispatcher-deployment -n zenith-dev
kubectl rollout status deployment/zenith-dispatcher-deployment -n zenith-dev
```

---

## Notes for Issue-605

Issue-605 will build directly on this issue:
- Change `zenith-ingestor` Service from `ClusterIP` to `LoadBalancer` to expose the gRPC + REST gateway externally.
- Verify and tune liveness/readiness probe thresholds for both Deployments (currently mirroring ingestor values; may be adjusted based on observed startup times).
- The Dispatcher Service remains `ClusterIP` — it has no public-facing API.
