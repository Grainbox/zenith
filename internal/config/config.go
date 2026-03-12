// Package config manages application configuration from environment variables.
package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// DatabaseConfig holds connection settings for the CockroachDB instance.
type DatabaseConfig struct {
	URL          string
	MaxOpenConns int
	MaxIdleConns int
}

// SecretsConfig holds sensitive application settings.
type SecretsConfig struct {
	APIKeySalt      string
	SlackWebhookURL string
}

// EngineConfig holds event processing pipeline settings.
type EngineConfig struct {
	WorkerCount     int
	EventBufferSize int
}

// Config holds the application configuration.
type Config struct {
	Port     string
	Database DatabaseConfig
	Secrets  SecretsConfig
	Engine   EngineConfig
}

// Load loads the configuration from environment variables.
// It attempts to load .env.config, and .env.secrets files if present.
func Load() (*Config, error) {
	// Load environment variables from files.
	// Order: .env.config and .env.secrets
	_ = godotenv.Load(".env.config", ".env.secrets")

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return nil, fmt.Errorf("DATABASE_URL environment variable is required")
	}

	port := os.Getenv("ZENITH_PORT")
	if port == "" {
		port = "50051" // Default port
	}

	maxOpen := parseEnvInt("DB_MAX_OPEN_CONNS", 25)
	maxIdle := parseEnvInt("DB_MAX_IDLE_CONNS", 25)

	apiKeySalt := os.Getenv("API_KEY_SALT")
	slackWebhook := os.Getenv("SLACK_WEBHOOK_URL")
	workerCount := parseEnvInt("ENGINE_WORKER_COUNT", 10)
	bufferSize := parseEnvInt("ENGINE_BUFFER_SIZE", 1024)

	return &Config{
		Port: port,
		Database: DatabaseConfig{
			URL:          dbURL,
			MaxOpenConns: maxOpen,
			MaxIdleConns: maxIdle,
		},
		Secrets: SecretsConfig{
			APIKeySalt:      apiKeySalt,
			SlackWebhookURL: slackWebhook,
		},
		Engine: EngineConfig{
			WorkerCount:     workerCount,
			EventBufferSize: bufferSize,
		},
	}, nil
}

func parseEnvInt(key string, defaultVal int) int {
	valStr := os.Getenv(key)
	if valStr == "" {
		return defaultVal
	}
	val, err := strconv.Atoi(valStr)
	if err != nil {
		return defaultVal
	}
	return val
}
