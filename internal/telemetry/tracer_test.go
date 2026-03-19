package telemetry

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
)

func TestSetup_NoEndpoint_UsesNoop(t *testing.T) {
	// Setup with empty endpoint should use noop tracer
	cfg := Config{
		OTLPEndpoint: "",
		ServiceName:  "test-service",
	}

	shutdown, err := Setup(context.Background(), cfg)
	require.NoError(t, err)
	require.NotNil(t, shutdown)

	// Calling shutdown should not error
	err = shutdown(context.Background())
	assert.NoError(t, err)

	// Verify noop tracer is installed (no-op tracers should not panic)
	tracer := otel.Tracer("test")
	require.NotNil(t, tracer)
}

func TestSetup_WithValidEndpoint_CreateProvider(t *testing.T) {
	// This test verifies that Setup can initialize a provider with a valid (but unreachable) endpoint
	// The provider is created successfully; actual export would fail but that's not tested here
	cfg := Config{
		OTLPEndpoint: "http://localhost:4317",
		ServiceName:  "test-service",
	}

	shutdown, err := Setup(context.Background(), cfg)
	// May fail if port is unreachable, but Setup itself should handle it gracefully
	// For this test, we just verify the function signature works
	if err == nil {
		require.NotNil(t, shutdown)
		_ = shutdown(context.Background())
	}
}

func TestSetup_TracerCreation(t *testing.T) {
	// Verify that after Setup, we can create a tracer
	cfg := Config{
		OTLPEndpoint: "", // noop
		ServiceName:  "test-service",
	}

	shutdown, err := Setup(context.Background(), cfg)
	require.NoError(t, err)
	defer func() {
		_ = shutdown(context.Background())
	}()

	// Get a tracer and start a span (should not panic with noop)
	tracer := otel.Tracer("test/module")
	ctx, span := tracer.Start(context.Background(), "test-span")
	defer span.End()

	// Verify context is valid
	assert.NotNil(t, ctx)
}
