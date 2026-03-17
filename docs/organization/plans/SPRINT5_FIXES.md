# Plan d'Implémentation — Sprint 5 Fixes (All Issues)

**Objectif** : Corriger les 3 problèmes critiques + 4 problèmes recommandés identifiés dans SPRINT5_REVIEW.md

**Temps estimé total** : 3-4h
**Complexité** : Modérée (modifications de config, code, manifests)

---

## 📊 Vue d'Ensemble

### Issues à Traiter

| Priorité | Issue | Problème | Effort | Status |
|----------|-------|---------|--------|--------|
| 🔴 CRIT | 502 | Secrets dans variables Terraform | 1h | [ ] |
| 🔴 CRIT | 504 | Missing readinessProbe | 30min | [ ] |
| 🔴 CRIT | 504 | No resource requests/limits | 30min | [ ] |
| 🟠 REC | 502 | Duplicate "Set up buf" | 15min | [ ] |
| 🟠 REC | 503 | Logger package level | 15min | [ ] |
| 🟠 REC | 503 | Dead code (payload == nil) | 10min | [ ] |
| 🟠 REC | 504 | imagePullPolicy | 10min | [ ] |

**Total** : ~2h45min (critiques) + 50min (recommandés) = ~3h45min

---

## 🔴 CRITIQUE #1 : Issue-502 — Supprimer Secrets des Variables Terraform

### Étape 1.1 : Modifier `deployments/terraform/variables.tf`

**Fichier** : `deployments/terraform/variables.tf`

**Action** : Supprimer les trois variables sensibles

```hcl
# ❌ SUPPRIMER ces trois sections:

variable "database_url" {
  description = "CockroachDB connection string (managed as GCP Secret, optional for terraform plan)"
  type        = string
  sensitive   = true
  default     = ""
}

variable "api_key_salt" {
  description = "Salt for API key generation"
  type        = string
  sensitive   = true
  default     = ""
}

variable "slack_webhook_url" {
  description = "Slack webhook URL for notifications"
  type        = string
  sensitive   = true
  default     = ""
}
```

**Résultat** : Le fichier ne doit contenir que :
- `project_id` (string)
- `region` (string, default)
- `environment` (string, default)
- `image_tag` (string, default)
- `port` (number, default)
- `db_max_open_conns` (number, default)
- `db_max_idle_conns` (number, default)

### Étape 1.2 : Modifier `.github/workflows/deploy.yml` (ligne 166)

**Fichier** : `.github/workflows/deploy.yml`

**Action** : Enlever les `-var` pour les secrets

**Avant** :
```yaml
- name: Terraform Plan
  run: |
    terraform plan \
      -var="project_id=${{ secrets.GCP_PROJECT_ID }}" \
      -var="image_tag=${{ env.IMAGE_TAG }}" \
      -var="database_url=${{ secrets.DATABASE_URL }}" \
      -var="api_key_salt=${{ secrets.API_KEY_SALT }}" \
      -var="slack_webhook_url=${{ secrets.SLACK_WEBHOOK_URL }}" \
      -out=tfplan -no-color -lock=false -input=false

- name: Terraform Apply
  run: terraform apply -auto-approve -lock=false tfplan
```

**Après** :
```yaml
- name: Terraform Plan
  run: |
    terraform plan \
      -var="project_id=${{ secrets.GCP_PROJECT_ID }}" \
      -var="image_tag=${{ env.IMAGE_TAG }}" \
      -out=tfplan -no-color -lock=false -input=false

- name: Terraform Apply
  run: terraform apply -auto-approve -lock=false tfplan
```

**Différences** :
- ✅ Garder : `-var="project_id=..."` et `-var="image_tag=..."`
- ❌ Supprimer : `-var="database_url=..."`, `-var="api_key_salt=..."`, `-var="slack_webhook_url=..."`

### Étape 1.3 : Vérifier que `cloud_run.tf` lit depuis Secret Manager

**Fichier** : `deployments/terraform/cloud_run.tf` (ligne ~45-73)

**Vérification** : Les secrets DOIVENT être injectés via `secret_key_ref`, pas via variables

```hcl
# ✅ DOIT RESSEMBLER À:
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

env {
  name = "SLACK_WEBHOOK_URL"
  value_source {
    secret_key_ref {
      secret  = google_secret_manager_secret.zenith_secrets["SLACK_WEBHOOK_URL"].secret_id
      version = "latest"
    }
  }
}
```

Si ce n'est pas là, l'ajouter. Sinon, c'est ✅ bon.

### Étape 1.4 : Vérifier `secrets.tf`

**Fichier** : `deployments/terraform/secrets.tf`

**Vérification** : Les ressources `google_secret_manager_secret` DOIVENT exister

```hcl
# ✅ DOIT EXISTER:
resource "google_secret_manager_secret" "zenith_secrets" {
  for_each  = toset(["DATABASE_URL", "API_KEY_SALT", "SLACK_WEBHOOK_URL"])
  secret_id = "zenith-${lower(replace(each.value, "_", "-"))}-${var.environment}"
  # ...
}
```

### Étape 1.5 : Validation Locale

```bash
cd deployments/terraform

# Vérifier la syntaxe
terraform fmt -recursive
terraform validate

# Vérifier le plan (sans appliquer)
terraform plan -var="project_id=my-project" -var="image_tag=test" -out=tfplan

# ✅ Correct si aucune erreur sur secrets manquants
```

### Étape 1.6 : Documenter la procédure manuelle de chargement des secrets

Ajouter dans `CLAUDE.md` (section Configuration) :

```markdown
### Chargement des Secrets GCP (hors Terraform)

Les secrets sensibles (`DATABASE_URL`, `API_KEY_SALT`, `SLACK_WEBHOOK_URL`) sont managés via
GCP Secret Manager, **pas Terraform**. Les charger manuellement une seule fois :

\`\`\`bash
# Variables
PROJECT_ID="my-gcp-project"
ENVIRONMENT="dev"

# Database URL
echo -n "postgresql://user:pass@host:26257/db?..." | gcloud secrets versions add \
  zenith-database-url-${ENVIRONMENT} --data-file=-

# API Key Salt
echo -n "your-random-salt-here" | gcloud secrets versions add \
  zenith-api-key-salt-${ENVIRONMENT} --data-file=-

# Slack Webhook
echo -n "https://hooks.slack.com/services/..." | gcloud secrets versions add \
  zenith-slack-webhook-url-${ENVIRONMENT} --data-file=-

# Vérifier
gcloud secrets versions list zenith-database-url-${ENVIRONMENT}
\`\`\`

**Important** : Cette étape est **manuelle et unique**. Terraform read-only les valeurs via Secret Manager.
```

---

## 🔴 CRITIQUE #2 : Issue-504 — Ajouter Readiness Probe

### Étape 2.1 : Modifier `deployments/k8s/local/ingestor-deployment.yml`

**Fichier** : `deployments/k8s/local/ingestor-deployment.yml`

**Action** : Ajouter `readinessProbe` après `startup_probe` (ou avant `livenessProbe`)

**Avant** (lignes 36-40) :
```yaml
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxUnavailable: 1
```

**Après** (version complète) :
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: zenith-ingestor-deployment
  labels:
    app: zenith-ingestor
spec:
  replicas: 3
  selector:
    matchLabels:
      app: zenith-ingestor
  template:
    metadata:
      labels:
        app: zenith-ingestor
    spec:
      containers:
      - name: zenith-ingestor
        image: zenith-ingestor:latest
        imagePullPolicy: IfNotPresent
        ports:
        - containerPort: 8080
        envFrom:
        - configMapRef:
            name: zenith-config
        - secretRef:
            name: zenith-secrets
        volumeMounts:
        - name: ca-cert
          mountPath: "/root/.postgresql"
          readOnly: true

        # ✅ STARTUP PROBE (bootstrap time)
        startupProbe:
          httpGet:
            path: /healthz
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
          timeoutSeconds: 5
          failureThreshold: 10

        # ✅ NEW: READINESS PROBE (traffic routing)
        readinessProbe:
          httpGet:
            path: /healthz
            port: 8080
          initialDelaySeconds: 3
          periodSeconds: 10
          timeoutSeconds: 5
          failureThreshold: 2

        # ✅ LIVENESS PROBE (restart if dead)
        livenessProbe:
          httpGet:
            path: /healthz
            port: 8080
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

### Étape 2.2 : Validation K8s

```bash
# Valider la syntaxe YAML
kubectl apply -f deployments/k8s/local/ingestor-deployment.yml --dry-run=client

# ✅ Correct si "Deployment ingestor-deployment validated"
```

---

## 🔴 CRITIQUE #3 : Issue-504 — Ajouter Resource Requests & Limits

### Étape 3.1 : Modifier `deployments/k8s/local/ingestor-deployment.yml`

**Action** : Ajouter section `resources:` au container spec

**Localisation** : Après `volumeMounts` (avant probes)

**Ajouter** (après la ligne `volumeMounts:`) :

```yaml
        # ✅ NEW: RESOURCE MANAGEMENT
        resources:
          requests:
            cpu: "100m"          # Reservation pour le scheduler (0.1 CPU)
            memory: "256Mi"      # Reservation (256 MiB)
          limits:
            cpu: "1000m"         # Hard limit (throttling après)
            memory: "512Mi"      # OOM killed si dépassé

        startupProbe:
```

### Étape 3.2 : Version Complète (après Étape 2.1)

Après les Étapes 2.1 et 3.1, le fichier doit ressembler à :

```yaml
spec:
  template:
    spec:
      containers:
      - name: zenith-ingestor
        image: zenith-ingestor:latest
        imagePullPolicy: IfNotPresent
        ports:
        - containerPort: 8080
        envFrom:
        - configMapRef:
            name: zenith-config
        - secretRef:
            name: zenith-secrets
        volumeMounts:
        - name: ca-cert
          mountPath: "/root/.postgresql"
          readOnly: true

        # ✅ RESOURCE MANAGEMENT
        resources:
          requests:
            cpu: "100m"
            memory: "256Mi"
          limits:
            cpu: "1000m"
            memory: "512Mi"

        # ✅ STARTUP PROBE
        startupProbe:
          httpGet:
            path: /healthz
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
          timeoutSeconds: 5
          failureThreshold: 10

        # ✅ READINESS PROBE
        readinessProbe:
          httpGet:
            path: /healthz
            port: 8080
          initialDelaySeconds: 3
          periodSeconds: 10
          timeoutSeconds: 5
          failureThreshold: 2

        # ✅ LIVENESS PROBE
        livenessProbe:
          httpGet:
            path: /healthz
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 30
          timeoutSeconds: 5
          failureThreshold: 3

      volumes:
      - name: ca-cert
        secret:
          secretName: zenith-ca-cert
```

### Étape 3.3 : Validation

```bash
# Appliquer et tester
kubectl apply -f deployments/k8s/local/ingestor-deployment.yml

# Vérifier que les pods ont les bonnes resources
kubectl describe pod zenith-ingestor-xyz -n zenith-dev | grep -A 5 "Requests\|Limits"

# ✅ Output doit montrer:
# Requests:
#   cpu: 100m
#   memory: 256Mi
# Limits:
#   cpu: 1000m
#   memory: 512Mi
```

---

## 🟠 RECOMMANDÉ #1 : Issue-502 — Consolider les Étapes Buf

### Étape 4.1 : Modifier `.github/workflows/deploy.yml`

**Fichier** : `.github/workflows/deploy.yml`

**Problème** : `Set up buf` apparaît deux fois (ligne 30-33 et 43-46)

**Solution** : Consolider en une seule étape

**Avant** (structure) :
```yaml
jobs:
  lint:
    steps:
      - name: Set up Go
      - name: Set up buf          # ← #1
        setup_only: true
      - name: Generate Protobuf code
        run: buf generate
      - name: Run golangci-lint

  test:
    steps:
      - name: Set up Go
      - name: Set up buf          # ← #2 DUPLICATE
        setup_only: true
      - name: Generate Protobuf code
        run: buf generate
      - name: Run unit tests
```

**Après** (réorganisé) :
```yaml
jobs:
  # Unique setup (partagé concept)
  # Pas de vrai partage possible en GitHub Actions sans composite actions
  # SOLUTION: Accepter la duplication, mais documenter que c'est OK pour Phase 3 solo

  lint:
    steps:
      - name: Checkout
        uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
          cache: true

      - name: Set up buf
        uses: bufbuild/buf-action@v1
        with:
          setup_only: true

      - name: Generate Protobuf code
        run: buf generate

      - name: Run golangci-lint
        uses: golangci/golangci-lint-action@v7
        with:
          version: v2.11.2

      - name: Run buf lint
        run: buf lint

  test:
    steps:
      - name: Checkout
        uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
          cache: true

      - name: Set up buf
        uses: bufbuild/buf-action@v1
        with:
          setup_only: true

      - name: Generate Protobuf code
        run: buf generate

      - name: Run unit tests
        run: go test -short -race -count=1 ./...
```

**Alternative (Meilleure)** : Créer une composite action réutilisable

`.github/actions/setup-buf/action.yml` :
```yaml
name: Setup Buf & Generate Protobuf
description: Install buf and generate protobuf code

runs:
  using: composite
  steps:
    - name: Set up buf
      uses: bufbuild/buf-action@v1
      with:
        setup_only: true

    - name: Generate Protobuf code
      run: buf generate
      shell: bash
```

Puis dans le workflow :
```yaml
lint:
  steps:
    - name: Checkout
      uses: actions/checkout@v4

    - name: Setup Buf
      uses: ./.github/actions/setup-buf        # ← Composite action

    - name: Run golangci-lint
      # ...

test:
  steps:
    - name: Checkout
      uses: actions/checkout@v4

    - name: Setup Buf
      uses: ./.github/actions/setup-buf        # ← Réutilisé

    - name: Run unit tests
      # ...
```

**Choix** :
- **Simple (Phase 3)** : Accepter la duplication pour l'instant
- **Meilleur (Phase 4)** : Créer la composite action

### Recommandation pour ce plan
Utiliser l'approche **Simple** (accepter la duplication) car :
- Overhead minimal (30 sec par job)
- Composite actions ajoutent de la complexité
- À revisiter en Phase 4 quand le workflow grandit

---

## 🟠 RECOMMANDÉ #2 : Issue-503 — Convertir Logger de Package Level

### Étape 5.1 : Modifier `internal/gateway/handler.go`

**Fichier** : `internal/gateway/handler.go`

**Action** : Convertir `writeJSON` et `writeError` en méthodes de Gateway

**Avant** (lignes 172-187) :
```go
// writeJSON writes a JSON response.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("Failed to encode JSON response", "error", err)  // ❌ Package level
	}
}

// writeError writes an error response.
func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorResponse{
		Code:    code,
		Message: message,
	})
}
```

**Après** :
```go
// writeJSON writes a JSON response.
func (g *Gateway) writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		g.logger.Error("Failed to encode JSON response", "error", err)  // ✅ Gateway logger
	}
}

// writeError writes an error response.
func (g *Gateway) writeError(w http.ResponseWriter, status int, code, message string) {
	g.writeJSON(w, status, errorResponse{
		Code:    code,
		Message: message,
	})
}
```

### Étape 5.2 : Mettre à Jour les Appels dans `HandleIngestEvent`

**Fichier** : `internal/gateway/handler.go`, dans la méthode `HandleIngestEvent`

Remplacer tous les appels :
- `writeError(...)` → `g.writeError(...)`
- `writeJSON(...)` → `g.writeJSON(...)`

**Exemples** :
```go
// ❌ Avant (ligne 79)
writeError(w, http.StatusBadRequest, "INVALID_JSON", "request body is empty")

// ✅ Après
g.writeError(w, http.StatusBadRequest, "INVALID_JSON", "request body is empty")
```

Locations dans `HandleIngestEvent` à mettre à jour :
- Ligne 79 : `writeError` → `g.writeError`
- Ligne 82 : `writeError` → `g.writeError`
- Ligne 88 : `writeError` → `g.writeError`
- Ligne 92 : `writeError` → `g.writeError`
- Ligne 96 : `writeError` → `g.writeError`
- Ligne 103 : `writeError` → `g.writeError`
- Ligne 114 : `writeError` → `g.writeError`
- Ligne 125 : `writeError` → `g.writeError`
- Ligne 147 : `writeError` → `g.writeError`
- Ligne 154 : `writeError` → `g.writeError`
- Ligne 166 : `writeJSON` → `g.writeJSON`

### Étape 5.3 : Vérifier les Tests

**Fichier** : `internal/gateway/handler_test.go`

**Vérification** : Les tests doivent toujours passer (ils n'appellent pas directement `writeJSON`/`writeError`)

```bash
go test ./internal/gateway/... -v

# ✅ Tous les tests doivent passer
```

---

## 🟠 RECOMMANDÉ #3 : Issue-503 — Nettoyer Code Mort

### Étape 6.1 : Modifier `internal/gateway/handler.go`

**Fichier** : `internal/gateway/handler.go`

**Action** : Simplifier la gestion du payload (lignes 129-133)

**Avant** :
```go
// Ensure payload is not nil
payload := []byte(req.Payload)
if payload == nil {
	payload = []byte("{}")
}
```

**Après** (Option A — Simple) :
```go
// Use payload as-is; default to empty object if missing
payload := req.Payload
if len(payload) == 0 {
	payload = []byte("{}")
}
```

**Après** (Option B — Robuste) :
```go
// Use payload as-is; default to empty object if empty
var payload []byte
if len(req.Payload) > 0 {
	payload = req.Payload
} else {
	payload = []byte("{}")
}
```

**Choix** : Utiliser **Option A** (plus lisible)

### Étape 6.2 : Valider

```bash
go test ./internal/gateway/... -v

# ✅ Vérifier que le test `TestHandleIngestEvent_SuccessValidEvent` passe toujours
```

---

## 🟠 RECOMMANDÉ #4 : Issue-504 — Changer imagePullPolicy

### Étape 7.1 : Modifier `deployments/k8s/local/ingestor-deployment.yml` (Ligne 20)

**Fichier** : `deployments/k8s/local/ingestor-deployment.yml`

**Action** : Ajouter un commentaire et documenter la politique

**Avant** :
```yaml
        imagePullPolicy: IfNotPresent
```

**Après** (pour local) :
```yaml
        # imagePullPolicy: IfNotPresent  # OK for local Kind cluster
        # NOTE: For production (Cloud Run), use 'Always' or pin SHA256 digest
        imagePullPolicy: IfNotPresent
```

**Pour production** (Phase 4+) : Utiliser `Always` ou pin exact :
```yaml
        image: zenith-ingestor:sha256:abc1234567890def  # Production: pin SHA
        imagePullPolicy: Always                         # Or use digest
```

**Documentation** : Ajouter dans `CLAUDE.md` (section Kubernetes) :

```markdown
### Image Pull Policy

- **Local/Dev (Kind)** : `imagePullPolicy: IfNotPresent` — réutilise les images locales
- **Production (Cloud Run)** : `imagePullPolicy: Always` — force re-pull
  - Ou mieux : pin le SHA256 digest (généré par CI/CD)
  - Exemple: `image: zenith-ingestor@sha256:abc123...`

Rolling updates avec `latest` tag + `IfNotPresent` = **non-déterministe**. En production, toujours pin l'image.
```

---

## ✅ Validation Globale

### Checklist Post-Implémentation

**Issue-502 (Terraform)** :
- [ ] `variables.tf` ne contient plus de secrets
- [ ] `.github/workflows/deploy.yml` n'a plus les `-var=secrets`
- [ ] `cloud_run.tf` injecte les secrets via Secret Manager
- [ ] `terraform plan -var="project_id=..." -var="image_tag=..."` valide sans erreurs
- [ ] Documentation ajoutée dans `CLAUDE.md`

**Issue-504 (Deployment)** :
- [ ] `ingestor-deployment.yml` a `readinessProbe`
- [ ] `ingestor-deployment.yml` a `resources.requests` et `resources.limits`
- [ ] `kubectl apply --dry-run=client` valide le manifest
- [ ] Tests locaux avec `kind` passent

**Issue-502 (Buf)** :
- [ ] Duplication documentée / acceptée pour Phase 3

**Issue-503 (Logger)** :
- [ ] `writeJSON` est une méthode de Gateway
- [ ] `writeError` est une méthode de Gateway
- [ ] Tous les appels utilisent `g.writeJSON` / `g.writeError`
- [ ] Tests passent : `go test ./internal/gateway/... -v`

**Issue-503 (Code mort)** :
- [ ] Condition `payload == nil` remplacée par `len(payload) == 0`
- [ ] Tests passent

**Issue-504 (imagePullPolicy)** :
- [ ] Commentaire ajouté au manifest
- [ ] Documentation dans `CLAUDE.md`

---

## 📋 Ordre Recommandé d'Exécution

### Phase 1 : Critiques (MUST-FIX) — ~2h
1. **Issue-502** : Supprimer secrets Terraform (30min)
2. **Issue-504** : Ajouter readinessProbe (15min)
3. **Issue-504** : Ajouter resources (15min)
4. **Tests & Validation** (30min)

### Phase 2 : Recommandés (SHOULD-FIX) — ~1h
5. **Issue-502** : Consolider Buf (accepter duplication pour Phase 3)
6. **Issue-503** : Logger methods (15min)
7. **Issue-503** : Code mort (5min)
8. **Issue-504** : imagePullPolicy documentation (5min)
9. **Tests globaux** (10min)

---

## 🔗 Dépendances Entre Tâches

```
┌─ Issue-502 Secrets (CRIT) ──────┐
│                                  │
├─ Issue-504 Readiness (CRIT)      │
│                                  ├─► Commit & Push
├─ Issue-504 Resources (CRIT)      │
│                                  │
├─ Issue-502 Buf (REC)             │
│                                  │
├─ Issue-503 Logger (REC)          │
│                                  │
├─ Issue-503 Code mort (REC)       │
│                                  │
└─ Issue-504 imagePullPolicy (REC) ─┘
```

**Dépendances** : Aucune entre les tâches — elles sont indépendantes et peuvent être parallélisées.

**Ordre suggéré** : Critiques d'abord (blocking), puis recommandés (hygiene).

---

## 🧪 Test Suite Post-Implémentation

### Tests Terraform
```bash
cd deployments/terraform
terraform init -backend-config="bucket=zenith-tfstate-<project-id>"
terraform fmt -recursive
terraform validate
terraform plan -var="project_id=<id>" -var="image_tag=test" -out=tfplan
# ✅ Pas d'erreurs secrets manquants
```

### Tests Go
```bash
# Gateway
go test ./internal/gateway/... -v

# Tous les tests
go test ./... -short -race

# Linter
golangci-lint run
```

### Tests Kubernetes
```bash
# Valider le manifest
kubectl apply -f deployments/k8s/local/ingestor-deployment.yml --dry-run=client

# Appliquer en local
kind create cluster --name zenith-lab
kubectl apply -f deployments/k8s/local/namespace.yaml
kubectl apply -f deployments/k8s/local/config.yaml
kubectl apply -f deployments/k8s/local/secrets.yaml
kubectl apply -f deployments/k8s/local/ingestor-deployment.yml

# Vérifier les probes
kubectl get deployment zenith-ingestor-deployment -n zenith-dev -o yaml | grep -A 5 "readinessProbe\|livenessProbe"

# Vérifier les resources
kubectl describe pod zenith-ingestor-xyz -n zenith-dev | grep -A 5 "Requests\|Limits"

# ✅ Tous les pods doivent être READY
kubectl get pods -n zenith-dev
```

---

## 📝 Commit Strategy

Créer **2 commits** :

### Commit #1 : Critiques
```
fix: Sprint 5 critical fixes

- Issue-502: Remove secrets from Terraform variables
  - Delete database_url, api_key_salt, slack_webhook_url from variables.tf
  - Update .github/workflows/deploy.yml to remove secret -var flags
  - Document manual secret injection via gcloud in CLAUDE.md

- Issue-504: Add readinessProbe to Kubernetes Deployment
  - Ensure traffic is only routed to ready pods during rolling updates

- Issue-504: Add CPU/Memory resource requests and limits
  - Prevent noisy neighbor problem
  - Enable proper scheduler placement and HPA scaling

Tested:
- terraform validate
- terraform plan
- go test ./...
- kubectl apply --dry-run=client
```

### Commit #2 : Recommandés
```
refactor: Sprint 5 recommended improvements

- Issue-503: Convert writeJSON/writeError to Gateway methods
  - Improves logging context with g.logger instead of package-level slog

- Issue-503: Remove dead code (payload == nil check)
  - Simplify payload handling logic

- Issue-502: Document buf duplication (Phase 3 acceptable)
  - Composite action deferred to Phase 4

- Issue-504: Document imagePullPolicy for production
  - Add CLAUDE.md note about Always policy and SHA256 pinning

Tested:
- go test ./internal/gateway/... -v
- golangci-lint run
```

---

## 🎯 Success Criteria

✅ **Tout doit passer** :

1. **Terraform** : `terraform plan` succeeds without secret variable errors
2. **Go Tests** : `go test -short -race ./...` ✅
3. **Linter** : `golangci-lint run` ✅
4. **Kubernetes** : `kubectl apply --dry-run=client` ✅
5. **Documentation** : `CLAUDE.md` updated with manual secret injection + imagePullPolicy notes
6. **Git** : No uncommitted files, clean history with 2 well-formed commits
