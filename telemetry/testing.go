package telemetry

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/trace"
)

// TestTelemetry holds test components for OpenTelemetry
type TestTelemetry struct {
	tp *trace.TracerProvider
	mp *metric.MeterProvider
	mr *metric.ManualReader
}

// NewTestTelemetry creates a new TestTelemetry instance for testing
func NewTestTelemetry(t *testing.T) *TestTelemetry {
	t.Helper()

	tp := trace.NewTracerProvider()
	otel.SetTracerProvider(tp)

	mr := metric.NewManualReader()
	mp := metric.NewMeterProvider(metric.WithReader(mr))
	otel.SetMeterProvider(mp)

	return &TestTelemetry{
		tp: tp,
		mp: mp,
		mr: mr,
	}
}

// Shutdown gracefully shuts down the test telemetry providers
func (tt *TestTelemetry) Shutdown(ctx context.Context) error {
	if err := tt.tp.Shutdown(ctx); err != nil {
		return err
	}
	if err := tt.mp.Shutdown(ctx); err != nil {
		return err
	}
	return nil
}

// GetReader returns the metric reader for testing
func (tt *TestTelemetry) GetReader() *sdkmetric.ManualReader {
	return tt.mr
}
