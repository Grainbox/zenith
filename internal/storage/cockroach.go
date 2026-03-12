// Package storage handles connection initialization and database abstractions.
package storage

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/Grainbox/zenith/internal/config"
	_ "github.com/jackc/pgx/v5/stdlib" // Import the pgx driver
)

// NewDatabase initializes a connection pool to CockroachDB using the pgx driver.
func NewDatabase(ctx context.Context, cfg config.DatabaseConfig) (*sql.DB, error) {
	db, err := sql.Open("pgx", cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool limits depending on CockroachDB cluster size
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)

	// Verify the connection is actually valid before returning
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close() // Ensure underlying resources are freed on failure
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return db, nil
}
