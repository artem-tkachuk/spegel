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
)

// ensureTestTracerProvider sets a global tracer provider that always samples.
func ensureTestTracerProvider(t *testing.T) {
	t.Helper()
	prevProvider := otel.GetTracerProvider()
	prevPropagator := otel.GetTextMapPropagator()
	tp := trace.NewTracerProvider(trace.WithSampler(trace.AlwaysSample()))
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	t.Cleanup(func() {
		otel.SetTracerProvider(prevProvider)
		otel.SetTextMapPropagator(prevPropagator)
		if err := tp.Shutdown(context.Background()); err != nil {
			t.Errorf("failed to shutdown tracer provider: %v", err)
		}
	})
}

func TestStartSpan_Otel(t *testing.T) {
	ensureTestTracerProvider(t)
	ctx := context.Background()
	newCtx, end := StartSpan(ctx, "test-span")
	assert.NotNil(t, end)
	assert.NotEqual(t, ctx, newCtx)
	end()
}

func TestWithEnrichedLogger_AddsTraceFields(t *testing.T) {
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
	tp := trace.NewTracerProvider()
	prevProvider := otel.GetTracerProvider()
	prevPropagator := otel.GetTextMapPropagator()
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	t.Cleanup(func() {
		otel.SetTracerProvider(prevProvider)
		otel.SetTextMapPropagator(prevPropagator)
	})

	shutdown, err := Setup(context.Background(), Config{})
	require.NoError(t, err)
	assert.Nil(t, shutdown)
	assert.Same(t, tp, otel.GetTracerProvider())
}
