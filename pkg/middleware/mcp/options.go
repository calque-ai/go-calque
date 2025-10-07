package mcp

import (
	"maps"
	"time"

	"github.com/calque-ai/go-calque/pkg/middleware/cache"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Option configures MCP client behavior
type Option func(*Client)

// CacheConfig configures caching behavior for different MCP operations.
type CacheConfig struct {
	// RegistryTTL is the time-to-live for registry list caches (tools, resources, prompts).
	// Registry lists rarely change during a session.
	RegistryTTL time.Duration

	// ResourceTTL is the time-to-live for resource content cache.
	ResourceTTL time.Duration

	// PromptTTL is the time-to-live for prompt template cache.
	PromptTTL time.Duration

	// ToolTTL is the time-to-live for tool result cache.
	// Note: Most tools are dynamic, so caching may not be beneficial.
	ToolTTL time.Duration

	// CompletionTTL is the time-to-live for completion cache.
	CompletionTTL time.Duration
}

// WithCapabilities filters which MCP capabilities the client will use.
//
// Input: capability names ("tools", "resources", "prompts")
// Output: Option function
// Behavior: Restricts client to specified capabilities only
//
// Allows limiting client functionality to specific MCP capabilities.
// Useful for security or performance when only certain features are needed.
//
// Example:
//
//	client, _ := mcp.NewStdio("python", []string{"server.py"},
//		mcp.WithCapabilities("tools", "resources"))
func WithCapabilities(caps ...string) Option {
	return func(c *Client) {
		c.capabilities = caps
	}
}

// WithTimeout sets the request timeout for MCP operations.
//
// Input: time.Duration for timeout
// Output: Option function
// Behavior: Sets timeout for all MCP requests
//
// Configures how long to wait for MCP server responses before timing out.
// Applies to tool calls, resource fetches, and prompt operations.
//
// Example:
//
//	client, _ := mcp.NewStdio("python", []string{"server.py"},
//		mcp.WithTimeout(45*time.Second))
func WithTimeout(timeout time.Duration) Option {
	return func(c *Client) {
		c.timeout = timeout
	}
}

// WithImplementation sets client identification for MCP handshake.
//
// Input: client name and version strings
// Output: Option function
// Behavior: Identifies client to MCP server during connection
//
// MCP servers can use this information for logging, compatibility checks,
// or feature detection. Similar to HTTP User-Agent headers.
//
// Example:
//
//	client, _ := mcp.NewStdio("python", []string{"server.py"},
//		mcp.WithImplementation("my-app", "v1.2.0"))
func WithImplementation(name, version string) Option {
	return func(c *Client) {
		c.implementation = &mcp.Implementation{
			Name:    name,
			Version: version,
		}
	}
}

// WithOnError configures error handling for MCP operations.
//
// Input: error callback function
// Output: Option function
// Behavior: Enables resilient error handling instead of failing flow
//
// When provided, MCP errors are passed to the callback and the flow continues.
// Without this option, MCP errors will bubble up and stop the entire flow.
// Follows the same pattern as cache middleware error handling.
//
// Example:
//
//	client, _ := mcp.NewStdio("python", []string{"server.py"},
//		mcp.WithOnError(func(err error) {
//			log.Printf("MCP error: %v", err)
//		}))
func WithOnError(callback func(error)) Option {
	return func(c *Client) {
		c.onError = callback
	}
}

// WithCompletion enables auto-completion support for MCP operations.
//
// Input: boolean flag to enable/disable completion
// Output: Option function
// Behavior: Enables completion capability for prompt/resource arguments
//
// When enabled, the client can provide auto-completion suggestions for
// prompt arguments and resource URIs. Helps users discover valid parameter
// values and reduces input errors in interactive environments.
//
// Example:
//
//	client, _ := mcp.NewStdio("python", []string{"server.py"},
//		mcp.WithCompletion(true))
func WithCompletion(enabled bool) Option {
	return func(c *Client) {
		c.completionEnabled = enabled
	}
}

// WithEnv configures environment variables for MCP transport.
//
// Input: map of environment variable names to values
// Output: Option function
// Behavior: Sets environment variables for transport-specific usage
//
// For stdio transports, these are passed as environment variables to the subprocess.
// For SSE transports, these are passed as HTTP headers in requests.
//
// Example:
//
//	client, _ := mcp.NewStdio("python", []string{"server.py"},
//		mcp.WithEnv(map[string]string{
//			"API_KEY": "your-api-key",
//			"DEBUG": "true",
//		}))
func WithEnv(env map[string]string) Option {
	return func(c *Client) {
		if c.env == nil {
			c.env = make(map[string]string)
		}
		maps.Copy(c.env, env)
	}
}

// WithCache enables caching for MCP operations using the provided store and optional configuration.
//
// Input: cache store and optional cache configuration
// Output: Option function
// Behavior: Enables response caching for Resource, Prompt, Tool, and Completion handlers
//
// Caches MCP operation results to improve performance for repeated requests.
// If no configuration is provided, uses sensible defaults. Different TTLs can be
// configured for different operation types based on their expected change frequency and cost.
//
// Example:
//
//	// With custom config
//	client, _ := mcp.NewStdio("python", []string{"server.py"},
//		mcp.WithCache(cache.NewInMemoryStore(), &CacheConfig{
//			ResourceTTL: 5 * time.Minute,
//			PromptTTL:   15 * time.Minute,
//		}))
//
//	// With defaults
//	client, _ := mcp.NewStdio("python", []string{"server.py"},
//		mcp.WithCache(cache.NewInMemoryStore()))
func WithCache(store cache.Store, config ...*CacheConfig) Option {
	return func(c *Client) {
		c.cache = cache.NewCacheWithStore(store)
		if len(config) > 0 && config[0] != nil {
			c.cacheConfig = config[0]
		} else {
			c.cacheConfig = defaultCacheConfig()
		}
	}
}

// defaultCacheConfig returns sensible defaults for MCP caching.
func defaultCacheConfig() *CacheConfig {
	return &CacheConfig{
		RegistryTTL:   15 * time.Minute, // Registries change rarely
		ResourceTTL:   5 * time.Minute,
		PromptTTL:     15 * time.Minute,
		ToolTTL:       0, // No caching by default for dynamic tools
		CompletionTTL: 10 * time.Minute,
	}
}
