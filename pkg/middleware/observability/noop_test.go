package observability

import (
	"context"
	"testing"
	"time"
)

func TestNoopMetricsProvider(t *testing.T) {
	t.Parallel()

	provider := &NoopMetricsProvider{}
	ctx := context.Background()
	labels := map[string]string{"test": "value"}

	// These should not panic
	provider.Counter(ctx, "test_counter", 1, labels)
	provider.Gauge(ctx, "test_gauge", 1.5, labels)
	provider.Histogram(ctx, "test_histogram", 0.5, labels)
	provider.RecordDuration(ctx, "test_duration", time.Second, labels)
}

func TestNoopTracerProvider(t *testing.T) {
	t.Parallel()

	provider := &NoopTracerProvider{}
	ctx := context.Background()

	newCtx, span := provider.StartSpan(ctx, "test-span")

	if newCtx != ctx {
		t.Error("Expected context to be returned unchanged")
	}

	// These should not panic
	span.SetAttribute("key", "value")
	span.AddEvent("event", map[string]any{"key": "value"})
	span.SetStatus(SpanStatusOK, "ok")
	span.End(nil)

	sc := span.SpanContext()
	if sc.TraceID != "" || sc.SpanID != "" {
		t.Error("Expected empty span context from noop provider")
	}

	err := provider.Shutdown(ctx)
	if err != nil {
		t.Errorf("Unexpected error from Shutdown: %v", err)
	}
}

func TestInMemoryMetricsProvider(t *testing.T) {
	t.Parallel()

	provider := NewInMemoryMetricsProvider()
	ctx := context.Background()
	labels := map[string]string{"service": "test"}

	// Test counter
	provider.Counter(ctx, "requests", 1, labels)
	provider.Counter(ctx, "requests", 2, labels)

	if got := provider.GetCounter("requests", labels); got != 3 {
		t.Errorf("Expected counter 3, got %d", got)
	}

	// Test gauge
	provider.Gauge(ctx, "connections", 5, labels)
	provider.Gauge(ctx, "connections", -2, labels)

	if got := provider.GetGauge("connections", labels); got != 3 {
		t.Errorf("Expected gauge 3, got %f", got)
	}

	// Test histogram
	provider.Histogram(ctx, "latency", 0.1, labels)
	provider.Histogram(ctx, "latency", 0.2, labels)

	hist := provider.GetHistogram("latency", labels)
	if len(hist) != 2 {
		t.Errorf("Expected 2 histogram values, got %d", len(hist))
	}

	// Test duration
	provider.RecordDuration(ctx, "duration", 100*time.Millisecond, labels)
	durations := provider.GetHistogram("duration", labels)
	if len(durations) != 1 {
		t.Errorf("Expected 1 duration value, got %d", len(durations))
	}

	// Test reset
	provider.Reset()
	if got := provider.GetCounter("requests", labels); got != 0 {
		t.Errorf("Expected counter 0 after reset, got %d", got)
	}
}

func TestInMemoryTracerProvider(t *testing.T) {
	t.Parallel()

	provider := NewInMemoryTracerProvider()
	ctx := context.Background()

	// Create a span
	_, span := provider.StartSpan(ctx, "test-operation", WithAttributes(map[string]any{
		"initial": "attr",
	}))

	span.SetAttribute("key", "value")
	span.AddEvent("test-event", map[string]any{"event_key": "event_value"})
	span.SetStatus(SpanStatusOK, "success")
	span.End(nil)

	// Check recorded spans
	spans := provider.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("Expected 1 span, got %d", len(spans))
	}

	s := spans[0]
	if s.Name != "test-operation" {
		t.Errorf("Expected span name 'test-operation', got '%s'", s.Name)
	}

	if s.Attributes["initial"] != "attr" {
		t.Errorf("Expected initial attribute, got %v", s.Attributes["initial"])
	}

	if s.Attributes["key"] != "value" {
		t.Errorf("Expected key=value attribute, got %v", s.Attributes["key"])
	}

	if len(s.Events) != 1 {
		t.Errorf("Expected 1 event, got %d", len(s.Events))
	}

	if s.Status != SpanStatusOK {
		t.Errorf("Expected SpanStatusOK, got %v", s.Status)
	}

	// Test GetSpansByName - start more spans but don't end them
	_, span2 := provider.StartSpan(ctx, "another-operation")
	_, span3 := provider.StartSpan(ctx, "test-operation")

	// End the new spans
	span2.End(nil)
	span3.End(nil)

	// Now we should have 3 spans total
	allSpans := provider.GetSpans()
	if len(allSpans) != 3 {
		t.Errorf("Expected 3 total spans, got %d", len(allSpans))
	}

	// Filter by name
	testSpans := provider.GetSpansByName("test-operation")
	if len(testSpans) != 2 {
		t.Errorf("Expected 2 spans with name 'test-operation', got %d", len(testSpans))
	}

	// Test reset
	provider.Reset()
	spans = provider.GetSpans()
	if len(spans) != 0 {
		t.Errorf("Expected 0 spans after reset, got %d", len(spans))
	}
}

func TestSpanContext(t *testing.T) {
	t.Parallel()

	provider := NewInMemoryTracerProvider()
	_, span := provider.StartSpan(context.Background(), "test")

	sc := span.SpanContext()
	if sc.TraceID == "" {
		t.Error("Expected non-empty TraceID")
	}
	if sc.SpanID == "" {
		t.Error("Expected non-empty SpanID")
	}
}
