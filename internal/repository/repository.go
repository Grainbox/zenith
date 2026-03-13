// Package repository defines the interfaces for data persistence.
package repository

import (
	"context"

	"github.com/Grainbox/zenith/internal/domain"
	"github.com/google/uuid"
)

// ListOptions provides common filtering/pagination (future-proofing).
type ListOptions struct {
	Offset int
	Limit  int
}

// SourceRepository defines the contract for source persistence.
type SourceRepository interface {
	Create(ctx context.Context, source *domain.Source) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Source, error)
	GetByAPIKey(ctx context.Context, apiKey string) (*domain.Source, error)
	GetByName(ctx context.Context, name string) (*domain.Source, error)
}

// RuleRepository defines the contract for rule persistence.
type RuleRepository interface {
	Create(ctx context.Context, rule *domain.Rule) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Rule, error)
	ListBySourceID(ctx context.Context, sourceID uuid.UUID, opts ListOptions) ([]*domain.Rule, error)
	Update(ctx context.Context, rule *domain.Rule) error
	Delete(ctx context.Context, id uuid.UUID) error
}
