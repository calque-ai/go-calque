package memory

import "sync"

// InMemoryStore provides a simple in-memory implementation mostly for examples or testing
type InMemoryStore struct {
	data map[string][]byte
	mu   sync.RWMutex
}

// NewInMemoryStore creates a new in-memory store
func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		data: make(map[string][]byte),
	}
}

// Get retrieves data for a key
func (s *InMemoryStore) Get(key string) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if data, exists := s.data[key]; exists {
		// Return copy to prevent external modification
		result := make([]byte, len(data))
		copy(result, data)
		return result, nil
	}

	return nil, nil // Not found, but not an error
}

// Set stores data for a key
func (s *InMemoryStore) Set(key string, value []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Store copy to prevent external modification
	s.data[key] = make([]byte, len(value))
	copy(s.data[key], value)
	return nil
}

// Delete removes data for a key
func (s *InMemoryStore) Delete(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.data, key)
	return nil
}

// List returns all keys
func (s *InMemoryStore) List() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	keys := make([]string, 0, len(s.data))
	for key := range s.data {
		keys = append(keys, key)
	}
	return keys
}

// Exists checks if a key exists
func (s *InMemoryStore) Exists(key string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	_, exists := s.data[key]
	return exists
}
