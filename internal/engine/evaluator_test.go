package engine

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/Grainbox/zenith/internal/domain"
	"github.com/Grainbox/zenith/internal/repository"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockSourceRepository is a test double for SourceRepository.
type MockSourceRepository struct {
	sources map[string]*domain.Source
}

func NewMockSourceRepository() *MockSourceRepository {
	return &MockSourceRepository{
		sources: make(map[string]*domain.Source),
	}
}

func (m *MockSourceRepository) Create(_ context.Context, _ *domain.Source) error {
	return nil
}

func (m *MockSourceRepository) GetByID(_ context.Context, _ uuid.UUID) (*domain.Source, error) {
	return nil, errors.New("not implemented")
}

func (m *MockSourceRepository) GetByAPIKey(_ context.Context, _ string) (*domain.Source, error) {
	return nil, errors.New("not implemented")
}

func (m *MockSourceRepository) GetByName(_ context.Context, name string) (*domain.Source, error) {
	source, ok := m.sources[name]
	if !ok {
		return nil, errors.New("source not found")
	}
	return source, nil
}

func (m *MockSourceRepository) AddSource(name string, source *domain.Source) {
	m.sources[name] = source
}

// MockRuleRepository is a test double for RuleRepository.
type MockRuleRepository struct {
	rules map[uuid.UUID][]*domain.Rule
}

func NewMockRuleRepository() *MockRuleRepository {
	return &MockRuleRepository{
		rules: make(map[uuid.UUID][]*domain.Rule),
	}
}

func (m *MockRuleRepository) Create(_ context.Context, _ *domain.Rule) error {
	return nil
}

func (m *MockRuleRepository) GetByID(_ context.Context, _ uuid.UUID) (*domain.Rule, error) {
	return nil, errors.New("not implemented")
}

func (m *MockRuleRepository) ListBySourceID(_ context.Context, sourceID uuid.UUID, _ repository.ListOptions) ([]*domain.Rule, error) {
	return m.rules[sourceID], nil
}

func (m *MockRuleRepository) Update(_ context.Context, _ *domain.Rule) error {
	return nil
}

func (m *MockRuleRepository) Delete(_ context.Context, _ uuid.UUID) error {
	return nil
}

func (m *MockRuleRepository) AddRules(sourceID uuid.UUID, rules []*domain.Rule) {
	m.rules[sourceID] = rules
}

func TestEvaluateCondition_Equality(t *testing.T) {
	tests := []struct {
		name      string
		condition domain.Condition
		payload   map[string]interface{}
		expected  bool
	}{
		{
			name:      "numeric_equal_match",
			condition: domain.Condition{Field: "amount", Operator: "==", Value: 100.0},
			payload:   map[string]interface{}{"amount": 100.0},
			expected:  true,
		},
		{
			name:      "numeric_equal_no_match",
			condition: domain.Condition{Field: "amount", Operator: "==", Value: 100.0},
			payload:   map[string]interface{}{"amount": 50.0},
			expected:  false,
		},
		{
			name:      "string_equal_match",
			condition: domain.Condition{Field: "status", Operator: "==", Value: "active"},
			payload:   map[string]interface{}{"status": "active"},
			expected:  true,
		},
		{
			name:      "field_missing",
			condition: domain.Condition{Field: "missing", Operator: "==", Value: 100.0},
			payload:   map[string]interface{}{"amount": 100.0},
			expected:  false,
		},
		{
			name:      "unknown_operator",
			condition: domain.Condition{Field: "amount", Operator: "contains", Value: 100.0},
			payload:   map[string]interface{}{"amount": 100.0},
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := evaluateCondition(tt.condition, tt.payload)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEvaluateCondition_Inequality(t *testing.T) {
	tests := []struct {
		name      string
		condition domain.Condition
		payload   map[string]interface{}
		expected  bool
	}{
		{
			name:      "not_equal_match",
			condition: domain.Condition{Field: "status", Operator: "!=", Value: "inactive"},
			payload:   map[string]interface{}{"status": "active"},
			expected:  true,
		},
		{
			name:      "not_equal_no_match",
			condition: domain.Condition{Field: "status", Operator: "!=", Value: "active"},
			payload:   map[string]interface{}{"status": "active"},
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := evaluateCondition(tt.condition, tt.payload)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEvaluateCondition_GreaterThan(t *testing.T) {
	tests := []struct {
		name      string
		condition domain.Condition
		payload   map[string]interface{}
		expected  bool
	}{
		{
			name:      "greater_than_match",
			condition: domain.Condition{Field: "amount", Operator: ">", Value: 100.0},
			payload:   map[string]interface{}{"amount": 150.0},
			expected:  true,
		},
		{
			name:      "greater_than_no_match",
			condition: domain.Condition{Field: "amount", Operator: ">", Value: 100.0},
			payload:   map[string]interface{}{"amount": 50.0},
			expected:  false,
		},
		{
			name:      "greater_than_equal_no_match",
			condition: domain.Condition{Field: "amount", Operator: ">", Value: 100.0},
			payload:   map[string]interface{}{"amount": 100.0},
			expected:  false,
		},
		{
			name:      "non_numeric_string_cond_value",
			condition: domain.Condition{Field: "amount", Operator: ">", Value: "not-a-number"},
			payload:   map[string]interface{}{"amount": 100.0},
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := evaluateCondition(tt.condition, tt.payload)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEvaluateCondition_GreaterEqualThan(t *testing.T) {
	tests := []struct {
		name      string
		condition domain.Condition
		payload   map[string]interface{}
		expected  bool
	}{
		{
			name:      "greater_equal_match",
			condition: domain.Condition{Field: "amount", Operator: ">=", Value: 100.0},
			payload:   map[string]interface{}{"amount": 150.0},
			expected:  true,
		},
		{
			name:      "greater_equal_equal_match",
			condition: domain.Condition{Field: "amount", Operator: ">=", Value: 100.0},
			payload:   map[string]interface{}{"amount": 100.0},
			expected:  true,
		},
		{
			name:      "greater_equal_no_match",
			condition: domain.Condition{Field: "amount", Operator: ">=", Value: 100.0},
			payload:   map[string]interface{}{"amount": 50.0},
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := evaluateCondition(tt.condition, tt.payload)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEvaluateCondition_LessThan(t *testing.T) {
	tests := []struct {
		name      string
		condition domain.Condition
		payload   map[string]interface{}
		expected  bool
	}{
		{
			name:      "less_than_match",
			condition: domain.Condition{Field: "amount", Operator: "<", Value: 100.0},
			payload:   map[string]interface{}{"amount": 50.0},
			expected:  true,
		},
		{
			name:      "less_than_no_match",
			condition: domain.Condition{Field: "amount", Operator: "<", Value: 100.0},
			payload:   map[string]interface{}{"amount": 150.0},
			expected:  false,
		},
		{
			name:      "less_than_equal_no_match",
			condition: domain.Condition{Field: "amount", Operator: "<", Value: 100.0},
			payload:   map[string]interface{}{"amount": 100.0},
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := evaluateCondition(tt.condition, tt.payload)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEvaluateCondition_LessEqualThan(t *testing.T) {
	tests := []struct {
		name      string
		condition domain.Condition
		payload   map[string]interface{}
		expected  bool
	}{
		{
			name:      "less_equal_match",
			condition: domain.Condition{Field: "amount", Operator: "<=", Value: 100.0},
			payload:   map[string]interface{}{"amount": 50.0},
			expected:  true,
		},
		{
			name:      "less_equal_equal_match",
			condition: domain.Condition{Field: "amount", Operator: "<=", Value: 100.0},
			payload:   map[string]interface{}{"amount": 100.0},
			expected:  true,
		},
		{
			name:      "less_equal_no_match",
			condition: domain.Condition{Field: "amount", Operator: "<=", Value: 100.0},
			payload:   map[string]interface{}{"amount": 150.0},
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := evaluateCondition(tt.condition, tt.payload)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEvaluator_Evaluate_MultipleRules(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))

	sourceID := uuid.New()
	source := &domain.Source{
		ID:   sourceID,
		Name: "payment_service",
	}

	rule1Condition, err := json.Marshal(domain.Condition{
		Field:    "amount",
		Operator: ">",
		Value:    500.0,
	})
	require.NoError(t, err)

	rule2Condition, err := json.Marshal(domain.Condition{
		Field:    "status",
		Operator: "==",
		Value:    "declined",
	})
	require.NoError(t, err)

	rules := []*domain.Rule{
		{
			ID:           uuid.New(),
			SourceID:     sourceID,
			Name:         "high_amount",
			Condition:    rule1Condition,
			TargetAction: "slack_alert",
			IsActive:     true,
		},
		{
			ID:           uuid.New(),
			SourceID:     sourceID,
			Name:         "payment_failed",
			Condition:    rule2Condition,
			TargetAction: "retry",
			IsActive:     true,
		},
	}

	srcRepo := NewMockSourceRepository()
	srcRepo.AddSource("payment_service", source)

	ruleRepo := NewMockRuleRepository()
	ruleRepo.AddRules(sourceID, rules)

	evaluator := NewEvaluator(ruleRepo, srcRepo, logger, nil)

	eventPayload, err := json.Marshal(map[string]interface{}{
		"amount": 750.0,
		"status": "success",
	})
	require.NoError(t, err)

	event := &domain.Event{
		ID:      "evt_123",
		Type:    "payment",
		Source:  "payment_service",
		Payload: eventPayload,
	}

	matched, err := evaluator.Evaluate(context.Background(), event)
	require.NoError(t, err)
	assert.Len(t, matched, 1)
	assert.Equal(t, "high_amount", matched[0].Name)
}

func TestEvaluator_Evaluate_UnknownSource(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))

	srcRepo := NewMockSourceRepository() // vide, aucune source enregistrée
	ruleRepo := NewMockRuleRepository()

	evaluator := NewEvaluator(ruleRepo, srcRepo, logger, nil)

	event := &domain.Event{
		ID:      "evt_unknown",
		Type:    "test",
		Source:  "nonexistent_source",
		Payload: []byte(`{"amount": 100}`),
	}

	matched, err := evaluator.Evaluate(context.Background(), event)
	assert.Error(t, err)
	assert.Nil(t, matched)
}

func TestEvaluator_Evaluate_InactiveRuleSkipped(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))

	sourceID := uuid.New()
	source := &domain.Source{
		ID:   sourceID,
		Name: "test_source",
	}

	condJSON, err := json.Marshal(domain.Condition{
		Field:    "amount",
		Operator: "==",
		Value:    100.0,
	})
	require.NoError(t, err)

	rules := []*domain.Rule{
		{
			ID:           uuid.New(),
			SourceID:     sourceID,
			Name:         "inactive_rule",
			Condition:    condJSON,
			TargetAction: "slack_alert",
			IsActive:     false, // doit être ignorée
		},
	}

	srcRepo := NewMockSourceRepository()
	srcRepo.AddSource("test_source", source)

	ruleRepo := NewMockRuleRepository()
	ruleRepo.AddRules(sourceID, rules)

	evaluator := NewEvaluator(ruleRepo, srcRepo, logger, nil)

	eventPayload, err := json.Marshal(map[string]interface{}{"amount": 100.0})
	require.NoError(t, err)

	event := &domain.Event{
		ID:      "evt_inactive",
		Type:    "test",
		Source:  "test_source",
		Payload: eventPayload,
	}

	matched, err := evaluator.Evaluate(context.Background(), event)
	require.NoError(t, err)
	assert.Len(t, matched, 0)
}

func TestEvaluator_Evaluate_InvalidPayload(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))

	sourceID := uuid.New()
	source := &domain.Source{
		ID:   sourceID,
		Name: "test_source",
	}

	srcRepo := NewMockSourceRepository()
	srcRepo.AddSource("test_source", source)

	ruleRepo := NewMockRuleRepository()

	evaluator := NewEvaluator(ruleRepo, srcRepo, logger, nil)

	event := &domain.Event{
		ID:      "evt_invalid",
		Type:    "test",
		Source:  "test_source",
		Payload: []byte("invalid json"),
	}

	matched, err := evaluator.Evaluate(context.Background(), event)
	assert.Error(t, err)
	assert.Nil(t, matched)
}
