package httpx

import (
	"context"
	"testing"

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
