package telemetry

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Rejection reason constants for zenith_events_rejected_total metric.
const (
	RejectReasonMissingAPIKey  = "missing_api_key"
	RejectReasonInvalidAPIKey  = "invalid_api_key"
	RejectReasonSourceMismatch = "source_mismatch"
	RejectReasonInvalidBody    = "invalid_body"
	RejectReasonPipelineFull   = "pipeline_full"
)

// Metrics holds all Prometheus metric instances for the pipeline.
type Metrics struct {
	eventsReceived   *prometheus.CounterVec
	eventsAccepted   *prometheus.CounterVec
	eventsRejected   *prometheus.CounterVec
	rulesEvaluated   *prometheus.CounterVec
	rulesMatched     *prometheus.CounterVec
	dispatchTotal    *prometheus.CounterVec
	dispatchDuration *prometheus.HistogramVec
	ruleEvalDuration prometheus.Histogram
	workerQueueDepth prometheus.Gauge
}

// NewMetrics creates and registers all metrics with the given registerer.
// If registerer is nil, metrics are registered with prometheus.DefaultRegisterer.
func NewMetrics(reg prometheus.Registerer) *Metrics {
	if reg == nil {
		reg = prometheus.DefaultRegisterer
	}

	m := &Metrics{
		eventsReceived: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "zenith_events_received_total",
				Help: "Total events received by the Gateway",
			},
			[]string{"source", "event_type"},
		),
		eventsAccepted: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "zenith_events_accepted_total",
				Help: "Events accepted (auth passed, payload valid)",
			},
			[]string{"source"},
		),
		eventsRejected: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "zenith_events_rejected_total",
				Help: "Events rejected (auth fail, invalid payload)",
			},
			[]string{"source", "reason"},
		),
		rulesEvaluated: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "zenith_rules_evaluated_total",
				Help: "Rule evaluations performed",
			},
			[]string{"source"},
		),
		rulesMatched: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "zenith_rules_matched_total",
				Help: "Rules matched per evaluation",
			},
			[]string{"source", "rule_id"},
		),
		dispatchTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "zenith_dispatch_total",
				Help: "Dispatch attempts (status: success, failed)",
			},
			[]string{"sink_type", "status"},
		),
		dispatchDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "zenith_dispatch_duration_seconds",
				Help:    "Dispatch latency distribution",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"sink_type"},
		),
		ruleEvalDuration: prometheus.NewHistogram(
			prometheus.HistogramOpts{
				Name:    "zenith_rule_evaluation_duration_seconds",
				Help:    "Rule engine evaluation latency",
				Buckets: []float64{0.0001, 0.0005, 0.001, 0.005, 0.01, 0.05, 0.1},
			},
		),
		workerQueueDepth: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "zenith_worker_queue_depth",
				Help: "Current depth of the matched-event channel",
			},
		),
	}

	// Register all metrics
	reg.MustRegister(m.eventsReceived)
	reg.MustRegister(m.eventsAccepted)
	reg.MustRegister(m.eventsRejected)
	reg.MustRegister(m.rulesEvaluated)
	reg.MustRegister(m.rulesMatched)
	reg.MustRegister(m.dispatchTotal)
	reg.MustRegister(m.dispatchDuration)
	reg.MustRegister(m.ruleEvalDuration)
	reg.MustRegister(m.workerQueueDepth)

	return m
}

// Nil-safe recording methods

// IncEventsReceived increments the events_received_total counter.
func (m *Metrics) IncEventsReceived(source, eventType string) {
	if m == nil {
		return
	}
	m.eventsReceived.WithLabelValues(source, eventType).Inc()
}

// IncEventsAccepted increments the events_accepted_total counter.
func (m *Metrics) IncEventsAccepted(source string) {
	if m == nil {
		return
	}
	m.eventsAccepted.WithLabelValues(source).Inc()
}

// IncEventsRejected increments the events_rejected_total counter.
func (m *Metrics) IncEventsRejected(source, reason string) {
	if m == nil {
		return
	}
	m.eventsRejected.WithLabelValues(source, reason).Inc()
}

// IncRulesEvaluated increments the rules_evaluated_total counter.
func (m *Metrics) IncRulesEvaluated(source string) {
	if m == nil {
		return
	}
	m.rulesEvaluated.WithLabelValues(source).Inc()
}

// IncRulesMatched increments the rules_matched_total counter.
// ruleID is the UUID string of the rule.
func (m *Metrics) IncRulesMatched(source, ruleID string) {
	if m == nil {
		return
	}
	m.rulesMatched.WithLabelValues(source, ruleID).Inc()
}

// IncDispatch increments the dispatch_total counter.
// status is either "success" or "failed".
func (m *Metrics) IncDispatch(sinkType, status string) {
	if m == nil {
		return
	}
	m.dispatchTotal.WithLabelValues(sinkType, status).Inc()
}

// ObserveDispatchDuration records the dispatch latency.
func (m *Metrics) ObserveDispatchDuration(sinkType string, d time.Duration) {
	if m == nil {
		return
	}
	m.dispatchDuration.WithLabelValues(sinkType).Observe(d.Seconds())
}

// ObserveRuleEvalDuration records rule evaluation latency.
func (m *Metrics) ObserveRuleEvalDuration(d time.Duration) {
	if m == nil {
		return
	}
	m.ruleEvalDuration.Observe(d.Seconds())
}

// SetWorkerQueueDepth sets the current queue depth gauge.
func (m *Metrics) SetWorkerQueueDepth(depth int) {
	if m == nil {
		return
	}
	m.workerQueueDepth.Set(float64(depth))
}

// Test accessors (for prometheus/testutil.ToFloat64)

// EventsReceivedCounter returns the events_received_total counter for testing.
func (m *Metrics) EventsReceivedCounter() *prometheus.CounterVec {
	return m.eventsReceived
}

// EventsAcceptedCounter returns the events_accepted_total counter for testing.
func (m *Metrics) EventsAcceptedCounter() *prometheus.CounterVec {
	return m.eventsAccepted
}

// EventsRejectedCounter returns the events_rejected_total counter for testing.
func (m *Metrics) EventsRejectedCounter() *prometheus.CounterVec {
	return m.eventsRejected
}

// DispatchTotalCounter returns the dispatch_total counter for testing.
func (m *Metrics) DispatchTotalCounter() *prometheus.CounterVec {
	return m.dispatchTotal
}
