// Package mcp provides Model Context Protocol (MCP) integration for calque flows.
//
// This package implements MCP client middleware that connects to MCP servers
// to access tools, resources, and prompts through the standardized protocol.
// Supports multiple transport types and configurable error handling.
package mcp

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/calque-ai/go-calque/pkg/middleware/cache"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Client provides MCP (Model Context Protocol) integration for calque flows.
//
// Connects to MCP servers to access tools, resources, and prompts through
// standardized protocol. Supports multiple transport types and configurable
// error handling for resilient or fail-fast behavior.
//
// Example:
//
//	client, _ := mcp.NewStdio("python", []string{"server.py"})
//	flow.Use(client.Tool("search", map[string]any{"query": "golang"}))
type Client struct {
	session           *mcp.ClientSession
	client            *mcp.Client
	transport         mcp.Transport
	onError           func(error)
	capabilities      []string
	timeout           time.Duration
	implementation    *mcp.Implementation
	progressCallbacks map[string][]func(*ProgressNotificationParams)
	subscriptions     map[string]func(*ResourceUpdatedNotificationParams)
	completionEnabled bool
	env               map[string]string
	cache             *cache.Memory
	cacheConfig       *CacheConfig
	mu                sync.RWMutex
}

// defaultImplementation provides default client identification
func defaultImplementation() *mcp.Implementation {
	return &mcp.Implementation{
		Name:    "calque-mcp-client",
		Version: "v0.1.0",
	}
}

// newClient creates a Client with the given MCP client and options
func newClient(mcpClient *mcp.Client, opts ...Option) *Client {
	client := &Client{
		client:            mcpClient,
		timeout:           0, // No timeout by default
		implementation:    defaultImplementation(),
		capabilities:      []string{}, // Empty by default - no required capabilities
		progressCallbacks: make(map[string][]func(*ProgressNotificationParams)),
		subscriptions:     make(map[string]func(*ResourceUpdatedNotificationParams)),
		completionEnabled: false,
	}

	for _, opt := range opts {
		opt(client)
	}

	return client
}

// connect establishes the MCP session if not already connected
func (c *Client) connect(ctx context.Context) error {
	if c.session != nil {
		return nil
	}

	if c.client == nil {
		return fmt.Errorf("MCP client not initialized")
	}

	if c.transport == nil {
		return fmt.Errorf("MCP transport not configured")
	}

	var err error
	c.session, err = c.client.Connect(ctx, c.transport, nil)
	if err != nil {
		return fmt.Errorf("failed to connect to MCP server: %w", err)
	}

	// Validate server capabilities match our requirements
	if err := c.validateCapabilities(ctx); err != nil {
		if closeErr := c.session.Close(); closeErr != nil {
			return fmt.Errorf("capability validation failed: %w, and failed to close session: %v", err, closeErr)
		}
		c.session = nil
		return fmt.Errorf("capability validation failed: %w", err)
	}

	return nil
}

// validateCapabilities checks if server supports the specified capabilities
func (c *Client) validateCapabilities(ctx context.Context) error {
	// If no capabilities are specified, skip validation
	if len(c.capabilities) == 0 {
		return nil
	}

	// Check tools capability
	if slices.Contains(c.capabilities, "tools") {
		if _, err := c.session.ListTools(ctx, &mcp.ListToolsParams{}); err != nil {
			return fmt.Errorf("server does not support tools capability: %w", err)
		}
	}

	// Check resources capability
	if slices.Contains(c.capabilities, "resources") {
		if _, err := c.session.ListResources(ctx, &mcp.ListResourcesParams{}); err != nil {
			return fmt.Errorf("server does not support resources capability: %w", err)
		}
	}

	// Check prompts capability
	if slices.Contains(c.capabilities, "prompts") {
		if _, err := c.session.ListPrompts(ctx, &mcp.ListPromptsParams{}); err != nil {
			return fmt.Errorf("server does not support prompts capability: %w", err)
		}
	}

	return nil
}

// handleError processes errors according to configured strategy
func (c *Client) handleError(err error) error {
	if c.onError != nil {
		c.onError(err)
		return nil
	}
	return err
}

// handleProgressNotification processes progress notifications from MCP server
func (c *Client) handleProgressNotification(params *ProgressNotificationParams) {
	if progressToken, ok := params.ProgressToken.(string); ok {
		c.mu.RLock()
		callbacks, exists := c.progressCallbacks[progressToken]
		c.mu.RUnlock()

		if exists {
			for _, callback := range callbacks {
				callback(params)
			}
		}
	}
}

// handleResourceUpdated processes resource update notifications from MCP server
func (c *Client) handleResourceUpdated(params *ResourceUpdatedNotificationParams) {
	c.mu.RLock()
	callback, exists := c.subscriptions[params.URI]
	c.mu.RUnlock()

	if exists {
		callback(params)
	}
}

// CleanupProgressCallback removes progress callbacks for completed operations
func (c *Client) CleanupProgressCallback(progressToken string) {
	c.mu.Lock()
	delete(c.progressCallbacks, progressToken)
	c.mu.Unlock()
}

// CleanupSubscription removes resource subscription callback
func (c *Client) CleanupSubscription(uri string) {
	c.mu.Lock()
	delete(c.subscriptions, uri)
	c.mu.Unlock()
}

// Close closes the MCP session
func (c *Client) Close() error {
	if c.session != nil {
		return c.session.Close()
	}
	return nil
}
