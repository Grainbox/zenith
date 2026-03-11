# 🚀 Roadmap: From Student to Cloud-Native Consultant (8-Week Sprint)

## PHASE 1: Foundations & Software Design (Weeks 1 & 2)

*Goal: Shift from "scripting" to "software architecture" and master the Go ecosystem.*

* **Engineering (Go & Proto):**
* Initialize the project using the [Standard Go Project Layout](https://github.com/golang-standards/project-layout).
* Define `.proto` files (the interface contract for Zenith) and generate Go code.
* Setup the Development Environment: `golangci-lint` for code quality and initial unit tests with `stretchr/testify`.


* **Certification (Kubernetes/CKAD):**
* Local setup: Install `minikube` or `kind`.
* Core Objects: Master Pods, Namespaces, and YAML manifest creation without a GUI.


* **Milestone:** A functional gRPC skeleton capable of receiving and logging a simple "ping" event.

## PHASE 2: Persistence & Distributed Logic (Weeks 3 & 4)

*Goal: Connect the "Brain" (Rule Engine) to "Memory" (CockroachDB) while handling concurrency.*

* **Engineering (DB & Concurrency):**
* Provision a **CockroachDB Serverless** instance (NewSQL).
* Implement the Rule Engine: Use *Goroutines* and *Channels* for non-blocking event filtering.
* Resilience: Implement "Graceful Shutdown" logic to ensure zero event loss during deployments.


* **Certification (Kubernetes/CKAD):**
* Persistence & Config: Master Volumes (PV, PVC), ConfigMaps, and Secrets.
* Configuration: Learn to inject environment variables and mount volumes into containers.


* **Milestone:** The engine runs locally, evaluates rules against a real database, and handles high-concurrency stress tests.

## PHASE 3: Infrastructure as Code & Cloud (Weeks 5 & 6)

*Goal: Become "Cloud-Native" by automating the entire infrastructure lifecycle.*

* **Engineering (Terraform & CI/CD):**
* Write **Terraform** scripts to provision Google Cloud Run or AWS Fargate and networking.
* Build a **GitHub Actions** pipeline for "Continuous Deployment" (Auto-deploy on `git push`).
* Add a REST Gateway (using `grpc-gateway` or `Gin`) to accept standard Webhooks.


* **Certification (Kubernetes/CKAD):**
* Workload Management: Master Deployments, Rolling Updates, and Services (ClusterIP, NodePort, LoadBalancer).
* Probes: Implement Liveness and Readiness probes to ensure zero-downtime.


* **Milestone:** Zenith is live on a public URL, fully provisioned via Code (IaC), with automated deployments.

## PHASE 4: Observability & Certification Final (Weeks 7 & 8)

*Goal: Polish the "Marketable" product and secure the industry-standard credential.*

* **Engineering (Production-Grade Polish):**
* Instrumentation: Integrate **OpenTelemetry** for distributed tracing and Prometheus for metrics.
* Documentation: Write a comprehensive `README.md` with architecture diagrams (Mermaid) and a `BENCHMARK.md`.
* Portfolio: Clean up the GitHub repo to look like a professional open-source project.


* **Certification (CKAD):**
* Intensive Training: Complete the *Killer.sh* mock exam (included with the CKAD voucher) until you score 90%+.
* **Final Exam: Sit for the CKAD certification.**


* **Milestone:** A "Production-Ready" project on GitHub and a CKAD badge on your LinkedIn profile.

---

### 💡 Critical Success Factors:

1. **Week 4 Deadline:** The core Go logic must be finished. If behind, simplify rule types, but do not sacrifice code quality.
2. **The "Terraform" Rule:** Never create resources manually in the Cloud Console. If it’s not in Terraform, it doesn’t exist.
3. **Muscle Memory:** Spend 30 minutes every day on `kubectl` imperative commands. Speed is the biggest challenge for the CKAD exam.
