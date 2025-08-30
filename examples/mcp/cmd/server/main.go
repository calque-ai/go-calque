// Package main provides an MCP server for calque examples
package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type GreetParams struct {
	Name string `json:"name" jsonschema:"the name of the person to greet"`
}

func greetTool(_ context.Context, _ *mcp.CallToolRequest, args GreetParams) (*mcp.CallToolResult, any, error) {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: "Hello, " + args.Name + "!"},
		},
	}, nil, nil
}

type CalculateParams struct {
	A int `json:"a" jsonschema:"first number"`
	B int `json:"b" jsonschema:"second number"`
}

func multiplyTool(_ context.Context, _ *mcp.CallToolRequest, args CalculateParams) (*mcp.CallToolResult, any, error) {
	result := args.A * args.B
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: fmt.Sprintf("%d", result)},
		},
	}, nil, nil
}

type SearchParams struct {
	Query string `json:"query" jsonschema:"search query"`
	Limit int    `json:"limit" jsonschema:"maximum number of results"`
}

func searchTool(_ context.Context, _ *mcp.CallToolRequest, args SearchParams) (*mcp.CallToolResult, any, error) {
	results := []string{
		"Go programming tutorial - Learn Go basics",
		"Advanced Go patterns and best practices",
		"Go concurrency with goroutines and channels",
		"Building web APIs in Go",
		"Go testing frameworks and strategies",
	}

	if args.Limit > 0 && args.Limit < len(results) {
		results = results[:args.Limit]
	}

	var output strings.Builder
	output.WriteString(fmt.Sprintf("Search results for '%s':\n", args.Query))
	for i, result := range results {
		output.WriteString(fmt.Sprintf("%d. %s\n", i+1, result))
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: output.String()},
		},
	}, nil, nil
}

type FileParams struct {
	Path string `json:"path" jsonschema:"file path to read"`
}

func readFileTool(_ context.Context, _ *mcp.CallToolRequest, args FileParams) (*mcp.CallToolResult, any, error) {
	content, err := os.ReadFile(args.Path)
	if err != nil {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Error reading file: %v", err)},
			},
		}, nil, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(content)},
		},
	}, nil, nil
}

type ProgressParams struct {
	Steps int `json:"steps" jsonschema:"number of steps to simulate"`
}

func progressTool(_ context.Context, _ *mcp.CallToolRequest, args ProgressParams) (*mcp.CallToolResult, any, error) {
	progressToken := fmt.Sprintf("progress-%d", time.Now().Unix())

	// Simulate work with progress updates
	for i := 0; i < args.Steps; i++ {
		// Note: In a real implementation, you'd send progress notifications here
		// For this example, we'll just simulate the work
		time.Sleep(100 * time.Millisecond)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: fmt.Sprintf("Completed %d steps", args.Steps)},
		},
		Meta: map[string]any{
			"progressToken": progressToken,
		},
	}, nil, nil
}

func docResourceHandler(_ context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	switch req.Params.URI {
	case "file:///api-docs":
		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{
				{
					URI:  req.Params.URI,
					Text: "# API Documentation\n\n## Authentication\nUse API key in Authorization header.\n\n## Endpoints\n- GET /users - List users\n- POST /users - Create user",
				},
			},
		}, nil
	case "file:///config-schema":
		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{
				{
					URI:  req.Params.URI,
					Text: `{"type": "object", "properties": {"database": {"type": "object", "properties": {"host": {"type": "string"}, "port": {"type": "number"}}}}}`,
				},
			},
		}, nil
	default:
		return nil, mcp.ResourceNotFoundError(req.Params.URI)
	}
}

func configResourceHandler(_ context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	path := strings.TrimPrefix(req.Params.URI, "file:///configs/")

	switch path {
	case "database.json":
		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{
				{
					URI:  req.Params.URI,
					Text: `{"host": "localhost", "port": 5432, "database": "app_db"}`,
				},
			},
		}, nil
	case "cache.json":
		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{
				{
					URI:  req.Params.URI,
					Text: `{"redis_url": "redis://localhost:6379", "ttl": 3600}`,
				},
			},
		}, nil
	default:
		return nil, mcp.ResourceNotFoundError(req.Params.URI)
	}
}

func codeReviewPromptHandler(_ context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	language := "unknown"
	style := "standard"

	if req.Params.Arguments != nil {
		if lang, ok := req.Params.Arguments["language"]; ok {
			language = lang
		}
		if s, ok := req.Params.Arguments["style"]; ok {
			style = s
		}
	}

	promptText := fmt.Sprintf("Please review this %s code using %s review criteria. Focus on:", language, style)
	if style == "security" {
		promptText += "\n- Security vulnerabilities\n- Input validation\n- Authentication/authorization"
	} else {
		promptText += "\n- Code quality\n- Best practices\n- Performance"
	}

	return &mcp.GetPromptResult{
		Description: "Code review prompt template",
		Messages: []*mcp.PromptMessage{
			{
				Role:    "user",
				Content: &mcp.TextContent{Text: promptText},
			},
		},
	}, nil
}

func NewServer() *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "calque-example-server",
		Version: "1.0.0",
	}, nil)

	// Add tools
	mcp.AddTool(server, &mcp.Tool{
		Name:        "greet",
		Description: "Greet a person by name",
	}, greetTool)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "multiply",
		Description: "Multiply two numbers",
	}, multiplyTool)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "search",
		Description: "Search for information",
	}, searchTool)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "read_file",
		Description: "Read contents of a file",
	}, readFileTool)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "progress_demo",
		Description: "Demonstrate progress tracking",
	}, progressTool)

	// Add static resources
	server.AddResource(&mcp.Resource{
		URI:         "file:///api-docs",
		Name:        "API Documentation",
		Description: "REST API documentation",
	}, docResourceHandler)

	server.AddResource(&mcp.Resource{
		URI:         "file:///config-schema",
		Name:        "Configuration Schema",
		Description: "JSON schema for configuration files",
	}, docResourceHandler)

	// Add resource template
	server.AddResourceTemplate(&mcp.ResourceTemplate{
		URITemplate: "file:///configs/{name}",
		Name:        "Configuration Files",
		Description: "Access to various config files",
	}, configResourceHandler)

	// Add prompt
	server.AddPrompt(&mcp.Prompt{
		Name:        "code_review",
		Description: "Generate code review prompts",
	}, codeReviewPromptHandler)

	return server
}

func main() {
	server := NewServer()
	transport := &mcp.StdioTransport{}
	if err := server.Run(context.Background(), transport); err != nil {
		panic(err)
	}
}
