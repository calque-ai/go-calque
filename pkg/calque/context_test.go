package calque

import (
	"context"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"
)

const testEventKey = "event"
const testEventValue = "started"

func TestNewMetadataBus(t *testing.T) {
	tests := []struct {
		name           string
		bufferSize     int
		expectedBuffer int
	}{
		{
			name:           "default buffer size when zero",
			bufferSize:     0,
			expectedBuffer: DefaultMetadataBusBuffer,
		},
		{
			name:           "default buffer size when negative",
			bufferSize:     -10,
			expectedBuffer: DefaultMetadataBusBuffer,
		},
		{
			name:           "custom buffer size",
			bufferSize:     50,
			expectedBuffer: 50,
		},
		{
			name:           "large buffer size",
			bufferSize:     1000,
			expectedBuffer: 1000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mb := NewMetadataBus(tt.bufferSize)
			if mb.BufferSize() != tt.expectedBuffer {
				t.Errorf("NewMetadataBus(%d).BufferSize() = %d, want %d",
					tt.bufferSize, mb.BufferSize(), tt.expectedBuffer)
			}
		})
	}
}

func TestMetadataBus_SetGet(t *testing.T) {
	mb := NewMetadataBus(10)

	// Set and get string
	mb.Set("trace_id", "abc-123")
	val, ok := mb.Get("trace_id")
	if !ok {
		t.Error("Get() returned false, want true")
	}
	if val != "abc-123" {
		t.Errorf("Get() = %v, want %v", val, "abc-123")
	}

	// Set and get int
	mb.Set("count", 42)
	val, ok = mb.Get("count")
	if !ok {
		t.Error("Get() returned false, want true")
	}
	if val != 42 {
		t.Errorf("Get() = %v, want %v", val, 42)
	}

	// Get non-existent key
	_, ok = mb.Get("non_existent")
	if ok {
		t.Error("Get() returned true for non-existent key, want false")
	}
}

func TestMetadataBus_GetString(t *testing.T) {
	mb := NewMetadataBus(10)

	// GetString with string value
	mb.Set("name", "test")
	val, ok := mb.GetString("name")
	if !ok {
		t.Error("GetString() returned false, want true")
	}
	if val != "test" {
		t.Errorf("GetString() = %v, want %v", val, "test")
	}

	// GetString with non-string value
	mb.Set("number", 123)
	val, ok = mb.GetString("number")
	if ok {
		t.Error("GetString() returned true for non-string value, want false")
	}
	if val != "" {
		t.Errorf("GetString() = %v, want empty string", val)
	}

	// GetString with non-existent key
	val, ok = mb.GetString("missing")
	if ok {
		t.Error("GetString() returned true for missing key, want false")
	}
	if val != "" {
		t.Errorf("GetString() = %v, want empty string", val)
	}
}

func TestMetadataBus_GetInt(t *testing.T) {
	mb := NewMetadataBus(10)

	// GetInt with int value
	mb.Set("count", 42)
	val, ok := mb.GetInt("count")
	if !ok {
		t.Error("GetInt() returned false, want true")
	}
	if val != 42 {
		t.Errorf("GetInt() = %v, want %v", val, 42)
	}

	// GetInt with non-int value
	mb.Set("name", "test")
	val, ok = mb.GetInt("name")
	if ok {
		t.Error("GetInt() returned true for non-int value, want false")
	}
	if val != 0 {
		t.Errorf("GetInt() = %v, want 0", val)
	}

	// GetInt with non-existent key
	val, ok = mb.GetInt("missing")
	if ok {
		t.Error("GetInt() returned true for missing key, want false")
	}
	if val != 0 {
		t.Errorf("GetInt() = %v, want 0", val)
	}
}

func TestMetadataBus_GetBool(t *testing.T) {
	mb := NewMetadataBus(10)

	// GetBool with bool value
	mb.Set("enabled", true)
	val, ok := mb.GetBool("enabled")
	if !ok {
		t.Error("GetBool() returned false, want true")
	}
	if !val {
		t.Errorf("GetBool() = %v, want %v", val, true)
	}

	// GetBool with false value
	mb.Set("disabled", false)
	val, ok = mb.GetBool("disabled")
	if !ok {
		t.Error("GetBool() returned false for bool value, want true")
	}
	if val {
		t.Errorf("GetBool() = %v, want %v", val, false)
	}

	// GetBool with non-bool value
	mb.Set("name", "test")
	val, ok = mb.GetBool("name")
	if ok {
		t.Error("GetBool() returned true for non-bool value, want false")
	}
	if val {
		t.Errorf("GetBool() = %v, want false", val)
	}

	// GetBool with non-existent key
	val, ok = mb.GetBool("missing")
	if ok {
		t.Error("GetBool() returned true for missing key, want false")
	}
	if val {
		t.Errorf("GetBool() = %v, want false", val)
	}
}

func TestMetadataBus_Delete(t *testing.T) {
	mb := NewMetadataBus(10)

	mb.Set("key", "value")
	_, ok := mb.Get("key")
	if !ok {
		t.Error("Get() returned false after Set(), want true")
	}

	mb.Delete("key")
	_, ok = mb.Get("key")
	if ok {
		t.Error("Get() returned true after Delete(), want false")
	}

	// Delete non-existent key should not panic
	mb.Delete("non_existent")
}

func TestMetadataBus_SetGet_Concurrent(_ *testing.T) {
	mb := NewMetadataBus(10)
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := "key"
			mb.Set(key, i)
			mb.Get(key)
		}(i)
	}

	wg.Wait()
}

func TestMetadataBus_SendReceive(t *testing.T) {
	mb := NewMetadataBus(10)

	// Send metadata
	ok := mb.Send(Metadata{Key: testEventKey, Value: testEventValue})
	if !ok {
		t.Error("Send() = false, want true")
	}

	// Receive metadata
	meta, ok := mb.TryReceive()
	if !ok {
		t.Error("TryReceive() returned false, want true")
	}
	if meta.Key != testEventKey || meta.Value != testEventValue {
		t.Errorf("TryReceive() = {%s, %v}, want {%s, %s}", meta.Key, meta.Value, testEventKey, testEventValue)
	}

	// Receive from empty channel
	_, ok = mb.TryReceive()
	if ok {
		t.Error("TryReceive() = true on empty channel, want false")
	}
}

func TestMetadataBus_SendNonBlocking(t *testing.T) {
	// Success case
	mb := NewMetadataBus(10)
	ok := mb.SendNonBlocking(Metadata{Key: "test", Value: "value"})
	if !ok {
		t.Error("SendNonBlocking() = false, want true")
	}

	// Buffer full case
	mb2 := NewMetadataBus(1)
	mb2.SendNonBlocking(Metadata{Key: "first", Value: "1"})
	ok = mb2.SendNonBlocking(Metadata{Key: "second", Value: "2"})
	if ok {
		t.Error("SendNonBlocking() = true when buffer full, want false")
	}
}

func TestMetadataBus_Receive(t *testing.T) {
	mb := NewMetadataBus(10)

	mb.Send(Metadata{Key: "a", Value: 1})
	mb.Send(Metadata{Key: "b", Value: 2})
	mb.Send(Metadata{Key: "c", Value: 3})

	mb.Close()

	received := make([]Metadata, 0)
	for meta := range mb.Receive() {
		received = append(received, meta)
	}

	if len(received) != 3 {
		t.Errorf("received %d items, want 3", len(received))
	}
}

func TestMetadataBus_Close(t *testing.T) {
	mb := NewMetadataBus(10)

	// Close prevents further sends
	mb.Close()
	ok := mb.Send(Metadata{Key: "test", Value: "value"})
	if ok {
		t.Error("Send() = true after Close(), want false")
	}

	// IsClosed returns correct state
	mb2 := NewMetadataBus(10)
	if mb2.IsClosed() {
		t.Error("IsClosed() = true initially, want false")
	}
	mb2.Close()
	if !mb2.IsClosed() {
		t.Error("IsClosed() = false after Close(), want true")
	}

	// Double close is safe
	mb3 := NewMetadataBus(10)
	mb3.Close()
	mb3.Close() // Should not panic
}

func TestMetadataBus_SendContext(t *testing.T) {
	mb := NewMetadataBus(10)
	ctx := context.Background()

	// Successful send
	err := mb.SendContext(ctx, Metadata{Key: testEventKey, Value: testEventValue})
	if err != nil {
		t.Errorf("SendContext() = %v, want nil", err)
	}

	// Verify data was sent
	meta, ok := mb.TryReceive()
	if !ok {
		t.Error("TryReceive() returned false after SendContext, want true")
	}
	if meta.Key != testEventKey || meta.Value != testEventValue {
		t.Errorf("TryReceive() = {%s, %v}, want {%s, %s}", meta.Key, meta.Value, testEventKey, testEventValue)
	}

	// Context cancellation with full buffer (so it must wait and check context)
	mbFull := NewMetadataBus(1)
	mbFull.Send(Metadata{Key: "fill", Value: "buffer"}) // Fill the buffer
	cancelCtx, cancel := context.WithCancel(ctx)
	cancel()
	err = mbFull.SendContext(cancelCtx, Metadata{Key: "test", Value: "value"})
	if err != context.Canceled {
		t.Errorf("SendContext() with cancelled context = %v, want context.Canceled", err)
	}

	// Closed bus
	mb2 := NewMetadataBus(10)
	mb2.Close()
	err = mb2.SendContext(ctx, Metadata{Key: "test", Value: "value"})
	if err != ErrBusClosed {
		t.Errorf("SendContext() on closed bus = %v, want ErrBusClosed", err)
	}
}

func TestMetadataBus_ReceiveContext(t *testing.T) {
	mb := NewMetadataBus(10)
	ctx := context.Background()

	// Send some data first
	mb.Send(Metadata{Key: testEventKey, Value: testEventValue})

	// Successful receive
	meta, err := mb.ReceiveContext(ctx)
	if err != nil {
		t.Errorf("ReceiveContext() error = %v, want nil", err)
	}
	if meta.Key != testEventKey || meta.Value != testEventValue {
		t.Errorf("ReceiveContext() = {%s, %v}, want {%s, %s}", meta.Key, meta.Value, testEventKey, testEventValue)
	}

	// Context cancellation
	cancelCtx, cancel := context.WithCancel(ctx)
	cancel()
	_, err = mb.ReceiveContext(cancelCtx)
	if err != context.Canceled {
		t.Errorf("ReceiveContext() = %v, want context.Canceled", err)
	}

	// Closed bus with empty channel
	mb2 := NewMetadataBus(10)
	mb2.Close()
	_, err = mb2.ReceiveContext(ctx)
	if err != ErrBusClosed {
		t.Errorf("ReceiveContext() on closed bus = %v, want ErrBusClosed", err)
	}
}

func TestMetadataBus_SendAfterClose_Race(_ *testing.T) {
	// Test that concurrent send/close doesn't panic
	for i := 0; i < 100; i++ {
		mb := NewMetadataBus(10)
		var wg sync.WaitGroup

		wg.Add(2)
		go func() {
			defer wg.Done()
			mb.Send(Metadata{Key: "test", Value: "value"})
		}()
		go func() {
			defer wg.Done()
			mb.Close()
		}()

		wg.Wait()
	}
}

func TestMetadataBus_Concurrent(t *testing.T) {
	mb := NewMetadataBus(100)
	var wg sync.WaitGroup

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				mb.Send(Metadata{Key: "sender", Value: id})
			}
		}(i)
	}

	received := make(chan int, 100)
	go func() {
		count := 0
		timeout := time.After(time.Second)
		for {
			select {
			case <-mb.Receive():
				count++
				if count >= 100 {
					received <- count
					return
				}
			case <-timeout:
				received <- count
				return
			}
		}
	}()

	wg.Wait()

	count := <-received
	if count != 100 {
		t.Errorf("received %d items, want 100", count)
	}
}

func TestWithMetadataBus(t *testing.T) {
	ctx := context.Background()
	mb := NewMetadataBus(10)

	ctx = WithMetadataBus(ctx, mb)

	retrieved := GetMetadataBus(ctx)
	if retrieved != mb {
		t.Error("GetMetadataBus() did not return the same MetadataBus")
	}
}

func TestGetMetadataBus_NotSet(t *testing.T) {
	ctx := context.Background()
	mb := GetMetadataBus(ctx)
	if mb != nil {
		t.Error("GetMetadataBus() = non-nil, want nil when not set")
	}
}

func TestWithLogger(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	ctx = WithLogger(ctx, logger)

	retrieved := Logger(ctx)
	if retrieved != logger {
		t.Error("Logger() did not return the same logger")
	}
}

func TestLogger_Default(t *testing.T) {
	ctx := context.Background()
	logger := Logger(ctx)

	if logger == nil {
		t.Error("Logger() = nil, want non-nil default logger")
	}
	if logger != slog.Default() {
		t.Error("Logger() did not return slog.Default() when no logger set")
	}
}

func TestWithTraceID(t *testing.T) {
	ctx := context.Background()
	ctx = WithTraceID(ctx, "abc-123-def-456")

	retrieved := TraceID(ctx)
	if retrieved != "abc-123-def-456" {
		t.Errorf("TraceID() = %q, want %q", retrieved, "abc-123-def-456")
	}
}

func TestTraceID_NotSet(t *testing.T) {
	ctx := context.Background()
	traceID := TraceID(ctx)

	if traceID != "" {
		t.Errorf("TraceID() = %q, want empty string when not set", traceID)
	}
}

func TestWithRequestID(t *testing.T) {
	ctx := context.Background()
	ctx = WithRequestID(ctx, "req-12345")

	retrieved := RequestID(ctx)
	if retrieved != "req-12345" {
		t.Errorf("RequestID() = %q, want %q", retrieved, "req-12345")
	}
}

func TestRequestID_NotSet(t *testing.T) {
	ctx := context.Background()
	requestID := RequestID(ctx)

	if requestID != "" {
		t.Errorf("RequestID() = %q, want empty string when not set", requestID)
	}
}

func TestContextHelpers_Chaining(t *testing.T) {
	ctx := context.Background()
	mb := NewMetadataBus(10)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	ctx = WithMetadataBus(ctx, mb)
	ctx = WithLogger(ctx, logger)
	ctx = WithTraceID(ctx, "trace-1")
	ctx = WithRequestID(ctx, "req-1")

	if GetMetadataBus(ctx) != mb {
		t.Error("GetMetadataBus() did not return expected MetadataBus after chaining")
	}
	if Logger(ctx) != logger {
		t.Error("Logger() did not return expected logger after chaining")
	}
	if TraceID(ctx) != "trace-1" {
		t.Errorf("TraceID() = %q, want %q", TraceID(ctx), "trace-1")
	}
	if RequestID(ctx) != "req-1" {
		t.Errorf("RequestID() = %q, want %q", RequestID(ctx), "req-1")
	}
}
