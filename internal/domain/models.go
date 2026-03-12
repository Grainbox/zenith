// Package domain contains the core business models.
package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Source represents an event sender.
type Source struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	APIKey    string    `json:"api_key"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Rule defines how events from a specific source should be filtered and routed.
type Rule struct {
	ID           uuid.UUID       `json:"id"`
	SourceID     uuid.UUID       `json:"source_id"`
	Name         string          `json:"name"`
	Condition    json.RawMessage `json:"condition"` // Flexible JSON logic
	TargetAction string          `json:"target_action"`
	IsActive     bool            `json:"is_active"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}
