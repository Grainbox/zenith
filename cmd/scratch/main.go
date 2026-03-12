package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/Grainbox/zenith/internal/config"
	"github.com/Grainbox/zenith/internal/domain"
	"github.com/Grainbox/zenith/internal/repository"
	"github.com/Grainbox/zenith/internal/repository/postgres"
	"github.com/Grainbox/zenith/internal/storage"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	
	cfg, err := config.Load()
	if err != nil {
		logger.Error("Failed to load config", "error", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	db, err := storage.NewDatabase(ctx, cfg.Database)
	if err != nil {
		logger.Error("Failed to connect to DB", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	sourceRepo := postgres.NewSourceRepo(db)
	ruleRepo := postgres.NewRuleRepo(db)

	// 1. Create Source
	source := &domain.Source{
		Name:   fmt.Sprintf("Test Source %d", time.Now().Unix()),
		APIKey: fmt.Sprintf("key-%d", time.Now().Unix()),
	}
	if err := sourceRepo.Create(ctx, source); err != nil {
		logger.Error("Failed to create source", "error", err)
		return
	}
	fmt.Printf("✅ Source Created: %s (ID: %s)\n", source.Name, source.ID)

	// 2. Create Rule
	condition := json.RawMessage(`{"field": "price", "operator": ">", "value": 100}`)
	rule := &domain.Rule{
		SourceID:     source.ID,
		Name:         "High Value Purchase",
		Condition:    condition,
		TargetAction: "slack_alert",
		IsActive:     true,
	}
	if err := ruleRepo.Create(ctx, rule); err != nil {
		logger.Error("Failed to create rule", "error", err)
		return
	}
	fmt.Printf("✅ Rule Created: %s (ID: %s)\n", rule.Name, rule.ID)

	// 3. List Rules for Source
	rules, err := ruleRepo.ListBySourceID(ctx, source.ID, repository.ListOptions{})
	if err != nil {
		logger.Error("Failed to list rules", "error", err)
		return
	}
	fmt.Printf("📄 Found %d rules for this source\n", len(rules))
}
