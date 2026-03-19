package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/Grainbox/zenith/internal/domain"
	"github.com/Grainbox/zenith/internal/repository"
	"github.com/google/uuid"
)

// RuleRepo handles rule persistence in CockroachDB.
type RuleRepo struct {
	db *sql.DB
}

// NewRuleRepo creates a new RuleRepo.
func NewRuleRepo(db *sql.DB) *RuleRepo {
	return &RuleRepo{db: db}
}

// Create inserts a new rule.
func (r *RuleRepo) Create(ctx context.Context, ru *domain.Rule) error {
	query := `
		INSERT INTO rules (source_id, name, condition, target_action, sink_type, is_active)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at, updated_at
	`
	err := r.db.QueryRowContext(ctx, query, ru.SourceID, ru.Name, ru.Condition, ru.TargetAction, ru.SinkType, ru.IsActive).Scan(
		&ru.ID, &ru.CreatedAt, &ru.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create rule: %w", err)
	}
	return nil
}

// GetByID retrieves a rule by ID.
func (r *RuleRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Rule, error) {
	query := `
		SELECT id, source_id, name, condition, target_action, sink_type, is_active, created_at, updated_at
		FROM rules
		WHERE id = $1
	`
	var ru domain.Rule
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&ru.ID, &ru.SourceID, &ru.Name, &ru.Condition, &ru.TargetAction, &ru.SinkType, &ru.IsActive, &ru.CreatedAt, &ru.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("rule not found")
		}
		return nil, fmt.Errorf("failed to get rule by id: %w", err)
	}
	return &ru, nil
}

// ListBySourceID lists all rules for a given source.
func (r *RuleRepo) ListBySourceID(ctx context.Context, sourceID uuid.UUID, opts repository.ListOptions) ([]*domain.Rule, error) {
	if opts.Limit == 0 {
		opts.Limit = 100 // Sensible default
	}

	query := `
		SELECT id, source_id, name, condition, target_action, sink_type, is_active, created_at, updated_at
		FROM rules
		WHERE source_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := r.db.QueryContext(ctx, query, sourceID, opts.Limit, opts.Offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list rules: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	var rules []*domain.Rule
	for rows.Next() {
		var ru domain.Rule
		if err := rows.Scan(
			&ru.ID, &ru.SourceID, &ru.Name, &ru.Condition, &ru.TargetAction, &ru.SinkType, &ru.IsActive, &ru.CreatedAt, &ru.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan rule row: %w", err)
		}
		rules = append(rules, &ru)
	}

	return rules, nil
}

// Update updates an existing rule.
func (r *RuleRepo) Update(ctx context.Context, ru *domain.Rule) error {
	query := `
		UPDATE rules
		SET name = $1, condition = $2, target_action = $3, sink_type = $4, is_active = $5, updated_at = now()
		WHERE id = $6
		RETURNING updated_at
	`
	err := r.db.QueryRowContext(ctx, query, ru.Name, ru.Condition, ru.TargetAction, ru.SinkType, ru.IsActive, ru.ID).Scan(&ru.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to update rule: %w", err)
	}
	return nil
}

// Delete removes a rule.
func (r *RuleRepo) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM rules WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete rule: %w", err)
	}
	return nil
}
