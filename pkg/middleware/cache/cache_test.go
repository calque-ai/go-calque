package cache

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/calque-ai/go-calque/pkg/calque"
)

const (
	opClear  = "clear"
	opDelete = "delete"
	opExists = "exists"
)

// mockStore implements Store interface for testing error scenarios
type mockStore struct {
	data       map[string][]byte
	ttls       map[string]time.Duration
	mu         sync.RWMutex
	getError   error
	setError   error
	delError   error
	clearErr   error
	getCalls   int
	setCalls   int
	delCalls   int
	clearCalls int
}

func newMockStore() *mockStore {
	return &mockStore{
		data: make(map[string][]byte),
		ttls: make(map[string]time.Duration),
	}
}

func (m *mockStore) Get(key string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.getCalls++
	if m.getError != nil {
		return nil, m.getError
	}
	data, exists := m.data[key]
	if !exists {
		return nil, nil
	}
	result := make([]byte, len(data))
	copy(result, data)
	return result, nil
}

func (m *mockStore) Set(key string, value []byte, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.setCalls++
	if m.setError != nil {
		return m.setError
	}
	m.data[key] = make([]byte, len(value))
	copy(m.data[key], value)
	m.ttls[key] = ttl
	return nil
}

func (m *mockStore) Delete(key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.delCalls++
	if m.delError != nil {
		return m.delError
	}
	delete(m.data, key)
	delete(m.ttls, key)
	return nil
}

func (m *mockStore) Clear() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.clearCalls++
	if m.clearErr != nil {
		return m.clearErr
	}
	m.data = make(map[string][]byte)
	m.ttls = make(map[string]time.Duration)
	return nil
}

func (m *mockStore) Exists(key string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, exists := m.data[key]
	return exists
}

func (m *mockStore) List() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	keys := make([]string, 0, len(m.data))
	for key := range m.data {
		keys = append(keys, key)
	}
	return keys
}

// errorReader simulates read errors
type errorReader struct {
	err error
}

func (e *errorReader) Read(_ []byte) (n int, err error) {
	return 0, e.err
}

// mockHandler implements calque.Handler for testing
type mockHandler struct {
	response  string
	err       error
	callCount int
	lastInput string
}

func (m *mockHandler) ServeFlow(r *calque.Request, w *calque.Response) error {
	m.callCount++
	if r.Data != nil {
		input, _ := io.ReadAll(r.Data)
		m.lastInput = string(input)
	}
	if m.err != nil {
		return m.err
	}
	_, err := w.Data.Write([]byte(m.response))
	return err
}

func TestNewCache(t *testing.T) {
	cache := NewCache()
	if cache == nil {
		t.Fatal("NewCache() returned nil")
	}
	if cache.store == nil {
		t.Error("Expected store to be initialized")
	}
	if cache.onError != nil {
		t.Error("Expected onError to be nil initially")
	}
}

func TestNewCacheWithStore(t *testing.T) {
	store := NewInMemoryStore()
	cache := NewCacheWithStore(store)

	if cache == nil {
		t.Fatal("NewCacheWithStore() returned nil")
	}
	if cache.store != store {
		t.Error("Expected store to be the provided store")
	}
}

func TestMemory_OnError(t *testing.T) {
	cache := NewCache()
	var capturedError error

	cache.OnError(func(err error) {
		capturedError = err
	})

	if cache.onError == nil {
		t.Error("Expected onError callback to be set")
	}

	// Test the callback works
	testErr := errors.New("test error")
	cache.onError(testErr)
	if capturedError != testErr {
		t.Errorf("Expected captured error to be %v, got %v", testErr, capturedError)
	}
}

func TestMemory_Cache_WithRealStore(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		handlerResp    string
		handlerErr     error
		ttl            time.Duration
		expectError    bool
		expectResponse string
		description    string
	}{
		{
			name:           "cache_miss_success",
			input:          "test input",
			handlerResp:    "test response",
			handlerErr:     nil,
			ttl:            time.Hour,
			expectError:    false,
			expectResponse: "test response",
			description:    "Cache miss should execute handler and cache result",
		},
		{
			name:           "cache_hit_success",
			input:          "same input",
			handlerResp:    "handler response",
			handlerErr:     nil,
			ttl:            time.Hour,
			expectError:    false,
			expectResponse: "handler response", // Same response since it's cached
			description:    "Cache hit should return cached data",
		},
		{
			name:           "handler_error",
			input:          "error input",
			handlerResp:    "",
			handlerErr:     errors.New("handler failed"),
			ttl:            time.Hour,
			expectError:    true,
			expectResponse: "",
			description:    "Handler error should be propagated",
		},
		{
			name:           "empty_input",
			input:          "",
			handlerResp:    "empty input response",
			handlerErr:     nil,
			ttl:            time.Hour,
			expectError:    false,
			expectResponse: "empty input response",
			description:    "Empty input should work",
		},
		{
			name:           "zero_ttl",
			input:          "zero ttl input",
			handlerResp:    "zero ttl response",
			handlerErr:     nil,
			ttl:            0,
			expectError:    false,
			expectResponse: "zero ttl response",
			description:    "Zero TTL should still work",
		},
		{
			name:           "large_input",
			input:          strings.Repeat("x", 10000),
			handlerResp:    "large input response",
			handlerErr:     nil,
			ttl:            time.Hour,
			expectError:    false,
			expectResponse: "large input response",
			description:    "Large input should work correctly",
		},
	}

	cache := NewCache()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockHandler := &mockHandler{
				response: tt.handlerResp,
				err:      tt.handlerErr,
			}

			cacheHandler := cache.Cache(mockHandler, tt.ttl)

			// Create request and response
			req := calque.NewRequest(context.Background(), strings.NewReader(tt.input))
			var buf bytes.Buffer
			resp := calque.NewResponse(&buf)

			// For cache hit test, run twice with same input
			if tt.name == "cache_hit_success" {
				// First call to populate cache
				req1 := calque.NewRequest(context.Background(), strings.NewReader(tt.input))
				var buf1 bytes.Buffer
				resp1 := calque.NewResponse(&buf1)
				cacheHandler.ServeFlow(req1, resp1)

				// Second call should hit cache
				req = calque.NewRequest(context.Background(), strings.NewReader(tt.input))
				buf = bytes.Buffer{}
				resp = calque.NewResponse(&buf)
			}

			// Execute
			err := cacheHandler.ServeFlow(req, resp)

			// Verify error expectation
			if tt.expectError && err == nil {
				t.Errorf("%s: expected error but got none", tt.description)
			}
			if !tt.expectError && err != nil {
				t.Errorf("%s: unexpected error: %v", tt.description, err)
			}

			// Verify response
			if !tt.expectError {
				response := buf.String()
				if response != tt.expectResponse {
					t.Errorf("%s: expected response %q, got %q", tt.description, tt.expectResponse, response)
				}
			}

			// For cache hit test, verify handler was called only once
			if tt.name == "cache_hit_success" {
				if mockHandler.callCount != 1 {
					t.Errorf("%s: expected handler to be called once, got %d calls", tt.description, mockHandler.callCount)
				}
			}

		})
	}
}

func TestMemory_Cache_ErrorScenarios(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		handlerResp    string
		handlerErr     error
		ttl            time.Duration
		storeGetErr    error
		storeSetErr    error
		expectError    bool
		expectResponse string
		expectGetCalls int
		expectSetCalls int
		description    string
	}{
		{
			name:           "store_get_error_fallback",
			input:          "test input",
			handlerResp:    "test response",
			handlerErr:     nil,
			ttl:            time.Hour,
			storeGetErr:    errors.New("get failed"),
			expectError:    false,
			expectResponse: "test response",
			expectGetCalls: 1,
			expectSetCalls: 1,
			description:    "Store get error should fallback to handler",
		},
		{
			name:           "store_set_error_with_callback",
			input:          "test input",
			handlerResp:    "test response",
			handlerErr:     nil,
			ttl:            time.Hour,
			storeSetErr:    errors.New("set failed"),
			expectError:    false,
			expectResponse: "test response",
			expectGetCalls: 1,
			expectSetCalls: 1,
			description:    "Store set error should still return response",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStore := newMockStore()
			cache := NewCacheWithStore(mockStore)

			// Set up store errors
			mockStore.getError = tt.storeGetErr
			mockStore.setError = tt.storeSetErr

			// Set up error callback
			var errorCallbackTriggered bool
			var capturedError error
			cache.OnError(func(err error) {
				errorCallbackTriggered = true
				capturedError = err
			})

			mockHandler := &mockHandler{
				response: tt.handlerResp,
				err:      tt.handlerErr,
			}

			cacheHandler := cache.Cache(mockHandler, tt.ttl)

			// Create request and response
			req := calque.NewRequest(context.Background(), strings.NewReader(tt.input))
			var buf bytes.Buffer
			resp := calque.NewResponse(&buf)

			// Execute
			err := cacheHandler.ServeFlow(req, resp)

			// Verify error expectation
			if tt.expectError && err == nil {
				t.Errorf("%s: expected error but got none", tt.description)
			}
			if !tt.expectError && err != nil {
				t.Errorf("%s: unexpected error: %v", tt.description, err)
			}

			// Verify response
			if !tt.expectError {
				response := buf.String()
				if response != tt.expectResponse {
					t.Errorf("%s: expected response %q, got %q", tt.description, tt.expectResponse, response)
				}
			}

			// Verify store interactions
			if mockStore.getCalls != tt.expectGetCalls {
				t.Errorf("%s: expected %d get calls, got %d", tt.description, tt.expectGetCalls, mockStore.getCalls)
			}
			if mockStore.setCalls != tt.expectSetCalls {
				t.Errorf("%s: expected %d set calls, got %d", tt.description, tt.expectSetCalls, mockStore.setCalls)
			}

			// Verify error callback for store set errors
			if tt.storeSetErr != nil {
				if !errorCallbackTriggered {
					t.Errorf("%s: expected error callback to be triggered", tt.description)
				}
				if !strings.Contains(capturedError.Error(), "cache write failed") {
					t.Errorf("%s: expected cache write error, got: %v", tt.description, capturedError)
				}
			}
		})
	}
}

func TestMemory_Cache_InputReadError(t *testing.T) {
	cache := NewCache()
	handler := &mockHandler{response: "test"}

	cacheHandler := cache.Cache(handler, time.Hour)

	// Create request with error reader
	readErr := errors.New("read failed")
	req := calque.NewRequest(context.Background(), &errorReader{err: readErr})
	var buf bytes.Buffer
	resp := calque.NewResponse(&buf)

	err := cacheHandler.ServeFlow(req, resp)
	if err != readErr {
		t.Errorf("Expected read error to be propagated, got: %v", err)
	}
}

func TestMemory_StoreOperations(t *testing.T) {
	tests := []struct {
		name        string
		operation   string
		key         string
		setupData   map[string][]byte
		expectError bool
		description string
	}{
		{
			name:        "clear_success",
			operation:   opClear,
			setupData:   map[string][]byte{"key1": []byte("data1"), "key2": []byte("data2")},
			expectError: false,
			description: "Clear should remove all cached data",
		},
		{
			name:        "delete_success",
			operation:   opDelete,
			key:         "testkey",
			setupData:   map[string][]byte{"testkey": []byte("data")},
			expectError: false,
			description: "Delete should remove specified key",
		},
		{
			name:        "delete_nonexistent",
			operation:   opDelete,
			key:         "nonexistent",
			expectError: false,
			description: "Delete of nonexistent key should not error",
		},
		{
			name:        "exists_true",
			operation:   opExists,
			key:         "testkey",
			setupData:   map[string][]byte{"testkey": []byte("data")},
			expectError: false,
			description: "Exists should return true for existing key",
		},
		{
			name:        "exists_false",
			operation:   opExists,
			key:         "nonexistent",
			expectError: false,
			description: "Exists should return false for nonexistent key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache := NewCache()

			// Set up test data using cache operations
			for key, data := range tt.setupData {
				// Use the cache to store data, which will use the underlying store
				handler := &mockHandler{response: string(data)}
				cacheHandler := cache.Cache(handler, time.Hour)
				req := calque.NewRequest(context.Background(), strings.NewReader(key))
				var buf bytes.Buffer
				resp := calque.NewResponse(&buf)
				cacheHandler.ServeFlow(req, resp)
			}

			var err error
			var result bool

			// Execute operation
			switch tt.operation {
			case opClear:
				err = cache.Clear()
			case opDelete:
				keyHash := fmt.Sprintf("%x", sha256.Sum256([]byte(tt.key)))
				err = cache.Delete(keyHash)
			case opExists:
				keyHash := fmt.Sprintf("%x", sha256.Sum256([]byte(tt.key)))
				result = cache.Exists(keyHash)
			}

			// Verify error expectation
			if tt.expectError && err == nil {
				t.Errorf("%s: expected error but got none", tt.description)
			}
			if !tt.expectError && err != nil {
				t.Errorf("%s: unexpected error: %v", tt.description, err)
			}

			// Verify operation-specific results
			switch tt.operation {
			case opClear:
				if !tt.expectError {
					keys := cache.ListKeys()
					if len(keys) != 0 {
						t.Errorf("%s: expected no keys after clear, got %d", tt.description, len(keys))
					}
				}
			case opDelete:
				if !tt.expectError {
					keyHash := fmt.Sprintf("%x", sha256.Sum256([]byte(tt.key)))
					if cache.Exists(keyHash) {
						t.Errorf("%s: expected key to be deleted", tt.description)
					}
				}
			case opExists:
				expectedResult := tt.setupData != nil && tt.setupData[tt.key] != nil
				if result != expectedResult {
					t.Errorf("%s: expected exists result %v, got %v", tt.description, expectedResult, result)
				}
			}
		})
	}
}

func TestMemory_ListKeys(t *testing.T) {
	cache := NewCache()

	// Setup test data by making cache requests
	testInputs := []string{"input1", "input2", "input3"}

	for _, input := range testInputs {
		handler := &mockHandler{response: "response for " + input}
		cacheHandler := cache.Cache(handler, time.Hour)
		req := calque.NewRequest(context.Background(), strings.NewReader(input))
		var buf bytes.Buffer
		resp := calque.NewResponse(&buf)
		cacheHandler.ServeFlow(req, resp)
	}

	keys := cache.ListKeys()

	if len(keys) != len(testInputs) {
		t.Errorf("Expected %d keys, got %d", len(testInputs), len(keys))
	}

	// Verify each input hash is in the keys
	for _, input := range testInputs {
		expectedKey := fmt.Sprintf("%x", sha256.Sum256([]byte(input)))
		found := slices.Contains(keys, expectedKey)
		if !found {
			t.Errorf("Expected key %s not found in results", expectedKey)
		}
	}
}

func TestMemory_Cache_TTLExpiration(t *testing.T) {
	cache := NewCache()
	handler := &mockHandler{response: "test response"}

	// Use very short TTL
	cacheHandler := cache.Cache(handler, 50*time.Millisecond)

	input := "test input"

	// First request
	req1 := calque.NewRequest(context.Background(), strings.NewReader(input))
	var buf1 bytes.Buffer
	resp1 := calque.NewResponse(&buf1)
	err1 := cacheHandler.ServeFlow(req1, resp1)

	if err1 != nil {
		t.Fatalf("First request failed: %v", err1)
	}
	if handler.callCount != 1 {
		t.Errorf("Expected 1 handler call, got %d", handler.callCount)
	}

	// Wait for TTL to expire
	time.Sleep(100 * time.Millisecond)

	// Second request after expiration
	req2 := calque.NewRequest(context.Background(), strings.NewReader(input))
	var buf2 bytes.Buffer
	resp2 := calque.NewResponse(&buf2)
	err2 := cacheHandler.ServeFlow(req2, resp2)

	if err2 != nil {
		t.Fatalf("Second request failed: %v", err2)
	}
	if handler.callCount != 2 {
		t.Errorf("Expected 2 handler calls after expiration, got %d", handler.callCount)
	}
}

func TestMemory_Cache_ConcurrentAccess(t *testing.T) {
	cache := NewCache()
	handler := &mockHandler{response: "concurrent response"}
	cacheHandler := cache.Cache(handler, time.Hour)

	const numGoroutines = 10
	const numRequests = 5

	results := make(chan string, numGoroutines*numRequests)
	errors := make(chan error, numGoroutines*numRequests)

	var wg sync.WaitGroup

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < numRequests; j++ {
				input := fmt.Sprintf("input-%d-%d", goroutineID, j)
				req := calque.NewRequest(context.Background(), strings.NewReader(input))
				var buf bytes.Buffer
				resp := calque.NewResponse(&buf)

				err := cacheHandler.ServeFlow(req, resp)
				if err != nil {
					errors <- err
				} else {
					results <- buf.String()
				}
			}
		}(i)
	}

	wg.Wait()
	close(results)
	close(errors)

	// Check for errors
	for err := range errors {
		t.Errorf("Unexpected error in concurrent test: %v", err)
	}

	// Verify all results
	resultCount := 0
	for result := range results {
		if result != "concurrent response" {
			t.Errorf("Expected 'concurrent response', got %q", result)
		}
		resultCount++
	}

	expectedResults := numGoroutines * numRequests
	if resultCount != expectedResults {
		t.Errorf("Expected %d results, got %d", expectedResults, resultCount)
	}
}

func TestMemory_Cache_KeyGeneration(t *testing.T) {
	tests := []struct {
		name        string
		input1      string
		input2      string
		sameKey     bool
		description string
	}{
		{
			name:        "identical_inputs",
			input1:      "test input",
			input2:      "test input",
			sameKey:     true,
			description: "Identical inputs should generate same cache key",
		},
		{
			name:        "different_inputs",
			input1:      "test input 1",
			input2:      "test input 2",
			sameKey:     false,
			description: "Different inputs should generate different cache keys",
		},
		{
			name:        "case_sensitive",
			input1:      "Test Input",
			input2:      "test input",
			sameKey:     false,
			description: "Cache keys should be case sensitive",
		},
		{
			name:        "whitespace_sensitive",
			input1:      "test input",
			input2:      "test  input",
			sameKey:     false,
			description: "Cache keys should be whitespace sensitive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key1 := fmt.Sprintf("%x", sha256.Sum256([]byte(tt.input1)))
			key2 := fmt.Sprintf("%x", sha256.Sum256([]byte(tt.input2)))

			if tt.sameKey {
				if key1 != key2 {
					t.Errorf("%s: expected same keys, got %q and %q", tt.description, key1, key2)
				}
			} else {
				if key1 == key2 {
					t.Errorf("%s: expected different keys, both got %q", tt.description, key1)
				}
			}
		})
	}
}

// Error scenarios with mock store for testing store operation failures
func TestMemory_StoreOperationErrors(t *testing.T) {
	tests := []struct {
		name        string
		operation   string
		key         string
		storeError  error
		expectError bool
		description string
	}{
		{
			name:        "clear_with_error",
			operation:   opClear,
			storeError:  errors.New("clear failed"),
			expectError: true,
			description: "Clear error should be propagated",
		},
		{
			name:        "delete_with_error",
			operation:   opDelete,
			key:         "testkey",
			storeError:  errors.New("delete failed"),
			expectError: true,
			description: "Delete error should be propagated",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStore := newMockStore()
			cache := NewCacheWithStore(mockStore)

			// Set up store error
			switch tt.operation {
			case opClear:
				mockStore.clearErr = tt.storeError
			case opDelete:
				mockStore.delError = tt.storeError
			}

			var err error

			// Execute operation
			switch tt.operation {
			case opClear:
				err = cache.Clear()
			case opDelete:
				err = cache.Delete(tt.key)
			}

			// Verify error expectation
			if tt.expectError && err == nil {
				t.Errorf("%s: expected error but got none", tt.description)
			}
			if !tt.expectError && err != nil {
				t.Errorf("%s: unexpected error: %v", tt.description, err)
			}
		})
	}
}

// Benchmark tests
func BenchmarkMemory_Cache_Hit(b *testing.B) {
	cache := NewCache()
	handler := &mockHandler{response: "benchmark response"}
	cacheHandler := cache.Cache(handler, time.Hour)

	// Warm up cache
	input := "benchmark input"
	req := calque.NewRequest(context.Background(), strings.NewReader(input))
	var buf bytes.Buffer
	resp := calque.NewResponse(&buf)
	cacheHandler.ServeFlow(req, resp)

	for b.Loop() {
		req := calque.NewRequest(context.Background(), strings.NewReader(input))
		var buf bytes.Buffer
		resp := calque.NewResponse(&buf)
		cacheHandler.ServeFlow(req, resp)
	}
}

func BenchmarkMemory_Cache_Miss(b *testing.B) {
	cache := NewCache()
	handler := &mockHandler{response: "benchmark response"}
	cacheHandler := cache.Cache(handler, time.Hour)

	for i := 0; b.Loop(); i++ {
		input := fmt.Sprintf("benchmark input %d", i)
		req := calque.NewRequest(context.Background(), strings.NewReader(input))
		var buf bytes.Buffer
		resp := calque.NewResponse(&buf)
		cacheHandler.ServeFlow(req, resp)
	}
}
