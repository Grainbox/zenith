# Implementation Plan: Issue-605 — Services & Liveness/Readiness Probes

**Sprint:** 6 — The Dispatcher & Cloud Deployment
**Goal:** Promote the Ingestor Service from ClusterIP to LoadBalancer for external reachability, and confirm that all health probes are correctly configured in both Deployments for zero-downtime rolling updates.

---

## Context & State After Issue-604

Issue-604 deliberately established the full K8s foundation — including probes — as a prerequisite for this issue. Here is what is already in place and what remains:

| Artifact | Status |
|---|---|
| `ingestor-deployment.yml` — startupProbe, readinessProbe, livenessProbe | ✅ Done in 604 |
| `dispatcher-deployment.yaml` — startupProbe, readinessProbe, livenessProbe | ✅ Done in 604 |
| `dispatcher-service.yaml` — ClusterIP :8081 (internal, correct) | ✅ Done in 604 |
| `ingestor-service.yaml` — ClusterIP :8080 (needs promotion) | 🔄 This issue |

The **single file change** for Issue-605 is promoting `ingestor-service.yaml` from `type: ClusterIP` to `type: LoadBalancer`.

---

## Target Architecture

```
Internet / kubectl port-forward
          │
          ▼
┌─────────────────────────────────────────────────────────────────────┐
│ zenith-dev namespace                                                │
│                                                                     │
│  ┌──────────────────────────────────┐                               │
│  │  Service: zenith-ingestor        │  ◄── type: LoadBalancer       │
│  │  ExternalIP :8080 → pods :8080   │      (NodePort in Kind)       │
│  └──────────────────────────────────┘                               │
│          │  selector: app=zenith-ingestor                           │
│          ▼                                                          │
│  ┌───────────────────────────────────┐                              │
│  │  Deployment: zenith-ingestor      │  3 replicas                  │
│  │  port 8080 (gRPC h2c + REST)      │  startupProbe ✅             │
│  │                                   │  readinessProbe ✅           │
│  │                                   │  livenessProbe ✅            │
│  └───────────────────────────────────┘                              │
│                                                                     │
│  ┌──────────────────────────────────┐                               │
│  │  Service: zenith-dispatcher      │  ◄── type: ClusterIP          │
│  │  ClusterIP :8081 (internal only) │      (no external access)     │
│  └──────────────────────────────────┘                               │
│          │  selector: app=zenith-dispatcher                         │
│          ▼                                                          │
│  ┌───────────────────────────────────┐                              │
│  │  Deployment: zenith-dispatcher    │  2 replicas                  │
│  │  port 8081 (/healthz + internal)  │  startupProbe ✅             │
│  │                                   │  readinessProbe ✅           │
│  │                                   │  livenessProbe ✅            │
│  └───────────────────────────────────┘                              │
│                                                                     │
│  ConfigMap: zenith-config   Secret: zenith-secrets   zenith-ca-cert │
└─────────────────────────────────────────────────────────────────────┘
```

---

## CKAD Concept Review

### Service Types

| Type | Reachability | Use Case |
|---|---|---|
| `ClusterIP` | Cluster-internal only | Inter-pod communication (Dispatcher) |
| `NodePort` | Node IP + static port 30000-32767 | Local dev without a cloud LB |
| `LoadBalancer` | External IP via cloud provider | Production internet-facing endpoints |
| `ExternalName` | DNS alias to external FQDN | Accessing external services by name |

**Why ClusterIP for Dispatcher:** The Dispatcher is a background worker — it consumes from an internal channel and writes audit logs. It exposes `/healthz` only for the Kubelet probes; nothing outside the cluster should call it directly.

**Why LoadBalancer for Ingestor:** The Ingestor is the public-facing entry point. It must receive event streams from external clients (gRPC producers, webhook senders). A `LoadBalancer` Service provisions an external IP via the cloud provider's LB (GKE, EKS) or, locally, via `cloud-provider-kind` / `metallb`.

### Kind Limitation

`type: LoadBalancer` Services in Kind remain in `<pending>` ExternalIP status by default — Kind has no built-in LB controller. The two practical workarounds are:

1. **`kubectl port-forward`** (recommended for local dev): zero config, ephemeral.
2. **`cloud-provider-kind`**: Google's official Kind cloud provider that allocates real local IPs. Requires one `cloud-provider-kind` process running in the background.

The manifest is written correctly for production (GKE). For local testing, `port-forward` is sufficient.

### Probe Semantics

```
Pod starts
    │
    ▼  startupProbe polling (initialDelaySeconds + periodSeconds × failureThreshold)
    │  → if fails: pod killed and restarted (restartPolicy)
    │  → if succeeds: kubelet switches to liveness + readiness
    ▼
 Running
    │
    ├──▶ livenessProbe  → failure: pod RESTARTED (OOM, deadlock, hung goroutines)
    │
    └──▶ readinessProbe → failure: pod REMOVED from Service endpoints (stops receiving traffic)
                          → used to gate traffic during rolling updates
```

**Zero-downtime rolling update flow:**
1. New pod starts → `startupProbe` must pass before anything else.
2. `readinessProbe` must pass → pod added to Service endpoint list.
3. Old pod drained → `readinessProbe` failure removes it from endpoints.
4. Only then is the old pod terminated (respects `terminationGracePeriodSeconds`).

Without `readinessProbe`, the Service would route traffic to a new pod the instant it starts, before the app is ready — causing 5xx errors during every deploy.

---

## Files to Change

| Action | File |
|---|---|
| Edit | `deployments/k8s/local/ingestor-service.yaml` |

No other files need modification. The Makefile's `build-kind` target already applies `ingestor-service.yaml`.

---

## Step 1 — Promote the Ingestor Service to LoadBalancer

Change `type: ClusterIP` → `type: LoadBalancer` in `ingestor-service.yaml`.

**Before:**
```yaml
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

**After:**
```yaml
spec:
  type: LoadBalancer
  selector:
    app: zenith-ingestor
  ports:
  - name: grpc-http
    port: 8080
    targetPort: 8080
    protocol: TCP
```

**Why no `nodePort` field?** Kubernetes auto-assigns a NodePort when `type: LoadBalancer` (or `NodePort`) is used and `nodePort` is omitted. Letting K8s assign it avoids manual port conflict management.

---

## Step 2 — Verify Probe Configuration (Audit)

Both Deployments were configured with probes during Issue-604. Confirm the settings match the intended semantics:

### Ingestor (`ingestor-deployment.yml`)

| Probe | Path | Timing | Threshold | Rationale |
|---|---|---|---|---|
| startupProbe | `/healthz` :8080 | delay=5s, period=5s | failureThreshold=10 → 55s budget | DB connection + schema check on boot |
| readinessProbe | `/healthz` :8080 | delay=3s, period=10s | failureThreshold=2 | Fast demotion from endpoints during issues |
| livenessProbe | `/healthz` :8080 | delay=10s, period=30s | failureThreshold=3 | Conservative — restart only after 90s of failure |

### Dispatcher (`dispatcher-deployment.yaml`)

Same thresholds as Ingestor — appropriate since both apps have the same startup pattern (DB connection, configuration load).

Both `/healthz` handlers are already implemented in `cmd/ingestor/main.go:170` and `cmd/dispatcher/main.go:130` respectively. No code changes needed.

---

## Verification

```bash
# Apply the updated Service (or redeploy via make)
kubectl apply -f deployments/k8s/local/ingestor-service.yaml -n zenith-dev

# Verify Service type changed to LoadBalancer
kubectl get svc -n zenith-dev

# Expected output (ExternalIP stays <pending> in Kind — this is normal):
# NAME                 TYPE           CLUSTER-IP      EXTERNAL-IP   PORT(S)          AGE
# zenith-ingestor      LoadBalancer   10.96.x.x       <pending>     8080:3xxxx/TCP   Xs
# zenith-dispatcher    ClusterIP      10.96.x.x       <none>        8081/TCP         Xs

# Access ingestor locally via port-forward (workaround for Kind)
kubectl port-forward svc/zenith-ingestor 8080:8080 -n zenith-dev

# Verify health endpoint responds
curl http://localhost:8080/healthz
# Expected: HTTP 200

# Verify probes are recognized by Kubernetes
kubectl describe pod -l app=zenith-ingestor -n zenith-dev | grep -A5 "Liveness\|Readiness\|Startup"
kubectl describe pod -l app=zenith-dispatcher -n zenith-dev | grep -A5 "Liveness\|Readiness\|Startup"

# Observe zero-downtime rolling update:
# 1. Trigger a rollout restart
kubectl rollout restart deployment/zenith-ingestor-deployment -n zenith-dev
# 2. Watch pods cycle — old pod terminates only after new pod passes readiness
kubectl get pods -n zenith-dev -w
# 3. Confirm no downtime in rollout status
kubectl rollout status deployment/zenith-ingestor-deployment -n zenith-dev
```

---

## Relationship to Issue-606

Issue-606 (Final Validation) requires the Ingestor to be publicly reachable at a cloud URL. In GKE, `type: LoadBalancer` provisions a real external IP automatically — no manifest change needed between local and cloud. The same `ingestor-service.yaml` works in both environments; the cloud provider's controller handles IP allocation.

For production (GKE), the flow is:
```
External IP (GCP L4 LB) → NodePort on GKE node → ClusterIP → Ingestor pods
```
