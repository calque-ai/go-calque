// Package calque provides context utilities for propagating metadata, logging,
// and tracing information through concurrent middleware chains.
//
// Since go-calque runs all middleware in a flow concurrently, traditional context
// value modification doesn't work (all handlers receive the same context at once).
// MetadataBus solves this by providing a channel-based communication mechanism
// that is set once in context but allows mutable operations through its methods.
package calque

import (
	"context"
	"errors"
	"log/slog"
	"sync"
)

type ctxKey string

const (
	metadataBusKey ctxKey = "calque.metadata_bus"
	loggerKey      ctxKey = "calque.logger"
	traceIDKey     ctxKey = "calque.trace_id"
	requestIDKey   ctxKey = "calque.request_id"
)

// DefaultMetadataBusBuffer is the default buffer size for MetadataBus channels.
const DefaultMetadataBusBuffer = 100

// ErrBusClosed is returned when operations are attempted on a closed MetadataBus.
var ErrBusClosed = errors.New("metadata bus is closed")

// Metadata represents a key-value pair for metadata communication.
type Metadata struct {
	Key   string
	Value any
}

// MetadataBus provides thread-safe metadata sharing between concurrent middleware.
//
// The MetadataBus pointer is stored in context (immutable reference), but the
// internal channels and sync.Map are mutable, allowing handlers to communicate
// even though they all receive the same context simultaneously.
//
// Usage patterns:
//   - Set/Get: For values that are written once and read many times (trace ID, request ID)
//   - Send/Receive: For streaming metadata between handlers
//
// Example:
//
//	mb := NewMetadataBus(100)
//	ctx = WithMetadataBus(ctx, mb)
//
//	// Handler 1: Set a value
//	mb.Set("user_id", "123")
//
//	// Handler 2: Get the value (non-blocking)
//	userID, ok := mb.Get("user_id")
//
//	// Handler 3: Send streaming metadata
//	mb.Send(Metadata{Key: "event", Value: "processed"})
//
//	// Handler 4: Receive streaming metadata
//	for meta := range mb.Receive() {
//	    // process metadata
//	}
type MetadataBus struct {
	// ch is the buffered channel for streaming metadata between handlers
	ch chan Metadata

	// closedCh is closed when the bus is closed - single source of truth for closed state
	// Used in select statements to unblock waiting senders
	closedCh chan struct{}

	// store is a sync.Map for values that are set once and read many times
	// This provides non-blocking reads for things like trace ID, request ID
	store sync.Map

	// bufferSize stores the configured buffer size
	bufferSize int

	// mu protects send and close operations
	mu sync.RWMutex
}

// NewMetadataBus creates a new MetadataBus with the specified buffer size.
//
// The buffer size determines how many metadata items can be queued before
// Send() blocks. Use a larger buffer for high-throughput scenarios.
//
// If bufferSize is 0 or negative, DefaultMetadataBusBuffer is used.
//
// Example:
//
//	mb := NewMetadataBus(100)  // Buffer of 100 items
//	mb := NewMetadataBus(0)   // Uses default buffer (100)
func NewMetadataBus(bufferSize int) *MetadataBus {
	if bufferSize <= 0 {
		bufferSize = DefaultMetadataBusBuffer
	}
	return &MetadataBus{
		ch:         make(chan Metadata, bufferSize),
		closedCh:   make(chan struct{}),
		bufferSize: bufferSize,
	}
}

// Set stores a value in the sync.Map for non-blocking retrieval.
//
// Use Set for values that are written once and read many times, such as
// trace ID, request ID, or user context. This is more efficient than
// Send/Receive for such use cases.
//
// Set is thread-safe and can be called from any goroutine.
//
// Example:
//
//	mb.Set("trace_id", "abc-123")
//	mb.Set("user_id", 42)
func (mb *MetadataBus) Set(key string, value any) {
	mb.store.Store(key, value)
}

// Get retrieves a value from the sync.Map without blocking.
//
// Returns the value and true if found, or nil and false if not found.
// This is the preferred method for reading values that were set with Set().
//
// Get is thread-safe and can be called from any goroutine.
//
// Example:
//
//	if traceID, ok := mb.Get("trace_id"); ok {
//	    fmt.Println("Trace ID:", traceID)
//	}
func (mb *MetadataBus) Get(key string) (any, bool) {
	return mb.store.Load(key)
}

// GetString retrieves a string value from the sync.Map.
//
// Returns the string value and true if found and is a string,
// or empty string and false otherwise.
//
// Example:
//
//	if traceID, ok := mb.GetString("trace_id"); ok {
//	    fmt.Println("Trace ID:", traceID)
//	}
func (mb *MetadataBus) GetString(key string) (string, bool) {
	if v, ok := mb.store.Load(key); ok {
		if s, ok := v.(string); ok {
			return s, true
		}
	}
	return "", false
}

// GetInt retrieves an int value from the sync.Map.
//
// Returns the int value and true if found and is an int,
// or 0 and false otherwise.
//
// Example:
//
//	if count, ok := mb.GetInt("retry_count"); ok {
//	    fmt.Println("Retries:", count)
//	}
func (mb *MetadataBus) GetInt(key string) (int, bool) {
	if v, ok := mb.store.Load(key); ok {
		if i, ok := v.(int); ok {
			return i, true
		}
	}
	return 0, false
}

// GetBool retrieves a bool value from the sync.Map.
//
// Returns the bool value and true if found and is a bool,
// or false and false otherwise.
//
// Example:
//
//	if enabled, ok := mb.GetBool("feature_enabled"); ok {
//	    fmt.Println("Enabled:", enabled)
//	}
func (mb *MetadataBus) GetBool(key string) (bool, bool) {
	if v, ok := mb.store.Load(key); ok {
		if b, ok := v.(bool); ok {
			return b, true
		}
	}
	return false, false
}

// Delete removes a value from the sync.Map.
//
// Example:
//
//	mb.Delete("temp_key")
func (mb *MetadataBus) Delete(key string) {
	mb.store.Delete(key)
}

// Send sends metadata through the channel for streaming communication.
//
// Use Send for metadata that needs to flow between handlers in real-time.
// Send will block if the channel buffer is full and bus is not closed.
//
// Returns false if the bus has been closed.
//
// Example:
//
//	mb.Send(Metadata{Key: "event", Value: "request_started"})
//	mb.Send(Metadata{Key: "latency_ms", Value: 42})
func (mb *MetadataBus) Send(meta Metadata) bool {
	mb.mu.RLock()
	defer mb.mu.RUnlock()

	// Quick check if already closed
	select {
	case <-mb.closedCh:
		return false
	default:
	}

	// Send with close detection
	select {
	case mb.ch <- meta:
		return true
	case <-mb.closedCh:
		return false
	}
}

// SendNonBlocking attempts to send metadata without blocking.
//
// Returns true if the metadata was sent, false if the channel is full or closed.
// Use this when you don't want to block the handler.
//
// Example:
//
//	if !mb.SendNonBlocking(Metadata{Key: "metric", Value: 42}) {
//	    // Channel full, handle accordingly
//	}
func (mb *MetadataBus) SendNonBlocking(meta Metadata) bool {
	mb.mu.RLock()
	defer mb.mu.RUnlock()

	select {
	case <-mb.closedCh:
		return false
	default:
	}

	select {
	case mb.ch <- meta:
		return true
	default:
		return false
	}
}

// SendContext sends metadata with context cancellation support.
//
// Returns nil if sent successfully, context error if cancelled,
// or ErrBusClosed if the bus has been closed.
//
// Example:
//
//	ctx, cancel := context.WithTimeout(ctx, time.Second)
//	defer cancel()
//	if err := mb.SendContext(ctx, Metadata{Key: "event", Value: "done"}); err != nil {
//	    // Handle timeout or cancellation
//	}
func (mb *MetadataBus) SendContext(ctx context.Context, meta Metadata) error {
	mb.mu.RLock()
	defer mb.mu.RUnlock()

	// Quick check if already closed
	select {
	case <-mb.closedCh:
		return ErrBusClosed
	default:
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-mb.closedCh:
		return ErrBusClosed
	case mb.ch <- meta:
		return nil
	}
}

// Receive returns the channel for receiving metadata.
//
// Use in a for-range loop to receive all metadata. The loop will exit
// when the channel is closed via Close().
//
// Example:
//
//	go func() {
//	    for meta := range mb.Receive() {
//	        fmt.Printf("Received: %s = %v\n", meta.Key, meta.Value)
//	    }
//	}()
func (mb *MetadataBus) Receive() <-chan Metadata {
	return mb.ch
}

// TryReceive attempts to receive metadata without blocking.
//
// Returns the metadata and true if available, or empty Metadata and false
// if no metadata is available.
//
// Example:
//
//	if meta, ok := mb.TryReceive(); ok {
//	    fmt.Printf("Got: %s = %v\n", meta.Key, meta.Value)
//	}
func (mb *MetadataBus) TryReceive() (Metadata, bool) {
	select {
	case meta, ok := <-mb.ch:
		return meta, ok
	default:
		return Metadata{}, false
	}
}

// ReceiveContext receives metadata with context cancellation support.
//
// Returns the metadata if received successfully, context error if cancelled,
// or ErrBusClosed if the bus has been closed.
//
// Example:
//
//	ctx, cancel := context.WithTimeout(ctx, time.Second)
//	defer cancel()
//	meta, err := mb.ReceiveContext(ctx)
//	if err != nil {
//	    // Handle timeout, cancellation, or closed bus
//	}
func (mb *MetadataBus) ReceiveContext(ctx context.Context) (Metadata, error) {
	select {
	case <-ctx.Done():
		return Metadata{}, ctx.Err()
	case meta, ok := <-mb.ch:
		if !ok {
			return Metadata{}, ErrBusClosed
		}
		return meta, nil
	}
}

// Close closes the metadata bus.
//
// After closing, Send will return false, IsClosed will return true,
// and Receive() will eventually drain and close.
// Close should be called when the flow completes. Multiple calls to Close are safe.
func (mb *MetadataBus) Close() {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	// Already closed - safe to call multiple times
	select {
	case <-mb.closedCh:
		return
	default:
	}

	close(mb.closedCh)
	close(mb.ch)
}

// IsClosed returns true if the bus has been closed.
// This is a lock-free check using the closedCh channel.
func (mb *MetadataBus) IsClosed() bool {
	select {
	case <-mb.closedCh:
		return true
	default:
		return false
	}
}

// BufferSize returns the configured buffer size.
func (mb *MetadataBus) BufferSize() int {
	return mb.bufferSize
}

// --- Context Helpers ---

// WithMetadataBus stores a MetadataBus in the context.
//
// The MetadataBus should be created once before calling Flow.Run and stored
// in context. All handlers will receive the same reference and can use its
// methods for communication.
//
// Example:
//
//	mb := NewMetadataBus(100)
//	ctx = WithMetadataBus(ctx, mb)
//	flow.Run(ctx, input, &output)
func WithMetadataBus(ctx context.Context, mb *MetadataBus) context.Context {
	return context.WithValue(ctx, metadataBusKey, mb)
}

// GetMetadataBus retrieves the MetadataBus from context.
//
// Returns nil if no MetadataBus is found in context.
//
// Example:
//
//	if mb := GetMetadataBus(ctx); mb != nil {
//	    mb.Set("key", "value")
//	}
func GetMetadataBus(ctx context.Context) *MetadataBus {
	if mb, ok := ctx.Value(metadataBusKey).(*MetadataBus); ok {
		return mb
	}
	return nil
}

// WithLogger stores a slog.Logger in the context.
//
// The logger will be used by LogInfo, LogDebug, LogWarn, LogError functions.
// If no logger is set, slog.Default() is used.
//
// Example:
//
//	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
//	ctx = WithLogger(ctx, logger)
func WithLogger(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerKey, logger)
}

// Logger retrieves the slog.Logger from context.
//
// Returns slog.Default() if no logger is found in context.
//
// Example:
//
//	logger := Logger(ctx)
//	logger.Info("message", "key", "value")
func Logger(ctx context.Context) *slog.Logger {
	if logger, ok := ctx.Value(loggerKey).(*slog.Logger); ok {
		return logger
	}
	return slog.Default()
}

// WithTraceID stores a trace ID in the context.
//
// Trace ID is typically set at the start of a request and used for
// correlating logs across the request lifecycle.
//
// Example:
//
//	ctx = WithTraceID(ctx, "abc-123-def-456")
func WithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, traceIDKey, traceID)
}

// TraceID retrieves the trace ID from context.
//
// Returns empty string if no trace ID is found.
//
// Example:
//
//	if traceID := TraceID(ctx); traceID != "" {
//	    fmt.Println("Trace:", traceID)
//	}
func TraceID(ctx context.Context) string {
	if id, ok := ctx.Value(traceIDKey).(string); ok {
		return id
	}
	return ""
}

// WithRequestID stores a request ID in the context.
//
// Request ID is typically unique per request and used for tracking
// individual requests through the system.
//
// Example:
//
//	ctx = WithRequestID(ctx, "req-12345")
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey, requestID)
}

// RequestID retrieves the request ID from context.
//
// Returns empty string if no request ID is found.
//
// Example:
//
//	if reqID := RequestID(ctx); reqID != "" {
//	    fmt.Println("Request:", reqID)
//	}
func RequestID(ctx context.Context) string {
	if id, ok := ctx.Value(requestIDKey).(string); ok {
		return id
	}
	return ""
}
