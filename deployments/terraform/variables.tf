variable "project_id" {
  description = "GCP project ID"
  type        = string
}

variable "region" {
  description = "GCP region (e.g., europe-west1)"
  type        = string
  default     = "europe-west1"
}

variable "environment" {
  description = "Target environment (dev, prod)"
  type        = string
  default     = "dev"
}

variable "image_tag" {
  description = "Docker image tag to deploy (e.g., git SHA)"
  type        = string
  default     = "latest"
}

variable "port" {
  description = "Service listening port"
  type        = number
  default     = 8080
}

variable "db_max_open_conns" {
  description = "Maximum number of open connections to the database"
  type        = number
  default     = 25
}

variable "db_max_idle_conns" {
  description = "Maximum number of idle connections to the database"
  type        = number
  default     = 25
}

variable "database_url" {
  description = "CockroachDB connection string (managed as GCP Secret, optional for terraform plan)"
  type        = string
  sensitive   = true
  default     = ""
}

variable "api_key_salt" {
  description = "Salt for API key generation"
  type        = string
  sensitive   = true
  default     = ""
}

variable "slack_webhook_url" {
  description = "Slack webhook URL for notifications"
  type        = string
  sensitive   = true
  default     = ""
}
