# Plan d'implémentation — [Issue-502] GitHub Actions CI/CD Pipeline

## Objectif

Construire un pipeline GitHub Actions qui, à chaque push sur `main`, exécute automatiquement la chaîne complète : lint → test → build → push → deploy. Le déploiement se fait en mettant à jour Cloud Run via `terraform apply` avec le SHA Git comme tag d'image, garantissant une traçabilité totale entre chaque commit et le container en production.

---

## Choix Technologiques & Justifications

### Authentification GCP : Workload Identity Federation

| Critère | Workload Identity Federation | Service Account Key (JSON) |
|---|---|---|
| Secret à stocker dans GitHub | Provider ID + SA email (non-secrets) | Clé JSON longue durée (risque de fuite) |
| Rotation | Automatique (OIDC token, 1h) | Manuelle (risk d'oubli) |
| Révocation immédiate | Oui (suppression du binding IAM) | Non (clé active jusqu'à expiration) |
| Complexité de setup | Modérée (1 fois) | Faible |

Workload Identity Federation est la pratique recommandée par Google. GitHub émet un token OIDC à chaque run — GCP le valide sans stocker aucune clé longue durée dans les secrets GitHub.

### Stratégie de déploiement : `terraform apply` piloté par CI

Le pipeline surcharge `var.image_tag` avec le SHA Git court. Terraform met à jour la ressource `google_cloud_run_v2_service` uniquement si l'image change, ce qui garantit que le state Terraform reste la source de vérité unique pour l'infrastructure.

### Tests d'intégration : exclus du pipeline (pour l'instant)

Les tests `internal/repository/postgres/` utilisent `testcontainers-go` et nécessitent Docker-in-Docker. Le pipeline n'exécute que les tests unitaires (`-short`). Un job séparé avec `services: cockroachdb` peut être ajouté en Phase 4.

---

## Prérequis

Avant d'écrire le workflow, les éléments suivants doivent être configurés **une seule fois** en dehors du repo :

1. Un Service Account GCP dédié au CI (distinct du SA runtime `zenith-runner`).
2. Un pool Workload Identity lié au repo GitHub.
3. Les secrets GitHub configurés dans les Settings du repo.

---

## Étape 0 — Service Account CI & Workload Identity Federation

### 0a. Créer le Service Account CI

```bash
gcloud iam service-accounts create zenith-ci \
  --display-name="Zenith GitHub Actions CI" \
  --project=<PROJECT_ID>
```

### 0b. Attribuer les rôles nécessaires

```bash
SA_EMAIL="zenith-ci@<PROJECT_ID>.iam.gserviceaccount.com"

# Pousser des images vers Artifact Registry
gcloud projects add-iam-policy-binding <PROJECT_ID> \
  --member="serviceAccount:${SA_EMAIL}" \
  --role="roles/artifactregistry.writer"

# Déployer sur Cloud Run (mise à jour du service existant)
gcloud projects add-iam-policy-binding <PROJECT_ID> \
  --member="serviceAccount:${SA_EMAIL}" \
  --role="roles/run.developer"

# Lire et écrire le state Terraform dans GCS
gsutil iam ch serviceAccount:${SA_EMAIL}:roles/storage.objectAdmin \
  gs://zenith-tfstate-<PROJECT_ID>

# Accéder aux secrets (requis par terraform plan pour lire les références)
gcloud projects add-iam-policy-binding <PROJECT_ID> \
  --member="serviceAccount:${SA_EMAIL}" \
  --role="roles/secretmanager.viewer"

# Gérer les ressources IAM et Cloud Run (terraform apply)
gcloud projects add-iam-policy-binding <PROJECT_ID> \
  --member="serviceAccount:${SA_EMAIL}" \
  --role="roles/iam.serviceAccountUser"
```

> **Principe de moindre privilège :** Le SA CI n'a pas `roles/editor`. Il peut uniquement pousser des images, mettre à jour Cloud Run, et gérer le state Terraform.

### 0c. Créer le pool Workload Identity

```bash
# Créer le pool
gcloud iam workload-identity-pools create "github-pool" \
  --project=<PROJECT_ID> \
  --location="global" \
  --display-name="GitHub Actions Pool"

# Créer le provider OIDC GitHub
gcloud iam workload-identity-pools providers create-oidc "github-provider" \
  --project=<PROJECT_ID> \
  --location="global" \
  --workload-identity-pool="github-pool" \
  --display-name="GitHub Actions Provider" \
  --attribute-mapping="google.subject=assertion.sub,attribute.repository=assertion.repository,attribute.actor=assertion.actor,attribute.aud=assertion.aud" \
  --issuer-uri="https://token.actions.githubusercontent.com"

# Autoriser uniquement le repo Zenith à se faire passer pour le SA CI
gcloud iam service-accounts add-iam-policy-binding "${SA_EMAIL}" \
  --project=<PROJECT_ID> \
  --role="roles/iam.workloadIdentityUser" \
  --member="principalSet://iam.googleapis.com/projects/<PROJECT_NUMBER>/locations/global/workloadIdentityPools/github-pool/attribute.repository/<GITHUB_ORG>/<REPO_NAME>"
```

> Remplacer `<PROJECT_NUMBER>` (≠ `<PROJECT_ID>`) par le numéro numérique obtenu via `gcloud projects describe <PROJECT_ID> --format='value(projectNumber)'`.

### 0d. Récupérer les identifiants du provider

```bash
# Valeur pour le secret WORKLOAD_IDENTITY_PROVIDER
gcloud iam workload-identity-pools providers describe github-provider \
  --project=<PROJECT_ID> \
  --location=global \
  --workload-identity-pool=github-pool \
  --format="value(name)"
# Résultat: projects/<NUMBER>/locations/global/workloadIdentityPools/github-pool/providers/github-provider
```

---

## Étape 1 — Secrets GitHub

Aller dans **Settings → Secrets and variables → Actions → New repository secret** et ajouter :

| Nom du secret | Valeur |
|---|---|
| `GCP_PROJECT_ID` | ID du projet GCP (ex: `my-zenith-project`) |
| `GCP_WORKLOAD_IDENTITY_PROVIDER` | Chemin complet du provider (obtenu à l'étape 0d) |
| `GCP_SERVICE_ACCOUNT` | Email du SA CI (ex: `zenith-ci@my-zenith-project.iam.gserviceaccount.com`) |
| `TF_BACKEND_BUCKET` | Nom du bucket GCS (ex: `zenith-tfstate-my-zenith-project`) |

Ces quatre valeurs sont les **seuls** secrets nécessaires. Aucune clé JSON, aucun mot de passe.

---

## Étape 2 — Structure du Workflow

```
.github/
└── workflows/
    └── deploy.yml          # Pipeline principal (lint → test → build → deploy)
```

Le pipeline est composé de **4 jobs** avec des dépendances explicites :

```
lint ──┐
       ├──► build-push ──► deploy  (push to main uniquement)
test ──┘
```

- `lint` et `test` sont **parallèles** (pas de dépendance entre eux).
- `build-push` attend que les deux passent.
- `deploy` n'est déclenché que sur push vers `main` (pas sur les PRs).

---

## Étape 3 — `.github/workflows/deploy.yml`

```yaml
name: CI/CD Pipeline

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

env:
  # Région GCP — doit correspondre à deployments/terraform/variables.tf
  GCP_REGION: europe-west1
  # Tag de l'image : SHA Git court (7 caractères) pour la traçabilité
  IMAGE_TAG: ${{ github.sha }}

jobs:
  # ──────────────────────────────────────────────────────────────────
  # Job 1: Lint — golangci-lint + buf lint
  # ──────────────────────────────────────────────────────────────────
  lint:
    name: Lint
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
          cache: true

      - name: Run golangci-lint
        uses: golangci/golangci-lint-action@v6
        with:
          version: latest

      - name: Set up buf
        uses: bufbuild/buf-action@v1
        with:
          setup_only: true

      - name: Run buf lint
        run: buf lint

  # ──────────────────────────────────────────────────────────────────
  # Job 2: Test — tests unitaires Go (sans tests d'intégration)
  # ──────────────────────────────────────────────────────────────────
  test:
    name: Test
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
          cache: true

      - name: Run unit tests
        # -short exclut les tests d'intégration (testcontainers nécessitent Docker)
        run: go test -short -race -count=1 ./...

  # ──────────────────────────────────────────────────────────────────
  # Job 3: Build & Push — image Docker vers Artifact Registry
  # ──────────────────────────────────────────────────────────────────
  build-push:
    name: Build & Push
    runs-on: ubuntu-latest
    needs: [lint, test]
    # Uniquement sur push (pas sur les PRs pour ne pas pousser d'images non validées)
    if: github.event_name == 'push'

    permissions:
      contents: read
      id-token: write   # Requis pour Workload Identity Federation

    steps:
      - name: Checkout
        uses: actions/checkout@v4

      - name: Authenticate to Google Cloud
        id: auth
        uses: google-github-actions/auth@v2
        with:
          workload_identity_provider: ${{ secrets.GCP_WORKLOAD_IDENTITY_PROVIDER }}
          service_account: ${{ secrets.GCP_SERVICE_ACCOUNT }}

      - name: Configure Docker for Artifact Registry
        run: gcloud auth configure-docker ${{ env.GCP_REGION }}-docker.pkg.dev --quiet

      - name: Build Docker image
        run: |
          docker build \
            -f build/package/Dockerfile \
            -t ${{ env.GCP_REGION }}-docker.pkg.dev/${{ secrets.GCP_PROJECT_ID }}/zenith/ingestor:${{ env.IMAGE_TAG }} \
            -t ${{ env.GCP_REGION }}-docker.pkg.dev/${{ secrets.GCP_PROJECT_ID }}/zenith/ingestor:latest \
            .

      - name: Push Docker image
        run: |
          docker push ${{ env.GCP_REGION }}-docker.pkg.dev/${{ secrets.GCP_PROJECT_ID }}/zenith/ingestor:${{ env.IMAGE_TAG }}
          docker push ${{ env.GCP_REGION }}-docker.pkg.dev/${{ secrets.GCP_PROJECT_ID }}/zenith/ingestor:latest

  # ──────────────────────────────────────────────────────────────────
  # Job 4: Deploy — terraform apply avec le nouveau tag d'image
  # ──────────────────────────────────────────────────────────────────
  deploy:
    name: Deploy
    runs-on: ubuntu-latest
    needs: [build-push]
    # Uniquement sur push vers main (pas sur les PRs, pas sur les autres branches)
    if: github.event_name == 'push' && github.ref == 'refs/heads/main'

    permissions:
      contents: read
      id-token: write

    defaults:
      run:
        working-directory: deployments/terraform

    steps:
      - name: Checkout
        uses: actions/checkout@v4

      - name: Authenticate to Google Cloud
        uses: google-github-actions/auth@v2
        with:
          workload_identity_provider: ${{ secrets.GCP_WORKLOAD_IDENTITY_PROVIDER }}
          service_account: ${{ secrets.GCP_SERVICE_ACCOUNT }}

      - name: Set up Terraform
        uses: hashicorp/setup-terraform@v3
        with:
          terraform_version: "~> 1.9"

      - name: Terraform Init
        run: |
          terraform init \
            -backend-config="bucket=${{ secrets.TF_BACKEND_BUCKET }}"

      - name: Terraform Plan
        run: |
          terraform plan \
            -var="project_id=${{ secrets.GCP_PROJECT_ID }}" \
            -var="image_tag=${{ env.IMAGE_TAG }}" \
            -out=tfplan \
            -no-color
        # -no-color pour des logs lisibles dans GitHub Actions

      - name: Terraform Apply
        run: terraform apply -auto-approve tfplan

      - name: Get deployed URL
        run: |
          SERVICE_URL=$(terraform output -raw service_url)
          echo "✅ Deployed to: ${SERVICE_URL}" >> $GITHUB_STEP_SUMMARY
          echo "SERVICE_URL=${SERVICE_URL}" >> $GITHUB_ENV
```

---

## Étape 4 — Gestion des variables Terraform non-secrètes

Le fichier `deployments/terraform/variables.tf` définit déjà `project_id` et `image_tag`. Le pipeline les surcharge via `-var` sans créer de fichier `.tfvars`.

Les variables avec des valeurs par défaut (`region`, `environment`, `port`, etc.) ne sont pas surchargées — leurs défauts dans `variables.tf` s'appliquent.

> **Pourquoi ne pas stocker `project_id` dans les défauts de `variables.tf` ?** Car `project_id` varie selon les environnements. Le passer via secret GitHub permet de réutiliser le même workflow pour un éventuel environnement `staging` avec un autre projet GCP.

---

## Étape 5 — Comportement par type d'événement

| Événement | lint | test | build-push | deploy |
|---|:---:|:---:|:---:|:---:|
| PR vers `main` | ✅ | ✅ | ❌ | ❌ |
| Push vers `main` | ✅ | ✅ | ✅ | ✅ |

Les PRs ne déclenchent que la validation (lint + test), sans pousser d'image ni déployer. Cela évite de polluer Artifact Registry avec des images de branches non mergées.

---

## Étape 6 — Résumé de déploiement dans GitHub Actions

Le job `deploy` écrit dans `$GITHUB_STEP_SUMMARY` l'URL publique du service après chaque déploiement réussi. L'URL est visible directement dans l'interface GitHub Actions sans avoir à aller sur la console GCP.

---

## Sécurité : Points de Vigilance

| Risque | Mitigation |
|---|---|
| Clé SA longue durée dans GitHub Secrets | Workload Identity Federation — aucune clé JSON stockée |
| `terraform apply` sans review humaine | Acceptable en Phase 3 (projet solo) ; à remplacer par PR-based `plan` + approbation manuelle en équipe |
| Image `latest` non reproductible | `latest` est poussé pour faciliter le debug ; le tag SHA est celui réellement déployé |
| `secrets.GCP_PROJECT_ID` dans les logs | Les secrets GitHub sont masqués automatiquement dans les logs Actions |
| `terraform apply -auto-approve` | Protégé par le `terraform plan` préalable dans le même job — le plan échoue si une destruction inattendue est détectée |
| Accès au state GCS depuis CI | Le SA CI a `storage.objectAdmin` uniquement sur le bucket de state, pas sur l'ensemble du projet |

---

## Livrable : Checklist de Validation

- [x] `zenith-ci` Service Account créé avec les rôles minimaux
- [x] Workload Identity Pool et Provider configurés sur GCP
- [x] Binding IAM restreint au repo `<ORG>/<REPO>` (pas à toute l'organisation)
- [x] 4 secrets GitHub configurés (`GCP_PROJECT_ID`, `GCP_WORKLOAD_IDENTITY_PROVIDER`, `GCP_SERVICE_ACCOUNT`, `TF_BACKEND_BUCKET`)
- [x] `.github/workflows/deploy.yml` commité sur `main`
- [x] Premier run du pipeline : tous les jobs passent au vert
- [x] Image visible dans Artifact Registry avec le SHA comme tag
- [x] `terraform output service_url` retourne l'URL du service mis à jour
- [x] Appel `curl` ou `grpcurl` sur l'URL publique après le déploiement automatique
- [x] Sur une PR : seuls `lint` et `test` s'exécutent (pas de build ni deploy)
