# 📋 Sprint 5 Review: Infrastructure as Code & CI/CD

**Date** : 2026-03-17
**Phase** : Phase 3 (Weeks 5 & 6)
**Status** : 85% Completed — 3 Critical Issues + 4 Recommended Fixes

---

## 🎯 Sprint 5 Objectif

Provisionner l'infrastructure cloud (Terraform), automatiser les déploiements (GitHub Actions CI/CD), ajouter une REST gateway pour webhooks, et démontrer les rolling updates Kubernetes.

### Issues Couvertes
- **[Issue-501]** Terraform Infrastructure Provisioning — [x] Completed
- **[Issue-502]** GitHub Actions CI/CD Pipeline — [x] Completed (avec issues)
- **[Issue-503]** REST Gateway for Webhook Ingestion — [x] Completed
- **[Issue-504]** [CKAD] Kubernetes Deployments & Rolling Updates — [x] Completed (avec issues)

---

## ✅ Points Forts

### Issue-501: Terraform Infrastructure Provisioning

| Aspect | Rating | Notes |
|--------|--------|-------|
| **Cloud Provider Choice** | ⭐⭐⭐⭐⭐ | Cloud Run > Fargate pour HTTP/2 natif (ConnectRPC) |
| **Dockerfile Migration** | ⭐⭐⭐⭐⭐ | `scratch` → `distroless/static-debian12` (CA certs + timezone) |
| **Secrets Management** | ⭐⭐⭐⭐⭐ | Secret Manager + pas de valeurs dans state Terraform |
| **Health Probes** | ⭐⭐⭐⭐⭐ | Startup + Liveness probes pour scaling/rollback |
| **IAM Principle** | ⭐⭐⭐⭐⭐ | Service account limité à logging, metrics, secret access (pas editor/owner) |
| **State Backend** | ⭐⭐⭐⭐⭐ | GCS avec versioning + locking natif |
| **Documentation** | ⭐⭐⭐⭐⭐ | Plan détaillé avec justifications pour chaque choix |

**Verdict** : ✅ **Excellent — 10/10**

Terraform est production-ready. L'approche "Infrastructure as Code Rule" (aucune ressource manuelle) est respectée.

---

### Issue-502: GitHub Actions CI/CD Pipeline

| Aspect | Rating | Notes |
|--------|--------|-------|
| **Authentication** | ⭐⭐⭐⭐⭐ | Workload Identity Federation (OIDC) — pas de clés JSON longue durée |
| **Pipeline Structure** | ⭐⭐⭐⭐⭐ | `lint` \|\| `test` → `build-push` → `deploy` (dépendances explicites) |
| **Artifact Traceability** | ⭐⭐⭐⭐⭐ | Tag SHA Git pour chaque image |
| **PR vs. Main Isolation** | ⭐⭐⭐⭐⭐ | PRs lint/test seulement ; deploy sur main seulement |
| **Protobuf Integration** | ⭐⭐⭐⭐⭐ | `buf generate` avant lint/test |
| **Permissions Model** | ⭐⭐⭐⭐⭐ | RBAC minimal (`contents:read`, `id-token:write`) |

**Verdict** : ⭐⭐⭐⭐ **Excellent (avec réserves) — 8/10**

Pipeline est robuste, mais **3 issues critiques** doivent être corrigées (voir section problèmes).

---

### Issue-503: REST Gateway

| Aspect | Rating | Notes |
|--------|--------|-------|
| **Dependency Minimalism** | ⭐⭐⭐⭐⭐ | Stdlib `net/http` (YAGNI — pas Gin/grpc-gateway) |
| **Input Validation** | ⭐⭐⭐⭐⭐ | Champs requis + `http.MaxBytesReader` (1 MB limit) |
| **Authentication** | ⭐⭐⭐⭐⭐ | `X-Api-Key` header + source name validation |
| **HTTP Status Codes** | ⭐⭐⭐⭐⭐ | 202 Accepted, 401/403/413/503 corrects |
| **Test Coverage** | ⭐⭐⭐⭐⭐ | 8 test cases exhaustifs (succès + tous les errors) |
| **Mock Pattern** | ⭐⭐⭐⭐⭐ | Suit la structure existante (`mockSourceRepository`, `mockPipeline`) |
| **DoS Protection** | ⭐⭐⭐⭐⭐ | `MaxBytesReader` (1 MB) |
| **Graceful Error Handling** | ⭐⭐⭐⭐ | Bon, mais logging de package level (mineure) |

**Verdict** : ✅ **Excellent — 9/10**

Implementation propre et sécurisée. Correction mineure de logging à faire.

---

### Issue-504: Kubernetes Deployments & Rolling Updates

| Aspect | Rating | Notes |
|--------|--------|-------|
| **Deployment Resource** | ⭐⭐⭐⭐⭐ | Remplace correctement le Pod bare metal |
| **Rolling Update Strategy** | ⭐⭐⭐⭐⭐ | `maxUnavailable: 1` → zéro-downtime |
| **Replicas** | ⭐⭐⭐⭐⭐ | 3 replicas (bonne redondance) |
| **Config Injection** | ⭐⭐⭐⭐⭐ | ConfigMap + Secret intégrés |
| **CA Certificate Mounting** | ⭐⭐⭐⭐ | ✅ Préparation pour TLS vers CockroachDB |
| **Startup Probe** | ⭐⭐⭐⭐ | ✅ Donne du temps pour la connexion DB |
| **Liveness Probe** | ⭐⭐⭐⭐ | ✅ Relance les pods defaillants |
| **Readiness Probe** | 🔴 | ❌ **MANQUANT** — Critical |
| **Resource Requests** | 🔴 | ❌ **MANQUANTS** — Critical |
| **Resource Limits** | ⭐⭐⭐ | Partiels (manquent requests) |
| **imagePullPolicy** | ⭐⭐⭐ | IfNotPresent OK pour local, pas pour production |

**Verdict** : ⭐⭐⭐ **Bon (mais incomplet) — 6/10**

Foundations solides, **2 problèmes critiques** bloquent l'utilisation en production.

---

## 🔴 Problèmes Critiques (Must-Fix)

### 🔴 **CRITIQUE #1 : Issue-502 — Secrets Passés en Variables Terraform**

**Fichier** : `.github/workflows/deploy.yml`, ligne 166

**Problème** :
```yaml
terraform plan \
  -var="database_url=${{ secrets.DATABASE_URL }}" \
  -var="api_key_salt=${{ secrets.API_KEY_SALT }}" \
  -var="slack_webhook_url=${{ secrets.SLACK_WEBHOOK_URL }}" \
  ...
```

**Risques** :
1. **Exposition en plan** : Les secrets s'affichent en clair dans `terraform show tfplan`
2. **Fuite GitHub Actions** : Logs Actions peuvent contenir les secrets (même masqués)
3. **State Terraform** : Si les secrets transitent via variables, ils finissent dans `.tfstate`
4. **Violation sécurité** : Contredit le plan Issue-502 original qui disait "pas de secrets dans Terraform"

**Root Cause** :
Le plan original (ISSUE_502_CICD.md) disait explicitement:
> "Les *valeurs* des secrets ne sont pas dans Terraform (elles n'apparaîtraient pas chiffrées dans le state). Les valeurs sont injectées une seule fois via CLI après le premier `terraform apply`"

Le code a violé cette règle.

**Solution** :
1. **Supprimer les variables sensibles** de `deployments/terraform/variables.tf` :
   ```hcl
   # ❌ Supprimer ces trois:
   variable "database_url" { ... }
   variable "api_key_salt" { ... }
   variable "slack_webhook_url" { ... }
   ```

2. **Enlever les `-var` du workflow** :
   ```yaml
   # ✅ AVANT:
   terraform plan -var="database_url=..." -var="api_key_salt=..." ...

   # ✅ APRÈS:
   terraform plan -var="project_id=${{ secrets.GCP_PROJECT_ID }}" \
                  -var="image_tag=${{ env.IMAGE_TAG }}" \
                  -out=tfplan
   ```

3. **Vérifier que cloud_run.tf lit depuis Secret Manager** (✅ déjà correct) :
   ```hcl
   env {
     name = "DATABASE_URL"
     value_source {
       secret_key_ref {
         secret  = google_secret_manager_secret.zenith_secrets["DATABASE_URL"].secret_id
         version = "latest"
       }
     }
   }
   ```

4. **Valeurs chargées hors Terraform** (manuel, une fois) :
   ```bash
   echo -n "postgresql://..." | gcloud secrets versions add \
     zenith-database-url-dev --data-file=-
   ```

**Impact** : 🔴 **Blocking — doit être corrigé avant Phase 4**

---

### 🔴 **CRITIQUE #2 : Issue-504 — Pas de Readiness Probe**

**Fichier** : `deployments/k8s/local/ingestor-deployment.yml`

**Problème** :
Le Deployment a `startupProbe` et `livenessProbe`, mais **pas de `readinessProbe`**.

```yaml
liveness_probe:
  http_get: { path: /healthz, port: 8080 }
  # ...

# ❌ MANQUANT:
# readinessProbe:
#   http_get: { path: /healthz, port: 8080 }
#   ...
```

**Risques** :
1. **Trafic vers pods non-prêts** : Lors d'un rolling update, K8s envoie des requêtes à des pods qui initialisent encore leur connexion DB
2. **Erreurs dans les requêtes** : `503 Service Unavailable` ou timeouts si DB n'est pas connectée
3. **Perte de zéro-downtime** : La stratégie `maxUnavailable: 1` ne marche correctement que si readiness est configurée

**Pourquoi c'est important** :
- **Startup Probe** = "Est-ce que le pod a terminé son démarrage ?" (une seule fois au boot)
- **Readiness Probe** = "Est-ce que le pod est prêt à recevoir du trafic ?" (continu pendant la vie du pod)
- **Liveness Probe** = "Est-ce que le pod est vivant ?" (relance si mort)

**Solution** :
Ajouter au template du Deployment YAML :
```yaml
spec:
  template:
    spec:
      containers:
      - name: zenith-ingestor
        # ...
        readinessProbe:
          httpGet:
            path: /healthz
            port: 8080
          initialDelaySeconds: 3
          periodSeconds: 10
          timeoutSeconds: 5
          failureThreshold: 2
        livenessProbe:
          httpGet:
            path: /healthz
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 30
          timeoutSeconds: 5
          failureThreshold: 3
        startupProbe:
          httpGet:
            path: /healthz
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
          timeoutSeconds: 5
          failureThreshold: 10
```

**Ordre recommandé** : Startup → Readiness → Liveness (dans le YAML)

**Impact** : 🔴 **Blocking — zéro-downtime deployment invalide sans ça**

---

### 🔴 **CRITIQUE #3 : Issue-504 — Pas de Resource Requests/Limits**

**Fichier** : `deployments/k8s/local/ingestor-deployment.yml`

**Problème** :
```yaml
spec:
  template:
    spec:
      containers:
      - name: zenith-ingestor
        # ❌ Aucune "resources:" section
```

**Risques** :
1. **Scheduler en aveugle** : K8s ne sait pas combien CPU/memory allouer → placement aléatoire sur les nœuds
2. **Noisy Neighbor** : Un pod peut dévorer 100% du CPU/RAM du nœud → autres pods crashent (OOM killer)
3. **No QoS Guarantees** : Sans requests, le pod est en classe `BestEffort` = tué en dernier si nœud saturé
4. **Scaling inefficace** : HPA (Horizontal Pod Autoscaler) ne peut pas décider quand scaler sans requests

**Solution** :
Ajouter au container spec :
```yaml
resources:
  requests:
    cpu: "100m"              # Réservation pour le scheduler (0.1 CPU)
    memory: "256Mi"          # Réservation (256 MiB)
  limits:
    cpu: "1000m"             # Hard limit (throttling après)
    memory: "512Mi"          # OOM killed si dépassé
```

**Calibrage** : Ces valeurs sont approximatives — à ajuster après **benchmark réel** :
```bash
# Mesurer la consommation réelle
kubectl top pod zenith-ingestor-xyz -n zenith-dev
```

**Impact** : 🔴 **Blocking — sans ça, cluster instable en production**

---

## 🟠 Problèmes Majeurs (Recommandé Avant Phase 6)

### 🟠 **MAJEUR #1 : Issue-504 — imagePullPolicy Incorrect**

**Fichier** : `deployments/k8s/local/ingestor-deployment.yml`, ligne 20

**Problème** :
```yaml
imagePullPolicy: IfNotPresent
```

**Risques** :
- `IfNotPresent` = "Utilise l'image locale si elle existe" → Ne re-pull pas
- Lors d'un rolling update avec tag `latest`, les vieux pods gardent l'ancienne image locale
- Le tag `latest` est non-déterministe

**Solution** :
```yaml
# Pour local/dev (Kind):
imagePullPolicy: IfNotPresent  # ✅ OK

# Pour production (Cloud Run):
imagePullPolicy: Always        # ✅ Correct
# OU mieux encore, utiliser un tag SHA:
image: zenith-ingestor:abc1234567  # Exact digest, pas "latest"
```

**Recommandation** :
- **Phase 5** : Garder `IfNotPresent` (local testing)
- **Phase 6+** : Passer à `Always` + tag SHA dans les manifests générés par Terraform/CD

---

### 🟠 **MAJEUR #2 : Issue-502 — Duplication des Étapes Buf**

**Fichier** : `.github/workflows/deploy.yml`, lignes 30-36 et 43-49

**Problème** :
```yaml
jobs:
  lint:
    steps:
      - name: Set up buf     # ← Première fois
        uses: bufbuild/buf-action@v1
        with:
          setup_only: true

      - name: Generate Protobuf code
        run: buf generate

  test:
    steps:
      - name: Set up buf     # ← DUPLICATE
        uses: bufbuild/buf-action@v1
        with:
          setup_only: true

      - name: Generate Protobuf code
        run: buf generate
```

**Impact** : Overhead temps (chaque job re-setup buf) + duplication de code.

**Solution** :
Fusionner la logique :
```yaml
lint:
  steps:
    - name: Set up Go
      uses: actions/setup-go@v5

    - name: Set up buf
      uses: bufbuild/buf-action@v1
      with:
        setup_only: true

    - name: Generate Protobuf code
      run: buf generate

    - name: Run golangci-lint
      uses: golangci/golangci-lint-action@v7

test:
  steps:
    - name: Set up Go
      uses: actions/setup-go@v5

    - name: Set up buf
      uses: bufbuild/buf-action@v1
      with:
        setup_only: true

    - name: Generate Protobuf code
      run: buf generate

    - name: Run unit tests
      run: go test -short -race -count=1 ./...
```

(Ou créer un action réutilisable si beaucoup de duplication.)

---

## 🟡 Problèmes Mineurs (Recommandé à Corriger)

### 🟡 **MINEUR #1 : Issue-503 — Logging de Package Level**

**Fichier** : `internal/gateway/handler.go`, ligne 177

**Problème** :
```go
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("Failed to encode JSON response", "error", err)  // ❌ Package-level
	}
}
```

**Impact** :
- Logging sans contexte structuré (pas de request ID, source, event ID)
- Isolation du logger de la Gateway

**Solution** :
Convertir en méthodes du receiver `Gateway` :
```go
func (g *Gateway) writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		g.logger.Error("Failed to encode JSON response", "error", err)  // ✅ Utilise g.logger
	}
}

func (g *Gateway) writeError(w http.ResponseWriter, status int, code, message string) {
	g.writeJSON(w, status, errorResponse{
		Code:    code,
		Message: message,
	})
}
```

Puis dans `HandleIngestEvent`, utiliser `g.writeJSON` et `g.writeError` au lieu de fonctions globales.

---

### 🟡 **MINEUR #2 : Issue-503 — Condition Morte**

**Fichier** : `internal/gateway/handler.go`, lignes 129-133

**Problème** :
```go
payload := []byte(req.Payload)
if payload == nil {         // ❌ Ne peut jamais être true
	payload = []byte("{}")
}
```

**Pourquoi** : `json.RawMessage` est un alias pour `[]byte`. La conversion `[]byte(req.Payload)` ne peut jamais donner `nil` si `req.Payload` est bien décodé.

**Solution** :
```go
payload := req.Payload
if len(payload) == 0 {
	payload = json.RawMessage("{}")
}
```

Ou plus robuste :
```go
if req.Payload == nil || len(req.Payload) == 0 {
	req.Payload = json.RawMessage("{}")
}
domainEvent := &domain.Event{
	// ...
	Payload: []byte(req.Payload),
}
```

---

### 🟡 **MINEUR #3 : Issue-504 — Tag Image Non-Versionnée**

**Fichier** : `deployments/k8s/local/ingestor-deployment.yml`, ligne 19

**Problème** :
```yaml
image: zenith-ingestor:latest
```

**Impact** :
- Non-déterministe : `latest` change sans notification
- Rollback difficile (quelle version est "latest" ?)
- GitOps incompatible

**Solution** :
Soit:
1. Utiliser un tag SHA : `zenith-ingestor:abc1234567`
2. Ou utiliser un manifest généré par Terraform (qui injecte le SHA du build)
3. Pour dev/local : acceptable, mais documenter

---

## 📊 Analyse Sécurité

| Aspect | Status | Notes |
|--------|--------|-------|
| **Secrets Management** | 🔴 Non compliant | Variables Terraform violent la spécification |
| **IAM Principle** | ✅ Excellent | Service accounts limités au moindre privilège |
| **Workload Identity** | ✅ Excellent | OIDC, pas de clés JSON longue durée |
| **Image Base** | ✅ Bon | `distroless` contient les CA certs |
| **Health Probes** | 🟡 Partiel | Liveness ✅, Startup ✅, Readiness ❌ |
| **Resource Limits** | 🔴 Manquant | Pas de requests/limits = noisy neighbor risk |
| **API Gateway Auth** | ✅ Bon | `X-Api-Key` + validation source |
| **Body Size Limit** | ✅ Bon | 1 MB avec `MaxBytesReader` |
| **TLS/SSL** | ✅ Bon | HTTPS natif sur Cloud Run ; CA cert mounted |
| **Secrets in Logs** | ✅ Bon | Secrets GitHub masqués automatiquement |

**Score de sécurité global** : 7/10 (après corrections = 9/10)

---

## 📋 Checklist de Corrections

### **Critiques (Must-Fix avant Phase 4)**
- [ ] **Issue-502**: Supprimer secrets des variables Terraform (`variables.tf` + workflow)
  - Impact: Bloque la spécification sécurité
  - Effort: 1h

- [ ] **Issue-504**: Ajouter `readinessProbe` au Deployment
  - Impact: Zéro-downtime deployment invalide sans ça
  - Effort: 30min

- [ ] **Issue-504**: Ajouter CPU/Memory `requests` et `limits`
  - Impact: Cluster instable en multi-tenant
  - Effort: 30min

**Temps total critique** : ~2h

---

### **Recommandé (Before Phase 6)**
- [ ] **Issue-503**: Convertir `writeJSON`/`writeError` en méthodes de Gateway
  - Impact: Logging cohérent
  - Effort: 15min

- [ ] **Issue-503**: Nettoyer condition morte `payload == nil`
  - Impact: Code clarity
  - Effort: 5min

- [ ] **Issue-502**: Consolider les étapes `Set up buf`
  - Impact: CI/CD hygiene
  - Effort: 10min

- [ ] **Issue-504**: Passer à `imagePullPolicy: Always` en production
  - Impact: Rolling updates déterministes
  - Effort: 5min (config generation)

**Temps total recommandé** : ~45min

---

## 🔍 Notes Architecturales

### **Phase 5 Préparation (Microservice Decomposition)**

Le Sprint 5 pose les fondations pour **Phase 5 (future)** : découpler Ingestor → Rule Engine → Dispatcher via un message broker.

**Éléments à garder en tête** :

1. **Broker de messages** : NATS / Kafka pour découpler les tiers
   - Actuellement : Rule Engine s'exécute dans le même process via Go channels
   - Futur : Ingestor → Broker → Evaluator → Broker → Dispatcher (3 binaries indépendants)

2. **Service-to-Service Communication** :
   - Dispatcher (Phase 6) doit parler à l'Ingestor pour querier les rules
   - Ou: Dispatcher lit le state depuis CockroachDB directement

3. **Audit Logging** :
   - Phase 6 introduit `AuditLogRepository.Write()`
   - Préparer les migrations DB pour la table `audit_logs` dès maintenant
   - Schema: `(id, event_id, dispatcher_action, sink_target, status, timestamp)`

---

## 📚 Références

- **Plan Issue-501** : `docs/organization/plans/ISSUE_501_TERRAFORM.md`
- **Plan Issue-502** : `docs/organization/plans/ISSUE_502_CICD.md`
- **Plan Issue-503** : `docs/organization/plans/ISSUE_503_REST_GATEWAY.md`
- **CLAUDE.md** : Commandes dev, configuration, standards de code
- **PHASE3_ROADMAP.md** : Objectifs Phase 3 complets

---

## 🎓 Verdict Final

### Complétude du Sprint 5
- **Terraform** : ✅ 95% (1 issue sécurité à corriger)
- **CI/CD** : ✅ 85% (secrets issue + duplication Buf)
- **REST Gateway** : ✅ 95% (logging mineure)
- **K8s Deployment** : ⭐⭐⭐ 60% (2 critiques, 2 recommandées manquent)

### Score Global Sprint 5
**Before fixes** : 7.5/10
**After critical fixes** : 9.5/10
**After all fixes** : 9.9/10

### Recommandation
1. ✅ **Corriger les 3 critiques** (2h) → Déploiement production-safe
2. ✅ **Appliquer les 4 recommandées** (45min) → Best practices alignées
3. ✅ **Proceeder à Phase 4** (Observability) — pas de blockers majeurs après corrections

### Blockers pour Phase 4
- ❌ Issue-502 secrets dans Terraform — **MUST FIX**
- ❌ Issue-504 readiness probe — **MUST FIX**
- ❌ Issue-504 resource requests — **MUST FIX**
