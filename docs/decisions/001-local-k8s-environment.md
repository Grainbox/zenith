# Architecture Decision Record: Local Kubernetes Environment Choice

## Context

For the development of the **Zenith** project (Go gRPC/Connect Ingestor) and preparation for the **CKAD** certification, we need a local Kubernetes environment capable of simulating a production environment while remaining lightweight and reproducible.

## Technical Comparison

| Feature | Minikube | Kind (Kubernetes in Docker) |
| --- | --- | --- |
| **Isolation** | Virtual Machine or Container | Docker Container only |
| **Boot Speed** | Slow (1 to 3 min) | Ultra-fast (< 1 min) |
| **Resources** | Heavy (dedicated RAM/CPU) | Very light (shares Docker resources) |
| **Multi-node** | Complex / Heavy | Native and trivial |
| **Addons** | `minikube addons` (Abstract) | Manual configuration (Realistic) |

---

## Why choose Kind for Zenith?

### 1. Alignment with Go & Docker Philosophy

Zenith is a "Cloud-Native" project. **Kind** runs Kubernetes *inside* Docker. This "Docker-in-Docker" approach is more consistent with our current workflow. It allows us to test our Ingestor images locally without having to push them to a remote registry (via the `kind load docker-image` command).

### 2. Fidelity to CKAD (No Magic)

Minikube offers shortcuts like `minikube addons enable ingress`. While convenient, these commands hide the actual complexity of Kubernetes.

* **For the CKAD**, it is crucial to know how to manipulate raw YAML files to install an Ingress controller or configure a NetworkPolicy.
* Kind forces us to install these components manually, which is excellent training for the exam.

### 3. High Availability (HA) Simulation

Kind allows defining a multi-node cluster (e.g., 1 control-plane and 2 workers) via a simple YAML file. For Zenith, this will allow us to test the behavior of our gRPC service during a multi-node deployment very early.

---

## Conclusion

We choose **Kind** for its operational efficiency and its ability to prepare us for the technical realities of the CKAD. The environment remains lightweight, allowing us to code on Zenith without slowing down the development machine, while ensuring full portability with future production environments (GCP/AWS).

---

## Validation Commands (Proof of Concept)

```bash
# 1. Create a multi-node cluster for Zenith
cat <<EOF | kind create cluster --name zenith-lab --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
- role: worker
- role: worker
EOF

# 2. Check cluster status
kubectl get nodes
```
