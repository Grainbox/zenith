# Monitoring configuration for GCP Managed Prometheus (GMP) and observability
#
# IMPORTANT: Cloud Run metrics scraping
# GCP Managed Prometheus does not natively support pull-scraping from Cloud Run services
# (Cloud Run instances don't have stable IPs accessible to GMP scrape jobs).
#
# For Cloud Run observability:
# - Option 1: Use OpenTelemetry (already configured in Issue-701) to push metrics to GCP Cloud Trace
# - Option 2: Use Prometheus remote_write sidecar to push metrics to GMP
# - Option 3: Use GCP Cloud Monitoring custom metrics API
#
# For Kubernetes (Kind, GKE):
# - Use PodMonitoring CRD (requires GMP operator: kubectl apply -f https://github.com/GoogleCloudPlatform/prometheus-engine/releases/latest/download/operator.yaml)
# - K8s service discovery + prometheus.io annotations work automatically
#
# The monitoring.googleapis.com API is enabled in apis.tf

# Note: GCP Managed Prometheus is automatically enabled for this project
# via the monitoring.googleapis.com API in apis.tf
# No additional resource creation needed for basic GMP functionality

# Output information for operators
output "monitoring_info" {
  description = "Monitoring configuration details"
  value = {
    gmp_enabled  = true
    metrics_port = 8082
    note         = "For Cloud Run: metrics are pushed via OTEL (Issue-701). For K8s: use PodMonitoring CRD with GMP operator."
  }
}
