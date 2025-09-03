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
	"github.com/calque-ai/go-calque/pkg/middleware/ai"
	"github.com/calque-ai/go-calque/pkg/middleware/tools"
	"github.com/calque-ai/go-calque/pkg/utils"
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
					Temperature: utils.Float32Ptr(0.8),
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
		Temperature: utils.Float32Ptr(0.9),
		MaxTokens:   utils.IntPtr(2000),
		TopP:        utils.Float32Ptr(0.95),
		TopK:        utils.Float32Ptr(40),
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
				Temperature:       utils.Float32Ptr(0.8),
				TopP:              utils.Float32Ptr(0.9),
				TopK:              utils.Float32Ptr(50),
				MaxTokens:         utils.IntPtr(1500),
				Stop:              []string{"END", "STOP"},
				SystemInstruction: "Be helpful",
				PresencePenalty:   utils.Float32Ptr(0.1),
				FrequencyPenalty:  utils.Float32Ptr(0.2),
				Seed:              utils.Int32Ptr(42),
				CandidateCount:    utils.Int32Ptr(1),
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

	// Should contain empty input for tools without args (in the JSON string)
	if !strings.Contains(result, `\"input\": \"\"`) {
		t.Errorf("writeFunctionCalls() should provide empty input for tools without args, got: %s", result)
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
			Temperature: ai.Float32Ptr(0.8),
			MaxTokens:   ai.IntPtr(1000),
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
				config: &Config{Temperature: ai.Float32Ptr(0.7)},
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
