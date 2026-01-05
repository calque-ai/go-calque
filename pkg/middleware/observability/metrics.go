// Package observability provides middleware for collecting metrics about flow execution.
// These metrics help you understand how your application is performing and can be
// exported to Prometheus, Grafana, or any other monitoring system.
package observability

import (
	"time"

	"github.com/calque-ai/go-calque/pkg/calque"
)

// MetricsConfig configures the metrics middleware behavior.
type MetricsConfig struct {
	// Namespace prefixes all metric names (e.g., "calque" → "calque_requests_total")
	Namespace string

	// Subsystem is added after namespace (e.g., "flow" → "calque_flow_requests_total")
	// Use to group related metrics (e.g., "api", "worker", "cache")
	Subsystem string

	// Labels are default labels applied to ALL metrics from this middleware.
	// Common choices: service name, version, environment.
	Labels Labels

	// RecordRequestSize enables recording of request body sizes
	RecordRequestSize bool

	// RecordResponseSize enables recording of response body sizes
	RecordResponseSize bool
}

// DefaultMetricsConfig returns the default metrics configuration
func DefaultMetricsConfig() MetricsConfig {
	return MetricsConfig{
		Namespace:          "calque",
		Subsystem:          "flow",
		Labels:             Labels{},
		RecordRequestSize:  true,
		RecordResponseSize: true,
	}
}

// MetricsOption configures the metrics middleware
type MetricsOption func(*MetricsConfig)

// WithMetricsNamespace sets the namespace for metrics
func WithMetricsNamespace(namespace string) MetricsOption {
	return func(cfg *MetricsConfig) {
		cfg.Namespace = namespace
	}
}

// WithMetricsSubsystem sets the subsystem for metrics
func WithMetricsSubsystem(subsystem string) MetricsOption {
	return func(cfg *MetricsConfig) {
		cfg.Subsystem = subsystem
	}
}

// WithMetricsLabels sets default labels for all metrics
func WithMetricsLabels(labels Labels) MetricsOption {
	return func(cfg *MetricsConfig) {
		cfg.Labels = labels
	}
}

// Metrics creates a passthrough middleware that collects metrics.
//
// This middleware reads input, records metrics, and passes data through unchanged.
// Use this when you want to add metrics at a specific point in your pipeline
// without wrapping a specific handler.
//
// What metrics does it record?
//
//  1. calque_flow_requests_total (Counter)
//     - Counts every request that goes through
//     - Only goes up (never down)
//     - Example: 1542 requests total
//
//  2. calque_flow_request_duration_seconds (Histogram)
//     - Records how long each request took
//     - Lets you calculate percentiles (p50, p90, p99)
//     - Example: 95% of requests took less than 0.5 seconds
//
//  3. calque_flow_errors_total (Counter)
//     - Counts requests that failed
//     - Only incremented when handler returns an error
//     - Example: 23 errors total
//
//  4. calque_flow_in_flight_requests (Gauge)
//     - Shows how many requests are currently being processed
//     - Goes up when request starts, down when it finishes
//     - Example: 5 requests currently processing
//
// Example:
//
//	provider := observability.NewPrometheusProvider()
//	labels := map[string]string{"service": "ai-assistant", "version": "v1.0.0"}
//
//	flow := calque.NewFlow().
//	    Use(observability.Metrics(provider, labels)).  // Adds metrics here
//	    Use(ai.Agent(client))
//
// See MetricsHandler for wrapping a specific handler.
func Metrics(provider MetricsProvider, labels map[string]string, opts ...MetricsOption) calque.Handler {
	cfg := DefaultMetricsConfig()
	for _, opt := range opts {
		opt(&cfg)
	}

	// Merge default labels with provided labels
	allLabels := cfg.Labels.Merge(Labels(labels))

	return calque.HandlerFunc(func(req *calque.Request, res *calque.Response) error {
		ctx := req.Context
		start := time.Now()

		// Increment in-flight requests
		provider.Gauge(ctx, metricName(cfg, "in_flight_requests"), 1, allLabels)

		// Read and process - we need to pass through to next handler
		// Since this is middleware, we copy data through
		handlerErr := passThrough(req, res)

		duration := time.Since(start)

		// Record metrics
		provider.Counter(ctx, metricName(cfg, "requests_total"), 1, allLabels)
		provider.RecordDuration(ctx, metricName(cfg, "request_duration_seconds"), duration, allLabels)

		// Decrement in-flight requests
		provider.Gauge(ctx, metricName(cfg, "in_flight_requests"), -1, allLabels)

		if handlerErr != nil {
			errorLabels := allLabels.Merge(Labels{"error_type": errorType(handlerErr)})
			provider.Counter(ctx, metricName(cfg, "errors_total"), 1, errorLabels)
		}

		return handlerErr
	})
}

// MetricsHandler wraps a specific handler with metrics collection.
//
// This is the recommended way to add metrics to your handlers. It wraps your
// handler and records metrics about its execution (duration, success/failure).
//
// Parameters:
//   - provider: The metrics backend (Prometheus, in-memory, etc.)
//   - labels: Labels applied to all metrics (service name, version, etc.)
//   - handler: The handler to wrap and measure
//   - opts: Optional configuration (namespace, subsystem, etc.)
//
// Example:
//
//	provider := observability.NewPrometheusProvider()
//	labels := map[string]string{"service": "user-api"}
//
//	// Wrap your AI handler with metrics
//	aiHandler := ai.Agent(client)
//	metricsHandler := observability.MetricsHandler(provider, labels, aiHandler)
//
//	// Use in a flow
//	flow := calque.NewFlow().Use(metricsHandler)
//
// What metrics does it record?
//
//  1. calque_flow_requests_total (Counter)
//     - Counts every request that goes through
//     - Only goes up (never down)
//     - Example: 1542 requests total
//
//  2. calque_flow_request_duration_seconds (Histogram)
//     - Records how long each request took
//     - Lets you calculate percentiles (p50, p90, p99)
//     - Example: 95% of requests took less than 0.5 seconds
//
//  3. calque_flow_errors_total (Counter)
//     - Counts requests that failed
//     - Only incremented when handler returns an error
//     - Example: 23 errors total
//
//  4. calque_flow_in_flight_requests (Gauge)
//     - Shows how many requests are currently being processed
//     - Goes up when request starts, down when it finishes
//     - Example: 5 requests currently processing
func MetricsHandler(provider MetricsProvider, labels map[string]string, handler calque.Handler, opts ...MetricsOption) calque.Handler {
	cfg := DefaultMetricsConfig()
	for _, opt := range opts {
		opt(&cfg)
	}

	allLabels := cfg.Labels.Merge(Labels(labels))

	return calque.HandlerFunc(func(req *calque.Request, res *calque.Response) error {
		ctx := req.Context
		start := time.Now()

		// Increment in-flight requests
		provider.Gauge(ctx, metricName(cfg, "in_flight_requests"), 1, allLabels)

		// Execute the wrapped handler
		handlerErr := handler.ServeFlow(req, res)

		duration := time.Since(start)

		// Record metrics
		provider.Counter(ctx, metricName(cfg, "requests_total"), 1, allLabels)
		provider.RecordDuration(ctx, metricName(cfg, "request_duration_seconds"), duration, allLabels)

		// Decrement in-flight requests
		provider.Gauge(ctx, metricName(cfg, "in_flight_requests"), -1, allLabels)

		if handlerErr != nil {
			errorLabels := allLabels.Merge(Labels{"error_type": errorType(handlerErr)})
			provider.Counter(ctx, metricName(cfg, "errors_total"), 1, errorLabels)
		}

		return handlerErr
	})
}

// metricName builds the full metric name with namespace and subsystem
func metricName(cfg MetricsConfig, name string) string {
	if cfg.Namespace != "" && cfg.Subsystem != "" {
		return cfg.Namespace + "_" + cfg.Subsystem + "_" + name
	}
	if cfg.Namespace != "" {
		return cfg.Namespace + "_" + name
	}
	if cfg.Subsystem != "" {
		return cfg.Subsystem + "_" + name
	}
	return name
}

// errorType extracts a type string from an error for labeling
func errorType(err error) string {
	if err == nil {
		return ""
	}
	// Check for calque error types
	if _, ok := err.(*calque.Error); ok {
		return "calque_error"
	}
	return "unknown"
}

// passThrough copies data from request to response (middleware pattern)
func passThrough(req *calque.Request, res *calque.Response) error {
	var input string
	if err := calque.Read(req, &input); err != nil {
		return err
	}
	return calque.Write(res, input)
}
