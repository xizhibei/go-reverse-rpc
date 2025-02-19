package telemetry

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

// Telemetry holds OpenTelemetry components
type Telemetry struct {
	tp              *sdktrace.TracerProvider
	mp              *sdkmetric.MeterProvider
	tracer          trace.Tracer
	meter           metric.Meter
	requestDuration metric.Float64Histogram
	errorCounter    metric.Int64Counter
}

// Config holds configuration for telemetry setup
type Config struct {
	ServiceName    string
	ServiceVersion string
	Environment    string
	OTLPEndpoint   string

	TraceWriter  io.Writer
	MetricWriter io.Writer
	Debug        bool
}

// New creates a new Telemetry instance
func New(ctx context.Context, cfg Config) (*Telemetry, error) {
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.ServiceVersion),
			semconv.DeploymentEnvironment(cfg.Environment),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	if cfg.TraceWriter == nil {
		cfg.TraceWriter = os.Stdout
	}

	if cfg.MetricWriter == nil {
		cfg.MetricWriter = os.Stdout
	}

	// Setup trace provider
	var traceExporter sdktrace.SpanExporter
	if cfg.Debug {
		traceExporter, err = stdouttrace.New(
			stdouttrace.WithWriter(cfg.TraceWriter),
			stdouttrace.WithPrettyPrint(),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create trace exporter: %w", err)
		}
	} else {
		traceExporter, err = otlptracegrpc.New(ctx,
			otlptracegrpc.WithEndpoint(cfg.OTLPEndpoint),
			otlptracegrpc.WithInsecure(),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create trace exporter: %w", err)
		}
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	// Setup metric provider
	var metricExporter sdkmetric.Exporter
	if cfg.Debug {
		enc := json.NewEncoder(cfg.MetricWriter)
		enc.SetIndent("", "  ")

		metricExporter, err = stdoutmetric.New(
			stdoutmetric.WithEncoder(enc),
			stdoutmetric.WithoutTimestamps(),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create metric exporter: %w", err)
		}
	} else {
		metricExporter, err = otlpmetricgrpc.New(ctx,
			otlpmetricgrpc.WithEndpoint(cfg.OTLPEndpoint),
			otlpmetricgrpc.WithInsecure(),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create metric exporter: %w", err)
		}
	}

	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(
			sdkmetric.NewPeriodicReader(
				metricExporter,
				sdkmetric.WithInterval(10*time.Second),
			),
		),
		sdkmetric.WithView(
			sdkmetric.NewView(
				sdkmetric.Instrument{Name: "request_duration"},
				sdkmetric.Stream{
					Aggregation: sdkmetric.AggregationExplicitBucketHistogram{
						Boundaries: []float64{1, 5, 10, 25, 50, 75, 100, 250, 500, 750, 1000, 2500, 5000, 7500, 10000},
					},
				},
			),
		),
	)
	otel.SetMeterProvider(mp)

	meter := mp.Meter("github.com/xizhibei/go-reverse-rpc")
	requestDuration, err := meter.Float64Histogram(
		"request_duration",
		metric.WithDescription("Duration of RPC requests"),
		metric.WithUnit("ms"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create request duration histogram: %w", err)
	}

	errorCounter, err := meter.Int64Counter(
		"error_count",
		metric.WithDescription("Number of RPC errors"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create error counter: %w", err)
	}

	tracer := tp.Tracer("github.com/xizhibei/go-reverse-rpc")

	return &Telemetry{
		tp:              tp,
		mp:              mp,
		tracer:          tracer,
		meter:           meter,
		requestDuration: requestDuration,
		errorCounter:    errorCounter,
	}, nil
}

// Shutdown gracefully shuts down the telemetry providers
func (t *Telemetry) Shutdown(ctx context.Context) error {
	if err := t.tp.Shutdown(ctx); err != nil {
		return fmt.Errorf("failed to shutdown trace provider: %w", err)
	}
	if err := t.mp.Shutdown(ctx); err != nil {
		return fmt.Errorf("failed to shutdown meter provider: %w", err)
	}
	return nil
}

// RecordRequest records request duration and optionally increments error counter
func (t *Telemetry) RecordRequest(ctx context.Context, duration time.Duration, method string, status string, err error) {
	attrs := []attribute.KeyValue{
		attribute.String("method", method),
		attribute.String("status", status),
	}

	t.requestDuration.Record(ctx, float64(duration.Milliseconds()), metric.WithAttributes(attrs...))

	if err != nil {
		attrs = append(attrs, attribute.String("error", err.Error()))
		t.errorCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
	}
}

// StartSpan starts a new span and returns the context and span
func (t *Telemetry) StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return t.tracer.Start(ctx, name, opts...)
}

// NewNoop creates a new Telemetry instance that does nothing.
// It provides a placeholder implementation that satisfies the interface
// but performs no actual telemetry operations.
func NewNoop() (*Telemetry, error) {
	// Create empty resource
	res, err := resource.New(context.Background(),
		resource.WithAttributes(
			semconv.ServiceName("noop"),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Create no-op trace provider with never sample
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.NeverSample()),
	)

	// Create no-op meter provider with no readers
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
	)

	// Get tracer and meter from providers
	tracer := tp.Tracer("github.com/xizhibei/go-reverse-rpc")
	meter := mp.Meter("github.com/xizhibei/go-reverse-rpc")

	// Create no-op instruments
	requestDuration, err := meter.Float64Histogram(
		"request_duration",
		metric.WithDescription("No-op request duration histogram"),
		metric.WithUnit("ms"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create request duration histogram: %w", err)
	}

	errorCounter, err := meter.Int64Counter(
		"error_count",
		metric.WithDescription("No-op error counter"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create error counter: %w", err)
	}

	return &Telemetry{
		tp:              tp,
		mp:              mp,
		tracer:          tracer,
		meter:           meter,
		requestDuration: requestDuration,
		errorCounter:    errorCounter,
	}, nil
}
