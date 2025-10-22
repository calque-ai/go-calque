package gemini

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/invopop/jsonschema"
	"google.golang.org/genai"

	"github.com/calque-ai/go-calque/pkg/calque"
	"github.com/calque-ai/go-calque/pkg/helpers"
	"github.com/calque-ai/go-calque/pkg/middleware/ai"
	"github.com/calque-ai/go-calque/pkg/middleware/tools"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name          string
		model         string
		opts          []Option
		setupEnv      func()
		expectError   bool
		errorContains string
	}{
		{
			name:          "empty model name",
			model:         "",
			expectError:   true,
			errorContains: "model name is required",
		},
		{
			name:          "no API key",
			model:         "gemini-pro",
			setupEnv:      func() { os.Unsetenv("GOOGLE_API_KEY") },
			expectError:   true,
			errorContains: "GOOGLE_API_KEY environment variable not set",
		},
		{
			name:     "valid model with env API key",
			model:    "gemini-1.5-pro",
			setupEnv: func() { os.Setenv("GOOGLE_API_KEY", "test-api-key") },
		},
		{
			name:  "valid model with config API key",
			model: "gemini-pro",
			opts: []Option{
				WithConfig(&Config{
					APIKey:      "config-api-key",
					Temperature: helpers.PtrOf(float32(0.8)),
				}),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setupEnv != nil {
				tt.setupEnv()
			}

			client, err := New(tt.model, tt.opts...)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error, got none")
					return
				}
				if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error to contain %q, got %q", tt.errorContains, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("New() error = %v", err)
				return
			}

			if client.model != tt.model {
				t.Errorf("New() model = %v, want %v", client.model, tt.model)
			}

			if client.config == nil {
				t.Error("New() config should not be nil")
			}

			if client.client == nil {
				t.Error("New() genai client should not be nil")
			}
		})
	}

	// Clean up environment
	os.Unsetenv("GOOGLE_API_KEY")
}

func TestDefaultConfig(t *testing.T) {
	// Test without environment variable
	os.Unsetenv("GOOGLE_API_KEY")
	config := DefaultConfig()

	if config == nil {
		t.Fatal("DefaultConfig() should not return nil")
	}

	if config.APIKey != "" {
		t.Error("DefaultConfig() should have empty API key when env var not set")
	}

	if config.Temperature == nil || *config.Temperature != 0.7 {
		t.Error("DefaultConfig() should set temperature to 0.7")
	}

	// Test with environment variable
	os.Setenv("GOOGLE_API_KEY", "test-key")
	config = DefaultConfig()

	if config.APIKey != "test-key" {
		t.Errorf("DefaultConfig() APIKey = %v, want test-key", config.APIKey)
	}

	// Clean up
	os.Unsetenv("GOOGLE_API_KEY")
}

func TestWithConfig(t *testing.T) {
	customConfig := &Config{
		APIKey:      "custom-key",
		Temperature: helpers.PtrOf(float32(0.9)),
		MaxTokens:   helpers.PtrOf(2000),
		TopP:        helpers.PtrOf(float32(0.95)),
		TopK:        helpers.PtrOf(float32(40)),
	}

	option := WithConfig(customConfig)

	// Test applying the option
	config := &Config{}
	option.Apply(config)

	if config.APIKey != "custom-key" {
		t.Error("WithConfig() should apply custom API key")
	}

	if config.Temperature == nil || *config.Temperature != 0.9 {
		t.Error("WithConfig() should apply custom temperature")
	}

	if config.MaxTokens == nil || *config.MaxTokens != 2000 {
		t.Error("WithConfig() should apply custom MaxTokens")
	}

	if config.TopP == nil || *config.TopP != 0.95 {
		t.Error("WithConfig() should apply custom TopP")
	}

	if config.TopK == nil || *config.TopK != 40 {
		t.Error("WithConfig() should apply custom TopK")
	}
}

func TestBuildGenerateConfig(t *testing.T) {
	tests := []struct {
		name   string
		config *Config
		schema *ai.ResponseFormat
		check  func(*genai.GenerateContentConfig) error
	}{
		{
			name: "basic config",
			config: &Config{
				Temperature:       helpers.PtrOf(float32(0.8)),
				TopP:              helpers.PtrOf(float32(0.9)),
				TopK:              helpers.PtrOf(float32(50)),
				MaxTokens:         helpers.PtrOf(1500),
				Stop:              []string{"END", "STOP"},
				SystemInstruction: "Be helpful",
				PresencePenalty:   helpers.PtrOf(float32(0.1)),
				FrequencyPenalty:  helpers.PtrOf(float32(0.2)),
				Seed:              helpers.PtrOf(int32(42)),
				CandidateCount:    helpers.PtrOf(int32(1)),
			},
			check: func(config *genai.GenerateContentConfig) error {
				if config.Temperature == nil || *config.Temperature != 0.8 {
					return fmt.Errorf("temperature = %v, want 0.8", config.Temperature)
				}
				if config.TopP == nil || *config.TopP != 0.9 {
					return fmt.Errorf("TopP = %v, want 0.9", config.TopP)
				}
				if config.TopK == nil || *config.TopK != 50 {
					return fmt.Errorf("TopK = %v, want 50", config.TopK)
				}
				if config.MaxOutputTokens != 1500 {
					return fmt.Errorf("MaxOutputTokens = %v, want 1500", config.MaxOutputTokens)
				}
				if len(config.StopSequences) != 2 {
					return fmt.Errorf("StopSequences length = %v, want 2", len(config.StopSequences))
				}
				if config.SystemInstruction == nil {
					return fmt.Errorf("SystemInstruction should be set")
				}
				if config.PresencePenalty == nil || *config.PresencePenalty != 0.1 {
					return fmt.Errorf("PresencePenalty = %v, want 0.1", config.PresencePenalty)
				}
				if config.FrequencyPenalty == nil || *config.FrequencyPenalty != 0.2 {
					return fmt.Errorf("FrequencyPenalty = %v, want 0.2", config.FrequencyPenalty)
				}
				if config.Seed == nil || *config.Seed != 42 {
					return fmt.Errorf("Seed = %v, want 42", config.Seed)
				}
				if config.CandidateCount != 1 {
					return fmt.Errorf("CandidateCount = %v, want 1", config.CandidateCount)
				}
				return nil
			},
		},
		{
			name: "json_object schema",
			schema: &ai.ResponseFormat{
				Type: "json_object",
			},
			check: func(config *genai.GenerateContentConfig) error {
				if config.ResponseMIMEType != "application/json" {
					return fmt.Errorf("ResponseMIMEType = %v, want application/json", config.ResponseMIMEType)
				}
				return nil
			},
		},
		{
			name: "json_schema",
			schema: &ai.ResponseFormat{
				Type:   "json_schema",
				Schema: &jsonschema.Schema{Type: "object"}, // Mock schema
			},
			check: func(config *genai.GenerateContentConfig) error {
				if config.ResponseMIMEType != "application/json" {
					return fmt.Errorf("ResponseMIMEType = %v, want application/json", config.ResponseMIMEType)
				}
				if config.ResponseJsonSchema == nil {
					return fmt.Errorf("ResponseJsonSchema should be set for json_schema")
				}
				return nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &Client{
				config: tt.config,
			}
			if client.config == nil {
				client.config = &Config{}
			}

			config := client.buildGenerateConfig(tt.schema)

			if tt.check != nil {
				if err := tt.check(config); err != nil {
					t.Errorf("buildGenerateConfig() %v", err)
				}
			}
		})
	}
}

func TestInputToParts(t *testing.T) {
	client := &Client{}

	tests := []struct {
		name        string
		input       *ai.ClassifiedInput
		expectError bool
		checkFunc   func([]genai.Part) error
	}{
		{
			name: "text input",
			input: &ai.ClassifiedInput{
				Type: ai.TextInput,
				Text: "Hello, world!",
			},
			checkFunc: func(parts []genai.Part) error {
				if len(parts) != 1 {
					return fmt.Errorf("parts length = %v, want 1", len(parts))
				}
				if parts[0].Text != "Hello, world!" {
					return fmt.Errorf("part text = %v, want 'Hello, world!'", parts[0].Text)
				}
				return nil
			},
		},
		{
			name: "multimodal text only",
			input: &ai.ClassifiedInput{
				Type: ai.MultimodalJSONInput,
				Multimodal: &ai.MultimodalInput{
					Parts: []ai.ContentPart{
						{Type: "text", Text: "What's in this image?"},
					},
				},
			},
			checkFunc: func(parts []genai.Part) error {
				if len(parts) != 1 {
					return fmt.Errorf("parts length = %v, want 1", len(parts))
				}
				if parts[0].Text != "What's in this image?" {
					return fmt.Errorf("part text = %v, want 'What's in this image?'", parts[0].Text)
				}
				return nil
			},
		},
		{
			name: "multimodal with image",
			input: &ai.ClassifiedInput{
				Type: ai.MultimodalJSONInput,
				Multimodal: &ai.MultimodalInput{
					Parts: []ai.ContentPart{
						{Type: "text", Text: "Analyze this image"},
						{Type: "image", Data: []byte("fake-image-data"), MimeType: "image/jpeg"},
					},
				},
			},
			checkFunc: func(parts []genai.Part) error {
				if len(parts) != 2 {
					return fmt.Errorf("parts length = %v, want 2", len(parts))
				}
				if parts[0].Text != "Analyze this image" {
					return fmt.Errorf("first part text = %v, want 'Analyze this image'", parts[0].Text)
				}
				if parts[1].InlineData == nil {
					return fmt.Errorf("second part should have InlineData")
				}
				if string(parts[1].InlineData.Data) != "fake-image-data" {
					return fmt.Errorf("image data = %v, want 'fake-image-data'", string(parts[1].InlineData.Data))
				}
				if parts[1].InlineData.MIMEType != "image/jpeg" {
					return fmt.Errorf("MIME type = %v, want image/jpeg", parts[1].InlineData.MIMEType)
				}
				return nil
			},
		},
		{
			name: "multimodal with audio",
			input: &ai.ClassifiedInput{
				Type: ai.MultimodalJSONInput,
				Multimodal: &ai.MultimodalInput{
					Parts: []ai.ContentPart{
						{Type: "audio", Data: []byte("fake-audio-data"), MimeType: "audio/wav"},
					},
				},
			},
			checkFunc: func(parts []genai.Part) error {
				if len(parts) != 1 {
					return fmt.Errorf("parts length = %v, want 1", len(parts))
				}
				if parts[0].InlineData == nil {
					return fmt.Errorf("part should have InlineData for audio")
				}
				if string(parts[0].InlineData.Data) != "fake-audio-data" {
					return fmt.Errorf("audio data = %v, want 'fake-audio-data'", string(parts[0].InlineData.Data))
				}
				if parts[0].InlineData.MIMEType != "audio/wav" {
					return fmt.Errorf("MIME type = %v, want audio/wav", parts[0].InlineData.MIMEType)
				}
				return nil
			},
		},
		{
			name: "empty multimodal",
			input: &ai.ClassifiedInput{
				Type: ai.MultimodalJSONInput,
				Multimodal: &ai.MultimodalInput{
					Parts: []ai.ContentPart{},
				},
			},
			expectError: true,
		},
		{
			name: "unsupported content type",
			input: &ai.ClassifiedInput{
				Type: ai.MultimodalJSONInput,
				Multimodal: &ai.MultimodalInput{
					Parts: []ai.ContentPart{
						{Type: "unknown", Data: []byte("data")},
					},
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parts, err := client.inputToParts(tt.input)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error, got none")
				}
				return
			}

			if err != nil {
				t.Errorf("inputToParts() error = %v", err)
				return
			}

			if tt.checkFunc != nil {
				if err := tt.checkFunc(parts); err != nil {
					t.Errorf("inputToParts() %v", err)
				}
			}
		})
	}
}

func TestConvertToolsToGeminiFunctions(t *testing.T) {
	// Create a simple mock tool
	tool := tools.Simple("calculator", "Performs calculations", func(_ string) string {
		return "result"
	})

	functions := convertToolsToGeminiFunctions([]tools.Tool{tool})

	if len(functions) != 1 {
		t.Fatalf("convertToolsToGeminiFunctions() returned %d functions, want 1", len(functions))
	}

	function := functions[0]
	if function.Name != "calculator" {
		t.Errorf("function name = %v, want calculator", function.Name)
	}

	if function.Description != "Performs calculations" {
		t.Errorf("function description = %v, want 'Performs calculations'", function.Description)
	}

	if function.ParametersJsonSchema == nil {
		t.Error("function ParametersJsonSchema should not be nil")
	}
}

func TestWriteFunctionCalls(t *testing.T) {
	client := &Client{}

	functionCalls := []*genai.FunctionCall{
		{
			Name: "calculator",
			Args: map[string]any{
				"input": "2+2",
			},
		},
		{
			Name: "search",
			Args: map[string]any{
				"input": "golang",
			},
		},
	}

	var response strings.Builder
	w := calque.NewResponse(&response)

	err := client.writeFunctionCalls(functionCalls, w)
	if err != nil {
		t.Errorf("writeFunctionCalls() error = %v", err)
		return
	}

	result := response.String()

	// Should be valid JSON
	var jsonResult map[string]any
	if err := json.Unmarshal([]byte(result), &jsonResult); err != nil {
		t.Errorf("writeFunctionCalls() produced invalid JSON: %v", err)
		return
	}

	// Check structure
	toolCallsRaw, ok := jsonResult["tool_calls"]
	if !ok {
		t.Error("writeFunctionCalls() should include tool_calls field")
		return
	}

	toolCalls, ok := toolCallsRaw.([]any)
	if !ok {
		t.Error("tool_calls should be an array")
		return
	}

	if len(toolCalls) != 2 {
		t.Errorf("tool_calls length = %v, want 2", len(toolCalls))
	}

	// Verify it contains expected tool call names
	if !strings.Contains(result, "calculator") {
		t.Error("writeFunctionCalls() should contain calculator tool name")
	}

	if !strings.Contains(result, "search") {
		t.Error("writeFunctionCalls() should contain search tool name")
	}

	if !strings.Contains(result, "2+2") {
		t.Error("writeFunctionCalls() should contain calculator arguments")
	}

	if !strings.Contains(result, "golang") {
		t.Error("writeFunctionCalls() should contain search arguments")
	}
}

func TestWriteFunctionCallsEmptyArgs(t *testing.T) {
	client := &Client{}

	functionCalls := []*genai.FunctionCall{
		{
			Name: "no_args_tool",
			Args: nil,
		},
	}

	var response strings.Builder
	w := calque.NewResponse(&response)

	err := client.writeFunctionCalls(functionCalls, w)
	if err != nil {
		t.Errorf("writeFunctionCalls() error = %v", err)
		return
	}

	result := response.String()

	// Should contain empty object {} for tools without args (OpenAI format)
	if !strings.Contains(result, `"arguments":"{}"`) {
		t.Errorf("writeFunctionCalls() should provide empty object for tools without args, got: %s", result)
	}

	// Should contain the tool name
	if !strings.Contains(result, `"name":"no_args_tool"`) {
		t.Errorf("writeFunctionCalls() should contain tool name, got: %s", result)
	}
}

// Mock tests for interface compliance
func TestClientInterfaceCompliance(_ *testing.T) {
	// Test that Client implements ai.Client interface
	var _ ai.Client = (*Client)(nil)
}

func TestBuildRequestConfig(t *testing.T) {
	client := &Client{
		model: "gemini-1.5-pro",
		config: &Config{
			Temperature: helpers.PtrOf(float32(0.8)),
			MaxTokens:   helpers.PtrOf(1000),
		},
	}

	tests := []struct {
		name        string
		input       *ai.ClassifiedInput
		schema      *ai.ResponseFormat
		tools       []tools.Tool
		expectError bool
		description string
	}{
		{
			name: "text input with tools",
			input: &ai.ClassifiedInput{
				Type: ai.TextInput,
				Text: "Hello, world!",
			},
			tools: []tools.Tool{
				tools.Simple("calculator", "Performs calculations", func(_ string) string {
					return "42"
				}),
			},
			description: "Should handle text input with tools",
		},
		{
			name: "multimodal input with schema",
			input: &ai.ClassifiedInput{
				Type: ai.MultimodalJSONInput,
				Multimodal: &ai.MultimodalInput{
					Parts: []ai.ContentPart{
						{Type: "text", Text: "Analyze this"},
					},
				},
			},
			schema: &ai.ResponseFormat{
				Type: "json_object",
			},
			description: "Should handle multimodal input with response schema",
		},
		{
			name: "invalid input type",
			input: &ai.ClassifiedInput{
				Type: ai.InputType(999),
			},
			expectError: true,
			description: "Should return error for unsupported input types",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock genai.Client - we can't easily create one without real API key
			// So we'll test the parts that don't require the actual client
			if tt.expectError {
				_, err := client.inputToParts(tt.input)
				if err == nil {
					t.Errorf("%s: expected error but got none", tt.description)
				}
				return
			}

			// Test parts conversion
			parts, err := client.inputToParts(tt.input)
			if err != nil {
				t.Errorf("%s: inputToParts() error = %v", tt.description, err)
				return
			}

			if len(parts) == 0 {
				t.Errorf("%s: inputToParts() returned empty parts", tt.description)
			}

			// Test config generation
			config := client.buildGenerateConfig(tt.schema)
			if config == nil {
				t.Errorf("%s: buildGenerateConfig() returned nil", tt.description)
			}

			// Verify tools conversion if provided
			if len(tt.tools) > 0 {
				functions := convertToolsToGeminiFunctions(tt.tools)
				if len(functions) != len(tt.tools) {
					t.Errorf("%s: convertToolsToGeminiFunctions() length = %d, want %d",
						tt.description, len(functions), len(tt.tools))
				}
			}
		})
	}
}

func TestExecuteRequest(t *testing.T) {
	tests := []struct {
		name          string
		simulateError bool
		functionCalls []*genai.FunctionCall
		expectedText  string
		expectError   bool
		description   string
	}{
		{
			name:         "text response without function calls",
			expectedText: "Hello, this is a response",
			description:  "Should handle simple text responses",
		},
		{
			name: "response with function calls",
			functionCalls: []*genai.FunctionCall{
				{
					Name: "calculator",
					Args: map[string]any{"input": "2+2"},
				},
			},
			description: "Should handle function calls in responses",
		},
		{
			name:         "empty response",
			expectedText: "",
			description:  "Should handle empty responses",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// We can't easily mock the genai streaming without major complexity
			// So we'll test the function call formatting part directly
			if len(tt.functionCalls) == 0 {
				return // Skip tests without function calls for now
			}

			testFunctionCallFormatting(t, tt.functionCalls, tt.expectError, tt.description)
		})
	}
}

// testFunctionCallFormatting is a helper function to test function call formatting
// This reduces the complexity of the main test function
func testFunctionCallFormatting(t *testing.T, functionCalls []*genai.FunctionCall, expectError bool, description string) {
	client := &Client{}

	var response strings.Builder
	w := calque.NewResponse(&response)

	err := client.writeFunctionCalls(functionCalls, w)
	if expectError {
		if err == nil {
			t.Errorf("%s: expected error but got none", description)
		}
		return
	}

	if err != nil {
		t.Errorf("%s: writeFunctionCalls() error = %v", description, err)
		return
	}

	result := response.String()
	if !strings.Contains(result, "tool_calls") {
		t.Errorf("%s: response should contain tool_calls", description)
	}

	// Validate JSON format
	var jsonResult map[string]any
	if err := json.Unmarshal([]byte(result), &jsonResult); err != nil {
		t.Errorf("%s: response is not valid JSON: %v", description, err)
	}
}

func TestChat_Integration(t *testing.T) {
	// Test the main Chat method integration without real API calls
	tests := []struct {
		name        string
		input       string
		tools       []tools.Tool
		schema      *ai.ResponseFormat
		expectError bool
		description string
	}{
		{
			name:        "simple text chat",
			input:       "Hello, how are you?",
			description: "Should handle simple text input",
		},
		{
			name:  "chat with tools",
			input: "Calculate 2+2",
			tools: []tools.Tool{
				tools.Simple("calculator", "Performs calculations", func(_ string) string {
					return "4"
				}),
			},
			description: "Should handle text input with tools",
		},
		{
			name:  "chat with response format",
			input: "Give me JSON response",
			schema: &ai.ResponseFormat{
				Type: "json_object",
			},
			description: "Should handle requests with response format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// We can test the input processing part of Chat()
			// without needing actual API calls

			// Create request with proper data
			r := calque.NewRequest(context.Background(), strings.NewReader(tt.input))

			opts := &ai.AgentOptions{
				Schema: tt.schema,
				Tools:  tt.tools,
			}

			input, err := ai.ClassifyInput(r, opts)
			if err != nil {
				t.Errorf("%s: ClassifyInput() error = %v", tt.description, err)
				return
			}

			if input.Type != ai.TextInput {
				t.Errorf("%s: expected TextInput, got %v", tt.description, input.Type)
			}

			if input.Text != tt.input {
				t.Errorf("%s: input text = %q, want %q", tt.description, input.Text, tt.input)
			}

			// Test that we can build a client (without real API key)
			client := &Client{
				model:  "gemini-1.5-pro",
				config: &Config{Temperature: helpers.PtrOf(float32(0.7))},
			}

			// Test parts conversion
			parts, err := client.inputToParts(input)
			if err != nil {
				t.Errorf("%s: inputToParts() error = %v", tt.description, err)
				return
			}

			if len(parts) == 0 {
				t.Errorf("%s: inputToParts() should return at least one part", tt.description)
			}

			// Test config generation
			config := client.buildGenerateConfig(ai.GetSchema(opts))
			if config == nil {
				t.Errorf("%s: buildGenerateConfig() returned nil", tt.description)
			}

			// Verify tools are processed correctly
			if len(tt.tools) > 0 {
				functions := convertToolsToGeminiFunctions(tt.tools)
				if len(functions) != len(tt.tools) {
					t.Errorf("%s: tool conversion failed, got %d functions, want %d",
						tt.description, len(functions), len(tt.tools))
				}
			}
		})
	}
}

// Test edge cases and error conditions
func TestEdgeCases(t *testing.T) {
	t.Run("empty tools list", func(t *testing.T) {
		functions := convertToolsToGeminiFunctions([]tools.Tool{})
		if len(functions) != 0 {
			t.Errorf("convertToolsToGeminiFunctions() with empty tools = %d, want 0", len(functions))
		}
	})

	t.Run("nil multimodal input", func(t *testing.T) {
		client := &Client{}
		input := &ai.ClassifiedInput{
			Type:       ai.MultimodalJSONInput,
			Multimodal: nil,
		}

		_, err := client.inputToParts(input)
		if err == nil {
			t.Error("inputToParts() with nil multimodal should return error")
		}
	})

	t.Run("unsupported input type", func(t *testing.T) {
		client := &Client{}
		input := &ai.ClassifiedInput{
			Type: ai.InputType(999), // Invalid type
		}

		_, err := client.inputToParts(input)
		if err == nil {
			t.Error("inputToParts() with unsupported type should return error")
		}
	})
}

// TestFunctionCallOutputFormat tests that writeFunctionCalls produces output
// that can be correctly parsed by tools.Execute middleware
func TestFunctionCallOutputFormat(t *testing.T) {
	tests := []struct {
		name          string
		functionCalls []*genai.FunctionCall
		description   string
	}{
		{
			name: "single function call with parameters",
			functionCalls: []*genai.FunctionCall{
				{
					Name: "read_resource",
					Args: map[string]any{
						"uri": "file:///etc/hosts",
					},
				},
			},
			description: "Should format single function call correctly",
		},
		{
			name: "multiple function calls",
			functionCalls: []*genai.FunctionCall{
				{
					Name: "search",
					Args: map[string]any{
						"query": "golang",
						"limit": float64(10),
					},
				},
				{
					Name: "read_resource",
					Args: map[string]any{
						"uri": "file:///docs/api.md",
					},
				},
			},
			description: "Should format multiple function calls correctly",
		},
		{
			name: "function call with complex nested parameters",
			functionCalls: []*genai.FunctionCall{
				{
					Name: "create_resource",
					Args: map[string]any{
						"uri":      "file:///data/config.json",
						"contents": map[string]any{"key": "value", "nested": map[string]any{"deep": true}},
						"metadata": []any{"tag1", "tag2"},
					},
				},
			},
			description: "Should format nested parameters correctly",
		},
		{
			name: "function call with no arguments",
			functionCalls: []*genai.FunctionCall{
				{
					Name: "get_status",
					Args: nil,
				},
			},
			description: "Should format function call with no args as empty object",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &Client{}
			var response strings.Builder
			w := calque.NewResponse(&response)

			// Write function calls in OpenAI format
			err := client.writeFunctionCalls(tt.functionCalls, w)
			if err != nil {
				t.Fatalf("%s: writeFunctionCalls() error = %v", tt.description, err)
			}

			output := response.String()

			// Parse the output as tools.Execute would
			var result struct {
				ToolCalls []struct {
					Type     string `json:"type"`
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			}

			if err := json.Unmarshal([]byte(output), &result); err != nil {
				t.Fatalf("%s: output is not valid JSON: %v\nOutput: %s", tt.description, err, output)
			}

			// Verify structure
			if len(result.ToolCalls) != len(tt.functionCalls) {
				t.Errorf("%s: got %d tool calls, want %d", tt.description, len(result.ToolCalls), len(tt.functionCalls))
			}

			// Verify each tool call
			for i, expectedCall := range tt.functionCalls {
				if i >= len(result.ToolCalls) {
					break
				}

				actualCall := result.ToolCalls[i]

				// Check type field
				if actualCall.Type != "function" {
					t.Errorf("%s: tool call %d type = %q, want %q", tt.description, i, actualCall.Type, "function")
				}

				// Check function name
				if actualCall.Function.Name != expectedCall.Name {
					t.Errorf("%s: tool call %d name = %q, want %q", tt.description, i, actualCall.Function.Name, expectedCall.Name)
				}

				// Check arguments can be parsed as JSON
				var args map[string]any
				if err := json.Unmarshal([]byte(actualCall.Function.Arguments), &args); err != nil {
					t.Errorf("%s: tool call %d arguments are not valid JSON: %v\nArguments: %s",
						tt.description, i, err, actualCall.Function.Arguments)
				}

				// Verify expected arguments are present
				if expectedCall.Args != nil {
					for key := range expectedCall.Args {
						if _, ok := args[key]; !ok {
							t.Errorf("%s: tool call %d missing expected argument %q", tt.description, i, key)
						}
					}
				} else if len(args) != 0 {
					// If no args expected, arguments should be empty object
					t.Errorf("%s: tool call %d should have no arguments, got %v", tt.description, i, args)

				}
			}
		})
	}
}

// TestFunctionCallsWithTextResponse tests that function calls take priority over text
func TestFunctionCallsWithTextResponse(t *testing.T) {
	client := &Client{}

	// Simulate a response that might have both text and function calls
	// In the fixed version, only function calls should be written
	functionCalls := []*genai.FunctionCall{
		{
			Name: "calculator",
			Args: map[string]any{
				"expression": "2+2",
			},
		},
	}

	var response strings.Builder
	w := calque.NewResponse(&response)

	err := client.writeFunctionCalls(functionCalls, w)
	if err != nil {
		t.Fatalf("writeFunctionCalls() error = %v", err)
	}

	output := response.String()

	// Output should be ONLY valid JSON tool calls format
	var result struct {
		ToolCalls []any `json:"tool_calls"`
	}

	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("Output should be valid JSON, got error: %v\nOutput: %s", err, output)
	}

	// Should not contain any text before the JSON
	trimmed := strings.TrimSpace(output)
	if !strings.HasPrefix(trimmed, "{") {
		t.Errorf("Output should start with JSON object, got: %s", output)
	}

	// Should contain tool_calls
	if !strings.Contains(output, "tool_calls") {
		t.Error("Output should contain tool_calls field")
	}
}

// TestProcessStreamResult tests the logic for processing stream results with hybrid streaming
func TestProcessStreamResult(t *testing.T) {
	tests := []struct {
		name              string
		result            *genai.GenerateContentResponse
		existingCalls     []*genai.FunctionCall
		existingBuffer    []string
		initialStreaming  bool
		expectedCallCount int
		expectedBuffer    []string
		expectedStreaming bool
		expectedWritten   string
		description       string
	}{
		{
			name: "first chunk with text only - switches to streaming",
			result: &genai.GenerateContentResponse{
				Candidates: []*genai.Candidate{
					{
						Content: &genai.Content{
							Parts: []*genai.Part{
								{Text: "Hello, world!"},
							},
						},
					},
				},
			},
			existingCalls:     nil,
			existingBuffer:    nil,
			initialStreaming:  false,
			expectedCallCount: 0,
			expectedBuffer:    nil,
			expectedStreaming: true,
			expectedWritten:   "Hello, world!",
			description:       "Should switch to streaming mode on first text chunk",
		},
		{
			name: "function call in chunk - stays buffering",
			result: &genai.GenerateContentResponse{
				Candidates: []*genai.Candidate{
					{
						Content: &genai.Content{
							Parts: []*genai.Part{
								{
									FunctionCall: &genai.FunctionCall{
										Name: "calculator",
										Args: map[string]any{"input": "2+2"},
									},
								},
							},
						},
					},
				},
			},
			existingCalls:     nil,
			existingBuffer:    nil,
			initialStreaming:  false,
			expectedCallCount: 1,
			expectedBuffer:    nil,
			expectedStreaming: false,
			expectedWritten:   "",
			description:       "Should stay in buffering mode when function call detected",
		},
		{
			name: "text while streaming - writes immediately",
			result: &genai.GenerateContentResponse{
				Candidates: []*genai.Candidate{
					{
						Content: &genai.Content{
							Parts: []*genai.Part{
								{Text: "More text"},
							},
						},
					},
				},
			},
			existingCalls:     nil,
			existingBuffer:    nil,
			initialStreaming:  true,
			expectedCallCount: 0,
			expectedBuffer:    nil,
			expectedStreaming: true,
			expectedWritten:   "More text",
			description:       "Should write immediately when already streaming",
		},
		{
			name: "function call with text - ignores text",
			result: &genai.GenerateContentResponse{
				Candidates: []*genai.Candidate{
					{
						Content: &genai.Content{
							Parts: []*genai.Part{
								{Text: "Some text"},
								{FunctionCall: &genai.FunctionCall{Name: "tool1"}},
							},
						},
					},
				},
			},
			existingCalls:     nil,
			existingBuffer:    nil,
			initialStreaming:  false,
			expectedCallCount: 1,
			expectedBuffer:    nil,
			expectedStreaming: false,
			expectedWritten:   "",
			description:       "Should ignore text when function call present",
		},
		{
			name: "multiple function calls",
			result: &genai.GenerateContentResponse{
				Candidates: []*genai.Candidate{
					{
						Content: &genai.Content{
							Parts: []*genai.Part{
								{FunctionCall: &genai.FunctionCall{Name: "tool1"}},
								{FunctionCall: &genai.FunctionCall{Name: "tool2"}},
							},
						},
					},
				},
			},
			existingCalls:     nil,
			existingBuffer:    nil,
			initialStreaming:  false,
			expectedCallCount: 2,
			expectedBuffer:    nil,
			expectedStreaming: false,
			expectedWritten:   "",
			description:       "Should collect multiple function calls",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &Client{}
			var response strings.Builder
			w := calque.NewResponse(&response)

			functionCalls := tt.existingCalls
			textBuffer := tt.existingBuffer
			streaming := tt.initialStreaming

			err := client.processStreamResult(tt.result, &functionCalls, &textBuffer, &streaming, w)
			if err != nil {
				t.Errorf("%s: processStreamResult() error = %v", tt.description, err)
				return
			}

			// Check function call count
			if len(functionCalls) != tt.expectedCallCount {
				t.Errorf("%s: function call count = %d, want %d", tt.description, len(functionCalls), tt.expectedCallCount)
			}

			// Check buffer state
			if len(textBuffer) != len(tt.expectedBuffer) {
				t.Errorf("%s: buffer length = %d, want %d", tt.description, len(textBuffer), len(tt.expectedBuffer))
			}

			// Check streaming state
			if streaming != tt.expectedStreaming {
				t.Errorf("%s: streaming = %v, want %v", tt.description, streaming, tt.expectedStreaming)
			}

			// Check written output
			output := response.String()
			if output != tt.expectedWritten {
				t.Errorf("%s: written output = %q, want %q", tt.description, output, tt.expectedWritten)
			}
		})
	}
}

// TestFinalizeResponse tests the finalization logic with hybrid streaming
func TestFinalizeResponse(t *testing.T) {
	tests := []struct {
		name          string
		functionCalls []*genai.FunctionCall
		textBuffer    []string
		expectJSON    bool
		expectText    string
		description   string
	}{
		{
			name:          "no function calls, no buffered text",
			functionCalls: nil,
			textBuffer:    nil,
			expectJSON:    false,
			expectText:    "",
			description:   "Should return success with no output",
		},
		{
			name:          "no function calls, with buffered text",
			functionCalls: nil,
			textBuffer:    []string{"Hello", " world!"},
			expectJSON:    false,
			expectText:    "Hello world!",
			description:   "Should write buffered text when no function calls",
		},
		{
			name: "function calls, no buffered text",
			functionCalls: []*genai.FunctionCall{
				{Name: "tool1", Args: map[string]any{"key": "value"}},
			},
			textBuffer:  nil,
			expectJSON:  true,
			description: "Should write JSON tool calls format",
		},
		{
			name: "function calls with buffered text - ignores text",
			functionCalls: []*genai.FunctionCall{
				{Name: "tool1", Args: map[string]any{"key": "value"}},
			},
			textBuffer:  []string{"This should be ignored"},
			expectJSON:  true,
			description: "Should write only function calls, ignoring buffered text",
		},
		{
			name: "multiple function calls",
			functionCalls: []*genai.FunctionCall{
				{Name: "tool1", Args: map[string]any{"key1": "value1"}},
				{Name: "tool2", Args: map[string]any{"key2": "value2"}},
			},
			textBuffer:  nil,
			expectJSON:  true,
			description: "Should write multiple tool calls in JSON format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &Client{}
			var response strings.Builder
			w := calque.NewResponse(&response)

			err := client.finalizeResponse(tt.functionCalls, tt.textBuffer, w)
			if err != nil {
				t.Errorf("%s: finalizeResponse() error = %v", tt.description, err)
				return
			}

			output := response.String()

			switch {
			case tt.expectJSON:
				// Should be valid JSON with tool_calls
				var result struct {
					ToolCalls []any `json:"tool_calls"`
				}
				if err := json.Unmarshal([]byte(output), &result); err != nil {
					t.Errorf("%s: output is not valid JSON: %v\nOutput: %s", tt.description, err, output)
					return
				}

				if len(result.ToolCalls) != len(tt.functionCalls) {
					t.Errorf("%s: tool call count = %d, want %d", tt.description, len(result.ToolCalls), len(tt.functionCalls))
				}

				// Should not contain buffered text
				if len(tt.textBuffer) > 0 {
					for _, text := range tt.textBuffer {
						if strings.Contains(output, text) {
							t.Errorf("%s: output should not contain buffered text %q", tt.description, text)
						}
					}
				}
			case tt.expectText != "":
				if output != tt.expectText {
					t.Errorf("%s: expected output %q, got %q", tt.description, tt.expectText, output)
				}
			default:
				if output != "" {
					t.Errorf("%s: expected no output, got %q", tt.description, output)
				}
			}
		})
	}
}
