// Package cache provides response caching middleware for the calque framework.
// It implements pluggable cache backends with TTL support to improve performance
// by storing and retrieving responses based on input content hashes.
package cache

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"time"

	"github.com/calque-ai/go-calque/pkg/calque"
)

// Store interface for cache backends with TTL support
type Store interface {
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

// Memory provides response caching using a pluggable store
type Memory struct {
	store   Store
	onError func(error) // Optional error handler
}

// NewCache creates a cache memory with default in-memory store
func NewCache() *Memory {
	return &Memory{
		store: NewInMemoryStore(),
	}
}

// NewCacheWithStore creates a cache memory with custom store
func NewCacheWithStore(store Store) *Memory {
	return &Memory{
		store: store,
	}
}

// OnError configures a callback for cache errors
func (cm *Memory) OnError(callback func(error)) {
	cm.onError = callback
}

// CacheWithKey creates a caching middleware with a custom key generation function
//
// Input: any data type (only buffered on cache miss)
// Output: same as wrapped handler or cached response
// Behavior: Skips reading input on cache hit; only reads on cache miss
//
// Allows custom cache key generation. The keyFunc receives the request and should
// generate a key based on context values (NOT by reading req.Data). This is useful
// for caching handlers where the cache key should be based on something other than
// the input content (e.g., resource URI from context, client ID, etc.).
//
// Example:
//
//	cacheM := cache.NewCache()
//	handler := cacheM.CacheWithKey(resourceHandler, 10*time.Minute, func(req *calque.Request) string {
//		uri := req.Context.Value("resourceURI").(string)
//		return fmt.Sprintf("resource:%s", uri)
//	})
func (cm *Memory) CacheWithKey(handler calque.Handler, ttl time.Duration, keyFunc func(*calque.Request) string) calque.Handler {
	return calque.HandlerFunc(func(r *calque.Request, w *calque.Response) error {
		// Generate cache key using custom function (should use context, not read data)
		key := keyFunc(r)

		// Try to get from cache
		if cached, err := cm.store.Get(key); err == nil && cached != nil {
			_, err := w.Data.Write(cached)
			return err
		}

		// Cache miss - read input and execute handler
		input, err := io.ReadAll(r.Data)
		if err != nil {
			return err
		}

		var output bytes.Buffer
		handlerReq := calque.NewRequest(r.Context, bytes.NewReader(input))
		handlerRes := calque.NewResponse(&output)
		if err := handler.ServeFlow(handlerReq, handlerRes); err != nil {
			return err
		}

		result := output.Bytes()

		// Store in cache
		if err := cm.store.Set(key, result, ttl); err != nil {
			if cm.onError != nil {
				cm.onError(fmt.Errorf("cache write failed: %w", err))
			}
		}

		_, err = w.Data.Write(result)
		return err
	})
}

// Cache creates a caching middleware that stores responses based on input hash
//
// Input: any data type (buffered - needs to hash input for cache key)
// Output: same as wrapped handler or cached response
// Behavior: BUFFERED - must read input to generate cache key
//
// Caches responses based on SHA256 hash of input content. On cache hit,
// returns cached response immediately. On cache miss, executes handler
// and caches the result for future requests.
//
// Example:
//
//	cacheM := cache.NewCache()
//	flow.Use(cacheM.Cache(llmHandler, 1*time.Hour)) // Cache for 1 hour
func (cm *Memory) Cache(handler calque.Handler, ttl time.Duration) calque.Handler {
	return calque.HandlerFunc(func(r *calque.Request, w *calque.Response) error {
		// Must read input to generate hash-based key
		input, err := io.ReadAll(r.Data)
		if err != nil {
			return err
		}

		key := fmt.Sprintf("%x", sha256.Sum256(input))

		// Try to get from cache
		if cached, err := cm.store.Get(key); err == nil && cached != nil {
			_, err := w.Data.Write(cached)
			return err
		}

		// Cache miss - execute handler
		var output bytes.Buffer
		handlerReq := calque.NewRequest(r.Context, bytes.NewReader(input))
		handlerRes := calque.NewResponse(&output)
		if err := handler.ServeFlow(handlerReq, handlerRes); err != nil {
			return err
		}

		result := output.Bytes()

		// Store in cache
		if err := cm.store.Set(key, result, ttl); err != nil {
			if cm.onError != nil {
				cm.onError(fmt.Errorf("cache write failed: %w", err))
			}
		}

		_, err = w.Data.Write(result)
		return err
	})
}

// Get retrieves cached data for a key
func (cm *Memory) Get(key string) ([]byte, error) {
	return cm.store.Get(key)
}

// Set stores data for a key with TTL
func (cm *Memory) Set(key string, value []byte, ttl time.Duration) error {
	return cm.store.Set(key, value, ttl)
}

// Clear removes all cached responses
func (cm *Memory) Clear() error {
	return cm.store.Clear()
}

// Delete removes a specific cached response by key
func (cm *Memory) Delete(key string) error {
	return cm.store.Delete(key)
}

// Exists checks if a response is cached for the given key
func (cm *Memory) Exists(key string) bool {
	return cm.store.Exists(key)
}

// ListKeys returns all cached keys
func (cm *Memory) ListKeys() []string {
	return cm.store.List()
}
