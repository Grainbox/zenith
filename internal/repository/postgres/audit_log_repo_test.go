package postgres

import (
	"context"
	"testing"

	"github.com/Grainbox/zenith/internal/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuditLogRepo_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db, teardown := setupTestDB(t)
	defer teardown()

	repo := NewAuditLogRepo(db)
	ctx := context.Background()

	// Seed a source so SourceID FK is valid.
	sourceRepo := NewSourceRepo(db)
	src := &domain.Source{Name: "audit-test-source", APIKey: "audit-key-123"}
	require.NoError(t, sourceRepo.Create(ctx, src))

	t.Run("Create SUCCESS log with source", func(t *testing.T) {
		truncateTables(t, db)
		require.NoError(t, sourceRepo.Create(ctx, src))

		log := &domain.AuditLog{
			EventID:             "evt-001",
			SourceID:            &src.ID,
			Status:              "SUCCESS",
			ProcessingLatencyMs: 42,
		}

		err := repo.Create(ctx, log)
		require.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, log.ID)
		assert.False(t, log.CreatedAt.IsZero())
	})

	t.Run("Create FAILED log with error message", func(t *testing.T) {
		truncateTables(t, db)
		require.NoError(t, sourceRepo.Create(ctx, src))

		errMsg := "connection refused"
		log := &domain.AuditLog{
			EventID:             "evt-002",
			SourceID:            &src.ID,
			Status:              "FAILED",
			ProcessingLatencyMs: 150,
			ErrorMessage:        &errMsg,
		}

		err := repo.Create(ctx, log)
		require.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, log.ID)
		assert.False(t, log.CreatedAt.IsZero())
	})

	t.Run("Create log with nil SourceID", func(t *testing.T) {
		truncateTables(t, db)

		log := &domain.AuditLog{
			EventID:             "evt-003",
			SourceID:            nil,
			Status:              "FAILED",
			ProcessingLatencyMs: 5,
		}

		err := repo.Create(ctx, log)
		require.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, log.ID)
		assert.False(t, log.CreatedAt.IsZero())
	})
}
