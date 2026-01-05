// Package observability implements MetricsProvider using Prometheus client library.
//
// Prometheus will scrape /metrics and you'll see data like:
//
//	# HELP calque_flow_requests_total Counter for calque_flow_requests_total
//	# TYPE calque_flow_requests_total counter
//	calque_flow_requests_total{service="api",version="v1.0"} 1542
//
//	# HELP calque_flow_request_duration_seconds Histogram for calque_flow_request_duration_seconds
//	# TYPE calque_flow_request_duration_seconds histogram
//	calque_flow_request_duration_seconds_bucket{service="api",le="0.1"} 1200
//	calque_flow_request_duration_seconds_bucket{service="api",le="0.5"} 1500
//	calque_flow_request_duration_seconds_bucket{service="api",le="+Inf"} 1542
//
// For visualization, connect Prometheus to Grafana and create dashboards.
package observability

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// PrometheusProvider implements MetricsProvider using the Prometheus client library
type PrometheusProvider struct {
	mu         sync.RWMutex
	registry   *prometheus.Registry
	counters   map[string]*prometheus.CounterVec
	gauges     map[string]*prometheus.GaugeVec
	histograms map[string]*prometheus.HistogramVec

	// Buckets for histogram metrics
	durationBuckets []float64
}

// PrometheusOption configures the Prometheus provider
type PrometheusOption func(*PrometheusProvider)

// WithDurationBuckets sets custom buckets for duration histograms
func WithDurationBuckets(buckets []float64) PrometheusOption {
	return func(p *PrometheusProvider) {
		p.durationBuckets = buckets
	}
}

// WithPrometheusRegistry uses a custom Prometheus registry
func WithPrometheusRegistry(registry *prometheus.Registry) PrometheusOption {
	return func(p *PrometheusProvider) {
		p.registry = registry
	}
}

// NewPrometheusProvider creates a new Prometheus metrics provider.
//
// By default, it creates a new registry and includes Go runtime collectors
// (memory usage, goroutine count, etc.).
//
// Example - Basic usage:
//
//	provider := observability.NewPrometheusProvider()
//
// Example - Custom histogram buckets for latency:
//
//	provider := observability.NewPrometheusProvider(
//	    observability.WithDurationBuckets([]float64{0.01, 0.05, 0.1, 0.5, 1, 5}),
//	)
//
// Example - Use existing Prometheus registry:
//
//	provider := observability.NewPrometheusProvider(
//	    observability.WithPrometheusRegistry(prometheus.DefaultRegisterer.(*prometheus.Registry)),
//	)
func NewPrometheusProvider(opts ...PrometheusOption) *PrometheusProvider {
	p := &PrometheusProvider{
		registry:   prometheus.NewRegistry(),
		counters:   make(map[string]*prometheus.CounterVec),
		gauges:     make(map[string]*prometheus.GaugeVec),
		histograms: make(map[string]*prometheus.HistogramVec),
		durationBuckets: []float64{
			0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10,
		},
	}

	for _, opt := range opts {
		opt(p)
	}

	// Register default Go metrics
	p.registry.MustRegister(collectors.NewGoCollector())
	p.registry.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))

	return p
}

// Counter increments a counter metric
func (p *PrometheusProvider) Counter(_ context.Context, name string, value int64, labels map[string]string) {
	counter := p.getOrCreateCounter(name, labels)
	counter.With(labels).Add(float64(value))
}

// Gauge sets a gauge metric value
func (p *PrometheusProvider) Gauge(_ context.Context, name string, value float64, labels map[string]string) {
	gauge := p.getOrCreateGauge(name, labels)
	gauge.With(labels).Add(value)
}

// Histogram records a value in a histogram
func (p *PrometheusProvider) Histogram(_ context.Context, name string, value float64, labels map[string]string) {
	histogram := p.getOrCreateHistogram(name, labels)
	histogram.With(labels).Observe(value)
}

// RecordDuration records a duration in a histogram
func (p *PrometheusProvider) RecordDuration(_ context.Context, name string, duration time.Duration, labels map[string]string) {
	histogram := p.getOrCreateHistogram(name, labels)
	histogram.With(labels).Observe(duration.Seconds())
}

// Handler returns an HTTP handler for Prometheus metrics scraping
func (p *PrometheusProvider) Handler() http.Handler {
	return promhttp.HandlerFor(p.registry, promhttp.HandlerOpts{
		EnableOpenMetrics: true,
	})
}

// Registry returns the underlying Prometheus registry
func (p *PrometheusProvider) Registry() *prometheus.Registry {
	return p.registry
}

// getOrCreateCounter gets or creates a counter metric
func (p *PrometheusProvider) getOrCreateCounter(name string, labels map[string]string) *prometheus.CounterVec {
	p.mu.RLock()
	counter, exists := p.counters[name]
	p.mu.RUnlock()

	if exists {
		return counter
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// Double-check after acquiring write lock
	if counter, exists = p.counters[name]; exists {
		return counter
	}

	labelNames := labelNamesFromMap(labels)
	counter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: name,
		Help: "Counter for " + name,
	}, labelNames)

	p.registry.MustRegister(counter)
	p.counters[name] = counter
	return counter
}

// getOrCreateGauge gets or creates a gauge metric
func (p *PrometheusProvider) getOrCreateGauge(name string, labels map[string]string) *prometheus.GaugeVec {
	p.mu.RLock()
	gauge, exists := p.gauges[name]
	p.mu.RUnlock()

	if exists {
		return gauge
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// Double-check after acquiring write lock
	if gauge, exists = p.gauges[name]; exists {
		return gauge
	}

	labelNames := labelNamesFromMap(labels)
	gauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: name,
		Help: "Gauge for " + name,
	}, labelNames)

	p.registry.MustRegister(gauge)
	p.gauges[name] = gauge
	return gauge
}

// getOrCreateHistogram gets or creates a histogram metric
func (p *PrometheusProvider) getOrCreateHistogram(name string, labels map[string]string) *prometheus.HistogramVec {
	p.mu.RLock()
	histogram, exists := p.histograms[name]
	p.mu.RUnlock()

	if exists {
		return histogram
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// Double-check after acquiring write lock
	if histogram, exists = p.histograms[name]; exists {
		return histogram
	}

	labelNames := labelNamesFromMap(labels)
	histogram = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    name,
		Help:    "Histogram for " + name,
		Buckets: p.durationBuckets,
	}, labelNames)

	p.registry.MustRegister(histogram)
	p.histograms[name] = histogram
	return histogram
}

// labelNamesFromMap extracts label names from a map
func labelNamesFromMap(labels map[string]string) []string {
	names := make([]string, 0, len(labels))
	for k := range labels {
		names = append(names, k)
	}
	return names
}
