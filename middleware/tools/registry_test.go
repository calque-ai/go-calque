package tools

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
)

func TestRegistry(t *testing.T) {
	// Create test tools
	tool1 := Simple("tool1", "Tool 1 description", func(s string) string { return "result1: " + s })
	tool2 := Simple("tool2", "Tool 2 description", func(s string) string { return "result2: " + s })
	tool3 := Simple("tool3", "Tool 3 description", func(s string) string { return "result3: " + s })

	tests := []struct {
		name     string
		tools    []Tool
		input    string
		expected string
	}{
		{
			name:     "registry with multiple tools",
			tools:    []Tool{tool1, tool2, tool3},
			input:    "test input",
			expected: "test input",
		},
		{
			name:     "registry with single tool",
			tools:    []Tool{tool1},
			input:    "single tool test",
			expected: "single tool test",
		},
		{
			name:     "registry with no tools",
			tools:    []Tool{},
			input:    "no tools test",
			expected: "no tools test",
		},
		{
			name:     "registry with empty input",
			tools:    []Tool{tool1, tool2},
			input:    "",
			expected: "",
		},
		{
			name:     "registry with large input",
			tools:    []Tool{tool1},
			input:    strings.Repeat("large data ", 1000),
			expected: strings.Repeat("large data ", 1000),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := Registry(tt.tools...)

			var buf bytes.Buffer
			reader := strings.NewReader(tt.input)

			// Create a context to capture the tools
			ctx := context.Background()

			// We need to test that the registry middleware stores tools in context,
			// but since we can't directly access the context from the registry,
			// we'll need to test this by creating a custom handler that checks the context
			contextChecker := func(ctx context.Context, r io.Reader, w io.Writer) error {
				// Verify tools are in context
				tools := GetTools(ctx)
				if len(tools) != len(tt.tools) {
					t.Errorf("Expected %d tools in context, got %d", len(tt.tools), len(tools))
				}

				// Pass through the data
				_, err := io.Copy(w, r)
				return err
			}

			// Create a pipeline that uses registry then checks context
			pipeline := func(ctx context.Context, r io.Reader, w io.Writer) error {
				// First apply registry
				var intermediateBuffer bytes.Buffer
				if err := registry.ServeFlow(ctx, r, &intermediateBuffer); err != nil {
					return err
				}

				// Then apply our context checker with the same context
				// Note: In real usage, the context would be automatically passed through the pipeline
				return contextChecker(ctx, &intermediateBuffer, w)
			}

			err := pipeline(ctx, reader, &buf)
			if err != nil {
				t.Errorf("Pipeline error = %v", err)
				return
			}

			if got := buf.String(); got != tt.expected {
				t.Errorf("Registry() output = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestRegistryStreamIntegrity(t *testing.T) {
	tool1 := Simple("tool1", "Tool 1 desc", func(s string) string { return s })
	registry := Registry(tool1)

	// Test with large input to ensure streaming works correctly
	largeInput := strings.Repeat("0123456789", 10000) // 100KB
	var buf bytes.Buffer
	reader := strings.NewReader(largeInput)

	err := registry.ServeFlow(context.Background(), reader, &buf)
	if err != nil {
		t.Errorf("Registry() with large input error = %v", err)
		return
	}

	if got := buf.String(); got != largeInput {
		t.Errorf("Registry() corrupted large stream, length got %d, want %d", len(got), len(largeInput))
	}
}

func TestGetTools(t *testing.T) {
	tool1 := Simple("tool1", "Tool 1 desc", func(s string) string { return s })
	tool2 := Simple("tool2", "Tool 2 desc", func(s string) string { return s })

	tests := []struct {
		name          string
		tools         []Tool
		expectedCount int
	}{
		{
			name:          "context with multiple tools",
			tools:         []Tool{tool1, tool2},
			expectedCount: 2,
		},
		{
			name:          "context with single tool",
			tools:         []Tool{tool1},
			expectedCount: 1,
		},
		{
			name:          "context with no tools",
			tools:         []Tool{},
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.WithValue(context.Background(), toolsContextKey{}, tt.tools)
			tools := GetTools(ctx)

			if len(tools) != tt.expectedCount {
				t.Errorf("GetTools() returned %d tools, want %d", len(tools), tt.expectedCount)
			}

			// Verify tool identity
			for i, tool := range tools {
				if i < len(tt.tools) && tool.Name() != tt.tools[i].Name() {
					t.Errorf("Tool %d name = %q, want %q", i, tool.Name(), tt.tools[i].Name())
				}
			}
		})
	}
}

func TestGetToolsFromEmptyContext(t *testing.T) {
	ctx := context.Background()
	tools := GetTools(ctx)

	if tools != nil {
		t.Errorf("GetTools() from empty context = %v, want nil", tools)
	}
}

func TestGetTool(t *testing.T) {
	tool1 := Simple("calculator", "Calculator tool", func(s string) string { return s })
	tool2 := Simple("search", "Search tool", func(s string) string { return s })
	tool3 := Simple("formatter", "Formatter tool", func(s string) string { return s })

	ctx := context.WithValue(context.Background(), toolsContextKey{}, []Tool{tool1, tool2, tool3})

	tests := []struct {
		name     string
		toolName string
		found    bool
	}{
		{
			name:     "find existing tool",
			toolName: "calculator",
			found:    true,
		},
		{
			name:     "find another existing tool",
			toolName: "search",
			found:    true,
		},
		{
			name:     "find non-existent tool",
			toolName: "nonexistent",
			found:    false,
		},
		{
			name:     "find with empty name",
			toolName: "",
			found:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tool := GetTool(ctx, tt.toolName)

			if tt.found {
				if tool == nil {
					t.Errorf("GetTool(%q) = nil, want tool", tt.toolName)
				} else if tool.Name() != tt.toolName {
					t.Errorf("GetTool(%q).Name() = %q, want %q", tt.toolName, tool.Name(), tt.toolName)
				}
			} else {
				if tool != nil {
					t.Errorf("GetTool(%q) = %v, want nil", tt.toolName, tool)
				}
			}
		})
	}
}

func TestGetToolFromEmptyContext(t *testing.T) {
	ctx := context.Background()
	tool := GetTool(ctx, "any_tool")

	if tool != nil {
		t.Errorf("GetTool() from empty context = %v, want nil", tool)
	}
}

func TestHasTool(t *testing.T) {
	tool1 := Simple("existing_tool", "Existing tool", func(s string) string { return s })
	tool2 := Simple("another_tool", "Another tool", func(s string) string { return s })

	ctx := context.WithValue(context.Background(), toolsContextKey{}, []Tool{tool1, tool2})

	tests := []struct {
		name     string
		toolName string
		expected bool
	}{
		{
			name:     "tool exists",
			toolName: "existing_tool",
			expected: true,
		},
		{
			name:     "another tool exists",
			toolName: "another_tool",
			expected: true,
		},
		{
			name:     "tool does not exist",
			toolName: "missing_tool",
			expected: false,
		},
		{
			name:     "empty tool name",
			toolName: "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HasTool(ctx, tt.toolName)

			if result != tt.expected {
				t.Errorf("HasTool(%q) = %v, want %v", tt.toolName, result, tt.expected)
			}
		})
	}
}

func TestHasToolFromEmptyContext(t *testing.T) {
	ctx := context.Background()
	result := HasTool(ctx, "any_tool")

	if result {
		t.Errorf("HasTool() from empty context = %v, want false", result)
	}
}

func TestListToolNames(t *testing.T) {
	tests := []struct {
		name          string
		tools         []Tool
		expectedNames []string
	}{
		{
			name: "multiple tools",
			tools: []Tool{
				Simple("tool1", "Tool 1", func(s string) string { return s }),
				Simple("tool2", "Tool 2", func(s string) string { return s }),
				Simple("tool3", "Tool 3", func(s string) string { return s }),
			},
			expectedNames: []string{"tool1", "tool2", "tool3"},
		},
		{
			name: "single tool",
			tools: []Tool{
				Simple("single", "Single tool", func(s string) string { return s }),
			},
			expectedNames: []string{"single"},
		},
		{
			name:          "no tools",
			tools:         []Tool{},
			expectedNames: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.WithValue(context.Background(), toolsContextKey{}, tt.tools)
			names := ListToolNames(ctx)

			if len(names) != len(tt.expectedNames) {
				t.Errorf("ListToolNames() returned %d names, want %d", len(names), len(tt.expectedNames))
			}

			for i, name := range names {
				if i < len(tt.expectedNames) && name != tt.expectedNames[i] {
					t.Errorf("ListToolNames()[%d] = %q, want %q", i, name, tt.expectedNames[i])
				}
			}
		})
	}
}

func TestListToolNamesFromEmptyContext(t *testing.T) {
	ctx := context.Background()
	names := ListToolNames(ctx)

	if names != nil {
		t.Errorf("ListToolNames() from empty context = %v, want nil", names)
	}
}

func TestListTools(t *testing.T) {
	tools := []Tool{
		Simple("tool1", "Tool 1 description", func(s string) string { return s }),
		Simple("tool2", "Tool 2 description", func(s string) string { return s }),
		Simple("tool3", "Tool 3 description", func(s string) string { return s }),
	}

	ctx := context.WithValue(context.Background(), toolsContextKey{}, tools)
	infos := ListTools(ctx)

	if len(infos) != len(tools) {
		t.Errorf("ListTools() returned %d infos, want %d", len(infos), len(tools))
	}

	for i, info := range infos {
		if i < len(tools) {
			expectedName := tools[i].Name()
			expectedDesc := tools[i].Description()

			if info.Name != expectedName {
				t.Errorf("ListTools()[%d].Name = %q, want %q", i, info.Name, expectedName)
			}

			if info.Description != expectedDesc {
				t.Errorf("ListTools()[%d].Description = %q, want %q", i, info.Description, expectedDesc)
			}
		}
	}
}

func TestListToolsFromEmptyContext(t *testing.T) {
	ctx := context.Background()
	infos := ListTools(ctx)

	if infos != nil {
		t.Errorf("ListTools() from empty context = %v, want nil", infos)
	}
}

func TestRegistryWithIOError(t *testing.T) {
	tool := Simple("test", "Test tool description", func(s string) string { return s })
	registry := Registry(tool)

	errorReader := &errorReader{err: io.ErrUnexpectedEOF}
	var buf bytes.Buffer

	err := registry.ServeFlow(context.Background(), errorReader, &buf)
	if err != io.ErrUnexpectedEOF {
		t.Errorf("Registry() with IO error = %v, want %v", err, io.ErrUnexpectedEOF)
	}
}
