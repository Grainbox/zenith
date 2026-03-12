package postgres

import (
	"context"
	"testing"

	"github.com/Grainbox/zenith/internal/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSourceRepo_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db, teardown := setupTestDB(t)
	defer teardown()

	repo := NewSourceRepo(db)
	ctx := context.Background()

	t.Run("Create and Get source", func(t *testing.T) {
		truncateTables(t, db)

		s := &domain.Source{
			Name:   "Test Source",
			APIKey: "test-key-123",
		}

		err := repo.Create(ctx, s)
		require.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, s.ID)
		assert.False(t, s.CreatedAt.IsZero())

		// Verify Fetch
		fetched, err := repo.GetByID(ctx, s.ID)
		require.NoError(t, err)
		require.NotNil(t, fetched)
		assert.Equal(t, s.Name, fetched.Name)
		assert.Equal(t, s.APIKey, fetched.APIKey)

		// Verify Fetch by API Key
		fetchedByKey, err := repo.GetByAPIKey(ctx, s.APIKey)
		require.NoError(t, err)
		require.NotNil(t, fetchedByKey)
		assert.Equal(t, s.ID, fetchedByKey.ID)
	})

	t.Run("Create duplicate name should fail", func(t *testing.T) {
		truncateTables(t, db)

		s1 := &domain.Source{Name: "Duplicate", APIKey: "key1"}
		require.NoError(t, repo.Create(ctx, s1))

		s2 := &domain.Source{Name: "Duplicate", APIKey: "key2"}
		err := repo.Create(ctx, s2)
		assert.Error(t, err)
	})
}
