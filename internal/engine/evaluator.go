package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/Grainbox/zenith/internal/domain"
	"github.com/Grainbox/zenith/internal/repository"
	"github.com/Grainbox/zenith/internal/telemetry"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

// Evaluator evaluates events against rules from the database.
type Evaluator struct {
	ruleRepo   repository.RuleRepository
	sourceRepo repository.SourceRepository
	logger     *slog.Logger
	metrics    *telemetry.Metrics
}

// NewEvaluator creates a new Evaluator.
func NewEvaluator(ruleRepo repository.RuleRepository, sourceRepo repository.SourceRepository, logger *slog.Logger, metrics *telemetry.Metrics) *Evaluator {
	return &Evaluator{
		ruleRepo:   ruleRepo,
		sourceRepo: sourceRepo,
		logger:     logger,
		metrics:    metrics,
	}
}

// Evaluate evaluates an event against all active rules for its source.
// Returns the list of matching rules or an error if source not found.
func (e *Evaluator) Evaluate(ctx context.Context, event *domain.Event) ([]*domain.Rule, error) {
	tracer := otel.Tracer("zenith/engine")
	ctx, span := tracer.Start(ctx, "engine.evaluate_rules")
	defer span.End()

	start := time.Now()

	// Resolve source name to UUID
	source, err := e.sourceRepo.GetByName(ctx, event.Source)
	if err != nil {
		e.logger.Warn("Failed to resolve source", "source", event.Source, "error", err)
		return nil, fmt.Errorf("failed to resolve source: %w", err)
	}

	// Fetch all active rules for this source
	rules, err := e.ruleRepo.ListBySourceID(ctx, source.ID, repository.ListOptions{Limit: 1000})
	if err != nil {
		e.logger.Error("Failed to fetch rules", "source_id", source.ID, "error", err)
		return nil, fmt.Errorf("failed to fetch rules: %w", err)
	}

	// Parse event payload to map
	var payload map[string]interface{}
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		e.logger.Warn("Failed to parse event payload", "event_id", event.ID, "error", err)
		return nil, fmt.Errorf("failed to parse event payload: %w", err)
	}

	// Evaluate each active rule
	var matched []*domain.Rule
	for _, rule := range rules {
		if !rule.IsActive {
			continue
		}

		var cond domain.Condition
		if err := json.Unmarshal(rule.Condition, &cond); err != nil {
			e.logger.Warn("Failed to parse rule condition", "rule_id", rule.ID, "error", err)
			continue
		}

		if evaluateCondition(cond, payload) {
			matched = append(matched, rule)
			e.logger.Debug("Rule matched",
				"event_id", event.ID,
				"rule_id", rule.ID,
				"rule_name", rule.Name,
			)
		}
	}

	if len(rules) == 1000 {
		e.logger.Warn("Rule list may be truncated at 1000; some rules may not be evaluated",
			"source_id", source.ID,
		)
	}

	span.SetAttributes(
		attribute.Int("rules.total", len(rules)),
		attribute.Int("rules.matched", len(matched)),
	)

	// Record metrics
	e.metrics.IncRulesEvaluated(event.Source)
	for _, rule := range matched {
		e.metrics.IncRulesMatched(event.Source, rule.ID.String())
	}
	e.metrics.ObserveRuleEvalDuration(time.Since(start))

	if len(matched) == 0 {
		e.logger.Debug("No rules matched",
			"event_id", event.ID,
			"source", event.Source,
			"total_rules", len(rules),
		)
	} else {
		e.logger.Info("Rules matched",
			"event_id", event.ID,
			"source", event.Source,
			"matched_count", len(matched),
			"total_rules", len(rules),
		)
	}

	return matched, nil
}

// evaluateCondition evaluates a single condition against a payload map.
// Returns true if the condition matches, false otherwise.
// If the field is missing or the condition is malformed, returns false.
func evaluateCondition(cond domain.Condition, payload map[string]interface{}) bool {
	// Get the field value from payload
	payloadValue, exists := payload[cond.Field]
	if !exists {
		return false
	}

	// Perform comparison based on operator
	switch cond.Operator {
	case "==":
		if aFloat, ok1 := toFloat64(payloadValue); ok1 {
			if bFloat, ok2 := toFloat64(cond.Value); ok2 {
				return aFloat == bFloat
			}
		}
		return payloadValue == cond.Value
	case "!=":
		if aFloat, ok1 := toFloat64(payloadValue); ok1 {
			if bFloat, ok2 := toFloat64(cond.Value); ok2 {
				return aFloat != bFloat
			}
		}
		return payloadValue != cond.Value
	case ">":
		return compareNumeric(payloadValue, cond.Value, func(a, b float64) bool { return a > b })
	case ">=":
		return compareNumeric(payloadValue, cond.Value, func(a, b float64) bool { return a >= b })
	case "<":
		return compareNumeric(payloadValue, cond.Value, func(a, b float64) bool { return a < b })
	case "<=":
		return compareNumeric(payloadValue, cond.Value, func(a, b float64) bool { return a <= b })
	default:
		return false
	}
}

// compareNumeric performs a numeric comparison between two values.
// Both values are converted to float64 (JSON numbers are float64).
func compareNumeric(payloadValue, condValue interface{}, compare func(a, b float64) bool) bool {
	aFloat, ok1 := toFloat64(payloadValue)
	bFloat, ok2 := toFloat64(condValue)
	if !ok1 || !ok2 {
		return false
	}
	return compare(aFloat, bFloat)
}

// toFloat64 attempts to convert a value to float64.
func toFloat64(v interface{}) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	case string:
		// Try to parse string to number
		var f float64
		if err := json.Unmarshal([]byte(val), &f); err == nil {
			return f, true
		}
		return 0, false
	default:
		return 0, false
	}
}
