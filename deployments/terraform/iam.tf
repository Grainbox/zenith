resource "google_service_account" "zenith_runner" {
  account_id   = "zenith-runner-${var.environment}"
  display_name = "Zenith Cloud Run SA (${var.environment})"
  depends_on   = [google_project_service.services]
}

resource "google_project_iam_member" "log_writer" {
  project = var.project_id
  role    = "roles/logging.logWriter"
  member  = "serviceAccount:${google_service_account.zenith_runner.email}"
}

resource "google_project_iam_member" "metric_writer" {
  project = var.project_id
  role    = "roles/monitoring.metricWriter"
  member  = "serviceAccount:${google_service_account.zenith_runner.email}"
}
