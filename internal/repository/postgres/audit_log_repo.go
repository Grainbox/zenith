package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/Grainbox/zenith/internal/domain"
)

// AuditLogRepo handles audit log persistence in CockroachDB.
type AuditLogRepo struct {
	db *sql.DB
}

// NewAuditLogRepo creates a new AuditLogRepo.
func NewAuditLogRepo(db *sql.DB) *AuditLogRepo {
	return &AuditLogRepo{db: db}
}

// Create inserts a new audit log record into the database.
func (r *AuditLogRepo) Create(ctx context.Context, l *domain.AuditLog) error {
	query := `
		INSERT INTO audit_logs (event_id, source_id, status, processing_latency_ms, error_message)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at
	`
	err := r.db.QueryRowContext(ctx, query,
		l.EventID, l.SourceID, l.Status, l.ProcessingLatencyMs, l.ErrorMessage,
	).Scan(&l.ID, &l.CreatedAt)
	if err != nil {
		return fmt.Errorf("failed to create audit log: %w", err)
	}
	return nil
}
