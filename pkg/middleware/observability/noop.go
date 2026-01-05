// Package observability provides no-op and in-memory implementations for testing and development.
package observability

import (
	"context"
	"sync"
	"time"
)

// NoopMetricsProvider is a no-op implementation of MetricsProvider.
// All methods do nothing. Use only in development or testing when you want to completely disable metrics.
type NoopMetricsProvider struct{}

// Counter does nothing
func (p *NoopMetricsProvider) Counter(_ context.Context, _ string, _ int64, _ map[string]string) {}

// Gauge does nothing
func (p *NoopMetricsProvider) Gauge(_ context.Context, _ string, _ float64, _ map[string]string) {}

// Histogram does nothing
func (p *NoopMetricsProvider) Histogram(_ context.Context, _ string, _ float64, _ map[string]string) {
}

// RecordDuration does nothing
func (p *NoopMetricsProvider) RecordDuration(_ context.Context, _ string, _ time.Duration, _ map[string]string) {
}

// NoopTracerProvider is a no-op implementation of TracerProvider.
// All methods do nothing. Use only in development or testing when you want to completely disable tracing.
type NoopTracerProvider struct{}

// StartSpan returns a no-op span
func (p *NoopTracerProvider) StartSpan(ctx context.Context, _ string, _ ...SpanOption) (context.Context, Span) {
	return ctx, &noopSpan{}
}

// Shutdown does nothing
func (p *NoopTracerProvider) Shutdown(_ context.Context) error {
	return nil
}

// noopSpan is a no-op span implementation
type noopSpan struct{}

func (s *noopSpan) End(_ error)                         {}
func (s *noopSpan) SetAttribute(_ string, _ any)        {}
func (s *noopSpan) AddEvent(_ string, _ map[string]any) {}
func (s *noopSpan) SetStatus(_ SpanStatus, _ string)    {}
func (s *noopSpan) SpanContext() SpanContext            { return SpanContext{} }

// InMemoryMetricsProvider stores metrics in memory for testing and debugging.
//
// Unlike NoopMetricsProvider, this actually stores the metrics so you can
// inspect them later. Perfect for unit tests where you want to verify
// that your code is recording the right metrics.
//
// Example:
//
//	provider := observability.NewInMemoryMetricsProvider()
//
//	// Use in your code
//	provider.Counter(ctx, "orders_created", 1, nil)
//	provider.Counter(ctx, "orders_created", 1, nil)
//
//	// Check in your test
//	count := provider.GetCounter("orders_created")
//	if count != 2 {
//	    t.Errorf("expected 2 orders, got %d", count)
//	}
type InMemoryMetricsProvider struct {
	mu         sync.RWMutex
	counters   map[string]int64
	gauges     map[string]float64
	histograms map[string][]float64
}

// NewInMemoryMetricsProvider creates a new in-memory metrics provider
func NewInMemoryMetricsProvider() *InMemoryMetricsProvider {
	return &InMemoryMetricsProvider{
		counters:   make(map[string]int64),
		gauges:     make(map[string]float64),
		histograms: make(map[string][]float64),
	}
}

// Counter increments a counter
func (p *InMemoryMetricsProvider) Counter(_ context.Context, name string, value int64, labels map[string]string) {
	key := metricsKey(name, labels)
	p.mu.Lock()
	defer p.mu.Unlock()
	p.counters[key] += value
}

// Gauge sets a gauge value
func (p *InMemoryMetricsProvider) Gauge(_ context.Context, name string, value float64, labels map[string]string) {
	key := metricsKey(name, labels)
	p.mu.Lock()
	defer p.mu.Unlock()
	p.gauges[key] += value
}

// Histogram records a histogram value
func (p *InMemoryMetricsProvider) Histogram(_ context.Context, name string, value float64, labels map[string]string) {
	key := metricsKey(name, labels)
	p.mu.Lock()
	defer p.mu.Unlock()
	p.histograms[key] = append(p.histograms[key], value)
}

// RecordDuration records a duration
func (p *InMemoryMetricsProvider) RecordDuration(_ context.Context, name string, duration time.Duration, labels map[string]string) {
	p.Histogram(context.Background(), name, duration.Seconds(), labels)
}

// GetCounter returns the current counter value
func (p *InMemoryMetricsProvider) GetCounter(name string, labels map[string]string) int64 {
	key := metricsKey(name, labels)
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.counters[key]
}

// GetGauge returns the current gauge value
func (p *InMemoryMetricsProvider) GetGauge(name string, labels map[string]string) float64 {
	key := metricsKey(name, labels)
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.gauges[key]
}

// GetHistogram returns all recorded histogram values
func (p *InMemoryMetricsProvider) GetHistogram(name string, labels map[string]string) []float64 {
	key := metricsKey(name, labels)
	p.mu.RLock()
	defer p.mu.RUnlock()
	values := make([]float64, len(p.histograms[key]))
	copy(values, p.histograms[key])
	return values
}

// Reset clears all metrics
func (p *InMemoryMetricsProvider) Reset() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.counters = make(map[string]int64)
	p.gauges = make(map[string]float64)
	p.histograms = make(map[string][]float64)
}

// metricsKey builds a key from metric name and labels
func metricsKey(name string, labels map[string]string) string {
	key := name
	for k, v := range labels {
		key += "|" + k + "=" + v
	}
	return key
}

// InMemoryTracerProvider stores traces in memory for testing and debugging.
//
// Unlike NoopTracerProvider, this actually records spans so you can inspect them.
// Perfect for unit tests where you want to verify that your code creates the
// right spans with the right attributes.
//
// Example:
//
//	provider := observability.NewInMemoryTracerProvider()
//
//	// Use in your code
//	ctx, span := provider.StartSpan(ctx, "process-order")
//	span.SetAttribute("order_id", "12345")
//	span.End(nil)
//
//	// Check in your test
//	spans := provider.GetSpans()
//	if len(spans) != 1 {
//	    t.Errorf("expected 1 span, got %d", len(spans))
//	}
//	if spans[0].Name != "process-order" {
//	    t.Errorf("expected span name 'process-order', got '%s'", spans[0].Name)
//	}
type InMemoryTracerProvider struct {
	mu    sync.RWMutex
	spans []*RecordedSpan
}

// RecordedSpan represents a recorded span for testing
type RecordedSpan struct {
	Name       string
	StartTime  time.Time
	EndTime    time.Time
	Attributes map[string]any
	Events     []RecordedEvent
	Status     SpanStatus
	StatusDesc string
	Error      error
	TraceID    string
	SpanID     string
}

// RecordedEvent represents a recorded span event
type RecordedEvent struct {
	Name       string
	Attributes map[string]any
	Time       time.Time
}

// NewInMemoryTracerProvider creates a new in-memory tracer provider
func NewInMemoryTracerProvider() *InMemoryTracerProvider {
	return &InMemoryTracerProvider{
		spans: make([]*RecordedSpan, 0),
	}
}

// StartSpan starts and records a new span
func (p *InMemoryTracerProvider) StartSpan(ctx context.Context, name string, opts ...SpanOption) (context.Context, Span) {
	cfg := &spanConfig{
		kind:       SpanKindInternal,
		attributes: make(map[string]any),
	}
	for _, opt := range opts {
		opt(cfg)
	}

	span := &RecordedSpan{
		Name:       name,
		StartTime:  time.Now(),
		Attributes: make(map[string]any),
		Events:     make([]RecordedEvent, 0),
		TraceID:    generateID(),
		SpanID:     generateID(),
	}

	// Copy initial attributes
	for k, v := range cfg.attributes {
		span.Attributes[k] = v
	}

	return ctx, &inMemorySpan{provider: p, span: span}
}

// Shutdown does nothing
func (p *InMemoryTracerProvider) Shutdown(_ context.Context) error {
	return nil
}

// GetSpans returns all recorded spans
func (p *InMemoryTracerProvider) GetSpans() []*RecordedSpan {
	p.mu.RLock()
	defer p.mu.RUnlock()
	spans := make([]*RecordedSpan, len(p.spans))
	copy(spans, p.spans)
	return spans
}

// GetSpansByName returns spans with the given name
func (p *InMemoryTracerProvider) GetSpansByName(name string) []*RecordedSpan {
	p.mu.RLock()
	defer p.mu.RUnlock()
	var result []*RecordedSpan
	for _, span := range p.spans {
		if span.Name == name {
			result = append(result, span)
		}
	}
	return result
}

// Reset clears all recorded spans
func (p *InMemoryTracerProvider) Reset() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.spans = make([]*RecordedSpan, 0)
}

func (p *InMemoryTracerProvider) recordSpan(span *RecordedSpan) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.spans = append(p.spans, span)
}

// inMemorySpan is an in-memory span implementation
type inMemorySpan struct {
	provider *InMemoryTracerProvider
	span     *RecordedSpan
}

func (s *inMemorySpan) End(err error) {
	s.span.EndTime = time.Now()
	s.span.Error = err
	s.provider.recordSpan(s.span)
}

func (s *inMemorySpan) SetAttribute(key string, value any) {
	s.span.Attributes[key] = value
}

func (s *inMemorySpan) AddEvent(name string, attrs map[string]any) {
	s.span.Events = append(s.span.Events, RecordedEvent{
		Name:       name,
		Attributes: attrs,
		Time:       time.Now(),
	})
}

func (s *inMemorySpan) SetStatus(code SpanStatus, description string) {
	s.span.Status = code
	s.span.StatusDesc = description
}

func (s *inMemorySpan) SpanContext() SpanContext {
	return SpanContext{
		TraceID: s.span.TraceID,
		SpanID:  s.span.SpanID,
	}
}

// generateID generates a simple ID for testing
func generateID() string {
	return time.Now().Format("20060102150405.000000000")
}
