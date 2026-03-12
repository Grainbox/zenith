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
	URL             string
	MaxOpenConns    int
	MaxIdleConns    int
}

// Config holds the application configuration.
type Config struct {
	Port     string
	Database DatabaseConfig
}

// Load loads the configuration from environment variables.
// It attempts to load a .env file if present, but does not fail if missing.
func Load() (*Config, error) {
	_ = godotenv.Load() // Ignore error if .env doesn't exist (e.g., in production)

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

	return &Config{
		Port: port,
		Database: DatabaseConfig{
			URL:          dbURL,
			MaxOpenConns: maxOpen,
			MaxIdleConns: maxIdle,
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
