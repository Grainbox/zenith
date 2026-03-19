// Package config manages application configuration from environment variables.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// DatabaseConfig holds connection settings for the CockroachDB instance.
type DatabaseConfig struct {
	URL          string
	MaxOpenConns int
	MaxIdleConns int
}

// SecretsConfig holds sensitive settings shared across both binaries.
type SecretsConfig struct {
	APIKeySalt string
}

// EngineConfig holds event processing pipeline settings.
type EngineConfig struct {
	WorkerCount     int
	EventBufferSize int
}

// TelemetryConfig holds OpenTelemetry tracing settings.
type TelemetryConfig struct {
	OTLPEndpoint string
	ServiceName  string
}

// Config holds the application configuration.
type Config struct {
	Port      string
	Database  DatabaseConfig
	Secrets   SecretsConfig
	Engine    EngineConfig
	Telemetry TelemetryConfig
}

// Load loads the configuration from environment variables.
// It attempts to load .env.config, and .env.secrets files if present.
// component is the service name (e.g. "ingestor", "dispatcher"); it is used to
// derive a service-specific port env var (e.g. INGESTOR_PORT) that takes
// precedence over the generic PORT variable. defaultPort is used when neither
// is set.
func Load(component, defaultPort string) (*Config, error) {
	// Load environment variables from files.
	// Order: .env.config and .env.secrets
	_ = godotenv.Load(".env.config", ".env.secrets")

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return nil, fmt.Errorf("DATABASE_URL environment variable is required")
	}

	portKey := strings.ToUpper(component) + "_PORT"
	port := os.Getenv(portKey)
	if port == "" {
		port = os.Getenv("PORT")
	}
	if port == "" {
		port = defaultPort
	}

	maxOpen := parseEnvInt("DB_MAX_OPEN_CONNS", 25)
	maxIdle := parseEnvInt("DB_MAX_IDLE_CONNS", 25)

	apiKeySalt := os.Getenv("API_KEY_SALT")
	workerCount := parseEnvInt("ENGINE_WORKER_COUNT", 10)
	bufferSize := parseEnvInt("ENGINE_BUFFER_SIZE", 1024)

	otlpEndpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	serviceName := os.Getenv("OTEL_SERVICE_NAME")
	if serviceName == "" {
		serviceName = component
	}

	return &Config{
		Port: port,
		Database: DatabaseConfig{
			URL:          dbURL,
			MaxOpenConns: maxOpen,
			MaxIdleConns: maxIdle,
		},
		Secrets: SecretsConfig{
			APIKeySalt: apiKeySalt,
		},
		Engine: EngineConfig{
			WorkerCount:     workerCount,
			EventBufferSize: bufferSize,
		},
		Telemetry: TelemetryConfig{
			OTLPEndpoint: otlpEndpoint,
			ServiceName:  serviceName,
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
