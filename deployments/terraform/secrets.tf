locals {
  secrets = ["DATABASE_URL", "API_KEY_SALT", "SLACK_WEBHOOK_URL"]
}

# Reference existing secrets (created manually via gcloud)
data "google_secret_manager_secret" "zenith_secrets" {
  for_each = toset(local.secrets)
  secret_id = "zenith-${lower(replace(each.value, "_", "-"))}-${var.environment}"
}

resource "google_secret_manager_secret_iam_member" "zenith_runner_access" {
  for_each  = data.google_secret_manager_secret.zenith_secrets
  secret_id = each.value.id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.zenith_runner.email}"
}
