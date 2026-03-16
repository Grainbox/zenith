resource "google_artifact_registry_repository" "zenith" {
  repository_id = "zenith"
  format        = "DOCKER"
  location      = var.region
  description   = "Docker images for Zenith"

  cleanup_policies {
    id     = "keep-last-10"
    action = "KEEP"
    most_recent_versions {
      keep_count = 10
    }
  }

  depends_on = [google_project_service.services]
}
