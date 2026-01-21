package otelx

import (
	"context"
	"testing"

	"github.com/go-logr/logr/funcr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

// ensureTestTracerProvider sets a global tracer provider that always samples.
func ensureTestTracerProvider(t *testing.T) {
	t.Helper()
	tp := trace.NewTracerProvider(trace.WithSampler(trace.AlwaysSample()))
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	t.Cleanup(func() {
		if err := tp.Shutdown(context.Background()); err != nil {
			t.Errorf("failed to shutdown tracer provider: %v", err)
		}
	})
}

func TestStartSpan_Otel(t *testing.T) {
	t.Parallel()
	ensureTestTracerProvider(t)
	ctx := context.Background()
	newCtx, end := StartSpan(ctx, "test-span")
	assert.NotNil(t, end)
	assert.NotEqual(t, ctx, newCtx)
	end()
}

func TestWithEnrichedLogger_AddsTraceFields(t *testing.T) {
	t.Parallel()
	ensureTestTracerProvider(t)
	ctx, end := StartSpan(context.Background(), "log-span")
	defer end()

	var captured string
	logger := funcr.New(func(prefix, args string) {
		captured = args
	}, funcr.Options{Verbosity: 0})

	WithEnrichedLogger(ctx, logger).Info("x")
	assert.Contains(t, captured, "\"trace_id\"=")
	assert.Contains(t, captured, "\"span_id\"=")
}

func TestSetup_UsesExistingTracerProvider(t *testing.T) {
	t.Parallel()
	tp := trace.NewTracerProvider()
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	t.Cleanup(func() {
		otel.SetTracerProvider(noop.NewTracerProvider())
	})

	shutdown, err := Setup(context.Background(), Config{})
	require.NoError(t, err)
	assert.Nil(t, shutdown)
	assert.Same(t, tp, otel.GetTracerProvider())
}
