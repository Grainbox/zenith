# 🚀 Roadmap Détaillée : Phase 1 - ZENITH (Semaines 1 & 2)

**Objectif Global :** Passer du "scripting" à "l'architecture logicielle" et maîtriser l'écosystème Go. Poser les bases d'un projet "Cloud-Native Consultant-Grade".

---

## 🏃 Sprint 1 : Fondations Projet & Contrats d'Interface (Semaine 1)

**Objectif du Sprint :** Initialiser le projet Go proprement (Standard Layout), configurer l'outillage de qualité, et définir les contrats gRPC/Protobuf.

### 📝 Backlog du Sprint 1

*   **[Issue-101] Initialisation du Module Go et de la Structure du Projet**
    *   **Description :** Création du dépôt git et initialisation du `go.mod`. Mise en place de l'arborescence standard "Standard Go Project Layout" (`/cmd`, `/internal`, `/pkg`, `/api`, `/build`, `/deployments`, etc.).
    *   **Livrables :**
        *   Fichier `go.mod` avec la version Go 1.24+ configurée.
        *   Dossiers standards créés.
        *   Un `main.go` basique dans `/cmd/zenith/main.go` affichant juste "Zenith is starting".

*   **[Issue-102] Configuration de l'Outillage et de la Qualité (Linter)**
    *   **Description :** Installation et configuration de `golangci-lint` pour garantir un code Go idiomatique et propre dès le départ. Configuration des règles strictes (Consultant-Grade).
    *   **Livrables :**
        *   Fichier `.golangci.yml` à la racine du projet avec les linters activés (ex: errcheck, revive, govet, staticcheck, etc.).
        *   Documentation dans un `CONTRIBUTING.md` sur comment lancer le linter localement (`golangci-lint run`).

*   **[Issue-103] Définition des Contrats Protocol Buffers (v1)**
    *   **Description :** Création des fichiers `.proto` qui définiront les structures de données (Events) et le service gRPC pour l'Ingestor.
    *   **Livrables :**
        *   Dossier `/api/proto/v1/event.proto`.
        *   Définition du message `Event` (ID, Source, Timestamp, Payload JSON/Bytes).
        *   Définition du service `IngestorService` avec une méthode `IngestEvent`.

*   **[Issue-104] Génération de Code Go depuis Protobuf**
    *   **Description :** Configuration de `protoc` (ou `buf`) pour compiler automatiquement les fichiers `.proto` en code Go utilisable par l'application.
    *   **Livrables :**
        *   Script Bash, `Makefile` ou configuration `buf.yaml` pour automatiser la génération.
        *   Code Go généré (ex: sous `/pkg/pb/v1/`).

*   **[Issue-105] [CKAD] Setup de l'Environnement Local Kubernetes**
    *   **Description :** Installation et configuration de `minikube` ou `kind` sur la machine de développement locale pour les futures expérimentations Kubernetes.
    *   **Livrables :**
        *   Outil CLI (`kubectl`, `minikube` / `kind`) installé et fonctionnel.
        *   Savoir démarrer et arrêter un cluster local.

---

## 🏃 Sprint 2 : Squelette gRPC & Déploiement K8s Basique (Semaine 2)

**Objectif du Sprint :** Implémenter le serveur gRPC capable de recevoir un "ping" et déployer ce serveur sur le cluster Kubernetes local.

### 📝 Backlog du Sprint 2

*   **[Issue-201] Implémentation du Serveur gRPC (Ingestor - Skeleton)**
    *   **Description :** Développement du composant serveur gRPC en utilisant le code généré à [Issue-104]. Le serveur doit écouter sur un port (ex: 50051) et implémenter l'interface `IngestorService`.
    *   **Livrables :**
        *   Code dans `/internal/ingestor/server.go`.
        *   Mise à jour de `/cmd/zenith/main.go` pour démarrer le serveur gRPC.
        *   Logs structurés (ex: via `slog`) confirmant le démarrage du serveur.

*   **[Issue-202] Gestion du Signal (Graceful Shutdown - Basique)**
    *   **Description :** Implémenter l'écoute des signaux OS (SIGINT, SIGTERM) pour stopper proprement le serveur gRPC, permettant aux requêtes en cours de se terminer (crucial pour la résilience).
    *   **Livrables :**
        *   Code dans `main.go` utilisant `os/signal` pour intercepter les signaux d'arrêt et appeler `server.GracefulStop()`.

*   **[Issue-203] Implémentation du Handler `IngestEvent` (Ping)**
    *   **Description :** Coder la logique basique dans le handler pour recevoir l'évènement, le loguer (console/slog) en tant que "ping reçu", et renvoyer une réponse de succès (Ack).
    *   **Livrables :**
        *   Logique fonctionnelle pour la méthode `IngestEvent`.

*   **[Issue-204] Tests Unitaires Initiaux (Ingestor)**
    *   **Description :** Mettre en place la librairie `stretchr/testify` et écrire le premier test unitaire pour vérifier que le handler `IngestEvent` traite correctement une requête mockée.
    *   **Livrables :**
        *   Import de `testify` dans `go.mod`.
        *   Fichier `/internal/ingestor/server_test.go`.

*   **[Issue-205] [CKAD] Création des Manifestes Kubernetes (Pod & Namespace)**
    *   **Description :** Créer les premiers fichiers YAML pour déployer l'application sur le cluster local. Pratique de la création de manifestes sans GUI (impératif ou déclaratif).
    *   **Livrables :**
        *   Dossier `/deployments/k8s/local/`.
        *   `namespace.yaml` (ex: `zenith-dev`).
        *   `pod.yaml` (définissant un Pod simple, potentiellement avec une image Docker temporaire avant dockerisation). *Note: Nécessitera un `Dockerfile` basique (Issue-206) si on veut déployer notre propre code dès maintenant.*

*   **[Issue-206] Containerisation Basiqe (Dockerfile)**
    *   **Description :** Créer un `Dockerfile` multi-stage pour compiler l'application Go et construire une image lègère (Alpine ou Scratch) contenant uniquement l'exécutable.
    *   **Livrables :**
        *   Fichier `build/package/Dockerfile`.

*   **[Issue-207] Validation Finale du Jalon 1 (Milestone)**
    *   **Description :** Test d'intégration de bout en bout localement. Démarrer le serveur gRPC et utiliser un client gRPC (comme `grpcurl` ou `Postman`) pour envoyer un événement "ping" et vérifier le log de réception et l'Ack.
    *   **Livrables :**
        *   Démonstration (ou logs) prouvant que le "ping" est reçu et acquitté par le système.
