package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/calque-ai/go-calque/pkg/calque"
	"github.com/calque-ai/go-calque/pkg/middleware/cache"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestExecuteCacheHelpers(t *testing.T) {
	t.Parallel()

	t.Run("makePromptCacheKey with empty args", func(t *testing.T) {
		t.Parallel()

		key := makePromptCacheKey("test_prompt", nil)

		if !strings.Contains(key, "test_prompt") {
			t.Error("Expected cache key to contain prompt name")
		}

		if !strings.Contains(key, "prompt") {
			t.Error("Expected cache key to contain 'prompt'")
		}
	})

	t.Run("makePromptCacheKey with args", func(t *testing.T) {
		t.Parallel()

		args := map[string]string{"key": "value"}
		key1 := makePromptCacheKey("test_prompt", args)
		key2 := makePromptCacheKey("test_prompt", args)
		key3 := makePromptCacheKey("test_prompt", map[string]string{"key": "different"})

		// Same args should produce same key
		if key1 != key2 {
			t.Error("Expected same cache key for same args")
		}

		// Different args should produce different key
		if key1 == key3 {
			t.Error("Expected different cache key for different args")
		}
	})

	t.Run("getCachedResult and setCachedResult", func(t *testing.T) {
		t.Parallel()

		// Create client with caching using WithCache option (matches production usage)
		store := cache.NewInMemoryStore()
		client := &Client{}
		WithCache(store, &CacheConfig{
			ResourceTTL: 5 * time.Minute,
		})(client)

		cacheKey := "test-key"
		ttl := 5 * time.Minute
		testResult := &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{
				{URI: "test://resource", Text: "test content"},
			},
		}

		// Test cache miss
		var retrieved *mcp.ReadResourceResult
		if getCached(client, cacheKey, ttl, &retrieved) {
			t.Error("Expected cache miss, got cache hit")
		}

		// Store in cache
		setCached(client, cacheKey, ttl, testResult)

		// Debug: check client state
		t.Logf("Client cache: %v, cacheConfig: %v, ttl: %v", client.cache != nil, client.cacheConfig != nil, ttl)

		// Test cache hit
		if !getCached(client, cacheKey, ttl, &retrieved) {
			t.Fatalf("Expected cache hit, got cache miss. Cache nil? %v, Config nil? %v", client.cache == nil, client.cacheConfig == nil)
		}

		// Verify data
		if retrieved == nil {
			t.Fatal("Retrieved result should not be nil after cache hit")
		}

		if len(retrieved.Contents) != len(testResult.Contents) {
			t.Errorf("Expected %d contents, got %d", len(testResult.Contents), len(retrieved.Contents))
		}

		if len(retrieved.Contents) > 0 && retrieved.Contents[0].Text != testResult.Contents[0].Text {
			t.Errorf("Content mismatch: %s != %s", retrieved.Contents[0].Text, testResult.Contents[0].Text)
		}
	})

	t.Run("getCachedResult with nil cache", func(t *testing.T) {
		t.Parallel()

		client := &Client{
			cache:       nil,
			cacheConfig: nil,
		}

		var retrieved *mcp.ReadResourceResult
		if getCached(client, "test", time.Minute, &retrieved) {
			t.Error("Expected no cache hit when cache is nil")
		}
	})

	t.Run("setCachedResult with zero TTL", func(t *testing.T) {
		t.Parallel()

		store := cache.NewInMemoryStore()
		client := &Client{}
		WithCache(store, &CacheConfig{})(client)

		testResult := &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{{URI: "test://resource", Text: "test"}},
		}

		// Should not cache with zero TTL
		setCached(client, "test", 0, testResult)

		var retrieved *mcp.ReadResourceResult
		if getCached(client, "test", 0, &retrieved) {
			t.Error("Expected no caching when TTL is 0")
		}
	})
}

func TestStoreInContextAndPassThrough(t *testing.T) {
	t.Parallel()

	t.Run("stores resource content in context", func(t *testing.T) {
		t.Parallel()

		resourceContent := &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{
				{URI: "test://resource", Text: "test content"},
			},
		}

		input := "user input"
		ctx := context.Background()
		req := calque.NewRequest(ctx, strings.NewReader(input))
		var output strings.Builder
		res := calque.NewResponse(&output)

		err := storeInContextAndPassThrough(ctx, req, res, resourceContentContextKey{}, resourceContent)
		if err != nil {
			t.Fatalf("storeInContextAndPassThrough failed: %v", err)
		}

		// Verify context was updated
		stored := GetResourceContent(req.Context)
		if stored == nil {
			t.Fatal("Expected resource content to be stored in context")
		}

		if len(stored.Contents) == 0 || stored.Contents[0].Text != resourceContent.Contents[0].Text {
			t.Error("Stored content doesn't match original")
		}

		// Verify input passed through
		if output.String() != input {
			t.Errorf("Expected output %q, got %q", input, output.String())
		}
	})

	t.Run("stores prompt content in context", func(t *testing.T) {
		t.Parallel()

		promptContent := &mcp.GetPromptResult{
			Description: "test prompt",
		}

		input := "user input"
		ctx := context.Background()
		req := calque.NewRequest(ctx, strings.NewReader(input))
		var output strings.Builder
		res := calque.NewResponse(&output)

		err := storeInContextAndPassThrough(ctx, req, res, promptContentContextKey{}, promptContent)
		if err != nil {
			t.Fatalf("storeInContextAndPassThrough failed: %v", err)
		}

		// Verify context was updated
		stored := GetPromptContent(req.Context)
		if stored == nil {
			t.Fatal("Expected prompt content to be stored in context")
		}

		if stored.Description != promptContent.Description {
			t.Error("Stored content doesn't match original")
		}

		// Verify input passed through
		if output.String() != input {
			t.Errorf("Expected output %q, got %q", input, output.String())
		}
	})

	t.Run("handles empty input", func(t *testing.T) {
		t.Parallel()

		resourceContent := &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{{URI: "test://resource", Text: "test"}},
		}

		ctx := context.Background()
		req := calque.NewRequest(ctx, strings.NewReader(""))
		var output strings.Builder
		res := calque.NewResponse(&output)

		err := storeInContextAndPassThrough(ctx, req, res, resourceContentContextKey{}, resourceContent)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		if output.String() != "" {
			t.Errorf("Expected empty output, got %q", output.String())
		}
	})
}

func TestExecuteResourceCaching(t *testing.T) {
	t.Parallel()

	t.Run("caches resource content", func(t *testing.T) {
		t.Parallel()

		client, err := NewStdio("go", []string{"run", "../../../examples/mcp/cmd/server"},
			WithCache(cache.NewInMemoryStore(), &CacheConfig{
				ResourceTTL: 5 * time.Minute,
			}),
		)
		if err != nil {
			t.Skipf("Skipping test - MCP server not available: %v", err)
		}
		defer client.Close()

		// Setup: Get resources
		registryHandler := ResourceRegistry(client)
		req1 := calque.NewRequest(context.Background(), strings.NewReader("test"))
		var regOutput strings.Builder
		res1 := calque.NewResponse(&regOutput)

		err = registryHandler.ServeFlow(req1, res1)
		if err != nil {
			t.Fatalf("ResourceRegistry failed: %v", err)
		}

		resources := GetResources(req1.Context)
		if len(resources) == 0 {
			t.Skip("No resources available")
		}

		// Select first resource
		selectedURI := resources[0].URI
		ctx := context.WithValue(req1.Context, selectedResourceContextKey{}, selectedURI)

		executeHandler := ExecuteResource(client)

		// First call - cache miss
		execReq1 := calque.NewRequest(ctx, strings.NewReader("fetch"))
		var execOutput1 strings.Builder
		execRes1 := calque.NewResponse(&execOutput1)

		err = executeHandler.ServeFlow(execReq1, execRes1)
		if err != nil {
			t.Fatalf("First ExecuteResource failed: %v", err)
		}

		result1 := execOutput1.String()
		content1 := GetResourceContent(execReq1.Context)

		// Second call - cache hit
		execReq2 := calque.NewRequest(ctx, strings.NewReader("fetch"))
		var execOutput2 strings.Builder
		execRes2 := calque.NewResponse(&execOutput2)

		err = executeHandler.ServeFlow(execReq2, execRes2)
		if err != nil {
			t.Fatalf("Second ExecuteResource failed: %v", err)
		}

		result2 := execOutput2.String()
		content2 := GetResourceContent(execReq2.Context)

		// Verify both calls succeeded
		if result1 == "" || result2 == "" {
			t.Error("Expected non-empty results")
		}

		if content1 == nil || content2 == nil {
			t.Error("Expected content to be stored in context")
		}

		// Results should match (from cache)
		if result1 != result2 {
			t.Error("Cache hit should return same result as cache miss")
		}
	})
}

func TestExecutePromptCaching(t *testing.T) {
	t.Parallel()

	t.Run("caches prompt with different args separately", func(t *testing.T) {
		t.Parallel()

		client, err := NewStdio("go", []string{"run", "../../../examples/mcp/cmd/server"},
			WithCache(cache.NewInMemoryStore(), &CacheConfig{
				PromptTTL: 5 * time.Minute,
			}),
		)
		if err != nil {
			t.Skipf("Skipping test - MCP server not available: %v", err)
		}
		defer client.Close()

		// Setup: Get prompts
		registryHandler := PromptRegistry(client)
		req1 := calque.NewRequest(context.Background(), strings.NewReader("test"))
		var regOutput strings.Builder
		res1 := calque.NewResponse(&regOutput)

		err = registryHandler.ServeFlow(req1, res1)
		if err != nil {
			t.Fatalf("PromptRegistry failed: %v", err)
		}

		prompts := GetPrompts(req1.Context)
		if len(prompts) == 0 {
			t.Skip("No prompts available")
		}

		// Find prompt with arguments
		var selectedPrompt *mcp.Prompt
		for _, p := range prompts {
			if len(p.Arguments) > 0 {
				selectedPrompt = p
				break
			}
		}

		if selectedPrompt == nil {
			t.Skip("No prompts with arguments available")
		}

		ctx := context.WithValue(req1.Context, selectedPromptContextKey{}, selectedPrompt.Name)
		executeHandler := ExecutePrompt(client)

		// Call with first args
		args1 := map[string]string{selectedPrompt.Arguments[0].Name: "value1"}
		args1JSON, _ := json.Marshal(args1)

		execReq1 := calque.NewRequest(ctx, strings.NewReader(string(args1JSON)))
		var execOutput1 strings.Builder
		execRes1 := calque.NewResponse(&execOutput1)

		err = executeHandler.ServeFlow(execReq1, execRes1)
		if err != nil {
			t.Fatalf("First ExecutePrompt failed: %v", err)
		}

		// Call with different args (should be cache miss)
		args2 := map[string]string{selectedPrompt.Arguments[0].Name: "value2"}
		args2JSON, _ := json.Marshal(args2)

		execReq2 := calque.NewRequest(ctx, strings.NewReader(string(args2JSON)))
		var execOutput2 strings.Builder
		execRes2 := calque.NewResponse(&execOutput2)

		err = executeHandler.ServeFlow(execReq2, execRes2)
		if err != nil {
			t.Fatalf("Second ExecutePrompt failed: %v", err)
		}

		// Both should have content
		if execOutput1.String() == "" || execOutput2.String() == "" {
			t.Error("Expected non-empty results")
		}

		// Call with first args again (should be cache hit)
		execReq3 := calque.NewRequest(ctx, strings.NewReader(string(args1JSON)))
		var execOutput3 strings.Builder
		execRes3 := calque.NewResponse(&execOutput3)

		err = executeHandler.ServeFlow(execReq3, execRes3)
		if err != nil {
			t.Fatalf("Third ExecutePrompt failed: %v", err)
		}

		// Should match first call (cached)
		if execOutput1.String() != execOutput3.String() {
			t.Error("Cache with same args should return same result")
		}
	})
}

func TestPassThrough(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
	}{
		{"normal text", "hello world"},
		{"empty string", ""},
		{"unicode", "Hello 世界 🌍"},
		{"large text", strings.Repeat("a", 10000)},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := calque.NewRequest(context.Background(), strings.NewReader(tt.input))
			var output strings.Builder
			res := calque.NewResponse(&output)

			err := passThrough(req, res)
			if err != nil {
				t.Fatalf("passThrough failed: %v", err)
			}

			if output.String() != tt.input {
				t.Errorf("Expected %q, got %q", tt.input, output.String())
			}
		})
	}
}
