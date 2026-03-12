package postgres

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Grainbox/zenith/internal/domain"
	"github.com/Grainbox/zenith/internal/repository"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRuleRepo_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db, teardown := setupTestDB(t)
	defer teardown()

	sourceRepo := NewSourceRepo(db)
	ruleRepo := NewRuleRepo(db)
	ctx := context.Background()

	// Setup a source first for foreign keys
	source := &domain.Source{Name: "Rule Test Source", APIKey: "rule-key"}
	require.NoError(t, sourceRepo.Create(ctx, source))

	t.Run("Rule CRUD lifecycle", func(t *testing.T) {
		truncateTables(t, db)
		require.NoError(t, sourceRepo.Create(ctx, source)) // Re-insert after truncate

		// 1. Create
		condition := json.RawMessage(`{"field": "price", "op": ">", "val": 100}`)
		rule := &domain.Rule{
			SourceID:     source.ID,
			Name:         "High Value Rule",
			Condition:    condition,
			TargetAction: "webhook_1",
			IsActive:     true,
		}

		err := ruleRepo.Create(ctx, rule)
		require.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, rule.ID)

		// 2. Get and Verify
		fetched, err := ruleRepo.GetByID(ctx, rule.ID)
		require.NoError(t, err)
		require.NotNil(t, fetched)
		assert.Equal(t, "High Value Rule", fetched.Name)
		assert.JSONEq(t, string(condition), string(fetched.Condition))

		// 3. Update
		rule.Name = "Updated Rule Name"
		rule.IsActive = false
		err = ruleRepo.Update(ctx, rule)
		require.NoError(t, err)

		fetchedAfter, _ := ruleRepo.GetByID(ctx, rule.ID)
		assert.Equal(t, "Updated Rule Name", fetchedAfter.Name)
		assert.False(t, fetchedAfter.IsActive)

		// 4. List
		rules, err := ruleRepo.ListBySourceID(ctx, source.ID, repository.ListOptions{})
		require.NoError(t, err)
		assert.Len(t, rules, 1)

		// 5. Delete
		err = ruleRepo.Delete(ctx, rule.ID)
		require.NoError(t, err)
		
		deleted, _ := ruleRepo.GetByID(ctx, rule.ID)
		assert.Nil(t, deleted)
	})
}
