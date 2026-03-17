# Implementation Plan: Issue-601B — Sink Config Cleanup (Option B)

**Contexte:** `SLACK_WEBHOOK_URL` a été ajouté en Sprint 5 avant que l'abstraction `Sink` existe. Ce plan corrige deux problèmes distincts : le mauvais propriétaire de la variable (Ingestor au lieu de Dispatcher), et l'introduction erronée d'une `DISPATCH_SINK_WEBHOOK_URL` statique qui contredit le modèle de données de Zenith.

---

## Décision d'architecture

### Problème 1 — Mauvais propriétaire
`SLACK_WEBHOOK_URL` est injecté dans l'**Ingestor** (Cloud Run, K8s), alors que le dispatch est la responsabilité exclusive du **Dispatcher**.

### Problème 2 — `DISPATCH_SINK_WEBHOOK_URL` n'a pas sa place en env var
Le modèle `Rule` possède déjà un champ `TargetAction string` prévu à cet effet. Une variable d'environnement statique signifierait que **toutes** les règles d'une instance Zenith pointent vers la même URL cible, ce qui contredit le principe fondamental du produit : c'est la source (via sa règle) qui décide de la cible.

```
Rule.TargetAction = "https://mon-endpoint.com/events"  ← la cible vit dans la DB
```

**Exception légitime : Slack.** L'URL Slack est un secret d'authentification (credential), pas une destination métier. On ne stocke pas de credentials dans la base de données. Elle reste donc en env var.

### Règle après ce plan

| Sink | Source de la cible | Raison |
|---|---|---|
| `WebhookSink` | `rule.TargetAction` | Chaque règle a sa propre cible — c'est le cœur du produit |
| `SlackSink` | Env var `DISPATCH_SINK_SLACK_URL` | Secret d'authentification, ne va pas en DB |

### Nouveaux noms

| Ancien | Nouveau | Propriétaire |
|---|---|---|
| `SLACK_WEBHOOK_URL` | `DISPATCH_SINK_SLACK_URL` | Dispatcher uniquement |
| _(supprimé)_ | ~~`DISPATCH_SINK_WEBHOOK_URL`~~ | N/A — vient de `rule.TargetAction` |

### Champ Go correspondant

| Ancien | Nouveau |
|---|---|
| `SecretsConfig.SlackWebhookURL` | `DispatcherConfig.SinkSlackURL` |
| _(supprimé)_ | ~~`DispatcherConfig.SinkWebhookURL`~~ |

---

## Inventaire complet des fichiers à modifier

### Code Go
| Fichier | Changement |
|---|---|
| `internal/config/config.go` | Remplacer `SecretsConfig` par `DispatcherConfig`, renommer les champs, lire les nouvelles vars |
| `cmd/dispatcher/main.go` | Mettre à jour le commentaire `_ = cfg` |

### Infrastructure
| Fichier | Changement |
|---|---|
| `deployments/terraform/secrets.tf` | Remplacer `SLACK_WEBHOOK_URL` par `DISPATCH_SINK_SLACK_URL` dans `locals.secrets` |
| `deployments/terraform/cloud_run.tf` | Supprimer l'injection `SLACK_WEBHOOK_URL` de l'Ingestor _(move vers Dispatcher en Issue-604)_ |
| `deployments/k8s/local/secrets.yaml` | Renommer la clé |

### Fichiers de config locaux
| Fichier | Changement |
|---|---|
| `.env.secrets` | Renommer `SLACK_WEBHOOK_URL` → `DISPATCH_SINK_SLACK_URL` |
| `.env.secrets.example` | Idem + ajouter `DISPATCH_SINK_WEBHOOK_URL` |

### Documentation
| Fichier | Changement |
|---|---|
| `CLAUDE.md` | Mettre à jour les deux références + section GCP Secret Manager |
| `README.md` | Mettre à jour la référence |
| `docs/organization/plans/ISSUE_501_TERRAFORM.md` | Mettre à jour les deux références |
| `docs/organization/plans/ISSUE_601_DISPATCHER.md` | Mettre à jour la section env vars |

---

## Étapes d'implémentation

### Étape 1 — Refactorer `internal/config/config.go`

Remplacer `SecretsConfig` par deux structs distinctes, chacune avec une responsabilité claire :

```go
// SecretsConfig holds sensitive settings shared across both binaries.
type SecretsConfig struct {
    APIKeySalt string
}

// DispatcherConfig holds Dispatcher-specific sink configuration.
// WebhookSink URLs are NOT here — they come from rule.TargetAction at runtime.
type DispatcherConfig struct {
    SinkSlackURL string // DISPATCH_SINK_SLACK_URL — secret d'auth Slack
}

// Config holds the application configuration.
type Config struct {
    Port       string
    Database   DatabaseConfig
    Secrets    SecretsConfig
    Engine     EngineConfig
    Dispatcher DispatcherConfig
}
```

Dans `Load()` :
```go
return &Config{
    // ...
    Secrets: SecretsConfig{
        APIKeySalt: os.Getenv("API_KEY_SALT"),
    },
    Dispatcher: DispatcherConfig{
        SinkSlackURL: os.Getenv("DISPATCH_SINK_SLACK_URL"),
    },
}, nil
```

**Pourquoi une struct séparée `DispatcherConfig` ?**
`SecretsConfig` implique des valeurs sensibles partagées entre binaires. `DispatcherConfig` exprime la responsabilité fonctionnelle et est scopée au bon binaire. `API_KEY_SALT` reste dans `SecretsConfig` car l'Ingestor en a besoin pour valider les clés API.

### Étape 2 — Mettre à jour `cmd/dispatcher/main.go`

```go
_ = cfg // cfg.Dispatcher.SinkSlackURL utilisé par SlackSink en Issue-602
        // WebhookSink lit sa cible depuis rule.TargetAction, pas depuis cfg
```

### Étape 3 — Infrastructure Terraform

**`deployments/terraform/secrets.tf`** — Renommer dans `locals.secrets` :
```hcl
locals {
  secrets = ["DATABASE_URL", "API_KEY_SALT", "DISPATCH_SINK_SLACK_URL"]
}
```

**`deployments/terraform/cloud_run.tf`** — Supprimer le bloc `SLACK_WEBHOOK_URL` de l'Ingestor :
```hcl
# Supprimer ce bloc (déplacé vers le Dispatcher en Issue-604) :
# env {
#   name = "SLACK_WEBHOOK_URL"
#   ...
# }
```

> **Note:** En Issue-604, quand le Cloud Run Dispatcher sera créé, le bloc `DISPATCH_SINK_SLACK_URL` sera injecté dans la ressource Dispatcher, pas l'Ingestor.

### Étape 4 — GCP Secret Manager (opération manuelle)

Le renommage de la variable nécessite de créer un nouveau secret dans GCP Secret Manager et de supprimer l'ancien :

```bash
# 1. Créer le nouveau secret
echo -n "https://hooks.slack.com/services/..." | \
  gcloud secrets versions add zenith-dispatch-sink-slack-url-${ENVIRONMENT} --data-file=-

# 2. Vérifier
gcloud secrets versions list zenith-dispatch-sink-slack-url-${ENVIRONMENT}

# 3. Supprimer l'ancien (après validation du déploiement)
gcloud secrets delete zenith-slack-webhook-url-${ENVIRONMENT}
```

> **Important:** Effectuer cette opération **après** avoir appliqué le `terraform apply` qui crée le nouveau secret container. L'ancien secret peut coexister pendant la transition.

### Étape 5 — Kubernetes local

**`deployments/k8s/local/secrets.yaml`** :
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: zenith-secrets
  namespace: zenith-dev
type: Opaque
data:
  DATABASE_URL: <base64>
  API_KEY_SALT: <base64>
  DISPATCH_SINK_SLACK_URL: <base64>   # renommé, Dispatcher uniquement
```

> Le manifest Dispatcher (Issue-604) injectera `DISPATCH_SINK_SLACK_URL` via `secretKeyRef`. L'Ingestor ne la montera plus. Les URLs webhook ne sont pas des secrets K8s — elles viennent de `rule.TargetAction` en base.

### Étape 6 — Fichiers de config locaux

**`.env.secrets`** :
```
DISPATCH_SINK_SLACK_URL=https://...
```

**`.env.secrets.example`** :
```
# Dispatcher — Slack sink credential (secret d'auth, ne va pas en DB)
# Les URLs webhook sont dans Rule.target_action, pas ici.
DISPATCH_SINK_SLACK_URL=https://hooks.slack.com/services/YOUR/WEBHOOK/URL
```

### Étape 7 — Documentation

**`CLAUDE.md`** — Section "Optional env vars" :
```
- `API_KEY_SALT`
- `DISPATCH_SINK_SLACK_URL` — Dispatcher uniquement, credential Slack (secret)
  Note: les URLs webhook ne sont pas des env vars — elles viennent de Rule.target_action
```

Section GCP Secret Manager :
```
sensitive variables (`DATABASE_URL`, `API_KEY_SALT`, `DISPATCH_SINK_SLACK_URL`)
```

**`README.md`** — Mettre à jour la référence env var.

**`docs/organization/plans/ISSUE_501_TERRAFORM.md`** — Mettre à jour les deux références.

**`docs/organization/plans/ISSUE_601_DISPATCHER.md`** — Mettre à jour la section env vars :
```
- `DISPATCH_SINK_SLACK_URL` — Dispatcher uniquement (cfg.Dispatcher.SinkSlackURL)
  Les URLs webhook viennent de rule.TargetAction, pas d'une env var.
```

---

## Ordre d'exécution recommandé

```
1. config.go          (Go — base de tout)
2. cmd/dispatcher     (Go — consommateur direct)
3. .env.secrets*      (local dev — pour tester)
4. Tests: go test ./...
5. Linter: golangci-lint run
6. secrets.yaml       (K8s local)
7. terraform/*.tf     (infrastructure)
8. Documentation      (CLAUDE.md, README.md, plans/)
9. Terraform apply    (si environnement cloud actif)
10. GCP Secret Manager migration (manuelle, post-deploy)
```

---

## Tests à vérifier

- `go test ./...` — aucun test ne référence directement `SlackWebhookURL` mais la compilation valide le renommage
- `golangci-lint run` — aucune variable inutilisée
- `go build ./...` — les deux binaires compilent

---

## Definition of Done

- [ ] `config.go` : `DispatcherConfig` avec `SinkSlackURL` uniquement (pas de SinkWebhookURL)
- [ ] Aucune référence à `SLACK_WEBHOOK_URL` ou `SlackWebhookURL` dans le code Go
- [ ] `terraform/secrets.tf` : `DISPATCH_SINK_SLACK_URL` dans `locals.secrets`
- [ ] `terraform/cloud_run.tf` : bloc `SLACK_WEBHOOK_URL` supprimé de l'Ingestor
- [ ] `deployments/k8s/local/secrets.yaml` : clé renommée, pas de `DISPATCH_SINK_WEBHOOK_URL`
- [ ] `.env.secrets.example` : `DISPATCH_SINK_SLACK_URL` documenté avec note sur `rule.TargetAction`
- [ ] `CLAUDE.md`, `README.md`, plans : aucune référence obsolète, principe `rule.TargetAction` documenté
- [ ] `go test ./...` et `golangci-lint run` passent
