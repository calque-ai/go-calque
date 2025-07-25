package cache

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"time"

	"github.com/calque-ai/calque-pipe/core"
)

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
func (cm *CacheMemory) Cache(handler core.Handler, ttl time.Duration) core.Handler {
	return core.HandlerFunc(func(ctx context.Context, r io.Reader, w io.Writer) error {
		input, err := io.ReadAll(r)
		if err != nil {
			return err
		}

		key := fmt.Sprintf("%x", sha256.Sum256(input))

		// Try to get from cache
		if cached, err := cm.store.Get(key); err == nil && cached != nil {
			_, err := w.Write(cached)
			return err
		}

		// Cache miss - execute handler
		var output bytes.Buffer
		if err := handler.ServeFlow(ctx, bytes.NewReader(input), &output); err != nil {
			return err
		}

		result := output.Bytes()

		// Store in cache
		if err := cm.store.Set(key, result, ttl); err != nil {
			// Log error but don't fail the request
			// Could add optional logger here
		}

		_, err = w.Write(result)
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
