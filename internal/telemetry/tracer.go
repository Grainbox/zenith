package telemetry

import (
	"context"
	"errors"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace/noop"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
)

// Config holds tracing configuration.
type Config struct {
	OTLPEndpoint string // OTEL_EXPORTER_OTLP_ENDPOINT; empty disables tracing
	ServiceName  string // OTEL_SERVICE_NAME; used as resource attribute
}

// Setup initializes the global TracerProvider.
// If OTLPEndpoint is empty, installs a noop provider (safe for local dev/tests).
// Returns a shutdown function to flush and close the exporter.
func Setup(ctx context.Context, cfg Config) (func(context.Context) error, error) {
	// If OTLP endpoint is not configured, use noop tracer
	if cfg.OTLPEndpoint == "" {
		otel.SetTracerProvider(noop.NewTracerProvider())
		otel.SetTextMapPropagator(propagation.TraceContext{})
		return func(context.Context) error { return nil }, nil
	}

	// Create OTLP HTTP exporter
	exporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint(cfg.OTLPEndpoint),
		otlptracehttp.WithInsecure(),
	)
	if err != nil {
		return nil, err
	}

	// Create resource with service name
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(cfg.ServiceName),
		),
	)
	if err != nil {
		return nil, errors.New("failed to create resource: " + err.Error())
	}

	// Create TracerProvider with batched exporter
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)

	// Set global tracer provider and propagator
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	// Return shutdown function
	return tp.Shutdown, nil
}
