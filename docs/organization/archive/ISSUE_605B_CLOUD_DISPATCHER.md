# Implementation Plan: Issue-605B — Cloud Deployment for Dispatcher Binary

**Sprint:** 6 — The Dispatcher & Cloud Deployment
**Goal:** Build and push the Dispatcher Docker image in CI/CD, and provision a Cloud Run service for it in Terraform, so the full pipeline is represented in the cloud before Issue-606's final validation.

---

## Critical Architectural Note — Read First

Reading `cmd/ingestor/main.go` reveals that the Dispatcher is **not** a separate process on Cloud Run today — it is embedded inside the Ingestor:

```go
// cmd/ingestor/main.go — setupPipeline()
matchCh := make(chan *domain.MatchedEvent, 256)
pipeline.SetDispatcher(matchCh)         // Rule Engine → channel
disp := dispatcher.New(matchCh, ...)    // Dispatcher reads from channel
return pipeline, disp                   // Both started in the same process
```

The Cloud Run Ingestor is therefore a **self-contained monolith**: Ingestor + Rule Engine + Dispatcher all run in the same process, communicating via an in-memory Go channel.

`cmd/dispatcher/main.go` also exists as a standalone binary, but it creates its own **empty** `matchCh` — nothing ever sends matched events to it without a message broker (Pub/Sub, NATS, etc.). In the current architecture, the standalone Dispatcher is intended for the local Kind cluster where in-process wiring is intentional.

### Consequences for this Issue

| Component | Cloud Run effect |
|---|---|
| CI/CD build+push of `dispatcher` image | ✅ Useful — image available in Artifact Registry for future use |
| Cloud Run `dispatcher` service | ⚠️ Provisions correctly, but `matchCh` receives nothing without a broker |
| Issue-606 end-to-end validation | ✅ Fully satisfied by the Ingestor alone (it includes the Dispatcher in-process) |

**Conclusion:** This issue is still worth implementing for two reasons:
1. It is a clean **Terraform + CI/CD exercise** (CKAD scope).
2. Having the image and Cloud Run service ready avoids rework if a broker (GCP Pub/Sub) is introduced in a future phase.

The architectural limitation is not a bug — it is the expected evolution path toward a distributed architecture.

---

## Files to Modify

| Action | File |
|---|---|
| Edit | `.github/workflows/deploy.yml` |
| Edit | `deployments/terraform/cloud_run.tf` |
| Edit | `deployments/terraform/outputs.tf` |

---

## Step 1 — CI/CD: Build & Push the Dispatcher Image

In `.github/workflows/deploy.yml`, extend the `build-push` job to build and push the Dispatcher image **in parallel** with the Ingestor build using a build matrix.

### Current structure (Ingestor only)

```yaml
- name: Build Docker image
  run: |
    docker build \
      -f build/package/Dockerfile \
      --build-arg COMMIT_HASH=${{ env.IMAGE_TAG }} \
      -t .../ingestor:${{ env.IMAGE_TAG }} \
      -t .../ingestor:latest \
      .

- name: Push Docker image
  run: |
    docker push .../ingestor:${{ env.IMAGE_TAG }}
    docker push .../ingestor:latest
```

### Target structure (both images, sequential)

Replace the two steps above with four steps that build and push both images sequentially. A matrix strategy would parallelize more but requires splitting the job, which adds complexity without material benefit here (both images share the same build context and the step is fast).

```yaml
- name: Build Ingestor image
  run: |
    docker build \
      -f build/package/Dockerfile \
      --build-arg COMMIT_HASH=${{ env.IMAGE_TAG }} \
      -t ${{ env.GCP_REGION }}-docker.pkg.dev/${{ secrets.GCP_PROJECT_ID }}/zenith/ingestor:${{ env.IMAGE_TAG }} \
      -t ${{ env.GCP_REGION }}-docker.pkg.dev/${{ secrets.GCP_PROJECT_ID }}/zenith/ingestor:latest \
      .

- name: Build Dispatcher image
  run: |
    docker build \
      -f build/package/Dockerfile.dispatcher \
      --build-arg COMMIT_HASH=${{ env.IMAGE_TAG }} \
      -t ${{ env.GCP_REGION }}-docker.pkg.dev/${{ secrets.GCP_PROJECT_ID }}/zenith/dispatcher:${{ env.IMAGE_TAG }} \
      -t ${{ env.GCP_REGION }}-docker.pkg.dev/${{ secrets.GCP_PROJECT_ID }}/zenith/dispatcher:latest \
      .

- name: Push images
  run: |
    docker push ${{ env.GCP_REGION }}-docker.pkg.dev/${{ secrets.GCP_PROJECT_ID }}/zenith/ingestor:${{ env.IMAGE_TAG }}
    docker push ${{ env.GCP_REGION }}-docker.pkg.dev/${{ secrets.GCP_PROJECT_ID }}/zenith/ingestor:latest
    docker push ${{ env.GCP_REGION }}-docker.pkg.dev/${{ secrets.GCP_PROJECT_ID }}/zenith/dispatcher:${{ env.IMAGE_TAG }}
    docker push ${{ env.GCP_REGION }}-docker.pkg.dev/${{ secrets.GCP_PROJECT_ID }}/zenith/dispatcher:latest
```

**Why consolidate the push step?** The `docker push` calls are independent — grouping them in one step avoids step overhead and keeps the job readable.

---

## Step 2 — Terraform: Cloud Run Service for the Dispatcher

Append to `deployments/terraform/cloud_run.tf`:

```hcl
resource "google_cloud_run_v2_service" "dispatcher" {
  name                = "zenith-dispatcher-${var.environment}"
  location            = var.region
  deletion_protection = false

  # Internal-only: the Dispatcher is a background worker, not a public API.
  # allUsers invoker role must NOT be added.
  ingress = "INGRESS_TRAFFIC_INTERNAL_ONLY"

  client = "terraform"

  template {
    service_account = google_service_account.zenith_runner.email

    scaling {
      min_instance_count = 0  # Scale to zero when idle (no inbound traffic)
      max_instance_count = 1  # One instance is enough for the standalone binary
    }

    containers {
      image = "${var.region}-docker.pkg.dev/${var.project_id}/zenith/dispatcher:${var.image_tag}"

      ports {
        container_port = var.dispatcher_port
      }

      resources {
        limits = {
          cpu    = "1"
          memory = "256Mi"
        }
        cpu_idle = true  # CPU only billed during request processing
      }

      env {
        name = "DATABASE_URL"
        value_source {
          secret_key_ref {
            secret  = google_secret_manager_secret.zenith_secrets["DATABASE_URL"].secret_id
            version = "latest"
          }
        }
      }

      env {
        name = "API_KEY_SALT"
        value_source {
          secret_key_ref {
            secret  = google_secret_manager_secret.zenith_secrets["API_KEY_SALT"].secret_id
            version = "latest"
          }
        }
      }

      startup_probe {
        http_get {
          path = "/healthz"
          port = var.dispatcher_port
        }
        initial_delay_seconds = 5
        period_seconds        = 5
        failure_threshold     = 10
      }

      liveness_probe {
        http_get {
          path = "/healthz"
          port = var.dispatcher_port
        }
        initial_delay_seconds = 10
        period_seconds        = 30
        failure_threshold     = 3
      }
    }
  }

  depends_on = [
    google_project_service.services,
    google_secret_manager_secret_iam_member.zenith_runner_access,
  ]
}
```

### Key decisions

| Decision | Rationale |
|---|---|
| `ingress = "INGRESS_TRAFFIC_INTERNAL_ONLY"` | Dispatcher is not a public endpoint — blocks all internet traffic at LB level |
| `min_instance_count = 0` | Scales to zero when idle; no cost when no events are being dispatched |
| `max_instance_count = 1` | Standalone binary with in-memory channel — multiple instances would each have their own empty channel, no benefit |
| Same `zenith_runner` service account | No new IAM needed — it already has Secret Manager access |
| No `allUsers` invoker | Intentionally omitted — unlike the Ingestor, the Dispatcher must not be publicly invokable |
| `cpu_idle = true` | CPU only billed during active processing — correct for a background worker |
| No `readiness_probe` | Cloud Run v2 does not support `readinessProbe` (Kubernetes concept). Traffic readiness is controlled by `startup_probe` passing. |

---

## Step 3 — Terraform: Add Dispatcher Output

In `deployments/terraform/outputs.tf`, append:

```hcl
output "dispatcher_url" {
  description = "Internal URL of the Dispatcher Cloud Run service"
  value       = google_cloud_run_v2_service.dispatcher.uri
}
```

This is useful for debugging and verifying the service is provisioned, even though it is not publicly reachable.

Also update the existing `service_url` output description for clarity:

```hcl
output "service_url" {
  description = "Public URL of the Ingestor Cloud Run service"  # was: "Public URL of the Cloud Run service"
  value       = google_cloud_run_v2_service.ingestor.uri
}
```

---

## Verification

```bash
# 1. Verify CI/CD builds both images (trigger on push to main, or check last run)
gh run list --workflow=deploy.yml --limit=1

# 2. After terraform apply, both Cloud Run services should appear
gcloud run services list --region=europe-west1

# Expected:
# SERVICE                    REGION         URL                              LAST DEPLOYED
# zenith-ingestor-dev        europe-west1   https://zenith-ingestor-...run.app   ...
# zenith-dispatcher-dev      europe-west1   https://zenith-dispatcher-...run.app  ...

# 3. Verify the Dispatcher is NOT publicly invokable
curl https://zenith-dispatcher-<hash>-ew.a.run.app/healthz
# Expected: HTTP 403 (Forbidden) — IAM blocks unauthenticated access

# 4. Verify the Ingestor's /healthz (public — used by Issue-606)
curl https://zenith-ingestor-<hash>-ew.a.run.app/healthz
# Expected: HTTP 200 OK

# 5. Confirm dispatcher image exists in Artifact Registry
gcloud artifacts docker images list europe-west1-docker.pkg.dev/<PROJECT_ID>/zenith/dispatcher
```

---

## Relationship to Issue-606

Issue-606 requires the full pipeline to work on cloud. Given the in-process architecture:

```
curl POST /v1/events → Cloud Run Ingestor
                              │
                              │  (in-memory channel, same process)
                              ▼
                        Rule Engine → Dispatcher → Discord/HTTP sink
                                            │
                                            ▼
                                      CockroachDB (audit_logs)
```

**The Cloud Run Ingestor already handles the full pipeline.** The Cloud Run Dispatcher provisioned in this issue is infrastructure groundwork for a future broker-based architecture — it does not participate in Issue-606's validation flow.
