package observability

import (
	"context"
	"strings"
	"testing"

	"github.com/calque-ai/go-calque/pkg/calque"
)

func TestMetricsHandler(t *testing.T) {
	t.Parallel()

	provider := NewInMemoryMetricsProvider()
	// Use single label to avoid map ordering issues
	labels := map[string]string{
		"service": "test-service",
	}

	// Create a simple handler that returns input uppercased
	innerHandler := calque.HandlerFunc(func(req *calque.Request, res *calque.Response) error {
		var input string
		if err := calque.Read(req, &input); err != nil {
			return err
		}
		return calque.Write(res, strings.ToUpper(input))
	})

	handler := MetricsHandler(provider, labels, innerHandler)

	// Execute the handler
	req := calque.NewRequest(context.Background(), strings.NewReader("hello"))
	buf := calque.NewWriter[string]()
	res := calque.NewResponse(buf)

	err := handler.ServeFlow(req, res)
	if err != nil {
		t.Fatalf("Handler failed: %v", err)
	}

	if buf.String() != "HELLO" {
		t.Errorf("Expected HELLO, got %s", buf.String())
	}

	// Check metrics were recorded - verify counter is > 0
	requestCount := provider.GetCounter("calque_flow_requests_total", labels)
	if requestCount != 1 {
		t.Errorf("Expected request count 1, got %d", requestCount)
	}

	// Check duration was recorded
	durations := provider.GetHistogram("calque_flow_request_duration_seconds", labels)
	if len(durations) != 1 {
		t.Errorf("Expected 1 duration recording, got %d", len(durations))
	}
}

func TestMetricsHandlerWithError(t *testing.T) {
	t.Parallel()

	provider := NewInMemoryMetricsProvider()
	labels := map[string]string{"service": "test"}

	// Create a handler that returns an error
	innerHandler := calque.HandlerFunc(func(req *calque.Request, _ *calque.Response) error {
		return calque.NewErr(req.Context, "test error")
	})

	handler := MetricsHandler(provider, labels, innerHandler)

	req := calque.NewRequest(context.Background(), strings.NewReader("input"))
	buf := calque.NewWriter[string]()
	res := calque.NewResponse(buf)

	err := handler.ServeFlow(req, res)
	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	// Check that requests counter was still incremented (even on error)
	requestCount := provider.GetCounter("calque_flow_requests_total", labels)
	if requestCount != 1 {
		t.Errorf("Expected request count 1, got %d", requestCount)
	}
}

func TestMetricsConfig(t *testing.T) {
	t.Parallel()

	cfg := DefaultMetricsConfig()

	if cfg.Namespace != "calque" {
		t.Errorf("Expected namespace 'calque', got '%s'", cfg.Namespace)
	}

	if cfg.Subsystem != "flow" {
		t.Errorf("Expected subsystem 'flow', got '%s'", cfg.Subsystem)
	}
}

func TestMetricName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		namespace string
		subsystem string
		metric    string
		expected  string
	}{
		{
			name:      "full name",
			namespace: "calque",
			subsystem: "flow",
			metric:    "requests_total",
			expected:  "calque_flow_requests_total",
		},
		{
			name:      "namespace only",
			namespace: "calque",
			subsystem: "",
			metric:    "requests_total",
			expected:  "calque_requests_total",
		},
		{
			name:      "subsystem only",
			namespace: "",
			subsystem: "flow",
			metric:    "requests_total",
			expected:  "flow_requests_total",
		},
		{
			name:      "metric only",
			namespace: "",
			subsystem: "",
			metric:    "requests_total",
			expected:  "requests_total",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := MetricsConfig{
				Namespace: tt.namespace,
				Subsystem: tt.subsystem,
			}
			result := metricName(cfg, tt.metric)
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestLabels_Merge(t *testing.T) {
	t.Parallel()

	l1 := Labels{"a": "1", "b": "2"}
	l2 := Labels{"b": "3", "c": "4"}

	merged := l1.Merge(l2)

	if merged["a"] != "1" {
		t.Errorf("Expected a=1, got a=%s", merged["a"])
	}
	if merged["b"] != "3" {
		t.Errorf("Expected b=3 (from l2), got b=%s", merged["b"])
	}
	if merged["c"] != "4" {
		t.Errorf("Expected c=4, got c=%s", merged["c"])
	}
}
