# Plan d'implémentation — [Issue-501] Terraform Infrastructure Provisioning

## Objectif

Provisionner l'infrastructure cloud de Zenith via Terraform, de façon entièrement reproductible et sans aucune action manuelle dans la console cloud. La commande `terraform apply` doit produire un endpoint public capable de recevoir des événements via ConnectRPC (gRPC + HTTP/2).

---

## Choix Technologiques & Justifications

### Cloud Provider : Google Cloud Run

| Critère | Cloud Run | AWS Fargate |
|---|---|---|
| HTTP/2 natif (requis pour h2c) | ✅ Oui | Partiel (ALB uniquement) |
| Scale-to-zero | ✅ Oui | ❌ Non (coût idle) |
| Complexité IAM/réseau | Faible | Élevée (VPC, task roles, ECR) |
| Intégration Secret Manager | Native | SSM Parameter Store (plus verbeux) |

Cloud Run supporte HTTP/2 nativement, ce qui est requis pour que ConnectRPC fonctionne sans proxy intermédiaire. La facturation à l'usage est idéale pour un projet portfolio.

### Registre : Google Artifact Registry

Successeur officiel de Container Registry (GCR). Supporte les politiques de rétention d'images, la signature, et s'intègre nativement avec Cloud Run.

### Secrets : Secret Manager

Les trois variables sensibles (`DATABASE_URL`, `API_KEY_SALT`, `SLACK_WEBHOOK_URL`) sont injectées depuis Secret Manager. Elles ne transitent jamais dans un fichier `.tfvars`, une variable d'environnement en clair, ou le state Terraform.

### State : Backend GCS

Le state Terraform est stocké dans un bucket GCS avec versioning activé. Cela évite les conflits en équipe (state locking natif GCS) et permet le rollback en cas de corruption.

---

## Prérequis

Avant de commencer, les éléments suivants doivent être en place :

1. Un projet GCP existant avec facturation activée.
2. `gcloud` CLI installé et authentifié (`gcloud auth application-default login`).
3. Terraform >= 1.9 installé.
4. Un bucket GCS créé manuellement pour le remote state (seule exception à la règle Terraform — un bucket ne peut pas se provisionner lui-même) :
   ```bash
   gcloud storage buckets create gs://zenith-tfstate-<project-id> --location=europe-west1
   gcloud storage buckets update gs://zenith-tfstate-<project-id> --versioning
   ```
5. Les valeurs secrètes disponibles pour être chargées via `gcloud secrets versions add` (étape 7).

---

## Correction Préalable : Dockerfile (`scratch` → `distroless`)

**Problème critique :** L'image `scratch` ne contient aucun certificat CA. La connexion TLS vers CockroachDB Serverless échouera au démarrage du container sur Cloud Run.

**Solution :** Remplacer `scratch` par `gcr.io/distroless/static-debian12`. Cette image inclut les certificats CA système et les fichiers timezone, sans shell ni outils superflus.

**Fichier :** [build/package/Dockerfile](../../../build/package/Dockerfile)

```dockerfile
# Avant
FROM scratch

# Après
FROM gcr.io/distroless/static-debian12
```

Le CockroachDB CA cert (monté en secret K8s en local) sera inclus dans l'image pour Cloud Run via une variable `ZENITH_DB_CACERT_B64` ou en utilisant `sslmode=require` qui délègue la vérification aux CA système (suffisant pour `sslmode=verify-full` si le cert est dans le trust store Debian).

---

## Structure Terraform

```
deployments/terraform/
├── versions.tf               # terraform block, backend GCS, required_providers
├── variables.tf              # Déclarations des variables d'entrée
├── outputs.tf                # URLs et identifiants exportés
├── apis.tf                   # Activation des APIs GCP
├── iam.tf                    # Service account + bindings IAM
├── registry.tf               # Artifact Registry repository
├── secrets.tf                # Secret Manager resources + IAM accessors
├── cloud_run.tf              # Cloud Run service (ingestor)
├── terraform.tfvars.example  # Template de configuration (commité)
└── .gitignore                # *.tfvars, .terraform/, *.tfstate*
```

---

## Étapes d'Implémentation

### Étape 1 — `versions.tf` : Backend & Provider

```hcl
terraform {
  required_version = ">= 1.9"

  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 6.0"
    }
  }

  backend "gcs" {
    bucket = "zenith-tfstate-<project-id>"   # remplacé dans terraform.tfvars
    prefix = "terraform/state"
  }
}

provider "google" {
  project = var.project_id
  region  = var.region
}
```

Le backend GCS offre le state locking natif via les métadonnées d'objet — pas besoin de table DynamoDB comme sur AWS.

---

### Étape 2 — `variables.tf` : Variables d'entrée

```hcl
variable "project_id" {
  description = "ID du projet GCP"
  type        = string
}

variable "region" {
  description = "Région GCP (ex: europe-west1)"
  type        = string
  default     = "europe-west1"
}

variable "environment" {
  description = "Environnement cible (dev, prod)"
  type        = string
  default     = "dev"
}

variable "image_tag" {
  description = "Tag de l'image Docker à déployer (ex: git SHA)"
  type        = string
  default     = "latest"
}

variable "port" {
  description = "Port d'écoute du service"
  type        = number
  default     = 8080  # Cloud Run force $PORT=8080 par défaut
}

variable "db_max_open_conns" {
  type    = number
  default = 25
}

variable "db_max_idle_conns" {
  type    = number
  default = 25
}
```

---

### Étape 3 — `apis.tf` : Activation des APIs GCP

```hcl
locals {
  gcp_services = [
    "run.googleapis.com",
    "artifactregistry.googleapis.com",
    "secretmanager.googleapis.com",
    "iam.googleapis.com",
    "cloudresourcemanager.googleapis.com",
  ]
}

resource "google_project_service" "services" {
  for_each = toset(local.gcp_services)
  service  = each.value

  disable_on_destroy = false  # Ne pas désactiver l'API si on détruit l'infra
}
```

`disable_on_destroy = false` évite de casser d'autres services du projet si on fait `terraform destroy` sur l'environnement dev.

---

### Étape 4 — `iam.tf` : Principe de moindre privilège

```hcl
# Service account dédié au runtime Cloud Run
resource "google_service_account" "zenith_runner" {
  account_id   = "zenith-runner-${var.environment}"
  display_name = "Zenith Cloud Run SA (${var.environment})"
  depends_on   = [google_project_service.services]
}

# Lire les secrets
resource "google_project_iam_member" "secret_accessor" {
  project = var.project_id
  role    = "roles/secretmanager.secretAccessor"
  member  = "serviceAccount:${google_service_account.zenith_runner.email}"
}

# Écrire des logs structurés
resource "google_project_iam_member" "log_writer" {
  project = var.project_id
  role    = "roles/logging.logWriter"
  member  = "serviceAccount:${google_service_account.zenith_runner.email}"
}

# Exposer des métriques (Phase 4 : OpenTelemetry)
resource "google_project_iam_member" "metric_writer" {
  project = var.project_id
  role    = "roles/monitoring.metricWriter"
  member  = "serviceAccount:${google_service_account.zenith_runner.email}"
}
```

Le service account n'a **pas** `roles/editor` ni `roles/owner`. Il ne peut que lire des secrets, écrire des logs et des métriques.

---

### Étape 5 — `registry.tf` : Artifact Registry

```hcl
resource "google_artifact_registry_repository" "zenith" {
  repository_id = "zenith"
  format        = "DOCKER"
  location      = var.region
  description   = "Images Docker pour Zenith"

  cleanup_policies {
    id     = "keep-last-10"
    action = "KEEP"
    most_recent_versions {
      keep_count = 10
    }
  }

  depends_on = [google_project_service.services]
}
```

La `cleanup_policy` évite l'accumulation d'images non taguées qui génèrent des coûts de stockage.

L'URL du registre sera : `{region}-docker.pkg.dev/{project_id}/zenith/ingestor:{tag}`

---

### Étape 6 — `secrets.tf` : Secret Manager

```hcl
locals {
  secrets = ["DATABASE_URL", "API_KEY_SALT", "SLACK_WEBHOOK_URL"]
}

# Déclaration des secrets (sans valeur — les valeurs sont chargées hors Terraform)
resource "google_secret_manager_secret" "zenith_secrets" {
  for_each  = toset(local.secrets)
  secret_id = "zenith-${lower(replace(each.value, "_", "-"))}-${var.environment}"

  replication {
    auto {}
  }

  depends_on = [google_project_service.services]
}

# Donner accès au service account sur chaque secret
resource "google_secret_manager_secret_iam_member" "zenith_runner_access" {
  for_each  = google_secret_manager_secret.zenith_secrets
  secret_id = each.value.id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.zenith_runner.email}"
}
```

> **Important :** Les *valeurs* des secrets ne sont pas dans Terraform (elles n'apparaîtraient pas chiffrées dans le state). Les valeurs sont injectées une seule fois via CLI après le premier `terraform apply` :
> ```bash
> echo -n "postgresql://..." | gcloud secrets versions add zenith-database-url-dev --data-file=-
> ```

---

### Étape 7 — `cloud_run.tf` : Service Cloud Run

```hcl
resource "google_cloud_run_v2_service" "ingestor" {
  name     = "zenith-ingestor-${var.environment}"
  location = var.region

  # HTTP/2 requis pour ConnectRPC (gRPC)
  client = "terraform"

  template {
    service_account = google_service_account.zenith_runner.email

    scaling {
      min_instance_count = 0
      max_instance_count = 3
    }

    containers {
      image = "${var.region}-docker.pkg.dev/${var.project_id}/zenith/ingestor:${var.image_tag}"

      ports {
        name           = "h2c"           # Indique à Cloud Run d'utiliser HTTP/2 (h2c)
        container_port = var.port
      }

      resources {
        limits = {
          cpu    = "1"
          memory = "512Mi"
        }
        cpu_idle = true   # Throttle CPU quand idle (économie)
      }

      # Variables non-sensibles
      env {
        name  = "PORT"
        value = tostring(var.port)
      }
      env {
        name  = "DB_MAX_OPEN_CONNS"
        value = tostring(var.db_max_open_conns)
      }
      env {
        name  = "DB_MAX_IDLE_CONNS"
        value = tostring(var.db_max_idle_conns)
      }

      # Variables sensibles depuis Secret Manager
      dynamic "env" {
        for_each = google_secret_manager_secret.zenith_secrets
        content {
          name = upper(replace(trimprefix(trimprefix(env.value.secret_id, "zenith-"), "-${var.environment}"), "-", "_"))
          value_source {
            secret_key_ref {
              secret  = env.value.secret_id
              version = "latest"
            }
          }
        }
      }

      # Liveness probe : vérifie que le process tourne
      liveness_probe {
        grpc {
          port    = var.port
          service = "grpc.health.v1.Health"
        }
        initial_delay_seconds = 10
        period_seconds        = 30
        failure_threshold     = 3
      }

      # Startup probe : donne plus de temps au démarrage (connexion DB)
      startup_probe {
        grpc {
          port    = var.port
          service = "grpc.health.v1.Health"
        }
        initial_delay_seconds = 5
        period_seconds        = 5
        failure_threshold     = 10
      }
    }
  }

  depends_on = [
    google_project_service.services,
    google_secret_manager_secret_iam_member.zenith_runner_access,
  ]
}

# Accès public (unauthenticated) — à restreindre en production avec un API gateway
resource "google_cloud_run_v2_service_iam_member" "public_invoker" {
  project  = var.project_id
  location = var.region
  name     = google_cloud_run_v2_service.ingestor.name
  role     = "roles/run.invoker"
  member   = "allUsers"
}
```

> **Note sur les probes :** Le service Go doit implémenter le protocole gRPC Health Checking (`grpc.health.v1.Health`). La réflexion gRPC est déjà activée dans `main.go` — il suffit d'enregistrer le `healthServer` avec `grpc_health_v1.RegisterHealthServer`. Sans ça, utiliser une probe HTTP sur `/healthz`.

---

### Étape 8 — `outputs.tf` : Valeurs exportées

```hcl
output "service_url" {
  description = "URL publique du service Cloud Run"
  value       = google_cloud_run_v2_service.ingestor.uri
}

output "registry_url" {
  description = "URL de base du registre Artifact Registry"
  value       = "${var.region}-docker.pkg.dev/${var.project_id}/zenith"
}

output "service_account_email" {
  description = "Email du service account Cloud Run"
  value       = google_service_account.zenith_runner.email
}
```

Ces outputs seront consommés par le pipeline GitHub Actions (Issue-502) via `terraform output -json`.

---

### Étape 9 — `terraform.tfvars.example` & `.gitignore`

**`terraform.tfvars.example`** (commité) :
```hcl
project_id  = "my-gcp-project-id"
region      = "europe-west1"
environment = "dev"
image_tag   = "latest"
```

**`.gitignore`** dans `deployments/terraform/` :
```
# State local (ne jamais commiter)
*.tfstate
*.tfstate.*
.terraform/
.terraform.lock.hcl

# Fichiers de variables réels (contiennent des valeurs sensibles)
*.tfvars
!*.tfvars.example
```

---

## Workflow d'Exécution

```bash
# 1. Initialiser avec le backend GCS
cd deployments/terraform
terraform init \
  -backend-config="bucket=zenith-tfstate-<project-id>"

# 2. Créer terraform.tfvars (non commité)
cp terraform.tfvars.example terraform.tfvars
# Éditer avec les vraies valeurs

# 3. Valider la syntaxe
terraform fmt -recursive
terraform validate

# 4. Prévisualiser sans appliquer
terraform plan -out=tfplan

# 5. Appliquer l'infrastructure
terraform apply tfplan

# 6. Charger les valeurs secrètes (une seule fois, hors Terraform)
echo -n "postgresql://..." | gcloud secrets versions add \
  zenith-database-url-dev --data-file=-

# 7. Vérifier l'endpoint
SERVICE_URL=$(terraform output -raw service_url)
grpcurl -proto api/proto/v1/event.proto $SERVICE_URL \
  proto.v1.IngestorService/IngestEvent
```

---

## Sécurité : Points de Vigilance

| Risque | Mitigation |
|---|---|
| Secrets dans le state Terraform | Les valeurs des secrets ne passent **jamais** dans Terraform — `google_secret_manager_secret` déclare le conteneur, pas la valeur |
| Accès public sans authentification | `allUsers` invoker est intentionnel pour Phase 3 ; à remplacer par Cloud Endpoints ou Identity-Aware Proxy en Phase 4 |
| Image basée sur `scratch` sans CA certs | Migration vers `distroless/static-debian12` (étape préalable) |
| État Terraform accessible | Bucket GCS avec accès restreint au SA Terraform et à l'administrateur du projet |
| `image_tag = "latest"` non reproductible | Le pipeline CI/CD (Issue-502) surchargera `image_tag` avec le SHA Git |

---

## Livrable : Checklist de Validation

- [ ] `terraform fmt -recursive` — aucune modification
- [ ] `terraform validate` — succès
- [ ] `terraform plan` — aucune ressource inattendue (`0 to destroy`)
- [ ] `terraform apply` — toutes les ressources créées sans erreur
- [ ] `terraform output service_url` — retourne une URL HTTPS
- [ ] Valeurs secrètes chargées via `gcloud secrets versions add`
- [ ] Image buildée, taguée et pushée vers Artifact Registry
- [ ] Cloud Run service en état `ACTIVE`
- [ ] Appel `grpcurl` ou `curl` sur l'URL publique — réponse valide
- [ ] Aucun fichier `*.tfstate` ou `*.tfvars` commité (`git status` propre)
