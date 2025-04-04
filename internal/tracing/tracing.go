package tracing

import (
	"context"
	"fmt"

	"github.com/not-nullexception/image-optimizer/internal/logger"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.23.1"
	"go.opentelemetry.io/otel/trace"
)

var (
	tracer trace.Tracer
	log    zerolog.Logger
)

// TracingConfig holds the configuration for tracing
type TracingConfig struct {
	ServiceName    string
	ServiceVersion string
	Environment    string
	OTLPEndpoint   string
	Enabled        bool
}

// Init initializes the OpenTelemetry tracer
func Init(ctx context.Context, cfg TracingConfig) (func(), error) {
	log = logger.GetLogger("tracing")

	if !cfg.Enabled {
		log.Info().Msg("Tracing is disabled")
		return func() {}, nil
	}

	// Validate configuration
	if cfg.OTLPEndpoint == "" {
		return nil, fmt.Errorf("OTLP endpoint is required")
	}

	// Create OTLP exporter
	traceExporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(cfg.OTLPEndpoint),
		otlptracegrpc.WithInsecure(), // For development; use TLS in production
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP trace exporter: %w", err)
	}

	// Create resource with service information
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.ServiceVersion),
			attribute.String("environment", cfg.Environment),
		),
		resource.WithOS(),
		resource.WithProcessRuntimeDescription(),
		resource.WithTelemetrySDK(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Configure trace provider with appropriate sampling
	tp := tracesdk.NewTracerProvider(
		tracesdk.WithBatcher(traceExporter),
		tracesdk.WithResource(res),
		tracesdk.WithSampler(tracesdk.ParentBased(tracesdk.TraceIDRatioBased(0.5))),
	)

	// Set global trace provider
	otel.SetTracerProvider(tp)

	// Set up propagator for context propagation
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// Create a tracer
	tracer = tp.Tracer(cfg.ServiceName)

	log.Info().
		Str("service", cfg.ServiceName).
		Str("version", cfg.ServiceVersion).
		Str("environment", cfg.Environment).
		Str("otlp_endpoint", cfg.OTLPEndpoint).
		Msg("Tracing initialized with OpenTelemetry")

	// Return a cleanup function
	return func() {
		if err := tp.Shutdown(ctx); err != nil {
			log.Error().Err(err).Msg("Error shutting down tracer provider")
		} else {
			log.Info().Msg("Tracer provider shut down successfully")
		}
	}, nil
}

// Tracer returns the global tracer
func Tracer() trace.Tracer {
	return tracer
}

// StartSpan starts a new span with the given name
func StartSpan(ctx context.Context, name string) (context.Context, trace.Span) {
	return tracer.Start(ctx, name)
}

// AddAttribute adds an attribute to the current span
func AddAttribute(ctx context.Context, key string, value interface{}) {
	span := trace.SpanFromContext(ctx)
	if !span.IsRecording() {
		return
	}

	var attr attribute.KeyValue
	switch v := value.(type) {
	case string:
		attr = attribute.String(key, v)
	case int:
		attr = attribute.Int(key, v)
	case int64:
		attr = attribute.Int64(key, v)
	case float64:
		attr = attribute.Float64(key, v)
	case bool:
		attr = attribute.Bool(key, v)
	default:
		attr = attribute.String(key, fmt.Sprintf("%v", v))
	}

	span.SetAttributes(attr)
}

// AddEvent adds an event to the current span
func AddEvent(ctx context.Context, name string, attributes ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	if !span.IsRecording() {
		return
	}

	span.AddEvent(name, trace.WithAttributes(attributes...))
}

// RecordError records an error on the current span
func RecordError(ctx context.Context, err error) {
	span := trace.SpanFromContext(ctx)
	if !span.IsRecording() {
		return
	}

	span.RecordError(err)
}

// GetLoggerFromContext extracts tracing information and creates a logger
func GetLoggerFromContext(ctx context.Context, component string) zerolog.Logger {
	span := trace.SpanFromContext(ctx)
	logger := logger.GetLogger(component)

	if span.IsRecording() {
		spanCtx := span.SpanContext()
		if spanCtx.IsValid() {
			logger = logger.With().
				Str("trace_id", spanCtx.TraceID().String()).
				Str("span_id", spanCtx.SpanID().String()).
				Logger()
		}
	}

	return logger
}
