package engine

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"testing"

	"github.com/Grainbox/zenith/internal/domain"
	"github.com/google/uuid"
)

// BenchmarkEvaluateCondition_1Rule benchmarks condition evaluation with a single rule.
func BenchmarkEvaluateCondition_1Rule(b *testing.B) {
	payload := map[string]interface{}{
		"amount": 150.0,
		"status": "active",
	}
	cond := domain.Condition{
		Field:    "amount",
		Operator: ">",
		Value:    100.0,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = evaluateCondition(cond, payload)
	}
}

// BenchmarkEvaluateCondition_10Rules benchmarks evaluating 10 conditions per event.
func BenchmarkEvaluateCondition_10Rules(b *testing.B) {
	payload := map[string]interface{}{
		"amount":   150.0,
		"status":   "active",
		"country":  "US",
		"severity": "high",
		"duration": 5000,
	}

	conditions := []domain.Condition{
		{Field: "amount", Operator: ">", Value: 100.0},
		{Field: "amount", Operator: "<=", Value: 1000.0},
		{Field: "status", Operator: "==", Value: "active"},
		{Field: "country", Operator: "!=", Value: "CN"},
		{Field: "severity", Operator: "==", Value: "high"},
		{Field: "duration", Operator: ">=", Value: 1000},
		{Field: "amount", Operator: "<", Value: 500.0},
		{Field: "status", Operator: "!=", Value: "inactive"},
		{Field: "country", Operator: "==", Value: "US"},
		{Field: "severity", Operator: "!=", Value: "low"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, cond := range conditions {
			_ = evaluateCondition(cond, payload)
		}
	}
}

// BenchmarkEvaluateCondition_100Rules benchmarks evaluating 100 conditions per event.
func BenchmarkEvaluateCondition_100Rules(b *testing.B) {
	payload := map[string]interface{}{
		"amount":    150.0,
		"status":    "active",
		"country":   "US",
		"severity":  "high",
		"duration":  5000,
		"threshold": 25.5,
		"user_id":   "user_abc_123",
	}

	// Generate 100 varied conditions
	conditions := make([]domain.Condition, 100)
	operators := []string{"==", "!=", ">", ">=", "<", "<="}
	fields := []string{"amount", "status", "country", "severity", "duration", "threshold"}

	for i := 0; i < 100; i++ {
		conditions[i] = domain.Condition{
			Field:    fields[i%len(fields)],
			Operator: operators[i%len(operators)],
			Value:    100.0 + float64(i),
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, cond := range conditions {
			_ = evaluateCondition(cond, payload)
		}
	}
}

// BenchmarkEvaluator_Evaluate_1Rule benchmarks full Evaluator.Evaluate() with 1 rule (no DB, mock repos).
func BenchmarkEvaluator_Evaluate_1Rule(b *testing.B) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))

	sourceID := uuid.New()
	source := &domain.Source{
		ID:   sourceID,
		Name: "bench_source",
	}

	condJSON, _ := json.Marshal(domain.Condition{
		Field:    "amount",
		Operator: ">",
		Value:    100.0,
	})

	rules := []*domain.Rule{
		{
			ID:           uuid.New(),
			SourceID:     sourceID,
			Name:         "rule_1",
			Condition:    condJSON,
			TargetAction: "https://webhook.test",
			IsActive:     true,
		},
	}

	srcRepo := NewMockSourceRepository()
	srcRepo.AddSource("bench_source", source)

	ruleRepo := NewMockRuleRepository()
	ruleRepo.AddRules(sourceID, rules)

	evaluator := NewEvaluator(ruleRepo, srcRepo, logger, nil)

	eventPayload, _ := json.Marshal(map[string]interface{}{
		"amount": 150.0,
	})

	event := &domain.Event{
		ID:      "evt_bench_1",
		Type:    "bench",
		Source:  "bench_source",
		Payload: eventPayload,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = evaluator.Evaluate(context.Background(), event)
	}
}

// BenchmarkEvaluator_Evaluate_10Rules benchmarks full Evaluator.Evaluate() with 10 rules.
func BenchmarkEvaluator_Evaluate_10Rules(b *testing.B) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))

	sourceID := uuid.New()
	source := &domain.Source{
		ID:   sourceID,
		Name: "bench_source",
	}

	rules := make([]*domain.Rule, 10)
	for i := 0; i < 10; i++ {
		condJSON, _ := json.Marshal(domain.Condition{
			Field:    "amount",
			Operator: ">",
			Value:    float64(i*50 + 50),
		})
		rules[i] = &domain.Rule{
			ID:           uuid.New(),
			SourceID:     sourceID,
			Name:         "rule_" + string(rune(i)),
			Condition:    condJSON,
			TargetAction: "https://webhook.test",
			IsActive:     true,
		}
	}

	srcRepo := NewMockSourceRepository()
	srcRepo.AddSource("bench_source", source)

	ruleRepo := NewMockRuleRepository()
	ruleRepo.AddRules(sourceID, rules)

	evaluator := NewEvaluator(ruleRepo, srcRepo, logger, nil)

	eventPayload, _ := json.Marshal(map[string]interface{}{
		"amount": 250.0,
	})

	event := &domain.Event{
		ID:      "evt_bench_10",
		Type:    "bench",
		Source:  "bench_source",
		Payload: eventPayload,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = evaluator.Evaluate(context.Background(), event)
	}
}

// BenchmarkEvaluator_Evaluate_100Rules benchmarks full Evaluator.Evaluate() with 100 rules.
func BenchmarkEvaluator_Evaluate_100Rules(b *testing.B) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))

	sourceID := uuid.New()
	source := &domain.Source{
		ID:   sourceID,
		Name: "bench_source",
	}

	rules := make([]*domain.Rule, 100)
	operators := []string{"==", "!=", ">", ">=", "<", "<="}
	for i := 0; i < 100; i++ {
		condJSON, _ := json.Marshal(domain.Condition{
			Field:    "amount",
			Operator: operators[i%len(operators)],
			Value:    float64(i*10 + 50),
		})
		rules[i] = &domain.Rule{
			ID:           uuid.New(),
			SourceID:     sourceID,
			Name:         "rule_" + string(rune(i%26+65)),
			Condition:    condJSON,
			TargetAction: "https://webhook.test",
			IsActive:     true,
		}
	}

	srcRepo := NewMockSourceRepository()
	srcRepo.AddSource("bench_source", source)

	ruleRepo := NewMockRuleRepository()
	ruleRepo.AddRules(sourceID, rules)

	evaluator := NewEvaluator(ruleRepo, srcRepo, logger, nil)

	eventPayload, _ := json.Marshal(map[string]interface{}{
		"amount": 500.0,
	})

	event := &domain.Event{
		ID:      "evt_bench_100",
		Type:    "bench",
		Source:  "bench_source",
		Payload: eventPayload,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = evaluator.Evaluate(context.Background(), event)
	}
}
