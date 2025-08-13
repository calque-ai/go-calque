package cache

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"time"

	"github.com/calque-ai/go-calque/pkg/calque"
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

// CacheMemory provides response caching using a pluggable store
type CacheMemory struct {
	store CacheStore
}

// NewCache creates a cache memory with default in-memory store
func NewCache() *CacheMemory {
	return &CacheMemory{
		store: NewInMemoryCacheStore(),
	}
}

// NewCacheWithStore creates a cache memory with custom store
func NewCacheWithStore(store CacheStore) *CacheMemory {
	return &CacheMemory{
		store: store,
	}
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
func (cm *CacheMemory) Cache(handler calque.Handler, ttl time.Duration) calque.Handler {
	return calque.HandlerFunc(func(r *calque.Request, w *calque.Response) error {
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
			// Log error but don't fail the request
			// Could add optional logger here
		}

		_, err = w.Data.Write(result)
		return err
	})
}

// Clear removes all cached responses
func (cm *CacheMemory) Clear() error {
	return cm.store.Clear()
}

// Delete removes a specific cached response by key
func (cm *CacheMemory) Delete(key string) error {
	return cm.store.Delete(key)
}

// Exists checks if a response is cached for the given key
func (cm *CacheMemory) Exists(key string) bool {
	return cm.store.Exists(key)
}

// ListKeys returns all cached keys
func (cm *CacheMemory) ListKeys() []string {
	return cm.store.List()
}
