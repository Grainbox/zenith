output "service_url" {
  description = "Public URL of the Ingestor Cloud Run service"
  value       = google_cloud_run_v2_service.ingestor.uri
}

output "dispatcher_url" {
  description = "Internal URL of the Dispatcher Cloud Run service"
  value       = google_cloud_run_v2_service.dispatcher.uri
}

output "registry_url" {
  description = "Base URL of the Artifact Registry"
  value       = "${var.region}-docker.pkg.dev/${var.project_id}/zenith"
}

output "service_account_email" {
  description = "Email of the Cloud Run service account"
  value       = google_service_account.zenith_runner.email
}
