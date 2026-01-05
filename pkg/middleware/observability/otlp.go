// Package observability implements TracerProvider using OpenTelemetry Protocol (OTLP).
// OTLP is the standard protocol for sending telemetry data to observability backends.
// This provider can export traces to:
//
//   - Jaeger: Popular open-source tracing backend
//
//   - Grafana Tempo: Scalable tracing backend from Grafana
//
//   - Honeycomb, Datadog, New Relic: Commercial observability platforms
//
//   - Any OTLP-compatible collector
//
//     Create the tracer provider (connects to Jaeger at localhost:4317)
//     provider, err := observability.NewOTLPTracerProvider("my-service", "localhost:4317")
//     if err != nil {
//     log.Fatal(err)
//     }
//     defer provider.Shutdown(context.Background())
//
//     // Use it with the tracing middleware
//     handler := observability.TracingHandler(provider, "process-request", myHandler)
//
//     // Run your flow
//     flow := calque.NewFlow().Use(handler)
//
// View traces in Jaeger UI at http://localhost:16686 (default Jaeger port).
package observability

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
)

// OTLPTracerProvider implements TracerProvider using OpenTelemetry OTLP exporter.
type OTLPTracerProvider struct {
	provider *sdktrace.TracerProvider
	tracer   trace.Tracer
}

// OTLPConfig configures the OTLP tracer provider.
// Most users only need ServiceName and Endpoint. Other options are for
// advanced use cases like production deployments with TLS.
type OTLPConfig struct {
	// ServiceName identifies your service in traces (e.g., "payment-api", "user-service")
	// This is how you'll find your traces in Jaeger/Tempo.
	ServiceName string

	// ServiceVersion is shown alongside traces (e.g., "v1.2.3", "abc123")
	// Helps identify which version of your code generated the trace.
	ServiceVersion string

	// Endpoint is the OTLP collector address (e.g., "localhost:4317", "tempo:4317")
	// Port 4317 is the standard gRPC port, 4318 for HTTP.
	Endpoint string

	// UseHTTP uses HTTP instead of gRPC for OTLP export.
	// Use HTTP when gRPC is blocked (e.g., some cloud environments).
	// Default: false (uses gRPC)
	UseHTTP bool

	// Insecure disables TLS. Use for local development only.
	// For production, use proper TLS certificates.
	// Default: true (insecure, for easy local development)
	Insecure bool

	// Headers are sent with every trace export request.
	// Use for authentication tokens or custom headers.
	Headers map[string]string

	// SampleRate controls what percentage of traces are recorded.
	// 1.0 = record everything (100%), 0.1 = record 10%, 0.0 = record nothing
	// For high-traffic production, use 0.1 or lower.
	// Default: 1.0 (record all traces)
	SampleRate float64

	// BatchTimeout is how long to wait before sending a batch of spans.
	// Lower = faster visibility, Higher = more efficient batching.
	// Default: 5 seconds
	BatchTimeout time.Duration
}

// DefaultOTLPConfig returns the default OTLP configuration
func DefaultOTLPConfig(serviceName, endpoint string) OTLPConfig {
	return OTLPConfig{
		ServiceName:    serviceName,
		ServiceVersion: "unknown",
		Endpoint:       endpoint,
		UseHTTP:        false,
		Insecure:       true,
		Headers:        nil,
		SampleRate:     1.0,
		BatchTimeout:   5 * time.Second,
	}
}

// OTLPOption configures the OTLP tracer provider
type OTLPOption func(*OTLPConfig)

// WithServiceVersion sets the service version
func WithServiceVersion(version string) OTLPOption {
	return func(cfg *OTLPConfig) {
		cfg.ServiceVersion = version
	}
}

// WithHTTPExporter uses HTTP instead of gRPC
func WithHTTPExporter() OTLPOption {
	return func(cfg *OTLPConfig) {
		cfg.UseHTTP = true
	}
}

// WithSecure enables TLS
func WithSecure() OTLPOption {
	return func(cfg *OTLPConfig) {
		cfg.Insecure = false
	}
}

// WithHeaders sets additional headers for the exporter
func WithHeaders(headers map[string]string) OTLPOption {
	return func(cfg *OTLPConfig) {
		cfg.Headers = headers
	}
}

// WithSampleRate sets the sampling rate
func WithSampleRate(rate float64) OTLPOption {
	return func(cfg *OTLPConfig) {
		cfg.SampleRate = rate
	}
}

// NewOTLPTracerProvider creates a new OTLP tracer provider.
//
// This is the main entry point for setting up distributed tracing. It connects
// to your OTLP collector (Jaeger, Tempo, etc.) and handles all the complexity
// of trace export, batching, and error handling.
//
// Parameters:
//   - serviceName: Name to identify your service in traces (e.g., "user-api")
//   - endpoint: OTLP collector address (e.g., "localhost:4317")
//   - opts: Optional configuration (version, sample rate, TLS, etc.)
//
// IMPORTANT: Always call Shutdown() when your application exits to flush
// any pending traces:
//
//	provider, err := observability.NewOTLPTracerProvider("my-service", "localhost:4317")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer provider.Shutdown(context.Background())
func NewOTLPTracerProvider(serviceName, endpoint string, opts ...OTLPOption) (*OTLPTracerProvider, error) {
	cfg := DefaultOTLPConfig(serviceName, endpoint)
	for _, opt := range opts {
		opt(&cfg)
	}

	ctx := context.Background()

	// Create the appropriate exporter
	exporter, err := createExporter(ctx, cfg)
	if err != nil {
		return nil, err
	}

	// Create resource with service information
	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.ServiceVersion),
		),
	)
	if err != nil {
		return nil, err
	}

	// Create sampler
	var sampler sdktrace.Sampler
	switch {
	case cfg.SampleRate >= 1.0:
		sampler = sdktrace.AlwaysSample()
	case cfg.SampleRate <= 0.0:
		sampler = sdktrace.NeverSample()
	default:
		sampler = sdktrace.TraceIDRatioBased(cfg.SampleRate)
	}

	// Create the trace provider
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter, sdktrace.WithBatchTimeout(cfg.BatchTimeout)),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler),
	)

	// Set as global provider
	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return &OTLPTracerProvider{
		provider: provider,
		tracer:   provider.Tracer(cfg.ServiceName),
	}, nil
}

// StartSpan starts a new span
func (p *OTLPTracerProvider) StartSpan(ctx context.Context, name string, opts ...SpanOption) (context.Context, Span) {
	cfg := &spanConfig{
		kind:       SpanKindInternal,
		attributes: make(map[string]any),
	}
	for _, opt := range opts {
		opt(cfg)
	}

	// Convert span kind
	var otelKind trace.SpanKind
	switch cfg.kind {
	case SpanKindServer:
		otelKind = trace.SpanKindServer
	case SpanKindClient:
		otelKind = trace.SpanKindClient
	case SpanKindProducer:
		otelKind = trace.SpanKindProducer
	case SpanKindConsumer:
		otelKind = trace.SpanKindConsumer
	default:
		otelKind = trace.SpanKindInternal
	}

	// Start the span
	ctx, otelSpan := p.tracer.Start(ctx, name, trace.WithSpanKind(otelKind))

	// Set initial attributes
	for k, v := range cfg.attributes {
		otelSpan.SetAttributes(anyToAttribute(k, v))
	}

	return ctx, &otlpSpan{span: otelSpan}
}

// Shutdown gracefully shuts down the tracer provider
func (p *OTLPTracerProvider) Shutdown(ctx context.Context) error {
	return p.provider.Shutdown(ctx)
}

// otlpSpan wraps an OpenTelemetry span
type otlpSpan struct {
	span trace.Span
}

// End ends the span
func (s *otlpSpan) End(err error) {
	if err != nil {
		s.span.RecordError(err)
		s.span.SetStatus(codes.Error, err.Error())
	}
	s.span.End()
}

// SetAttribute sets a key-value attribute
func (s *otlpSpan) SetAttribute(key string, value any) {
	s.span.SetAttributes(anyToAttribute(key, value))
}

// AddEvent adds an event to the span
func (s *otlpSpan) AddEvent(name string, attrs map[string]any) {
	otelAttrs := make([]attribute.KeyValue, 0, len(attrs))
	for k, v := range attrs {
		otelAttrs = append(otelAttrs, anyToAttribute(k, v))
	}
	s.span.AddEvent(name, trace.WithAttributes(otelAttrs...))
}

// SetStatus sets the span status
func (s *otlpSpan) SetStatus(code SpanStatus, description string) {
	switch code {
	case SpanStatusOK:
		s.span.SetStatus(codes.Ok, description)
	case SpanStatusError:
		s.span.SetStatus(codes.Error, description)
	default:
		s.span.SetStatus(codes.Unset, description)
	}
}

// SpanContext returns the span's context
func (s *otlpSpan) SpanContext() SpanContext {
	sc := s.span.SpanContext()
	return SpanContext{
		TraceID: sc.TraceID().String(),
		SpanID:  sc.SpanID().String(),
	}
}

// anyToAttribute converts an any value to an OpenTelemetry attribute
func anyToAttribute(key string, value any) attribute.KeyValue {
	switch v := value.(type) {
	case string:
		return attribute.String(key, v)
	case int:
		return attribute.Int(key, v)
	case int64:
		return attribute.Int64(key, v)
	case float64:
		return attribute.Float64(key, v)
	case bool:
		return attribute.Bool(key, v)
	case []string:
		return attribute.StringSlice(key, v)
	case []int:
		return attribute.IntSlice(key, v)
	case []int64:
		return attribute.Int64Slice(key, v)
	case []float64:
		return attribute.Float64Slice(key, v)
	case []bool:
		return attribute.BoolSlice(key, v)
	default:
		return attribute.String(key, "")
	}
}

// createExporter creates the appropriate OTLP exporter based on configuration
func createExporter(ctx context.Context, cfg OTLPConfig) (*otlptrace.Exporter, error) {
	if cfg.UseHTTP {
		return createHTTPExporter(ctx, cfg)
	}
	return createGRPCExporter(ctx, cfg)
}

// createHTTPExporter creates an HTTP-based OTLP exporter
func createHTTPExporter(ctx context.Context, cfg OTLPConfig) (*otlptrace.Exporter, error) {
	options := []otlptracehttp.Option{
		otlptracehttp.WithEndpoint(cfg.Endpoint),
	}
	if cfg.Insecure {
		options = append(options, otlptracehttp.WithInsecure())
	}
	if len(cfg.Headers) > 0 {
		options = append(options, otlptracehttp.WithHeaders(cfg.Headers))
	}
	return otlptracehttp.New(ctx, options...)
}

// createGRPCExporter creates a gRPC-based OTLP exporter
func createGRPCExporter(ctx context.Context, cfg OTLPConfig) (*otlptrace.Exporter, error) {
	options := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(cfg.Endpoint),
	}
	if cfg.Insecure {
		options = append(options, otlptracegrpc.WithInsecure())
	}
	if len(cfg.Headers) > 0 {
		options = append(options, otlptracegrpc.WithHeaders(cfg.Headers))
	}
	return otlptracegrpc.New(ctx, options...)
}
