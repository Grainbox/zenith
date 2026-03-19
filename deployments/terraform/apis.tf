locals {
  gcp_services = [
    "run.googleapis.com",
    "artifactregistry.googleapis.com",
    "secretmanager.googleapis.com",
    "iam.googleapis.com",
    "cloudresourcemanager.googleapis.com",
    "monitoring.googleapis.com",
  ]
}

resource "google_project_service" "services" {
  for_each = toset(local.gcp_services)
  service  = each.value

  disable_on_destroy = false
}
