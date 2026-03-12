// Package postgres implements the repository interfaces using a PostgreSQL connection.
package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/Grainbox/zenith/internal/domain"
	"github.com/google/uuid"
)

// SourceRepo handles source persistence in CockroachDB.
type SourceRepo struct {
	db *sql.DB
}

// NewSourceRepo creates a new SourceRepo.
func NewSourceRepo(db *sql.DB) *SourceRepo {
	return &SourceRepo{db: db}
}

// Create inserts a new source into the database.
func (r *SourceRepo) Create(ctx context.Context, s *domain.Source) error {
	query := `
		INSERT INTO sources (name, api_key)
		VALUES ($1, $2)
		RETURNING id, created_at, updated_at
	`
	err := r.db.QueryRowContext(ctx, query, s.Name, s.APIKey).Scan(
		&s.ID, &s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create source: %w", err)
	}
	return nil
}

// GetByID retrieves a source by its UUID.
func (r *SourceRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Source, error) {
	query := `SELECT id, name, api_key, created_at, updated_at FROM sources WHERE id = $1`
	var s domain.Source
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&s.ID, &s.Name, &s.APIKey, &s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Or a specific ErrNotFound
		}
		return nil, fmt.Errorf("failed to get source by id: %w", err)
	}
	return &s, nil
}

// GetByAPIKey retrieves a source by its API key.
func (r *SourceRepo) GetByAPIKey(ctx context.Context, apiKey string) (*domain.Source, error) {
	query := `SELECT id, name, api_key, created_at, updated_at FROM sources WHERE api_key = $1`
	var s domain.Source
	err := r.db.QueryRowContext(ctx, query, apiKey).Scan(
		&s.ID, &s.Name, &s.APIKey, &s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get source by api key: %w", err)
	}
	return &s, nil
}
