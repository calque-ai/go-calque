package mcp

import (
	"encoding/json"
	"time"
)

// getCached attempts to retrieve cached data and unmarshal it.
// Returns true if cache hit and unmarshal succeeded, false otherwise.
func getCached[T any](client *Client, cacheKey string, ttl time.Duration, data *T) bool {
	if client.cache == nil || client.cacheConfig == nil || ttl <= 0 {
		return false
	}

	cached, err := client.cache.Get(cacheKey)
	if err != nil || cached == nil {
		return false
	}

	return json.Unmarshal(cached, data) == nil
}

// setCached stores data in cache if caching is enabled.
func setCached[T any](client *Client, cacheKey string, ttl time.Duration, data T) {
	if client.cache == nil || client.cacheConfig == nil || ttl <= 0 {
		return
	}

	if jsonData, err := json.Marshal(data); err == nil {
		_ = client.cache.Set(cacheKey, jsonData, ttl)
	}
}

// getCachedRegistry retrieves registry data using RegistryTTL.
func getCachedRegistry[T any](client *Client, cacheKey string, data *T) bool {
	if client.cacheConfig == nil {
		return false
	}
	return getCached(client, cacheKey, client.cacheConfig.RegistryTTL, data)
}

// setCachedRegistry stores registry data using RegistryTTL.
func setCachedRegistry[T any](client *Client, cacheKey string, data T) {
	if client.cacheConfig == nil {
		return
	}
	setCached(client, cacheKey, client.cacheConfig.RegistryTTL, data)
}

// getCachedResource retrieves resource data using ResourceTTL.
func getCachedResource[T any](client *Client, cacheKey string, data *T) bool {
	if client.cacheConfig == nil {
		return false
	}
	return getCached(client, cacheKey, client.cacheConfig.ResourceTTL, data)
}

// setCachedResource stores resource data using ResourceTTL.
func setCachedResource[T any](client *Client, cacheKey string, data T) {
	if client.cacheConfig == nil {
		return
	}
	setCached(client, cacheKey, client.cacheConfig.ResourceTTL, data)
}

// getCachedPrompt retrieves prompt data using PromptTTL.
func getCachedPrompt[T any](client *Client, cacheKey string, data *T) bool {
	if client.cacheConfig == nil {
		return false
	}
	return getCached(client, cacheKey, client.cacheConfig.PromptTTL, data)
}

// setCachedPrompt stores prompt data using PromptTTL.
func setCachedPrompt[T any](client *Client, cacheKey string, data T) {
	if client.cacheConfig == nil {
		return
	}
	setCached(client, cacheKey, client.cacheConfig.PromptTTL, data)
}
