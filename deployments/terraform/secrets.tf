locals {
  secrets = ["DATABASE_URL", "API_KEY_SALT", "SLACK_WEBHOOK_URL"]

  secret_values = {
    "DATABASE_URL"     = var.database_url
    "API_KEY_SALT"     = var.api_key_salt
    "SLACK_WEBHOOK_URL" = var.slack_webhook_url
  }
}

resource "google_secret_manager_secret" "zenith_secrets" {
  for_each  = toset(local.secrets)
  secret_id = "zenith-${lower(replace(each.value, "_", "-"))}-${var.environment}"

  replication {
    auto {}
  }
}

resource "google_secret_manager_secret_version" "zenith_secrets" {
  for_each    = { for k, v in local.secret_values : k => v if v != "" }
  secret      = google_secret_manager_secret.zenith_secrets[each.key].id
  secret_data = each.value
}

resource "google_secret_manager_secret_iam_member" "zenith_runner_access" {
  for_each  = google_secret_manager_secret.zenith_secrets
  secret_id = each.value.id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.zenith_runner.email}"
}
