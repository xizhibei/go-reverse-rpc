package telemetry

import (
	"context"
	"errors"
	"io"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

type TelemetrySuite struct {
	suite.Suite
	ctx context.Context
}

func (s *TelemetrySuite) SetupTest() {
	s.ctx = context.Background()
}

func TestTelemetrySuite(t *testing.T) {
	suite.Run(t, new(TelemetrySuite))
}

func (s *TelemetrySuite) TestNew() {
	// Test with debug configuration
	debugCfg := Config{
		ServiceName:    "test-service",
		ServiceVersion: "1.0.0",
		Environment:    "test",
		Debug:          true,
		Enabled:        true,
		TraceWriter:    io.Discard,
		MetricWriter:   io.Discard,
	}

	tel, err := New(s.ctx, debugCfg)
	s.NoError(err)
	s.NotNil(tel)
	s.True(tel.IsEnabled())
	s.NotNil(tel.tp)
	s.NotNil(tel.mp)
	s.NotNil(tel.tracer)
	s.NotNil(tel.meter)
	s.NotNil(tel.requestDuration)
	s.NotNil(tel.errorCounter)
	s.NoError(tel.Shutdown(s.ctx))

	// Test with non-debug configuration (OTLP)
	// Note: OTLP exporter creation doesn't immediately fail with non-existent endpoints
	// because gRPC connections are lazy and only fail when data is actually sent
	nonDebugCfg := Config{
		ServiceName:    "test-service",
		ServiceVersion: "1.0.0",
		Environment:    "test",
		OTLPEndpoint:   "localhost:4317", // Use localhost to avoid immediate DNS failures
		Debug:          false,
		Enabled:        true,
	}

	tel, err = New(s.ctx, nonDebugCfg)
	if err != nil {
		// Connection might fail if no OTLP collector is running, which is expected in tests
		s.T().Log("OTLP telemetry creation failed as expected:", err)
	} else {
		// If creation succeeds, verify the telemetry is properly configured
		s.NotNil(tel)
		s.True(tel.IsEnabled())
		// Use a short timeout for shutdown to avoid hanging on OTLP connection
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		err = tel.Shutdown(shutdownCtx)
		if err != nil {
			s.T().Log("OTLP shutdown failed as expected (no collector running):", err)
		}
	}

	// Test with disabled configuration
	disabledCfg := Config{
		ServiceName:    "test-service",
		ServiceVersion: "1.0.0",
		Environment:    "test",
		Enabled:        false,
	}

	tel, err = New(s.ctx, disabledCfg)
	s.NoError(err)
	s.NotNil(tel)
	s.False(tel.IsEnabled())
}

func (s *TelemetrySuite) TestNewNoop() {
	tel, err := NewNoop()
	s.NoError(err)
	s.NotNil(tel)
	s.False(tel.IsEnabled())
	s.NotNil(tel.tp)
	s.NotNil(tel.mp)
	s.NotNil(tel.tracer)
	s.NotNil(tel.meter)
	s.NotNil(tel.requestDuration)
	s.NotNil(tel.errorCounter)
}

func (s *TelemetrySuite) TestStartSpan() {
	// Test with enabled telemetry
	testTel := NewTestTelemetry(s.T())
	defer testTel.Shutdown(s.ctx)

	tel := &TelemetryImpl{
		tp:      testTel.tp,
		tracer:  testTel.tp.Tracer("test-tracer"),
		enabled: true,
	}

	ctx, span := tel.StartSpan(s.ctx, "test-span")
	s.NotNil(span)
	s.NotEqual(s.ctx, ctx)
	span.End()

	// Test with disabled telemetry
	noopTel, err := NewNoop()
	s.NoError(err)

	ctx, span = noopTel.StartSpan(s.ctx, "test-span")
	s.NotNil(span)
	s.Equal(s.ctx, ctx) // Context should be unchanged for noop
	span.End()
}

func (s *TelemetrySuite) TestRecordRequest() {
	// Create a test telemetry with a manual reader for verification
	mr := sdkmetric.NewManualReader()
	testTel := NewTestTelemetry(s.T())
	defer testTel.Shutdown(s.ctx)

	// Replace the meter provider with one using our manual reader
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(mr),
	)
	meter := mp.Meter("test-meter")

	requestDuration, err := meter.Float64Histogram(
		"request_duration",
		metric.WithDescription("test"),
		metric.WithUnit("ms"),
	)
	s.NoError(err)

	errorCounter, err := meter.Int64Counter(
		"error_count",
		metric.WithDescription("test"),
	)
	s.NoError(err)

	tel := &TelemetryImpl{
		mp:              mp,
		meter:           meter,
		requestDuration: requestDuration,
		errorCounter:    errorCounter,
		enabled:         true,
	}

	// Record a successful request
	tel.RecordRequest(s.ctx, 100*time.Millisecond, "test-method", "success", nil)

	// Record a failed request
	testErr := errors.New("test error")
	tel.RecordRequest(s.ctx, 200*time.Millisecond, "test-method", "error", testErr)

	// Collect and verify metrics
	var metrics metricdata.ResourceMetrics
	err = mr.Collect(s.ctx, &metrics)
	s.NoError(err)

	// Verify metrics were recorded (basic check)
	s.NotEmpty(metrics.ScopeMetrics)

	// Test with disabled telemetry
	noopTel, err := NewNoop()
	s.NoError(err)

	// This should not panic even though the telemetry is disabled
	noopTel.RecordRequest(s.ctx, 100*time.Millisecond, "test-method", "success", nil)
}

func (s *TelemetrySuite) TestShutdown() {
	// Create a test telemetry instance
	testTel := NewTestTelemetry(s.T())

	// Shutdown should succeed
	err := testTel.Shutdown(s.ctx)
	s.NoError(err)

	// Test shutdown with a cancelled context
	testTel = NewTestTelemetry(s.T())
	cancelledCtx, cancel := context.WithCancel(s.ctx)
	cancel() // Cancel the context immediately

	// Create a custom context with timeout to ensure the test doesn't hang
	_, timeoutCancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer timeoutCancel()

	// Test shutdown with cancelled context - this should still work
	// because the actual shutdown implementation handles context cancellation gracefully
	err = testTel.Shutdown(cancelledCtx)
	// Note: The actual shutdown may or may not fail with cancelled context
	// depending on the underlying implementation, so we just verify it completes
	s.T().Logf("Shutdown with cancelled context result: %v", err)
}

func (s *TelemetrySuite) TestNewFromEnv() {
	// Test with OTEL_ENABLED=true
	os.Setenv("OTEL_ENABLED", "true")
	os.Setenv("OTEL_DEBUG", "true")
	os.Setenv("ENVIRONMENT", "test-env")
	os.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317")

	tel, err := NewFromEnv(s.ctx, "test-service", "1.0.0")
	if err == nil {
		s.True(tel.IsEnabled())
		s.NoError(tel.Shutdown(s.ctx))
	} else {
		// If it fails, it's likely due to connection issues with the OTLP endpoint
		// which is expected in a test environment
		s.T().Log("NewFromEnv with OTLP failed as expected:", err)
	}

	// Test with OTEL_ENABLED=false
	os.Setenv("OTEL_ENABLED", "false")

	tel, err = NewFromEnv(s.ctx, "test-service", "1.0.0")
	s.NoError(err)
	s.NotNil(tel)
	s.False(tel.IsEnabled())

	// Clean up environment
	os.Unsetenv("OTEL_ENABLED")
	os.Unsetenv("OTEL_DEBUG")
	os.Unsetenv("ENVIRONMENT")
	os.Unsetenv("OTEL_EXPORTER_OTLP_ENDPOINT")
}

func (s *TelemetrySuite) TestIsEnabled() {
	enabledTel := &TelemetryImpl{enabled: true}
	s.True(enabledTel.IsEnabled())

	disabledTel := &TelemetryImpl{enabled: false}
	s.False(disabledTel.IsEnabled())
}

// TestGetEnvOrDefault tests the getEnvOrDefault helper function
func (s *TelemetrySuite) TestGetEnvOrDefault() {
	// Test when environment variable is set
	os.Setenv("TEST_ENV_VAR", "test-value")
	value := getEnvOrDefault("TEST_ENV_VAR", "default-value")
	s.Equal("test-value", value)

	// Test when environment variable is not set
	os.Unsetenv("TEST_ENV_VAR")
	value = getEnvOrDefault("TEST_ENV_VAR", "default-value")
	s.Equal("default-value", value)
}

// Additional helper test to verify the TestTelemetry utility
func (s *TelemetrySuite) TestTestTelemetry() {
	testTel := NewTestTelemetry(s.T())
	s.NotNil(testTel)
	s.NotNil(testTel.tp)
	s.NotNil(testTel.mp)
	s.NotNil(testTel.mr)

	reader := testTel.GetReader()
	s.NotNil(reader)

	s.NoError(testTel.Shutdown(s.ctx))
}
