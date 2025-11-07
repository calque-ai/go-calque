package mcp

import (
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestValidateResourceSelection(t *testing.T) {
	t.Parallel()

	// Create test resources
	resources := []*mcp.Resource{
		{
			URI:  "file:///docs/api.md",
			Name: "API Documentation",
		},
		{
			URI:  "file:///config/settings.json",
			Name: "Settings",
		},
		{
			URI:  "file:///data/users.db",
			Name: "User Database",
		},
	}

	tests := []struct {
		name             string
		selectedResource string
		resources        []*mcp.Resource
		expected         string
	}{
		{
			name:             "exact URI match",
			selectedResource: "file:///docs/api.md",
			resources:        resources,
			expected:         "file:///docs/api.md",
		},
		{
			name:             "case insensitive URI match",
			selectedResource: "FILE:///DOCS/API.MD",
			resources:        resources,
			expected:         "file:///docs/api.md",
		},
		{
			name:             "name match",
			selectedResource: "API Documentation",
			resources:        resources,
			expected:         "file:///docs/api.md",
		},
		{
			name:             "case insensitive name match",
			selectedResource: "settings",
			resources:        resources,
			expected:         "file:///config/settings.json",
		},
		{
			name:             "URI prefix match",
			selectedResource: "file:///config",
			resources:        resources,
			expected:         "file:///config/settings.json",
		},
		{
			name:             "no match",
			selectedResource: "file:///nonexistent.txt",
			resources:        resources,
			expected:         "",
		},
		{
			name:             "empty input",
			selectedResource: "",
			resources:        resources,
			expected:         "",
		},
		{
			name:             "empty resources",
			selectedResource: "file:///docs/api.md",
			resources:        []*mcp.Resource{},
			expected:         "",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := validateResourceSelection(tt.selectedResource, tt.resources)
			if result != tt.expected {
				t.Errorf("validateResourceSelection(%s) = %s, expected %s", tt.selectedResource, result, tt.expected)
			}
		})
	}
}

func TestValidatePromptSelection(t *testing.T) {
	t.Parallel()

	// Create test prompts
	prompts := []*mcp.Prompt{
		{
			Name: "blog_writer",
		},
		{
			Name: "code_review",
		},
		{
			Name: "summarizer",
		},
	}

	tests := []struct {
		name           string
		selectedPrompt string
		prompts        []*mcp.Prompt
		expected       string
	}{
		{
			name:           "exact match",
			selectedPrompt: "blog_writer",
			prompts:        prompts,
			expected:       "blog_writer",
		},
		{
			name:           "case insensitive match",
			selectedPrompt: "BLOG_WRITER",
			prompts:        prompts,
			expected:       "blog_writer",
		},
		{
			name:           "case insensitive match mixed case",
			selectedPrompt: "Code_Review",
			prompts:        prompts,
			expected:       "code_review",
		},
		{
			name:           "prefix match",
			selectedPrompt: "blog",
			prompts:        prompts,
			expected:       "blog_writer",
		},
		{
			name:           "prefix match partial",
			selectedPrompt: "summ",
			prompts:        prompts,
			expected:       "summarizer",
		},
		{
			name:           "no match",
			selectedPrompt: "nonexistent",
			prompts:        prompts,
			expected:       "",
		},
		{
			name:           "empty input",
			selectedPrompt: "",
			prompts:        prompts,
			expected:       "",
		},
		{
			name:           "empty prompts",
			selectedPrompt: "blog_writer",
			prompts:        []*mcp.Prompt{},
			expected:       "",
		},
		{
			name:           "prefix match - returns first",
			selectedPrompt: "c", // matches "code_review" (starts with 'c')
			prompts:        prompts,
			expected:       "code_review", // First prefix match
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := validatePromptSelection(tt.selectedPrompt, tt.prompts)
			if result != tt.expected {
				t.Errorf("validatePromptSelection(%s) = %s, expected %s", tt.selectedPrompt, result, tt.expected)
			}
		})
	}
}

func TestGetResourceSelectionTemplateData(t *testing.T) {
	t.Parallel()

	resources := []*mcp.Resource{
		{
			URI:         "file:///docs/api.md",
			Name:        "API Documentation",
			Description: "Complete API docs",
		},
		{
			URI:         "file:///config/settings.json",
			Name:        "Settings",
			Description: "App settings",
		},
	}

	tests := []struct {
		name          string
		userInput     string
		resources     []*mcp.Resource
		checkFields   bool
		expectedInput string
		expectedCount int
	}{
		{
			name:          "normal input with resources",
			userInput:     "show me the API docs",
			resources:     resources,
			checkFields:   true,
			expectedInput: "show me the API docs",
			expectedCount: 2,
		},
		{
			name:          "empty input",
			userInput:     "",
			resources:     resources,
			checkFields:   true,
			expectedInput: "",
			expectedCount: 2,
		},
		{
			name:          "empty resources",
			userInput:     "test",
			resources:     []*mcp.Resource{},
			checkFields:   true,
			expectedInput: "test",
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := getResourceSelectionTemplateData(tt.userInput, tt.resources)

			if !tt.checkFields {
				return
			}

			input, ok := result["Input"].(string)
			if !ok {
				t.Error("Input field not found or wrong type")
			} else if input != tt.expectedInput {
				t.Errorf("Input = %q, expected %q", input, tt.expectedInput)
			}

			resources, ok := result["Resources"].([]ResourceTemplateData)
			if !ok {
				t.Error("Resources field not found or wrong type")
			} else if len(resources) != tt.expectedCount {
				t.Errorf("Resources count = %d, expected %d", len(resources), tt.expectedCount)
			}
		})
	}
}

func TestGetPromptSelectionTemplateData(t *testing.T) {
	t.Parallel()

	prompts := []*mcp.Prompt{
		{
			Name:        "blog_writer",
			Description: "Write blog posts",
		},
		{
			Name:        "code_review",
			Description: "Review code",
		},
	}

	tests := []struct {
		name          string
		userInput     string
		prompts       []*mcp.Prompt
		checkFields   bool
		expectedInput string
		expectedCount int
	}{
		{
			name:          "normal input with prompts",
			userInput:     "help me write a blog",
			prompts:       prompts,
			checkFields:   true,
			expectedInput: "help me write a blog",
			expectedCount: 2,
		},
		{
			name:          "empty input",
			userInput:     "",
			prompts:       prompts,
			checkFields:   true,
			expectedInput: "",
			expectedCount: 2,
		},
		{
			name:          "empty prompts",
			userInput:     "test",
			prompts:       []*mcp.Prompt{},
			checkFields:   true,
			expectedInput: "test",
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := getPromptSelectionTemplateData(tt.userInput, tt.prompts)

			if !tt.checkFields {
				return
			}

			input, ok := result["Input"].(string)
			if !ok {
				t.Error("Input field not found or wrong type")
			} else if input != tt.expectedInput {
				t.Errorf("Input = %q, expected %q", input, tt.expectedInput)
			}

			prompts, ok := result["Prompts"].([]PromptTemplateData)
			if !ok {
				t.Error("Prompts field not found or wrong type")
			} else if len(prompts) != tt.expectedCount {
				t.Errorf("Prompts count = %d, expected %d", len(prompts), tt.expectedCount)
			}
		})
	}
}
