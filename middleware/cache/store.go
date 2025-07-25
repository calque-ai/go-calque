package cache

import (
	"sync"
	"time"
)

// CacheStore interface for cache backends with TTL support
type CacheStore interface {
	// Get retrieves data for a key, returns nil if not found or expired
	Get(key string) ([]byte, error)

	// Set stores data for a key with TTL
	Set(key string, value []byte, ttl time.Duration) error

	// Delete removes data for a key
	Delete(key string) error

	// Clear removes all cached data
	Clear() error

	// Exists checks if a key exists and hasn't expired
	Exists(key string) bool

	// List returns all non-expired keys
	List() []string
}

// InMemoryCacheStore provides a simple in-memory cache implementation with TTL
type InMemoryCacheStore struct {
	data map[string]*cacheEntry
	mu   sync.RWMutex
}

type cacheEntry struct {
	data      []byte
	timestamp time.Time
	ttl       time.Duration
}

// NewInMemoryCacheStore creates a new in-memory cache store
func NewInMemoryCacheStore() *InMemoryCacheStore {
	store := &InMemoryCacheStore{
		data: make(map[string]*cacheEntry),
	}

	// Start background cleanup goroutine
	go store.backgroundCleanup()

	return store
}

// Get retrieves data for a key
func (s *InMemoryCacheStore) Get(key string) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entry, exists := s.data[key]
	if !exists {
		return nil, nil // Not found
	}

	// Check if entry has expired
	if time.Since(entry.timestamp) > entry.ttl {
		return nil, nil // Expired
	}

	// Return copy to prevent external modification
	result := make([]byte, len(entry.data))
	copy(result, entry.data)
	return result, nil
}

// Set stores data for a key with TTL
func (s *InMemoryCacheStore) Set(key string, value []byte, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Store copy to prevent external modification
	dataCopy := make([]byte, len(value))
	copy(dataCopy, value)

	s.data[key] = &cacheEntry{
		data:      dataCopy,
		timestamp: time.Now(),
		ttl:       ttl,
	}

	return nil
}

// Delete removes data for a key
func (s *InMemoryCacheStore) Delete(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.data, key)
	return nil
}

// Clear removes all cached data
func (s *InMemoryCacheStore) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.data = make(map[string]*cacheEntry)
	return nil
}

// Exists checks if a key exists and hasn't expired
func (s *InMemoryCacheStore) Exists(key string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entry, exists := s.data[key]
	if !exists {
		return false
	}

	// Check if entry has expired
	return time.Since(entry.timestamp) <= entry.ttl
}

// List returns all non-expired keys
func (s *InMemoryCacheStore) List() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var keys []string
	now := time.Now()

	for key, entry := range s.data {
		if now.Sub(entry.timestamp) <= entry.ttl {
			keys = append(keys, key)
		}
	}

	return keys
}

// backgroundCleanup runs periodically to remove expired entries
func (s *InMemoryCacheStore) backgroundCleanup() {
	ticker := time.NewTicker(5 * time.Minute) // Cleanup every 5 minutes
	defer ticker.Stop()

	for range ticker.C {
		s.cleanup()
	}
}

// cleanup removes expired entries (internal method)
func (s *InMemoryCacheStore) cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for key, entry := range s.data {
		if now.Sub(entry.timestamp) > entry.ttl {
			delete(s.data, key)
		}
	}
}
