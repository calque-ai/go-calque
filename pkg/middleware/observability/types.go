// Package observability provides middleware for metrics, tracing, and health checks
// in go-calque flows. It uses OpenTelemetry as the core abstraction for vendor-neutral
// instrumentation that can export to Prometheus, Jaeger, Grafana, and more.
package observability

import (
	"context"
	"time"
)

// MetricsProvider defines the interface for collecting and exposing metrics.
// There are three main types of metrics:
//   - Counter: A value that only goes up (e.g., total requests, total errors)
//   - Gauge: A value that can go up or down (e.g., active connections, queue size)
//   - Histogram: Distribution of values (e.g., request latencies, response sizes)
type MetricsProvider interface {
	// Counter increments a counter metric by the given value.
	// Counters are cumulative metrics that only increase (or reset to zero on restart).
	// Use counters for: total requests, total errors, bytes sent, etc.
	Counter(ctx context.Context, name string, value int64, labels map[string]string)

	// Gauge sets a gauge metric to the given value.
	// Gauges represent a single numerical value that can go up and down.
	// Use gauges for: current temperature, memory usage, active connections, queue size.
	//
	// Note: This implementation uses Add semantics, so pass negative values to decrease.
	Gauge(ctx context.Context, name string, value float64, labels map[string]string)

	// Histogram records a value in a histogram metric.
	// Histograms track the distribution of values across predefined buckets.
	// Use histograms for: request latencies, response sizes, batch sizes.
	//
	// The histogram automatically calculates percentiles (p50, p90, p99, etc.)
	Histogram(ctx context.Context, name string, value float64, labels map[string]string)

	// RecordDuration is a convenience method for recording time durations.
	// It converts the duration to seconds and records it as a histogram.
	// Use for: request processing time, database query time, API call latency.
	RecordDuration(ctx context.Context, name string, duration time.Duration, labels map[string]string)
}

// TracerProvider defines the interface for distributed tracing.
//
// Distributed tracing allows you to follow a request as it travels through
// your system across multiple services and components. Each operation creates
// a "span" that records timing, errors, and custom attributes.
//
// Key concepts:
//   - Trace: The entire journey of a request (made up of multiple spans)
//   - Span: A single operation within a trace (e.g., "database-query", "api-call")
//   - TraceID: Unique identifier that links all spans in a trace together
//   - SpanID: Unique identifier for a single span
//
// Implementations include:
//   - OTLPTracerProvider: Exports traces via OTLP to Jaeger, Tempo, etc.
//   - InMemoryTracerProvider: Stores traces in memory (for testing)
//   - NoopTracerProvider: Does nothing (for disabled tracing)
//
// Example usage:
//
//	provider, _ := observability.NewOTLPTracerProvider("my-service", "localhost:4317")
//	ctx, span := provider.StartSpan(ctx, "process-order")
//	defer span.End(nil)
//	span.SetAttribute("order_id", "12345")
type TracerProvider interface {
	// StartSpan starts a new span with the given operation name.
	// It returns a new context containing the span (for propagation to child operations)
	// and the span itself (for adding attributes and ending).
	StartSpan(ctx context.Context, name string, opts ...SpanOption) (context.Context, Span)

	// Shutdown gracefully shuts down the tracer provider.
	// This flushes any pending traces and releases resources.
	// Always call this before your application exits.
	Shutdown(ctx context.Context) error
}

// Span represents a single operation within a trace.
// Always call End() when the operation completes, typically using defer.
// Example:
// ctx, span := tracer.StartSpan(ctx, "my-operation")
// defer span.End(nil) // or span.End(err) if there's an error.
type Span interface {
	// End ends the span and records the final status.
	// If err is non-nil, the span is marked as failed.
	// This method should be called exactly once per span.
	End(err error)

	// SetAttribute sets a key-value attribute on the span.
	// Attributes provide additional context about the operation.
	// Examples: user_id, order_total, cache_hit, retry_count
	SetAttribute(key string, value any)

	// AddEvent adds a timestamped event to the span.
	// Events mark interesting moments during the operation.
	// Examples: "cache-miss", "retry-attempt", "fallback-triggered"
	AddEvent(name string, attrs map[string]any)

	// SetStatus sets the span status explicitly.
	// Use SpanStatusOK for success, SpanStatusError for failure.
	SetStatus(code SpanStatus, description string)

	// SpanContext returns the span's trace and span IDs.
	// Use this for logging correlation or manual context propagation.
	SpanContext() SpanContext
}

// SpanContext contains identifying trace information about a span.
// These IDs allow you to correlate logs, metrics, and traces together.
type SpanContext struct {
	// TraceID is the unique identifier for the entire trace.
	// All spans in the same request share this ID.
	TraceID string

	// SpanID is the unique identifier for this specific span.
	SpanID string
}

// SpanStatus represents the status of a span
type SpanStatus int

const (
	// SpanStatusUnset is the default status
	SpanStatusUnset SpanStatus = iota
	// SpanStatusOK indicates the operation completed successfully
	SpanStatusOK
	// SpanStatusError indicates the operation failed
	SpanStatusError
)

// SpanOption configures span creation
type SpanOption func(*spanConfig)

type spanConfig struct {
	kind       SpanKind
	attributes map[string]any
}

// SpanKind describes the relationship between the Span, its parents, and its children
type SpanKind int

const (
	// SpanKindInternal is the default kind
	SpanKindInternal SpanKind = iota
	// SpanKindServer indicates the span covers server-side handling
	SpanKindServer
	// SpanKindClient indicates the span describes a request to some remote service
	SpanKindClient
	// SpanKindProducer indicates the span describes producer sending a message
	SpanKindProducer
	// SpanKindConsumer indicates the span describes consumer receiving a message
	SpanKindConsumer
)

// WithSpanKind sets the kind of span
func WithSpanKind(kind SpanKind) SpanOption {
	return func(cfg *spanConfig) {
		cfg.kind = kind
	}
}

// WithAttributes sets initial attributes on the span
func WithAttributes(attrs map[string]any) SpanOption {
	return func(cfg *spanConfig) {
		cfg.attributes = attrs
	}
}

// HealthChecker defines the interface for health checks.
//
// Health checks verify that your application's dependencies are working correctly.
// They're typically exposed via HTTP endpoints like /health or /ready for:
//   - Kubernetes liveness/readiness probes
//   - Load balancer health checks
//   - Monitoring and alerting systems
//
// Built-in implementations:
//   - TCPHealthCheck: Checks if a TCP port is reachable (e.g., database)
//   - HTTPHealthCheck: Checks if an HTTP endpoint returns 200 OK
//   - FuncHealthCheck: Runs a custom function (for any custom logic)
type HealthChecker interface {
	// Name returns a human-readable name for this health check.
	// This appears in the health report JSON output.
	// Examples: "postgres", "redis", "openai-api"
	Name() string

	// Check performs the health check and returns nil if healthy.
	// Return an error to indicate the dependency is unhealthy.
	// The context may have a timeout set - respect ctx.Done().
	Check(ctx context.Context) error

	// Timeout returns the maximum time to wait for this check.
	// If zero, a default timeout is used.
	Timeout() time.Duration
}

// HealthStatus represents the overall health status of the system.
type HealthStatus string

const (
	// HealthStatusHealthy indicates all checks passed - the system is fully operational.
	HealthStatusHealthy HealthStatus = "healthy"

	// HealthStatusUnhealthy indicates one or more critical checks failed.
	// The system should not receive traffic.
	HealthStatusUnhealthy HealthStatus = "unhealthy"

	// HealthStatusDegraded indicates some non-critical checks failed.
	// The system can still operate but with reduced functionality.
	HealthStatusDegraded HealthStatus = "degraded"
)

// HealthCheckResult represents the result of a single health check.
// This is included in the JSON health report.
type HealthCheckResult struct {
	Name    string        `json:"name"`            // Name of the check (e.g., "postgres")
	Status  string        `json:"status"`          // "ok" or "error"
	Error   string        `json:"error,omitempty"` // Error message if status is "error"
	Latency time.Duration `json:"latency"`         // How long the check took
}

// HealthReport represents the complete health report.
type HealthReport struct {
	Status    HealthStatus                 `json:"status"`    // Overall status: healthy/unhealthy/degraded
	Checks    map[string]HealthCheckResult `json:"checks"`    // Individual check results
	Uptime    time.Duration                `json:"uptime"`    // Time since application start
	Timestamp time.Time                    `json:"timestamp"` // When this report was generated
}

// Labels is a convenience type for metric and span labels.
//
// Labels (also called tags or dimensions) add context to metrics and traces.
// They allow you to filter and group your data in dashboards.
//
// Common labels:
//   - service: The name of your service ("api", "worker", "gateway")
//   - version: Your application version ("v1.2.3")
//   - environment: The deployment environment ("prod", "staging")
//   - method: HTTP method ("GET", "POST")
//   - status: Response status ("success", "error")
type Labels map[string]string

// Merge combines two label maps into a new map.
// If both maps have the same key, the value from 'other' takes precedence.
//
// Example:
//
//	base := Labels{"service": "api", "env": "prod"}
//	extra := Labels{"version": "v1.0", "env": "staging"}
//	merged := base.Merge(extra)
//	// Result: {"service": "api", "env": "staging", "version": "v1.0"}
func (l Labels) Merge(other Labels) Labels {
	result := make(Labels, len(l)+len(other))
	for k, v := range l {
		result[k] = v
	}
	for k, v := range other {
		result[k] = v
	}
	return result
}
