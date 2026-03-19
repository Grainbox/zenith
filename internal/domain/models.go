// Package domain contains the core business models.
package domain

import (
	"context"
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
	SinkType     string          `json:"sink_type"` // e.g. "discord", "http"
	IsActive     bool            `json:"is_active"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}

// Event represents a normalized, domain-level event ready for processing.
type Event struct {
	ID           string            `json:"id"`
	Type         string            `json:"type"`
	Source       string            `json:"source"`
	Payload      []byte            `json:"payload"`
	Timestamp    time.Time         `json:"timestamp"`
	TraceContext map[string]string `json:"-"` // W3C trace propagation carrier
}

// Condition represents a filtering rule condition (JSON DSL).
type Condition struct {
	Field    string      `json:"field"`
	Operator string      `json:"operator"`
	Value    interface{} `json:"value"`
}

// MatchedEvent pairs an event with the rule it matched.
// It is produced by the Rule Engine and consumed by the Dispatcher.
type MatchedEvent struct {
	Event *Event
	Rule  *Rule
	Ctx   context.Context // Trace context propagated from gateway
}

// AuditLog records the outcome of a single dispatch attempt.
type AuditLog struct {
	ID                  uuid.UUID  `json:"id"`
	EventID             string     `json:"event_id"`
	SourceID            *uuid.UUID `json:"source_id"` // nullable (ON DELETE SET NULL)
	Status              string     `json:"status"`    // "SUCCESS" | "FAILED"
	ProcessingLatencyMs int64      `json:"processing_latency_ms"`
	ErrorMessage        *string    `json:"error_message,omitempty"`
	CreatedAt           time.Time  `json:"created_at"`
}
