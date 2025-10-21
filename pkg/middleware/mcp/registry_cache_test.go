package mcp

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/calque-ai/go-calque/pkg/calque"
	"github.com/calque-ai/go-calque/pkg/middleware/cache"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestRegistryCacheHelpers(t *testing.T) {
	t.Parallel()

	t.Run("getCachedRegistry and setCachedRegistry", func(t *testing.T) {
		t.Parallel()

		// Create client with caching using WithCache option (matches production usage)
		store := cache.NewInMemoryStore()
		client := &Client{}
		WithCache(store, &CacheConfig{
			RegistryTTL: 5 * time.Minute,
		})(client)

		cacheKey := "test-registry"
		testData := []*mcp.Resource{
			{URI: "file:///test1", Name: "Test 1"},
			{URI: "file:///test2", Name: "Test 2"},
		}

		// Test cache miss
		var retrieved []*mcp.Resource
		if getCachedRegistry(client, cacheKey, &retrieved) {
			t.Error("Expected cache miss, got cache hit")
		}

		// Store in cache
		setCachedRegistry(client, cacheKey, testData)

		// Test cache hit
		if !getCachedRegistry(client, cacheKey, &retrieved) {
			t.Error("Expected cache hit, got cache miss")
		}

		// Verify data
		if len(retrieved) != len(testData) {
			t.Errorf("Expected %d items, got %d", len(testData), len(retrieved))
		}

		for i, item := range retrieved {
			if item.URI != testData[i].URI {
				t.Errorf("Item %d URI mismatch: %s != %s", i, item.URI, testData[i].URI)
			}
		}
	})

	t.Run("setCachedRegistry with nil cache", func(t *testing.T) {
		t.Parallel()

		client := &Client{
			cache:       nil,
			cacheConfig: nil,
		}

		// Should not panic
		setCachedRegistry(client, "test", []*mcp.Resource{})

		// Verify nothing was cached
		var retrieved []*mcp.Resource
		if getCachedRegistry(client, "test", &retrieved) {
			t.Error("Expected no cache hit when cache is nil")
		}
	})

	t.Run("getCachedRegistry with zero TTL", func(t *testing.T) {
		t.Parallel()

		store := cache.NewInMemoryStore()
		client := &Client{}
		WithCache(store, &CacheConfig{
			RegistryTTL: 0, // Disabled
		})(client)

		// Should not use cache
		setCachedRegistry(client, "test", []*mcp.Resource{{URI: "file:///test"}})

		var retrieved []*mcp.Resource
		if getCachedRegistry(client, "test", &retrieved) {
			t.Error("Expected no caching when TTL is 0")
		}
	})

	t.Run("makeRegistryCacheKey uniqueness", func(t *testing.T) {
		t.Parallel()

		client1 := &Client{}
		client2 := &Client{}

		key1 := makeRegistryCacheKey("resource", client1)
		key2 := makeRegistryCacheKey("resource", client2)
		key3 := makeRegistryCacheKey("prompt", client1)

		// Different clients should have different keys
		if key1 == key2 {
			t.Error("Expected different cache keys for different clients")
		}

		// Different types should have different keys
		if key1 == key3 {
			t.Error("Expected different cache keys for different types")
		}

		// Verify key format
		if !strings.Contains(key1, "resource") {
			t.Error("Expected cache key to contain registry type")
		}
	})
}

func TestToolsRegistryCacheHelpers(t *testing.T) {
	t.Parallel()

	t.Run("getCachedToolsRegistry and setCachedToolsRegistry", func(t *testing.T) {
		t.Parallel()

		client, err := NewStdio("go", []string{"run", "../../../examples/mcp/cmd/server"},
			WithCache(cache.NewInMemoryStore(), &CacheConfig{
				RegistryTTL: 5 * time.Minute,
			}),
		)
		if err != nil {
			t.Skipf("Skipping test - MCP server not available: %v", err)
		}
		defer client.Close()

		ctx := context.Background()

		// Get tools to cache
		tools, err := Tools(ctx, client)
		if err != nil {
			t.Fatalf("Tools() failed: %v", err)
		}

		if len(tools) == 0 {
			t.Skip("No tools available")
		}

		cacheKey := makeToolsRegistryCacheKey(client)

		// Should now be cached
		cached := getCachedToolsRegistry(client, cacheKey)
		if cached == nil {
			t.Error("Expected tools to be cached")
		}

		if len(cached) != len(tools) {
			t.Errorf("Cached tools count mismatch: %d != %d", len(cached), len(tools))
		}

		// Verify tool properties preserved
		for i, tool := range cached {
			if tool.Name() != tools[i].Name() {
				t.Errorf("Tool %d name mismatch: %s != %s", i, tool.Name(), tools[i].Name())
			}
		}
	})

	t.Run("getCachedToolsRegistry with nil cache", func(t *testing.T) {
		t.Parallel()

		client := &Client{
			cache:       nil,
			cacheConfig: nil,
		}

		cacheKey := makeToolsRegistryCacheKey(client)
		cached := getCachedToolsRegistry(client, cacheKey)

		if cached != nil {
			t.Error("Expected nil when cache is disabled")
		}
	})

	t.Run("makeToolsRegistryCacheKey uniqueness", func(t *testing.T) {
		t.Parallel()

		client1 := &Client{}
		client2 := &Client{}

		key1 := makeToolsRegistryCacheKey(client1)
		key2 := makeToolsRegistryCacheKey(client2)

		// Different clients should have different keys
		if key1 == key2 {
			t.Error("Expected different cache keys for different clients")
		}

		// Verify key format
		if !strings.Contains(key1, "tools-native-registry") {
			t.Error("Expected cache key to contain 'tools-native-registry'")
		}
	})
}

func TestResourceRegistryWithCache(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	client, err := NewStdio("go", []string{"run", "../../../examples/mcp/cmd/server"},
		WithCache(cache.NewInMemoryStore(), &CacheConfig{
			RegistryTTL: 5 * time.Minute,
		}),
	)
	if err != nil {
		t.Skipf("Skipping test - MCP server not available: %v", err)
	}
	defer client.Close()

	handler := ResourceRegistry(client)

	// First call - cache miss
	req1 := calque.NewRequest(context.Background(), strings.NewReader("test"))
	var output1 strings.Builder
	res1 := calque.NewResponse(&output1)

	err = handler.ServeFlow(req1, res1)
	if err != nil {
		t.Fatalf("First ResourceRegistry call failed: %v", err)
	}

	resources1 := GetResources(req1.Context)

	// Second call - cache hit
	req2 := calque.NewRequest(context.Background(), strings.NewReader("test"))
	var output2 strings.Builder
	res2 := calque.NewResponse(&output2)

	err = handler.ServeFlow(req2, res2)
	if err != nil {
		t.Fatalf("Second ResourceRegistry call failed: %v", err)
	}

	resources2 := GetResources(req2.Context)

	// Verify both calls returned resources
	if len(resources1) == 0 || len(resources2) == 0 {
		t.Skip("No resources available from server")
	}

	// Verify cache hit returned same data
	if len(resources1) != len(resources2) {
		t.Errorf("Resource count mismatch: %d != %d", len(resources1), len(resources2))
	}
}

func TestPromptRegistryWithCache(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	client, err := NewStdio("go", []string{"run", "../../../examples/mcp/cmd/server"},
		WithCache(cache.NewInMemoryStore(), &CacheConfig{
			RegistryTTL: 5 * time.Minute,
		}),
	)
	if err != nil {
		t.Skipf("Skipping test - MCP server not available: %v", err)
	}
	defer client.Close()

	handler := PromptRegistry(client)

	// First call - cache miss
	req1 := calque.NewRequest(context.Background(), strings.NewReader("test"))
	var output1 strings.Builder
	res1 := calque.NewResponse(&output1)

	err = handler.ServeFlow(req1, res1)
	if err != nil {
		t.Fatalf("First PromptRegistry call failed: %v", err)
	}

	prompts1 := GetPrompts(req1.Context)

	// Second call - cache hit
	req2 := calque.NewRequest(context.Background(), strings.NewReader("test"))
	var output2 strings.Builder
	res2 := calque.NewResponse(&output2)

	err = handler.ServeFlow(req2, res2)
	if err != nil {
		t.Fatalf("Second PromptRegistry call failed: %v", err)
	}

	prompts2 := GetPrompts(req2.Context)

	// Verify both calls returned prompts
	if len(prompts1) == 0 || len(prompts2) == 0 {
		t.Skip("No prompts available from server")
	}

	// Verify cache hit returned same data
	if len(prompts1) != len(prompts2) {
		t.Errorf("Prompt count mismatch: %d != %d", len(prompts1), len(prompts2))
	}
}
