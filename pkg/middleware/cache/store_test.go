package cache

import (
	"bytes"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestNewInMemoryStore(t *testing.T) {
	store := NewInMemoryStore()

	if store == nil {
		t.Fatal("NewInMemoryStore() returned nil")
	}

	if store.data == nil {
		t.Error("NewInMemoryStore() data map is nil")
	}

	if len(store.data) != 0 {
		t.Errorf("NewInMemoryStore() data map should be empty, got %d items", len(store.data))
	}
}

func TestInMemoryStoreGet(t *testing.T) {
	store := NewInMemoryStore()

	tests := []struct {
		name        string
		setup       func()
		key         string
		expectedVal []byte
		expectedErr error
	}{
		{
			name:        "get non-existent key",
			key:         "missing",
			expectedVal: nil,
			expectedErr: nil,
		},
		{
			name: "get existing key",
			setup: func() {
				store.Set("existing", []byte("value"), time.Hour)
			},
			key:         "existing",
			expectedVal: []byte("value"),
			expectedErr: nil,
		},
		{
			name: "get empty value",
			setup: func() {
				store.Set("empty", []byte(""), time.Hour)
			},
			key:         "empty",
			expectedVal: []byte(""),
			expectedErr: nil,
		},
		{
			name: "get binary data",
			setup: func() {
				store.Set("binary", []byte{0x00, 0x01, 0x02, 0x03}, time.Hour)
			},
			key:         "binary",
			expectedVal: []byte{0x00, 0x01, 0x02, 0x03},
			expectedErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				tt.setup()
			}

			got, err := store.Get(tt.key)

			if err != tt.expectedErr {
				t.Errorf("Get() error = %v, want %v", err, tt.expectedErr)
				return
			}

			if !bytes.Equal(got, tt.expectedVal) {
				t.Errorf("Get() = %v, want %v", got, tt.expectedVal)
			}
		})
	}
}

func TestInMemoryStoreGetReturnsCopy(t *testing.T) {
	store := NewInMemoryStore()
	original := []byte("original value")

	store.Set("test", original, time.Hour)

	retrieved, _ := store.Get("test")

	// Modify the retrieved data
	retrieved[0] = 'X'

	// Original should be unchanged
	retrievedAgain, _ := store.Get("test")
	if !bytes.Equal(retrievedAgain, original) {
		t.Errorf("Get() should return copy, original data was modified")
	}
}

func TestInMemoryStoreSet(t *testing.T) {
	store := NewInMemoryStore()

	tests := []struct {
		name    string
		key     string
		value   []byte
		ttl     time.Duration
		wantErr bool
	}{
		{
			name:    "set string value",
			key:     "string_key",
			value:   []byte("hello world"),
			ttl:     time.Hour,
			wantErr: false,
		},
		{
			name:    "set empty value",
			key:     "empty_key",
			value:   []byte(""),
			ttl:     time.Hour,
			wantErr: false,
		},
		{
			name:    "set binary value",
			key:     "binary_key",
			value:   []byte{0x00, 0x01, 0x02, 0x03},
			ttl:     time.Hour,
			wantErr: false,
		},
		{
			name:    "set nil value",
			key:     "nil_key",
			value:   nil,
			ttl:     time.Hour,
			wantErr: false,
		},
		{
			name:    "overwrite existing key",
			key:     "overwrite_key",
			value:   []byte("new value"),
			ttl:     time.Hour,
			wantErr: false,
		},
		{
			name:    "set with short TTL",
			key:     "short_ttl_key",
			value:   []byte("value"),
			ttl:     time.Millisecond,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// For overwrite test, set initial value
			if tt.name == "overwrite existing key" {
				store.Set("overwrite_key", []byte("old value"), time.Hour)
			}

			err := store.Set(tt.key, tt.value, tt.ttl)

			if (err != nil) != tt.wantErr {
				t.Errorf("Set() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Verify the value was stored correctly
			got, _ := store.Get(tt.key)
			if !bytes.Equal(got, tt.value) {
				t.Errorf("Set() stored %v, want %v", got, tt.value)
			}
		})
	}
}

func TestInMemoryStoreSetStoresCopy(t *testing.T) {
	store := NewInMemoryStore()
	original := []byte("original value")

	store.Set("test", original, time.Hour)

	// Modify the original data
	original[0] = 'X'

	// Stored data should be unchanged
	retrieved, _ := store.Get("test")
	if bytes.Equal(retrieved, original) {
		t.Errorf("Set() should store copy, stored data was modified")
	}

	if !bytes.Equal(retrieved, []byte("original value")) {
		t.Errorf("Set() stored data should be unchanged, got %v, want %v", retrieved, []byte("original value"))
	}
}

func TestInMemoryStoreDelete(t *testing.T) {
	store := NewInMemoryStore()

	tests := []struct {
		name    string
		setup   func()
		key     string
		wantErr bool
	}{
		{
			name:    "delete non-existent key",
			key:     "missing",
			wantErr: false,
		},
		{
			name: "delete existing key",
			setup: func() {
				store.Set("existing", []byte("value"), time.Hour)
			},
			key:     "existing",
			wantErr: false,
		},
		{
			name: "delete already deleted key",
			setup: func() {
				store.Set("deleted", []byte("value"), time.Hour)
				store.Delete("deleted")
			},
			key:     "deleted",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				tt.setup()
			}

			err := store.Delete(tt.key)

			if (err != nil) != tt.wantErr {
				t.Errorf("Delete() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Verify the key was deleted
			if store.Exists(tt.key) {
				t.Errorf("Delete() key %q still exists", tt.key)
			}

			got, _ := store.Get(tt.key)
			if got != nil {
				t.Errorf("Delete() key %q still has value %v", tt.key, got)
			}
		})
	}
}

func TestInMemoryStoreClear(t *testing.T) {
	store := NewInMemoryStore()

	// Add some data
	store.Set("key1", []byte("value1"), time.Hour)
	store.Set("key2", []byte("value2"), time.Hour)
	store.Set("key3", []byte("value3"), time.Hour)

	err := store.Clear()
	if err != nil {
		t.Errorf("Clear() unexpected error: %v", err)
	}

	// Verify all keys were cleared
	keys := store.List()
	if len(keys) != 0 {
		t.Errorf("Clear() expected 0 keys, got %d", len(keys))
	}

	// Verify keys don't exist
	if store.Exists("key1") || store.Exists("key2") || store.Exists("key3") {
		t.Error("Clear() keys still exist after clearing")
	}
}

func TestInMemoryStoreList(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(*InMemoryStore)
		expected []string
	}{
		{
			name:     "empty store",
			expected: nil,
		},
		{
			name: "single key",
			setup: func(s *InMemoryStore) {
				s.Set("key1", []byte("value1"), time.Hour)
			},
			expected: []string{"key1"},
		},
		{
			name: "multiple keys",
			setup: func(s *InMemoryStore) {
				s.Set("key1", []byte("value1"), time.Hour)
				s.Set("key2", []byte("value2"), time.Hour)
				s.Set("key3", []byte("value3"), time.Hour)
			},
			expected: []string{"key1", "key2", "key3"},
		},
		{
			name: "keys after deletion",
			setup: func(s *InMemoryStore) {
				s.Set("key1", []byte("value1"), time.Hour)
				s.Set("key2", []byte("value2"), time.Hour)
				s.Set("key3", []byte("value3"), time.Hour)
				s.Delete("key2")
			},
			expected: []string{"key1", "key3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := NewInMemoryStore()

			if tt.setup != nil {
				tt.setup(store)
			}

			got := store.List()

			if len(got) != len(tt.expected) {
				t.Errorf("List() returned %d keys, want %d", len(got), len(tt.expected))
				return
			}

			// Convert to map for order-independent comparison
			gotMap := make(map[string]bool)
			for _, key := range got {
				gotMap[key] = true
			}

			for _, expectedKey := range tt.expected {
				if !gotMap[expectedKey] {
					t.Errorf("List() missing expected key %q", expectedKey)
				}
			}

			// Check for unexpected keys
			expectedMap := make(map[string]bool)
			for _, key := range tt.expected {
				expectedMap[key] = true
			}

			for _, gotKey := range got {
				if !expectedMap[gotKey] {
					t.Errorf("List() returned unexpected key %q", gotKey)
				}
			}
		})
	}
}

func TestInMemoryStoreExists(t *testing.T) {
	store := NewInMemoryStore()

	tests := []struct {
		name     string
		setup    func()
		key      string
		expected bool
	}{
		{
			name:     "non-existent key",
			key:      "missing",
			expected: false,
		},
		{
			name: "existing key",
			setup: func() {
				store.Set("existing", []byte("value"), time.Hour)
			},
			key:      "existing",
			expected: true,
		},
		{
			name: "deleted key",
			setup: func() {
				store.Set("deleted", []byte("value"), time.Hour)
				store.Delete("deleted")
			},
			key:      "deleted",
			expected: false,
		},
		{
			name: "key with empty value",
			setup: func() {
				store.Set("empty", []byte(""), time.Hour)
			},
			key:      "empty",
			expected: true,
		},
		{
			name: "key with nil value",
			setup: func() {
				store.Set("nil", nil, time.Hour)
			},
			key:      "nil",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				tt.setup()
			}

			got := store.Exists(tt.key)

			if got != tt.expected {
				t.Errorf("Exists() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestInMemoryStoreTTLExpiration(t *testing.T) {
	store := NewInMemoryStore()

	// Set value with short TTL
	store.Set("short_ttl", []byte("value"), 50*time.Millisecond)

	// Value should exist immediately
	if !store.Exists("short_ttl") {
		t.Error("Key should exist immediately after set")
	}

	value, _ := store.Get("short_ttl")
	if !bytes.Equal(value, []byte("value")) {
		t.Errorf("Expected value 'value', got %v", value)
	}

	// Wait for TTL to expire
	time.Sleep(100 * time.Millisecond)

	// Value should be expired
	if store.Exists("short_ttl") {
		t.Error("Key should not exist after TTL expiration")
	}

	expiredValue, _ := store.Get("short_ttl")
	if expiredValue != nil {
		t.Errorf("Expected nil for expired key, got %v", expiredValue)
	}

	// List should not include expired key
	keys := store.List()
	for _, key := range keys {
		if key == "short_ttl" {
			t.Error("List() should not include expired key")
		}
	}
}

func TestInMemoryStoreConcurrency(t *testing.T) {
	store := NewInMemoryStore()
	numGoroutines := 10
	numOperations := 100

	var wg sync.WaitGroup

	// Test concurrent writes
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			for j := 0; j < numOperations; j++ {
				key := fmt.Sprintf("key-%d-%d", id, j)
				value := fmt.Sprintf("value-%d-%d", id, j)

				err := store.Set(key, []byte(value), time.Hour)
				if err != nil {
					t.Errorf("Concurrent Set() failed: %v", err)
				}
			}
		}(i)
	}

	wg.Wait()

	// Verify all writes succeeded
	expectedCount := numGoroutines * numOperations
	keys := store.List()
	if len(keys) != expectedCount {
		t.Errorf("Expected %d keys after concurrent writes, got %d", expectedCount, len(keys))
	}

	// Test concurrent reads
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			for j := 0; j < numOperations; j++ {
				key := fmt.Sprintf("key-%d-%d", id, j)
				expectedValue := fmt.Sprintf("value-%d-%d", id, j)

				got, err := store.Get(key)
				if err != nil {
					t.Errorf("Concurrent Get() failed: %v", err)
				}

				if !bytes.Equal(got, []byte(expectedValue)) {
					t.Errorf("Concurrent Get() = %v, want %v", got, []byte(expectedValue))
				}
			}
		}(i)
	}

	wg.Wait()

	// Test concurrent deletes
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			for j := 0; j < numOperations; j++ {
				key := fmt.Sprintf("key-%d-%d", id, j)

				err := store.Delete(key)
				if err != nil {
					t.Errorf("Concurrent Delete() failed: %v", err)
				}
			}
		}(i)
	}

	wg.Wait()

	// Verify all deletes succeeded
	keys = store.List()
	if len(keys) != 0 {
		t.Errorf("Expected 0 keys after concurrent deletes, got %d", len(keys))
	}
}

func TestInMemoryStoreBackgroundCleanup(t *testing.T) {
	store := NewInMemoryStore()

	// Add some keys with short TTL
	for i := 0; i < 5; i++ {
		key := fmt.Sprintf("key-%d", i)
		store.Set(key, []byte("value"), 50*time.Millisecond)
	}

	// Add some keys with long TTL
	for i := 5; i < 10; i++ {
		key := fmt.Sprintf("key-%d", i)
		store.Set(key, []byte("value"), time.Hour)
	}

	// Wait for short TTL keys to expire
	time.Sleep(100 * time.Millisecond)

	// Manually trigger cleanup
	store.cleanup()

	// Verify only long TTL keys remain
	keys := store.List()
	if len(keys) != 5 {
		t.Errorf("Expected 5 keys after cleanup, got %d", len(keys))
	}

	// Verify expired keys are gone
	for i := 0; i < 5; i++ {
		key := fmt.Sprintf("key-%d", i)
		if store.Exists(key) {
			t.Errorf("Expired key %s still exists after cleanup", key)
		}
	}

	// Verify non-expired keys still exist
	for i := 5; i < 10; i++ {
		key := fmt.Sprintf("key-%d", i)
		if !store.Exists(key) {
			t.Errorf("Non-expired key %s was incorrectly removed", key)
		}
	}
}
