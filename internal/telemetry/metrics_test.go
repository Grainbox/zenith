package telemetry

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
)

func TestMetrics_NilSafe(t *testing.T) {
	var m *Metrics
	// Should not panic
	assert.NotPanics(t, func() {
		m.IncEventsReceived("source", "type")
		m.IncEventsAccepted("source")
		m.IncEventsRejected("source", "reason")
		m.IncRulesEvaluated("source")
		m.IncRulesMatched("source", "rule-id")
		m.IncDispatch("http", "success")
		m.SetWorkerQueueDepth(10)
	})
}

func TestMetrics_IncEventsReceived(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewMetrics(reg)

	m.IncEventsReceived("shop-a", "purchase")
	m.IncEventsReceived("shop-a", "purchase")
	m.IncEventsReceived("shop-b", "refund")

	val := testutil.ToFloat64(m.EventsReceivedCounter().WithLabelValues("shop-a", "purchase"))
	assert.Equal(t, 2.0, val)

	val = testutil.ToFloat64(m.EventsReceivedCounter().WithLabelValues("shop-b", "refund"))
	assert.Equal(t, 1.0, val)
}

func TestMetrics_IncEventsAccepted(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewMetrics(reg)

	m.IncEventsAccepted("shop-a")
	m.IncEventsAccepted("shop-a")
	m.IncEventsAccepted("shop-b")

	val := testutil.ToFloat64(m.EventsAcceptedCounter().WithLabelValues("shop-a"))
	assert.Equal(t, 2.0, val)

	val = testutil.ToFloat64(m.EventsAcceptedCounter().WithLabelValues("shop-b"))
	assert.Equal(t, 1.0, val)
}

func TestMetrics_IncEventsRejected(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewMetrics(reg)

	m.IncEventsRejected("unknown", RejectReasonInvalidBody)
	m.IncEventsRejected("unknown", RejectReasonInvalidBody)
	m.IncEventsRejected("shop-a", RejectReasonPipelineFull)

	val := testutil.ToFloat64(m.EventsRejectedCounter().WithLabelValues("unknown", RejectReasonInvalidBody))
	assert.Equal(t, 2.0, val)

	val = testutil.ToFloat64(m.EventsRejectedCounter().WithLabelValues("shop-a", RejectReasonPipelineFull))
	assert.Equal(t, 1.0, val)
}

func TestMetrics_IncDispatch(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewMetrics(reg)

	m.IncDispatch("http", "success")
	m.IncDispatch("http", "success")
	m.IncDispatch("http", "failed")
	m.IncDispatch("discord", "success")

	val := testutil.ToFloat64(m.DispatchTotalCounter().WithLabelValues("http", "success"))
	assert.Equal(t, 2.0, val)

	val = testutil.ToFloat64(m.DispatchTotalCounter().WithLabelValues("http", "failed"))
	assert.Equal(t, 1.0, val)

	val = testutil.ToFloat64(m.DispatchTotalCounter().WithLabelValues("discord", "success"))
	assert.Equal(t, 1.0, val)
}

func TestMetrics_AllMetricsRegistered(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewMetrics(reg)
	assert.NotNil(t, m)

	// Verify we can record metrics without panicking and they show up
	m.IncEventsReceived("test-source", "test-type")
	m.IncEventsAccepted("test-source")
	m.IncEventsRejected("test-source", "test-reason")
	m.IncRulesEvaluated("test-source")
	m.IncRulesMatched("test-source", "test-rule-id")
	m.IncDispatch("test-sink", "success")
	m.ObserveDispatchDuration("test-sink", 100)
	m.ObserveRuleEvalDuration(50)
	m.SetWorkerQueueDepth(5)

	// Verify metrics are present by checking names exist
	families, err := reg.Gather()
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(families), 8) // At least 8 metric families (some have multiple variants like _total, _bucket, etc)

	metricNames := make(map[string]bool)
	for _, fam := range families {
		metricNames[*fam.Name] = true
	}

	expectedMetrics := []string{
		"zenith_events_received_total",
		"zenith_events_accepted_total",
		"zenith_events_rejected_total",
		"zenith_rules_evaluated_total",
		"zenith_rules_matched_total",
		"zenith_dispatch_total",
		"zenith_dispatch_duration_seconds",
		"zenith_rule_evaluation_duration_seconds",
		"zenith_worker_queue_depth",
	}

	for _, name := range expectedMetrics {
		assert.True(t, metricNames[name], "expected metric %s not found", name)
	}
}
