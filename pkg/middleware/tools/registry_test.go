package tools

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/calque-ai/calque-pipe/pkg/core"
)

func TestRegistry(t *testing.T) {
	calc := Simple("calculator", "Calculate something", func(s string) string { return s })
	search := Simple("search", "Search the web", func(s string) string { return s })
	weather := Simple("weather", "Get weather", func(s string) string { return s })

	tests := []struct {
		name     string
		tools    []Tool
		input    string
		expected string
	}{
		{
			name:     "registry with multiple tools",
			tools:    []Tool{calc, search, weather},
			input:    "test input",
			expected: "test input",
		},
		{
			name:     "registry with single tool",
			tools:    []Tool{calc},
			input:    "single tool test",
			expected: "single tool test",
		},
		{
			name:     "registry with empty input",
			tools:    []Tool{calc, search},
			input:    "",
			expected: "",
		},
		{
			name:     "registry with large input",
			tools:    []Tool{calc},
			input:    strings.Repeat("large input ", 100),
			expected: strings.Repeat("large input ", 100),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test that Registry creates a working handler
			registry := Registry(tt.tools...)

			var buf bytes.Buffer
			reader := strings.NewReader(tt.input)
			req := core.NewRequest(context.Background(), reader)
			res := core.NewResponse(&buf)
			err := registry.ServeFlow(req, res)
			if err != nil {
				t.Errorf("Registry.ServeFlow() error = %v", err)
				return
			}

			if got := buf.String(); got != tt.expected {
				t.Errorf("Registry() output = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestRegistryStreamIntegrity(t *testing.T) {
	calc := Simple("calculator", "Math calculator", func(s string) string { return s })
	registry := Registry(calc)

	// Test that streaming data passes through correctly
	input := "streaming test data"
	var buf bytes.Buffer
	reader := strings.NewReader(input)

	req := core.NewRequest(context.Background(), reader)
	res := core.NewResponse(&buf)
	err := registry.ServeFlow(req, res)
	if err != nil {
		t.Errorf("Registry streaming error = %v", err)
		return
	}

	if got := buf.String(); got != input {
		t.Errorf("Registry streaming integrity check failed: got %q, want %q", got, input)
	}
}

func TestGetTools(t *testing.T) {
	calc := Simple("calculator", "Math calculator", func(s string) string { return s })
	search := Simple("search", "Web search", func(s string) string { return s })
	tools := []Tool{calc, search}

	// Test GetTools with tools in context
	ctx := context.WithValue(context.Background(), toolsContextKey{}, tools)
	retrieved := GetTools(ctx)

	if len(retrieved) != len(tools) {
		t.Errorf("GetTools() returned %d tools, want %d", len(retrieved), len(tools))
		return
	}

	for i, tool := range retrieved {
		if tool.Name() != tools[i].Name() {
			t.Errorf("GetTools()[%d].Name() = %q, want %q", i, tool.Name(), tools[i].Name())
		}
		if tool.Description() != tools[i].Description() {
			t.Errorf("GetTools()[%d].Description() = %q, want %q", i, tool.Description(), tools[i].Description())
		}
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
	calc := Simple("calculator", "Math calculator", func(s string) string { return s })
	search := Simple("search", "Web search", func(s string) string { return s })
	tools := []Tool{calc, search}

	ctx := context.WithValue(context.Background(), toolsContextKey{}, tools)

	// Test finding existing tool
	found := GetTool(ctx, "calculator")
	if found == nil {
		t.Error("GetTool() could not find 'calculator'")
		return
	}
	if found.Name() != "calculator" {
		t.Errorf("GetTool() returned tool with name %q, want 'calculator'", found.Name())
	}

	// Test finding non-existent tool
	notFound := GetTool(ctx, "nonexistent")
	if notFound != nil {
		t.Errorf("GetTool() found non-existent tool: %v", notFound)
	}
}

func TestGetToolFromEmptyContext(t *testing.T) {
	ctx := context.Background()
	tool := GetTool(ctx, "any")

	if tool != nil {
		t.Errorf("GetTool() from empty context = %v, want nil", tool)
	}
}

func TestHasTool(t *testing.T) {
	calc := Simple("calculator", "Math calculator", func(s string) string { return s })
	search := Simple("search", "Web search", func(s string) string { return s })
	tools := []Tool{calc, search}

	ctx := context.WithValue(context.Background(), toolsContextKey{}, tools)

	// Test existing tool
	if !HasTool(ctx, "calculator") {
		t.Error("HasTool() returned false for existing tool 'calculator'")
	}

	// Test non-existing tool
	if HasTool(ctx, "nonexistent") {
		t.Error("HasTool() returned true for non-existent tool")
	}
}

func TestHasToolFromEmptyContext(t *testing.T) {
	ctx := context.Background()

	if HasTool(ctx, "any") {
		t.Error("HasTool() from empty context should return false")
	}
}

func TestListToolNames(t *testing.T) {
	calc := Simple("calculator", "Math calculator", func(s string) string { return s })
	search := Simple("search", "Web search", func(s string) string { return s })
	tools := []Tool{calc, search}

	ctx := context.WithValue(context.Background(), toolsContextKey{}, tools)
	names := ListToolNames(ctx)

	expected := []string{"calculator", "search"}
	if len(names) != len(expected) {
		t.Errorf("ListToolNames() returned %d names, want %d", len(names), len(expected))
		return
	}

	for i, name := range names {
		if name != expected[i] {
			t.Errorf("ListToolNames()[%d] = %q, want %q", i, name, expected[i])
		}
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
	calc := Simple("calculator", "Math calculator", func(s string) string { return s })
	search := Simple("search", "Web search", func(s string) string { return s })
	tools := []Tool{calc, search}

	ctx := context.WithValue(context.Background(), toolsContextKey{}, tools)
	infos := ListTools(ctx)

	if len(infos) != len(tools) {
		t.Errorf("ListTools() returned %d infos, want %d", len(infos), len(tools))
		return
	}

	for i, info := range infos {
		if info.Name != tools[i].Name() {
			t.Errorf("ListTools()[%d].Name = %q, want %q", i, info.Name, tools[i].Name())
		}
		if info.Description != tools[i].Description() {
			t.Errorf("ListTools()[%d].Description = %q, want %q", i, info.Description, tools[i].Description())
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
	calc := Simple("calculator", "Math calculator", func(s string) string { return s })
	registry := Registry(calc)

	errorReader := &errorReader{err: io.ErrUnexpectedEOF}
	var buf bytes.Buffer

	req := core.NewRequest(context.Background(), errorReader)
	res := core.NewResponse(&buf)
	err := registry.ServeFlow(req, res)
	if err != io.ErrUnexpectedEOF {
		t.Errorf("Registry() with IO error = %v, want %v", err, io.ErrUnexpectedEOF)
	}
}
