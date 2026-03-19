# Implementation Plan: Issue-606 — Level 3 Final Validation

**Sprint:** 6 — The Dispatcher & Cloud Deployment
**Goal:** End-to-end smoke test on the live GCP deployment. Trigger the full pipeline (ingest → rule engine → dispatcher → Discord sink → audit log) from a real HTTP request to the public Cloud Run URL.

---

## Architecture Reminder

The full pipeline runs **in a single Cloud Run process** (Ingestor):

```
curl POST /v1/events
        │
        ▼
  Cloud Run: zenith-ingestor-dev  (public, allUsers invoker)
        │
        ├─ Gateway (HTTP handler, validates X-Api-Key)
        ├─ Rule Engine (evaluates payload against JSONB conditions)
        └─ Dispatcher (in-process workers → Discord sink → audit_logs write)
```

The `zenith-dispatcher-dev` Cloud Run service (Issue-605B) is **not** part of this validation — it is infrastructure groundwork for a future broker-based architecture.

---

## No Code Changes Required

Issue-606 is a **validation milestone**, not a feature implementation. All code is in place:

| Component | Status |
|---|---|
| `POST /v1/events` REST gateway | ✅ Implemented (Issue-503) |
| Rule Engine (evaluator, conditions) | ✅ Implemented |
| Dispatcher workers | ✅ Implemented (Issue-601) |
| Discord sink | ✅ Implemented (Issue-602) |
| Audit log write-back | ✅ Implemented (Issue-603) |
| Cloud Run Ingestor service | ✅ Provisioned (Issue-501) |
| CI/CD pipeline (auto-deploy on push) | ✅ Implemented (Issue-502) |

The only action required is: **provision test data in the database** and **send a real HTTP request**.

---

## Pre-flight Checklist

Before running the smoke test, verify the cloud infrastructure is ready:

```powershell
# 1. Confirm CI/CD last run succeeded on main
gh run list --workflow=deploy.yml --limit=3

# 2. Confirm both Cloud Run services are deployed
gcloud run services list --region=europe-west1

# Expected output:
# SERVICE                   REGION         LAST DEPLOYED
# zenith-dispatcher-dev     europe-west1   ...
# zenith-ingestor-dev       europe-west1   ...

# 3. Get the Ingestor's public URL
cd deployments/terraform
terraform init -backend-config="bucket=$env:TF_BACKEND_BUCKET"
terraform output -raw service_url
# Example: https://zenith-ingestor-dev-<hash>-ew.a.run.app

# 4. Verify /healthz responds
$SERVICE_URL = terraform output -raw service_url
curl "$SERVICE_URL/healthz"
# Expected: HTTP 200, body: "OK"
```

---

## Step 1 — Prepare a Discord Webhook

1. In Discord, open a channel (or create one: `#zenith-alerts`).
2. Channel Settings → Integrations → Webhooks → **New Webhook**.
3. Name it `Zenith` and copy the URL: `https://discord.com/api/webhooks/<id>/<token>`
4. Keep this URL — it will be the `target_action` for the rule.

> **Why Discord?** The `DiscordSink` (Issue-602) formats the payload as `{"content": "..."}` with an embed and POSTs to the webhook URL. Discord expects `204 No Content` in response.

---

## Step 2 — Seed Test Data in CockroachDB

Connect to CockroachDB using the same `DATABASE_URL` from `.env.secrets`. CockroachDB is PostgreSQL-compatible, so `psql` works:

```powershell
# Load DATABASE_URL from .env.secrets
Get-Content .env.secrets | ForEach-Object { if ($_ -match "^DATABASE_URL=(.+)$") { $env:DATABASE_URL = $matches[1] } }

# Open a psql session
psql $env:DATABASE_URL
```

In the psql session, run the following SQL:

### 2a. Create the Source

```sql
-- Insert a source with a plain-text API key
-- (api_key is stored as-is and compared directly in GetByAPIKey)
INSERT INTO sources (name, api_key)
VALUES ('zenith-demo', 'demo-api-key-606')
RETURNING id, name, api_key, created_at;
```

> **Important:** Note the `id` returned — you will need it for the rule.

### 2b. Create the Rule

Replace `<source_id>` with the UUID from step 2a, and `<discord_webhook_url>` with the Discord webhook from Step 1:

```sql
INSERT INTO rules (source_id, name, condition, target_action, sink_type, is_active)
VALUES (
  '<source_id>',
  'Demo Alert Rule',
  '{"field": "severity", "operator": "==", "value": "critical"}',
  '<discord_webhook_url>',
  'discord',
  true
)
RETURNING id, name, condition, sink_type, is_active;
```

**How the condition works:**

| DSL Field | Meaning |
|---|---|
| `field` | Key to look up in `event.payload` JSON |
| `operator` | `==`, `!=`, `>`, `>=`, `<`, `<=` |
| `value` | Expected value for match |

This rule matches any event with payload `{"severity": "critical", ...}`. The Rule Engine unmarshals `event.payload` and evaluates `payload["severity"] == "critical"`.

### 2c. Verify the seed data

```sql
-- Confirm source and rule are correctly linked
SELECT
  s.name AS source_name,
  s.api_key,
  r.name AS rule_name,
  r.condition,
  r.sink_type,
  r.target_action,
  r.is_active
FROM rules r
JOIN sources s ON s.id = r.source_id
WHERE s.name = 'zenith-demo';
```

---

## Step 3 — Execute the Smoke Test

Send a POST request to the Ingestor's public URL:

```powershell
$SERVICE_URL = "https://zenith-ingestor-dev-upbbdaxtca-ew.a.run.app/"  # from terraform output

$body = @{
  event_id   = "evt-606-smoke-001"
  event_type = "alert"
  source     = "zenith-demo"
  payload    = @{ severity = "critical"; environment = "production"; region = "eu-west" }
} | ConvertTo-Json -Compress

Invoke-WebRequest `
  -Uri "$SERVICE_URL/v1/events" `
  -Method POST `
  -Headers @{ "X-Api-Key" = "demo-api-key-606"; "Content-Type" = "application/json" } `
  -Body $body

# Expected response: HTTP 202 Accepted
# Body: {"success":true,"message":"Event accepted"}
```

> **What happens after 202:** The response is returned immediately (fire-and-forget). The Rule Engine evaluates the event asynchronously in background goroutines. The Discord notification and audit log write typically complete within 1-2 seconds.

```
StatusCode        : 202
StatusDescription : Accepted
Content           : {"success":true,"message":"Event accepted"}

RawContent        : HTTP/1.1 202 Accepted
                    Date: Thu, 19 Mar 2026 10:22:50 GMT
                    Server: Google
                    Server: Frontend
                    x-cloud-trace-context: 15fe4b0aa16932b0241bbf49110ad4aa/12657968444138346893
                    traceparent: 00-15fe4b0aa169…
Headers           : {[Date, System.String[]], [Server, System.String[]], [x-cloud-trace-context, System.String[]], [traceparent, System.String[]]…}
Images            : {}
InputFields       : {}
Links             : {}
RawContentLength  : 44
RelationLink      : {}
```

---

## Step 4 — Verify Each Stage

### 4a. Discord Notification

Check the `#zenith-alerts` channel (or whichever channel you used for the webhook). You should see a Discord embed with:

- **Title:** Zenith Alert (from `DiscordSink.Send()`)
- **Fields:** Event ID, Source name, Rule Name
- **Color:** Red (alert styling)

### 4b. Cloud Run Logs

```powershell
# Stream the last 50 log lines from the Ingestor
gcloud logging read `
  "resource.type=cloud_run_revision AND resource.labels.service_name=zenith-ingestor-dev" `
  --limit=50 --format="table(timestamp, jsonPayload.msg, jsonPayload.event_id)" `
  --project=$env:GCP_PROJECT_ID

# Expected log sequence:
# ... "Event received via gateway"    event_id=evt-606-smoke-001
# ... "Rules matched"                 matched_count=1
# ... "Event dispatched"              event_id=evt-606-smoke-001
```

```
{
  "insertId": "69bbce7a00019046f97cd63f",
  "jsonPayload": {
    "msg": "Event received via gateway",
    "level": "INFO",
    "source": "zenith-demo",
    "event_id": "evt-606-smoke-001",
    "event_type": "alert"
  },
  "resource": {
    "type": "cloud_run_revision",
    "labels": {
      "revision_name": "zenith-ingestor-dev-00025-w2k",
      "configuration_name": "zenith-ingestor-dev",
      "service_name": "zenith-ingestor-dev",
      "project_id": "zenith-490409",
      "location": "europe-west1"
    }
  },
  "timestamp": "2026-03-19T10:22:50.103848502Z",
  "labels": {
    "instanceId": "00da6cd2c416558944f7e1780f7c3c34a5b3010a6dd39cba91d48967a3c723616ddec1187827c1ec13978cd11120cd30dc89e9bfafc10de6e00e7b4028bcb4dd73e8b372ef7d23b97b8410b045bd"
  },
  "logName": "projects/zenith-490409/logs/run.googleapis.com%2Fstdout",
  "receiveTimestamp": "2026-03-19T10:22:50.108444680Z"
}
{
  "insertId": "69bbce7a00023313faee37d6",
  "jsonPayload": {
    "source": "zenith-demo",
    "event_id": "evt-606-smoke-001",
    "total_rules": 1,
    "level": "INFO",
    "matched_count": 1,
    "msg": "Rules matched"
  },
  "resource": {
    "type": "cloud_run_revision",
    "labels": {
      "revision_name": "zenith-ingestor-dev-00025-w2k",
      "configuration_name": "zenith-ingestor-dev",
      "service_name": "zenith-ingestor-dev",
      "project_id": "zenith-490409",
      "location": "europe-west1"
    }
  },
  "timestamp": "2026-03-19T10:22:50.145516403Z",
  "labels": {
    "instanceId": "00da6cd2c416558944f7e1780f7c3c34a5b3010a6dd39cba91d48967a3c723616ddec1187827c1ec13978cd11120cd30dc89e9bfafc10de6e00e7b4028bcb4dd73e8b372ef7d23b97b8410b045bd"
  },
  "logName": "projects/zenith-490409/logs/run.googleapis.com%2Fstdout",
  "receiveTimestamp": "2026-03-19T10:22:50.441174592Z"
}
{
  "insertId": "69bbce7a0002332458b13172",
  "jsonPayload": {
    "event_id": "evt-606-smoke-001",
    "matched_count": 1,
    "level": "INFO",
    "msg": "Event matched rules",
    "worker_id": 1
  },
  "resource": {
    "type": "cloud_run_revision",
    "labels": {
      "service_name": "zenith-ingestor-dev",
      "project_id": "zenith-490409",
      "location": "europe-west1",
      "revision_name": "zenith-ingestor-dev-00025-w2k",
      "configuration_name": "zenith-ingestor-dev"
    }
  },
  "timestamp": "2026-03-19T10:22:50.145596047Z",
  "labels": {
    "instanceId": "00da6cd2c416558944f7e1780f7c3c34a5b3010a6dd39cba91d48967a3c723616ddec1187827c1ec13978cd11120cd30dc89e9bfafc10de6e00e7b4028bcb4dd73e8b372ef7d23b97b8410b045bd"
  },
  "logName": "projects/zenith-490409/logs/run.googleapis.com%2Fstdout",
  "receiveTimestamp": "2026-03-19T10:22:50.441174592Z"
}
{
  "insertId": "69bbce7a000d2a0e6a6751c6",
  "jsonPayload": {
    "worker_id": 0,
    "level": "INFO",
    "msg": "Event dispatched",
    "rule_id": "c9340d1a-2bd7-4826-9db0-ab619aa23e5f",
    "sink": "discord",
    "event_id": "evt-606-smoke-001"
  },
  "resource": {
    "type": "cloud_run_revision",
    "labels": {
      "revision_name": "zenith-ingestor-dev-00025-w2k",
      "configuration_name": "zenith-ingestor-dev",
      "project_id": "zenith-490409",
      "location": "europe-west1",
      "service_name": "zenith-ingestor-dev"
    }
  },
  "timestamp": "2026-03-19T10:22:50.864228330Z",
  "labels": {
    "instanceId": "00da6cd2c416558944f7e1780f7c3c34a5b3010a6dd39cba91d48967a3c723616ddec1187827c1ec13978cd11120cd30dc89e9bfafc10de6e00e7b4028bcb4dd73e8b372ef7d23b97b8410b045bd"
  },
  "logName": "projects/zenith-490409/logs/run.googleapis.com%2Fstdout",
  "receiveTimestamp": "2026-03-19T10:22:51.105937180Z"
}
```

Alternatively, via Cloud Console:
**GCP Console → Cloud Run → zenith-ingestor-dev → Logs**

### 4c. Audit Log in CockroachDB

```sql
-- Confirm the audit_log row was written
SELECT
  id,
  event_id,
  status,
  processing_latency_ms,
  error_message,
  created_at
FROM audit_logs
WHERE event_id = 'evt-606-smoke-001';
```

Expected result:

| Column | Expected value |
|---|---|
| `status` | `SUCCESS` |
| `processing_latency_ms` | Some positive integer (e.g., 350) |
| `error_message` | `NULL` |
| `created_at` | Timestamp within seconds of the request |

### 4d. Verify the Dispatcher is NOT publicly reachable

```powershell
$DISPATCHER_URL = terraform output -raw dispatcher_url
# Attempt a direct call — should fail with 403 Forbidden
Invoke-WebRequest -Uri "$DISPATCHER_URL/healthz" -Method GET
# Expected: HTTP 403 Forbidden (INGRESS_TRAFFIC_INTERNAL_ONLY blocks unauthenticated external requests)
```

OK

---

## Step 5 — Failure Mode Test (Optional but Recommended)

Validate that **failed dispatches** are also audited. Send an event that matches a rule but with an invalid Discord webhook URL:

```sql
-- Add a second rule with a broken target
INSERT INTO rules (source_id, name, condition, target_action, sink_type, is_active)
SELECT id, 'Broken Sink Rule', '{"field": "severity", "operator": "==", "value": "test-fail"}',
       'https://discord.com/api/webhooks/invalid/url', 'discord', true
FROM sources WHERE name = 'zenith-demo';
```

```powershell
$body = @{
  event_id   = "evt-606-smoke-002"
  event_type = "alert"
  source     = "zenith-demo"
  payload    = @{ severity = "test-fail" }
} | ConvertTo-Json -Compress

Invoke-WebRequest `
  -Uri "$SERVICE_URL/v1/events" `
  -Method POST `
  -Headers @{ "X-Api-Key" = "demo-api-key-606"; "Content-Type" = "application/json" } `
  -Body $body
```

Expected audit_log:

```sql
SELECT status, error_message FROM audit_logs WHERE event_id = 'evt-606-smoke-002';
-- status: FAILED
-- error_message: non-2xx response: 401 (or similar)
```

---

## Success Criteria

| Check | Expected result |
|---|---|
| `GET /healthz` | HTTP 200 `OK` |
| `POST /v1/events` | HTTP 202 `{"success":true}` |
| Discord channel | Embed received within ~2s |
| Cloud Run logs | `"Event dispatched"` log line |
| `audit_logs` table | Row with `status=SUCCESS` and `processing_latency_ms > 0` |
| Dispatcher URL | HTTP 403 (not publicly invokable) |

---

## Cleanup (After Validation)

```sql
-- Remove test data after validation
DELETE FROM sources WHERE name = 'zenith-demo';
-- Cascade delete removes linked rules and sets source_id=NULL in audit_logs
```

---

## Evidence to Capture

Per the Issue-606 deliverables, capture the following as screenshots or terminal output:

1. **`terraform output`** showing both service URLs
2. **`POST /v1/events`** HTTP response (202 + body)
3. **Discord embed** received in the webhook channel
4. **`SELECT * FROM audit_logs`** result showing the `SUCCESS` row
5. **Cloud Run logs** showing the pipeline log sequence

These four artifacts together prove the full end-to-end pipeline is operational on GCP.
