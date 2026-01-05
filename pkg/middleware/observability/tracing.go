// Package observability provides middleware for distributed tracing in go-calque flows.
// Each operation creates a "span" that records:
//
//   - When the operation started and ended
//   - Whether it succeeded or failed
//   - Custom attributes you add (user ID, order ID, etc.)
//   - Create a tracer provider (sends traces to Jaeger/Tempo)
//
// Example:
//
//	provider, _ := observability.NewOTLPTracerProvider("my-service", "localhost:4317")
//	defer provider.Shutdown(context.Background())
//
//	Wrap your handlers with tracing
//	handler := observability.TracingHandler(provider, "process-order", orderHandler)
//
//	Use in a flow
//	flow := calque.NewFlow().Use(handler)
package observability

import (
	"github.com/calque-ai/go-calque/pkg/calque"
)

// TracingConfig configures the tracing middleware behavior.
type TracingConfig struct {
	// RecordInput records the input data as a span attribute.
	// WARNING: Be careful with sensitive data (passwords, tokens, PII).
	// Default: false (disabled for security)
	RecordInput bool

	// RecordOutput records the output data as a span attribute.
	// Default: false (disabled for security)
	RecordOutput bool

	// MaxAttributeLength truncates attribute values to this length.
	// This prevents huge payloads from bloating your traces.
	// Set to 0 for no limit (not recommended for production).
	// Default: 1024 characters
	MaxAttributeLength int
}

// DefaultTracingConfig returns the default tracing configuration
func DefaultTracingConfig() TracingConfig {
	return TracingConfig{
		RecordInput:        false,
		RecordOutput:       false,
		MaxAttributeLength: 1024,
	}
}

// TracingOption configures the tracing middleware
type TracingOption func(*TracingConfig)

// WithRecordInput enables recording of input data in spans
func WithRecordInput() TracingOption {
	return func(cfg *TracingConfig) {
		cfg.RecordInput = true
	}
}

// WithRecordOutput enables recording of output data in spans
func WithRecordOutput() TracingOption {
	return func(cfg *TracingConfig) {
		cfg.RecordOutput = true
	}
}

// WithMaxAttributeLength sets the maximum length for attribute values
func WithMaxAttributeLength(length int) TracingOption {
	return func(cfg *TracingConfig) {
		cfg.MaxAttributeLength = length
	}
}

// Tracing creates a passthrough middleware that adds a trace span.
//
// This middleware reads input, creates a span, and passes data through unchanged.
// Use this when you want to add a trace point at a specific location in your pipeline
// without wrapping a specific handler.
//
// The operationName should be descriptive but not too specific:
//   - Good: "validate-input", "call-openai", "save-to-db"
//   - Bad: "validate-user-123", "call-api-at-10:30am"
//
// Example - Add trace points between handlers:
//
//	provider, _ := observability.NewOTLPTracerProvider("ai-service", "localhost:4317")
//
//	flow := calque.NewFlow().
//	    Use(observability.Tracing(provider, "input-validation")).
//	    Use(guardrails.InputFilter(rules...)).
//	    Use(observability.Tracing(provider, "ai-processing")).
//	    Use(ai.Agent(client)).
//	    Use(observability.Tracing(provider, "output-validation")).
//	    Use(guardrails.OutputValidator(schema))
//
// This creates a trace like:
//
//	input-validation (2ms) → ai-processing (150ms) → output-validation (3ms)
//
// See TracingHandler for wrapping a specific handler.
func Tracing(provider TracerProvider, operationName string, opts ...TracingOption) calque.Handler {
	cfg := DefaultTracingConfig()
	for _, opt := range opts {
		opt(&cfg)
	}

	return calque.HandlerFunc(func(req *calque.Request, res *calque.Response) error {
		ctx := req.Context

		// Start a new span
		ctx, span := provider.StartSpan(ctx, operationName, WithSpanKind(SpanKindInternal))

		// Optionally record input
		if cfg.RecordInput {
			var input string
			if err := calque.Read(req, &input); err == nil {
				span.SetAttribute("input", truncate(input, cfg.MaxAttributeLength))
				// Create new request with the read data for downstream handlers
				req = calque.NewRequest(ctx, calque.NewReader(input))
			}
		} else {
			// Update request context with span context
			req = req.WithContext(ctx)
		}

		// Pass through to next handler
		var output string
		handlerErr := calque.Read(req, &output)
		if handlerErr == nil {
			handlerErr = calque.Write(res, output)
		}

		// Optionally record output
		if cfg.RecordOutput && handlerErr == nil {
			span.SetAttribute("output", truncate(output, cfg.MaxAttributeLength))
		}

		// Record error if any
		if handlerErr != nil {
			span.SetStatus(SpanStatusError, handlerErr.Error())
			span.SetAttribute("error", handlerErr.Error())
		} else {
			span.SetStatus(SpanStatusOK, "")
		}

		// End the span
		span.End(handlerErr)

		return handlerErr
	})
}

// TracingHandler wraps a specific handler with a trace span.
// It wraps your handler and creates a span that measures its execution time and captures errors.
//
// Example:
//
//	provider, _ := observability.NewOTLPTracerProvider("payment-service", "localhost:4317")
//
//	// Wrap your payment handler with tracing
//	paymentHandler := payments.ProcessPayment(stripe)
//	tracedHandler := observability.TracingHandler(provider, "process-payment", paymentHandler)
//
//	// Use in a flow
//	flow := calque.NewFlow().Use(tracedHandler)
//
// With options to capture input/output (be careful with sensitive data):
//
//	tracedHandler := observability.TracingHandler(
//	    provider,
//	    "ai-completion",
//	    aiHandler,
//	    observability.WithRecordInput(),   // Records prompt
//	    observability.WithRecordOutput(),  // Records response
//	    observability.WithMaxAttributeLength(500), // Limit size
//	)
func TracingHandler(provider TracerProvider, operationName string, handler calque.Handler, opts ...TracingOption) calque.Handler {
	cfg := DefaultTracingConfig()
	for _, opt := range opts {
		opt(&cfg)
	}

	return calque.HandlerFunc(func(req *calque.Request, res *calque.Response) error {
		ctx := req.Context

		// Start a new span
		ctx, span := provider.StartSpan(ctx, operationName, WithSpanKind(SpanKindInternal))

		// Update request context with span context
		req = req.WithContext(ctx)

		// Optionally record input (requires buffering)
		if cfg.RecordInput {
			var input string
			if err := calque.Read(req, &input); err == nil {
				span.SetAttribute("input", truncate(input, cfg.MaxAttributeLength))
				// Create new request with the read data
				req = calque.NewRequest(ctx, calque.NewReader(input))
			}
		}

		// Capture output if needed
		var outputBuf *calque.Buffer[string]
		var targetRes *calque.Response
		if cfg.RecordOutput {
			outputBuf = calque.NewWriter[string]()
			targetRes = calque.NewResponse(outputBuf)
		} else {
			targetRes = res
		}

		// Execute the wrapped handler
		handlerErr := handler.ServeFlow(req, targetRes)

		// If we captured output, write it to the real response and record it
		if cfg.RecordOutput && handlerErr == nil {
			output := outputBuf.String()
			span.SetAttribute("output", truncate(output, cfg.MaxAttributeLength))
			if err := calque.Write(res, output); err != nil {
				handlerErr = err
			}
		}

		// Record error if any
		if handlerErr != nil {
			span.SetStatus(SpanStatusError, handlerErr.Error())
			span.SetAttribute("error", handlerErr.Error())
		} else {
			span.SetStatus(SpanStatusOK, "")
		}

		// End the span
		span.End(handlerErr)

		return handlerErr
	})
}

// truncate truncates a string to the given length
func truncate(s string, maxLen int) string {
	if maxLen <= 0 || len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
