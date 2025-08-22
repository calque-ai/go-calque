package memory

import (
	"bytes"
	"fmt"
	"sync"
	"testing"
)

func TestNewInMemoryStore(t *testing.T) {
	store := NewInMemoryStore()

	if store == nil { //nolint:staticcheck
		t.Error("NewInMemoryStore() returned nil")
	}

	if store.data == nil { //nolint:staticcheck
		t.Error("NewInMemoryStore() data map is nil")
	}

	if len(store.data) != 0 { //nolint:staticcheck
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
				store.Set("existing", []byte("value"))
			},
			key:         "existing",
			expectedVal: []byte("value"),
			expectedErr: nil,
		},
		{
			name: "get empty value",
			setup: func() {
				store.Set("empty", []byte(""))
			},
			key:         "empty",
			expectedVal: []byte(""),
			expectedErr: nil,
		},
		{
			name: "get binary data",
			setup: func() {
				store.Set("binary", []byte{0x00, 0x01, 0x02, 0x03})
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

	store.Set("test", original)

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
		wantErr bool
	}{
		{
			name:    "set string value",
			key:     "string_key",
			value:   []byte("hello world"),
			wantErr: false,
		},
		{
			name:    "set empty value",
			key:     "empty_key",
			value:   []byte(""),
			wantErr: false,
		},
		{
			name:    "set binary value",
			key:     "binary_key",
			value:   []byte{0x00, 0x01, 0x02, 0x03},
			wantErr: false,
		},
		{
			name:    "set nil value",
			key:     "nil_key",
			value:   nil,
			wantErr: false,
		},
		{
			name:    "overwrite existing key",
			key:     "overwrite_key",
			value:   []byte("new value"),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// For overwrite test, set initial value
			if tt.name == "overwrite existing key" {
				store.Set("overwrite_key", []byte("old value"))
			}

			err := store.Set(tt.key, tt.value)

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

	store.Set("test", original)

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
				store.Set("existing", []byte("value"))
			},
			key:     "existing",
			wantErr: false,
		},
		{
			name: "delete already deleted key",
			setup: func() {
				store.Set("deleted", []byte("value"))
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

func TestInMemoryStoreList(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(*InMemoryStore)
		expected []string
	}{
		{
			name:     "empty store",
			expected: []string{},
		},
		{
			name: "single key",
			setup: func(s *InMemoryStore) {
				s.Set("key1", []byte("value1"))
			},
			expected: []string{"key1"},
		},
		{
			name: "multiple keys",
			setup: func(s *InMemoryStore) {
				s.Set("key1", []byte("value1"))
				s.Set("key2", []byte("value2"))
				s.Set("key3", []byte("value3"))
			},
			expected: []string{"key1", "key2", "key3"},
		},
		{
			name: "keys after deletion",
			setup: func(s *InMemoryStore) {
				s.Set("key1", []byte("value1"))
				s.Set("key2", []byte("value2"))
				s.Set("key3", []byte("value3"))
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
				store.Set("existing", []byte("value"))
			},
			key:      "existing",
			expected: true,
		},
		{
			name: "deleted key",
			setup: func() {
				store.Set("deleted", []byte("value"))
				store.Delete("deleted")
			},
			key:      "deleted",
			expected: false,
		},
		{
			name: "key with empty value",
			setup: func() {
				store.Set("empty", []byte(""))
			},
			key:      "empty",
			expected: true,
		},
		{
			name: "key with nil value",
			setup: func() {
				store.Set("nil", nil)
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

				err := store.Set(key, []byte(value))
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

func TestInMemoryStoreStoreInterface(t *testing.T) {
	// Test that InMemoryStore implements the Store interface
	var _ Store = (*InMemoryStore)(nil)

	// Test interface methods work correctly
	var store Store = NewInMemoryStore()

	// Test Set
	err := store.Set("interface_test", []byte("value"))
	if err != nil {
		t.Errorf("Store interface Set() failed: %v", err)
	}

	// Test Get
	got, err := store.Get("interface_test")
	if err != nil {
		t.Errorf("Store interface Get() failed: %v", err)
	}
	if !bytes.Equal(got, []byte("value")) {
		t.Errorf("Store interface Get() = %v, want %v", got, []byte("value"))
	}

	// Test Exists
	if !store.Exists("interface_test") {
		t.Error("Store interface Exists() returned false for existing key")
	}

	// Test List
	keys := store.List()
	if len(keys) != 1 || keys[0] != "interface_test" {
		t.Errorf("Store interface List() = %v, want [interface_test]", keys)
	}

	// Test Delete
	err = store.Delete("interface_test")
	if err != nil {
		t.Errorf("Store interface Delete() failed: %v", err)
	}

	if store.Exists("interface_test") {
		t.Error("Store interface key still exists after delete")
	}
}
