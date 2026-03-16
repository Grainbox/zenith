resource "google_cloud_run_v2_service" "ingestor" {
  name                = "zenith-ingestor-${var.environment}"
  location            = var.region
  deletion_protection = false

  client = "terraform"

  template {
    service_account = google_service_account.zenith_runner.email

    scaling {
      min_instance_count = 1
      max_instance_count = 3
    }

    containers {
      image = "${var.region}-docker.pkg.dev/${var.project_id}/zenith/ingestor:${var.image_tag}"

      ports {
        name           = "h2c"
        container_port = var.port
      }

      resources {
        limits = {
          cpu    = "1"
          memory = "512Mi"
        }
        cpu_idle = true
      }

      # Non-sensitive environment variables
      # Note: PORT is automatically set by Cloud Run to 8080
      env {
        name  = "DB_MAX_OPEN_CONNS"
        value = tostring(var.db_max_open_conns)
      }

      env {
        name  = "DB_MAX_IDLE_CONNS"
        value = tostring(var.db_max_idle_conns)
      }

      # Sensitive variables from Secret Manager
      env {
        name = "DATABASE_URL"
        value_source {
          secret_key_ref {
            secret  = data.google_secret_manager_secret.zenith_secrets["DATABASE_URL"].secret_id
            version = "latest"
          }
        }
      }

      env {
        name = "API_KEY_SALT"
        value_source {
          secret_key_ref {
            secret  = data.google_secret_manager_secret.zenith_secrets["API_KEY_SALT"].secret_id
            version = "latest"
          }
        }
      }

      env {
        name = "SLACK_WEBHOOK_URL"
        value_source {
          secret_key_ref {
            secret  = data.google_secret_manager_secret.zenith_secrets["SLACK_WEBHOOK_URL"].secret_id
            version = "latest"
          }
        }
      }

      # Liveness probe: verify process is running
      liveness_probe {
        http_get {
          path = "/healthz"
          port = var.port
        }
        initial_delay_seconds = 10
        period_seconds        = 30
        failure_threshold     = 3
      }

      # Startup probe: give more time on startup (DB connection)
      startup_probe {
        http_get {
          path = "/healthz"
          port = var.port
        }
        initial_delay_seconds = 5
        period_seconds        = 5
        failure_threshold     = 10
      }
    }
  }

  depends_on = [
    google_project_service.services,
    google_secret_manager_secret_iam_member.zenith_runner_access,
  ]
}

resource "google_cloud_run_v2_service_iam_member" "public_invoker" {
  project  = var.project_id
  location = var.region
  name     = google_cloud_run_v2_service.ingestor.name
  role     = "roles/run.invoker"
  member   = "allUsers"
}
