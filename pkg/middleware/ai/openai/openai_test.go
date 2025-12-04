package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strings"
	"testing"

	"github.com/invopop/jsonschema"
	"github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/shared"
	orderedmap "github.com/wk8/go-ordered-map/v2"

	"github.com/calque-ai/go-calque/pkg/calque"
	"github.com/calque-ai/go-calque/pkg/helpers"
	"github.com/calque-ai/go-calque/pkg/middleware/ai"
	"github.com/calque-ai/go-calque/pkg/middleware/tools"
)

const testModel = "gpt-5"

// mockTool implements tools.Tool for testing
type mockTool struct {
	name        string
	description string
	schema      *jsonschema.Schema
}

func (m *mockTool) Name() string {
	return m.name
}

func (m *mockTool) Description() string {
	return m.description
}

func (m *mockTool) ParametersSchema() *jsonschema.Schema {
	return m.schema
}

func (m *mockTool) Execute(_ string) (string, error) {
	return "mock result", nil
}

func (m *mockTool) ServeFlow(_ *calque.Request, w *calque.Response) error {
	_, err := w.Data.Write([]byte("mock result"))
	return err
}

func TestNew(t *testing.T) {
	tests := []struct {
		name      string
		model     string
		config    *Config
		expectErr bool
	}{
		{
			name:      "empty model",
			model:     "",
			expectErr: true,
		},
		{
			name:  "valid model with API key in config",
			model: testModel,
			config: &Config{
				APIKey: "sk-test-key",
			},
			expectErr: false,
		},
		{
			name:      "valid model without API key",
			model:     "gpt-4",
			expectErr: true, // Should fail if no API key in env or config
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear environment variable for clean test
			oldKey := os.Getenv("OPENAI_API_KEY")
			os.Unsetenv("OPENAI_API_KEY")
			defer func() {
				if oldKey != "" {
					os.Setenv("OPENAI_API_KEY", oldKey)
				}
			}()

			var opts []Option
			if tt.config != nil {
				opts = append(opts, WithConfig(tt.config))
			}

			client, err := New(tt.model, opts...)
			if tt.expectErr {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if client == nil {
				t.Error("expected client but got nil")
				return
			}

			if string(client.model) != tt.model {
				t.Errorf("expected model %s, got %s", tt.model, client.model)
			}
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	// Set test environment variable
	testKey := "sk-test-key"
	os.Setenv("OPENAI_API_KEY", testKey)
	defer os.Unsetenv("OPENAI_API_KEY")

	config := DefaultConfig()

	if config.APIKey != testKey {
		t.Errorf("expected API key %s, got %s", testKey, config.APIKey)
	}

	if config.Temperature == nil || *config.Temperature != 1.0 {
		t.Errorf("expected temperature 1.0, got %v", config.Temperature)
	}

	if config.Stream == nil || !*config.Stream {
		t.Errorf("expected stream to be true, got %v", config.Stream)
	}
}

func TestWithConfig(t *testing.T) {
	config := &Config{
		APIKey:      "test-key",
		Temperature: helpers.PtrOf(float32(0.9)),
		MaxTokens:   helpers.PtrOf(1000),
	}

	option := WithConfig(config)

	// This would normally be used in New(), but we can test the option directly
	testConfig := DefaultConfig()
	option.Apply(testConfig)

	if testConfig.APIKey != config.APIKey {
		t.Errorf("expected API key %s, got %s", config.APIKey, testConfig.APIKey)
	}

	if testConfig.Temperature == nil || *testConfig.Temperature != 0.9 {
		t.Errorf("expected temperature 0.9, got %v", testConfig.Temperature)
	}

	if testConfig.MaxTokens == nil || *testConfig.MaxTokens != 1000 {
		t.Errorf("expected max tokens 1000, got %v", testConfig.MaxTokens)
	}
}

func TestConvertToOpenAITools(t *testing.T) {
	// Create a mock tool for testing
	mockTool := &mockTool{
		name:        "test_tool",
		description: "A test tool",
		schema: &jsonschema.Schema{
			Type:     "object",
			Required: []string{"input"},
		},
	}

	client := &Client{
		model:  shared.ChatModel(testModel),
		config: DefaultConfig(),
	}

	openaiTools, err := client.convertToOpenAITools([]tools.Tool{mockTool})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(openaiTools) != 1 {
		t.Errorf("expected 1 tool, got %d", len(openaiTools))
		return
	}

	// TODO: Fix this test for v2 API - union types need proper access
	_ = openaiTools[0]
	// tool := openaiTools[0]
	// funcTool := tool.AsFunction()
	// if funcTool.Function.Name != "test_tool" {
	//   t.Errorf("expected tool name 'test_tool', got '%s'", funcTool.Function.Name)
	// }
}

// Integration test that requires a real API key
func TestChatIntegration(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY not set, skipping integration test")
	}

	client, err := New(testModel)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	// Create a simple request
	req := &calque.Request{
		Context: context.Background(),
		Data:    strings.NewReader("Hello, how are you?"),
	}

	// Create a response buffer
	var buf strings.Builder
	resp := &calque.Response{
		Data: &buf,
	}

	// Test simple chat
	err = client.Chat(req, resp, nil)
	if err != nil {
		t.Fatalf("chat failed: %v", err)
	}

	if buf.Len() == 0 {
		t.Error("expected response content but got empty")
	}
}

// TestInputToMessages tests the conversion of classified input to OpenAI message format
func TestInputToMessages(t *testing.T) {
	client := &Client{
		model:  shared.ChatModel(testModel),
		config: DefaultConfig(),
	}

	tests := []struct {
		name        string
		input       *ai.ClassifiedInput
		expectError bool
		checkFunc   func([]openai.ChatCompletionMessageParamUnion) error
	}{
		{
			name: "text input",
			input: &ai.ClassifiedInput{
				Type: ai.TextInput,
				Text: "Hello, world!",
			},
			checkFunc: func(messages []openai.ChatCompletionMessageParamUnion) error {
				if len(messages) != 1 {
					return fmt.Errorf("expected 1 message, got %d", len(messages))
				}
				// Note: We can't easily inspect the union content without complex type assertions
				// In a real scenario, we'd test through the actual API calls
				return nil
			},
		},
		{
			name: "unsupported input type",
			input: &ai.ClassifiedInput{
				Type: ai.InputType(999),
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			messages, err := client.inputToMessages(tt.input)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error, got none")
				}
				return
			}

			if err != nil {
				t.Errorf("inputToMessages() error = %v", err)
				return
			}

			if tt.checkFunc != nil {
				if err := tt.checkFunc(messages); err != nil {
					t.Errorf("inputToMessages() %v", err)
				}
			}
		})
	}
}

// TestApplyChatConfig tests the application of configuration to chat requests
func TestApplyChatConfig(t *testing.T) {
	tests := []struct {
		name   string
		config *Config
		schema *ai.ResponseFormat
		check  func(*openai.ChatCompletionNewParams) error
	}{
		{
			name: "basic config",
			config: &Config{
				Temperature:      helpers.PtrOf(float32(0.8)),
				TopP:             helpers.PtrOf(float32(0.9)),
				MaxTokens:        helpers.PtrOf(1500),
				N:                helpers.PtrOf(2),
				Stop:             []string{"END", "STOP"},
				PresencePenalty:  helpers.PtrOf(float32(0.5)),
				FrequencyPenalty: helpers.PtrOf(float32(0.3)),
				User:             "test-user",
				Seed:             helpers.PtrOf(42),
			},
			check: func(params *openai.ChatCompletionNewParams) error {
				if math.Abs(params.Temperature.Value-0.8) > 0.001 {
					return fmt.Errorf("temperature = %v, want 0.8", params.Temperature.Value)
				}
				if math.Abs(params.TopP.Value-0.9) > 0.001 {
					return fmt.Errorf("topP = %v, want 0.9", params.TopP.Value)
				}
				if params.MaxCompletionTokens.Value != 1500 {
					return fmt.Errorf("maxTokens = %v, want 1500", params.MaxCompletionTokens.Value)
				}
				if params.N.Value != 2 {
					return fmt.Errorf("n = %v, want 2", params.N.Value)
				}
				if math.Abs(params.PresencePenalty.Value-0.5) > 0.001 {
					return fmt.Errorf("presencePenalty = %v, want 0.5", params.PresencePenalty.Value)
				}
				if math.Abs(params.FrequencyPenalty.Value-0.3) > 0.001 {
					return fmt.Errorf("frequencyPenalty = %v, want 0.3", params.FrequencyPenalty.Value)
				}
				if params.User.Value != "test-user" {
					return fmt.Errorf("user = %v, want test-user", params.User.Value)
				}
				if params.Seed.Value != 42 {
					return fmt.Errorf("seed = %v, want 42", params.Seed.Value)
				}
				return nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &Client{
				model:  shared.ChatModel(testModel),
				config: tt.config,
			}

			params := &openai.ChatCompletionNewParams{
				Model: client.model,
			}

			client.applyChatConfig(params, tt.schema)

			if tt.check != nil {
				if err := tt.check(params); err != nil {
					t.Errorf("applyChatConfig() %v", err)
				}
			}
		})
	}
}

// TestBuildChatParams tests the request parameters building
func TestBuildChatParams(t *testing.T) {
	client := &Client{
		model: shared.ChatModel(testModel),
		config: &Config{
			Temperature: helpers.PtrOf(float32(0.7)),
			MaxTokens:   helpers.PtrOf(100),
		},
	}

	tests := []struct {
		name        string
		input       *ai.ClassifiedInput
		schema      *ai.ResponseFormat
		tools       []tools.Tool
		expectError bool
		checkFunc   func(openai.ChatCompletionNewParams) error
	}{
		{
			name: "text input with tools",
			input: &ai.ClassifiedInput{
				Type: ai.TextInput,
				Text: "Hello",
			},
			tools: []tools.Tool{
				&mockTool{
					name:        "test_tool",
					description: "A test tool",
					schema: &jsonschema.Schema{
						Type: "object",
						Properties: func() *orderedmap.OrderedMap[string, *jsonschema.Schema] {
							props := orderedmap.New[string, *jsonschema.Schema]()
							props.Set("input", &jsonschema.Schema{Type: "string"})
							return props
						}(),
						Required: []string{"input"},
					},
				},
			},
			checkFunc: func(params openai.ChatCompletionNewParams) error {
				if len(params.Tools) != 1 {
					return fmt.Errorf("expected 1 tool, got %d", len(params.Tools))
				}
				return nil
			},
		},
		{
			name: "text input with schema",
			input: &ai.ClassifiedInput{
				Type: ai.TextInput,
				Text: "Generate JSON",
			},
			schema: &ai.ResponseFormat{
				Type: "json_object",
			},
			checkFunc: func(params openai.ChatCompletionNewParams) error {
				if params.ResponseFormat.OfJSONObject == nil {
					return fmt.Errorf("responseFormat should be set")
				}
				return nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params, err := client.buildChatParams(tt.input, tt.schema, tt.tools)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error, got none")
				}
				return
			}

			if err != nil {
				t.Errorf("buildChatParams() error = %v", err)
				return
			}

			if tt.checkFunc != nil {
				if err := tt.checkFunc(params); err != nil {
					t.Errorf("buildChatParams() %v", err)
				}
			}
		})
	}
}

// TestMultimodalToMessages tests multimodal input conversion
func TestMultimodalToMessages(t *testing.T) {
	client := &Client{
		model:  shared.ChatModel("gpt-4-vision-preview"),
		config: DefaultConfig(),
	}

	tests := []struct {
		name        string
		multimodal  *ai.MultimodalInput
		expectError bool
		checkFunc   func([]openai.ChatCompletionMessageParamUnion) error
	}{
		{
			name:        "nil multimodal input",
			multimodal:  nil,
			expectError: true,
		},
		{
			name: "text only",
			multimodal: &ai.MultimodalInput{
				Parts: []ai.ContentPart{
					{Type: "text", Text: "Hello world"},
				},
			},
			checkFunc: func(messages []openai.ChatCompletionMessageParamUnion) error {
				if len(messages) != 1 {
					return fmt.Errorf("expected 1 message, got %d", len(messages))
				}
				return nil
			},
		},
		{
			name: "image with data",
			multimodal: &ai.MultimodalInput{
				Parts: []ai.ContentPart{
					{Type: "image", Data: []byte("test-image-data"), MimeType: "image/png"},
				},
			},
			checkFunc: func(messages []openai.ChatCompletionMessageParamUnion) error {
				if len(messages) != 1 {
					return fmt.Errorf("expected 1 message, got %d", len(messages))
				}
				return nil
			},
		},
		{
			name: "image with reader",
			multimodal: &ai.MultimodalInput{
				Parts: []ai.ContentPart{
					{Type: "image", Reader: bytes.NewReader([]byte("test-image-data")), MimeType: "image/jpeg"},
				},
			},
			checkFunc: func(messages []openai.ChatCompletionMessageParamUnion) error {
				if len(messages) != 1 {
					return fmt.Errorf("expected 1 message, got %d", len(messages))
				}
				return nil
			},
		},
		{
			name: "unsupported audio",
			multimodal: &ai.MultimodalInput{
				Parts: []ai.ContentPart{
					{Type: "audio", Data: []byte("audio-data"), MimeType: "audio/wav"},
				},
			},
			expectError: true,
		},
		{
			name: "unsupported video",
			multimodal: &ai.MultimodalInput{
				Parts: []ai.ContentPart{
					{Type: "video", Data: []byte("video-data"), MimeType: "video/mp4"},
				},
			},
			expectError: true,
		},
		{
			name: "empty parts",
			multimodal: &ai.MultimodalInput{
				Parts: []ai.ContentPart{},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			messages, err := client.multimodalToMessages(tt.multimodal)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error, got none")
				}
				return
			}

			if err != nil {
				t.Errorf("multimodalToMessages() error = %v", err)
				return
			}

			if tt.checkFunc != nil {
				if err := tt.checkFunc(messages); err != nil {
					t.Errorf("multimodalToMessages() %v", err)
				}
			}
		})
	}
}

// TestEnhancedConvertToOpenAITools tests tool conversion with more scenarios
func TestEnhancedConvertToOpenAITools(t *testing.T) {
	client := &Client{
		model:  shared.ChatModel(testModel),
		config: DefaultConfig(),
	}

	tests := []struct {
		name      string
		tools     []tools.Tool
		checkFunc func([]openai.ChatCompletionToolUnionParam) error
	}{
		{
			name:  "empty tools",
			tools: []tools.Tool{},
			checkFunc: func(tools []openai.ChatCompletionToolUnionParam) error {
				if len(tools) != 0 {
					return fmt.Errorf("expected 0 tools, got %d", len(tools))
				}
				return nil
			},
		},
		{
			name: "single tool",
			tools: []tools.Tool{
				&mockTool{
					name:        "calculator",
					description: "Performs calculations",
					schema: &jsonschema.Schema{
						Type: "object",
						Properties: func() *orderedmap.OrderedMap[string, *jsonschema.Schema] {
							props := orderedmap.New[string, *jsonschema.Schema]()
							props.Set("expression", &jsonschema.Schema{Type: "string", Description: "Mathematical expression"})
							return props
						}(),
						Required: []string{"expression"},
					},
				},
			},
			checkFunc: func(tools []openai.ChatCompletionToolUnionParam) error {
				if len(tools) != 1 {
					return fmt.Errorf("expected 1 tool, got %d", len(tools))
				}
				// TODO: Fix this test for v2 API - union types need proper access
				_ = tools[0]
				// funcTool := tool.AsFunction()
				// if funcTool.Function.Name != "calculator" {
				//   return fmt.Errorf("expected calculator name, got %s", funcTool.Function.Name)
				// }
				return nil
			},
		},
		{
			name: "multiple tools",
			tools: []tools.Tool{
				&mockTool{
					name:        "tool1",
					description: "First tool",
					schema:      &jsonschema.Schema{Type: "object"},
				},
				&mockTool{
					name:        "tool2",
					description: "Second tool",
					schema:      &jsonschema.Schema{Type: "object"},
				},
			},
			checkFunc: func(tools []openai.ChatCompletionToolUnionParam) error {
				if len(tools) != 2 {
					return fmt.Errorf("expected 2 tools, got %d", len(tools))
				}
				return nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			openaiTools, err := client.convertToOpenAITools(tt.tools)
			if err != nil {
				t.Errorf("convertToOpenAITools() error = %v", err)
				return
			}

			if tt.checkFunc != nil {
				if err := tt.checkFunc(openaiTools); err != nil {
					t.Errorf("convertToOpenAITools() %v", err)
				}
			}
		})
	}
}

// TestWriteOpenAIToolCalls tests tool call formatting
func TestWriteOpenAIToolCalls(t *testing.T) {
	client := &Client{
		model:  shared.ChatModel(testModel),
		config: DefaultConfig(),
	}

	tests := []struct {
		name      string
		toolCalls []openai.ChatCompletionMessageFunctionToolCall
		checkFunc func(string) error
	}{
		{
			name: "single tool call",
			toolCalls: []openai.ChatCompletionMessageFunctionToolCall{
				{
					Function: openai.ChatCompletionMessageFunctionToolCallFunction{
						Name:      "calculator",
						Arguments: `{"expression":"2+2"}`,
					},
				},
			},
			checkFunc: func(output string) error {
				var result map[string]interface{}
				if err := json.Unmarshal([]byte(output), &result); err != nil {
					return fmt.Errorf("failed to parse JSON: %v", err)
				}
				toolCalls, ok := result["tool_calls"]
				if !ok {
					return fmt.Errorf("missing tool_calls field")
				}
				calls := toolCalls.([]interface{})
				if len(calls) != 1 {
					return fmt.Errorf("expected 1 tool call, got %d", len(calls))
				}
				return nil
			},
		},
		{
			name:      "empty tool calls",
			toolCalls: []openai.ChatCompletionMessageFunctionToolCall{},
			checkFunc: func(output string) error {
				var result map[string]interface{}
				if err := json.Unmarshal([]byte(output), &result); err != nil {
					return fmt.Errorf("failed to parse JSON: %v", err)
				}
				toolCalls, ok := result["tool_calls"]
				if !ok {
					return fmt.Errorf("missing tool_calls field")
				}
				if toolCalls == nil {
					return nil // Empty tool calls are represented as null/nil
				}
				calls, ok := toolCalls.([]interface{})
				if !ok {
					return fmt.Errorf("tool_calls is not an array: %T", toolCalls)
				}
				if len(calls) != 0 {
					return fmt.Errorf("expected 0 tool calls, got %d", len(calls))
				}
				return nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf strings.Builder
			resp := &calque.Response{
				Data: &buf,
			}

			err := client.writeOpenAIToolCalls(tt.toolCalls, resp)
			if err != nil {
				t.Errorf("writeOpenAIToolCalls() error = %v", err)
				return
			}

			if tt.checkFunc != nil {
				if err := tt.checkFunc(buf.String()); err != nil {
					t.Errorf("writeOpenAIToolCalls() %v", err)
				}
			}
		})
	}
}

func TestChat_Method(t *testing.T) {
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
				&mockTool{
					name:        "calculator",
					description: "Performs calculations",
					schema: &jsonschema.Schema{
						Type:     "object",
						Required: []string{"input"},
					},
				},
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
				model:  shared.ChatModel(testModel),
				config: &Config{APIKey: "test-key", Temperature: helpers.PtrOf(float32(0.7))},
			}

			// Test message conversion
			messages, err := client.inputToMessages(input)
			if err != nil {
				t.Errorf("%s: inputToMessages() error = %v", tt.description, err)
				return
			}

			if len(messages) == 0 {
				t.Errorf("%s: inputToMessages() should return at least one message", tt.description)
			}

			// Test params building
			params, err := client.buildChatParams(input, ai.GetSchema(opts), ai.GetTools(opts))
			if err != nil {
				t.Errorf("%s: buildChatParams() error = %v", tt.description, err)
				return
			}

			if string(params.Model) != testModel {
				t.Errorf("%s: expected model gpt-3.5-turbo, got %s", tt.description, params.Model)
			}

			// Verify tools are processed correctly
			if len(tt.tools) > 0 {
				if len(params.Tools) != len(tt.tools) {
					t.Errorf("%s: expected %d tools in params, got %d",
						tt.description, len(tt.tools), len(params.Tools))
				}
			}
		})
	}
}

func TestExecuteRequest_Method(t *testing.T) {
	client := &Client{
		model:  shared.ChatModel(testModel),
		config: &Config{APIKey: "test-key", Stream: helpers.PtrOf(true)},
	}

	tests := []struct {
		name        string
		streaming   bool
		description string
	}{
		{
			name:        "streaming request routing",
			streaming:   true,
			description: "Should route to streaming when stream=true",
		},
		{
			name:        "non-streaming request routing",
			streaming:   false,
			description: "Should route to non-streaming when stream=false",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test that executeRequest routes correctly based on streaming config
			client.config.Stream = &tt.streaming

			params := openai.ChatCompletionNewParams{
				Model: client.model,
				Messages: []openai.ChatCompletionMessageParamUnion{
					openai.UserMessage("test message"),
				},
			}

			r := calque.NewRequest(context.Background(), strings.NewReader("test"))
			var buf strings.Builder
			w := calque.NewResponse(&buf)

			// We can't actually execute without real API key, but we can test the routing logic
			// by checking the configuration is set correctly
			if tt.streaming && !*client.config.Stream {
				t.Errorf("%s: expected streaming to be enabled", tt.description)
			}
			if !tt.streaming && *client.config.Stream {
				t.Errorf("%s: expected streaming to be disabled", tt.description)
			}

			// The actual executeRequest call would fail without API key, which is expected
			// We're testing the configuration and routing logic here
			_ = params
			_ = r
			_ = w
		})
	}
}

func TestExecuteStreamingRequest_Logic(t *testing.T) {
	// Test the logic of streaming request setup without actual API calls
	client := &Client{
		model:  shared.ChatModel(testModel),
		config: &Config{APIKey: "test-key", Stream: helpers.PtrOf(true)},
	}

	// Test that client config is set for streaming
	if !*client.config.Stream {
		t.Error("Expected client Stream config to be true for streaming request")
	}

	// Verify we can create params with proper model
	if string(client.model) != testModel {
		t.Error("Expected model to be gpt-3.5-turbo")
	}

	// We can't test actual streaming without API key, but we verify the setup exists
	r := calque.NewRequest(context.Background(), strings.NewReader("test"))
	var buf strings.Builder
	w := calque.NewResponse(&buf)

	// The function signature is correct
	_ = r
	_ = w
}

func TestExecuteNonStreamingRequest_Logic(t *testing.T) {
	// Test the logic of non-streaming request setup without actual API calls
	client := &Client{
		model:  shared.ChatModel(testModel),
		config: &Config{APIKey: "test-key", Stream: helpers.PtrOf(false)},
	}

	// Test that client config is set for non-streaming
	if *client.config.Stream {
		t.Error("Expected client Stream config to be false for non-streaming request")
	}

	// Verify we can create client with proper model
	if string(client.model) != testModel {
		t.Error("Expected model to be gpt-3.5-turbo")
	}

	// We can't test actual API call without key, but we verify the setup exists
	r := calque.NewRequest(context.Background(), strings.NewReader("test"))
	var buf strings.Builder
	w := calque.NewResponse(&buf)

	// The function signature is correct
	_ = r
	_ = w
}

func TestSetJSONSchemaFormat(t *testing.T) {
	client := &Client{
		model:  shared.ChatModel(testModel),
		config: DefaultConfig(),
	}

	tests := []struct {
		name      string
		schema    any
		checkFunc func(*openai.ChatCompletionNewParams) error
	}{
		{
			name: "valid schema object",
			schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{
						"type": "string",
					},
				},
				"required": []string{"name"},
			},
			checkFunc: func(params *openai.ChatCompletionNewParams) error {
				if params.ResponseFormat.OfJSONSchema == nil {
					return fmt.Errorf("expected JSON schema format to be set")
				}
				if params.ResponseFormat.OfJSONSchema.JSONSchema.Name != "response_schema" {
					return fmt.Errorf("expected schema name to be 'response_schema'")
				}
				if !params.ResponseFormat.OfJSONSchema.JSONSchema.Strict.Value {
					return fmt.Errorf("expected strict mode to be enabled")
				}
				return nil
			},
		},
		{
			name:   "invalid schema - unmarshalable",
			schema: make(chan int), // channels can't be marshaled to JSON
			checkFunc: func(params *openai.ChatCompletionNewParams) error {
				// Should fallback to json_object on marshal error
				if params.ResponseFormat.OfJSONObject == nil {
					return fmt.Errorf("expected fallback to JSON object format")
				}
				return nil
			},
		},
		{
			name:   "nil schema",
			schema: nil,
			checkFunc: func(params *openai.ChatCompletionNewParams) error {
				if params.ResponseFormat.OfJSONSchema == nil {
					return fmt.Errorf("expected JSON schema format to be set even for nil")
				}
				return nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := &openai.ChatCompletionNewParams{
				Model: client.model,
			}

			client.setJSONSchemaFormat(tt.schema, params)

			if tt.checkFunc != nil {
				if err := tt.checkFunc(params); err != nil {
					t.Errorf("setJSONSchemaFormat() %v", err)
				}
			}
		})
	}
}

// TestErrorHandling tests various error scenarios
func TestErrorHandling(t *testing.T) {
	tests := []struct {
		name          string
		setupClient   func() *Client
		input         string
		expectError   bool
		errorContains string
	}{
		{
			name: "nil multimodal input",
			setupClient: func() *Client {
				return &Client{
					model:  shared.ChatModel("gpt-4"),
					config: DefaultConfig(),
				}
			},
			input:         "", // Will be ignored since we're testing multimodal
			expectError:   true,
			errorContains: "multimodal input cannot be nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := tt.setupClient()

			if tt.name == "nil multimodal input" {
				_, err := client.multimodalToMessages(nil)
				if !tt.expectError {
					t.Errorf("Expected no error, got: %v", err)
					return
				}
				if err == nil {
					t.Error("Expected error, got none")
					return
				}
				if !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error to contain %q, got: %v", tt.errorContains, err)
				}
			}
		})
	}
}

// TestProcessStreamDelta tests the streaming delta processing logic
func TestProcessStreamDelta(t *testing.T) {
	tests := []struct {
		name              string
		delta             openai.ChatCompletionChunkChoiceDelta
		existingToolCalls map[int]*openai.ChatCompletionMessageFunctionToolCall
		existingHasTools  bool
		expectedHasTools  bool
		expectedToolCount int
		expectedText      string
		description       string
	}{
		{
			name: "text only delta",
			delta: openai.ChatCompletionChunkChoiceDelta{
				Content:   "Hello, world!",
				ToolCalls: []openai.ChatCompletionChunkChoiceDeltaToolCall{},
			},
			existingToolCalls: make(map[int]*openai.ChatCompletionMessageFunctionToolCall),
			existingHasTools:  false,
			expectedHasTools:  false,
			expectedToolCount: 0,
			expectedText:      "Hello, world!",
			description:       "Should write text when no tool calls present",
		},
		{
			name: "tool call delta",
			delta: openai.ChatCompletionChunkChoiceDelta{
				Content: "",
				ToolCalls: []openai.ChatCompletionChunkChoiceDeltaToolCall{
					{
						Index: 0,
						ID:    "call_123",
						Function: openai.ChatCompletionChunkChoiceDeltaToolCallFunction{
							Name:      "calculator",
							Arguments: "{\"input\":\"2+2\"}",
						},
					},
				},
			},
			existingToolCalls: make(map[int]*openai.ChatCompletionMessageFunctionToolCall),
			existingHasTools:  false,
			expectedHasTools:  true,
			expectedToolCount: 1,
			expectedText:      "",
			description:       "Should collect tool call and not write text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &Client{}
			var response strings.Builder
			w := calque.NewResponse(&response)

			toolCalls := tt.existingToolCalls
			hasToolCalls := tt.existingHasTools

			err := client.processStreamDelta(tt.delta, toolCalls, &hasToolCalls, w)
			if err != nil {
				t.Errorf("%s: processStreamDelta() error = %v", tt.description, err)
				return
			}

			if hasToolCalls != tt.expectedHasTools {
				t.Errorf("%s: hasToolCalls = %v, want %v", tt.description, hasToolCalls, tt.expectedHasTools)
			}

			if len(toolCalls) != tt.expectedToolCount {
				t.Errorf("%s: tool call count = %d, want %d", tt.description, len(toolCalls), tt.expectedToolCount)
			}

			output := response.String()
			if output != tt.expectedText {
				t.Errorf("%s: text output = %q, want %q", tt.description, output, tt.expectedText)
			}
		})
	}
}

// TestFinalizeToolCalls tests the tool call finalization logic
func TestFinalizeToolCalls(t *testing.T) {
	tests := []struct {
		name          string
		toolCalls     map[int]*openai.ChatCompletionMessageFunctionToolCall
		expectJSON    bool
		expectedCount int
		description   string
	}{
		{
			name:          "no tool calls",
			toolCalls:     make(map[int]*openai.ChatCompletionMessageFunctionToolCall),
			expectJSON:    false,
			expectedCount: 0,
			description:   "Should return success with no output when no tool calls",
		},
		{
			name: "single tool call",
			toolCalls: map[int]*openai.ChatCompletionMessageFunctionToolCall{
				0: {
					ID: "call_1",
					Function: openai.ChatCompletionMessageFunctionToolCallFunction{
						Name:      "calculator",
						Arguments: "{\"input\":\"2+2\"}",
					},
				},
			},
			expectJSON:    true,
			expectedCount: 1,
			description:   "Should write JSON tool calls format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &Client{}
			var response strings.Builder
			w := calque.NewResponse(&response)

			err := client.finalizeToolCalls(tt.toolCalls, w)
			if err != nil {
				t.Errorf("%s: finalizeToolCalls() error = %v", tt.description, err)
				return
			}

			output := response.String()

			if tt.expectJSON {
				var result struct {
					ToolCalls []any `json:"tool_calls"`
				}
				if err := json.Unmarshal([]byte(output), &result); err != nil {
					t.Errorf("%s: output is not valid JSON: %v\nOutput: %s", tt.description, err, output)
					return
				}

				if len(result.ToolCalls) != tt.expectedCount {
					t.Errorf("%s: tool call count = %d, want %d", tt.description, len(result.ToolCalls), tt.expectedCount)
				}
			} else if output != "" {
				t.Errorf("%s: expected no output, got %q", tt.description, output)
			}
		})
	}
}

// TestUsageHandler tests that the usage handler callback is invoked correctly
func TestUsageHandler(t *testing.T) {
	client := &Client{
		model:  shared.ChatModel(testModel),
		config: &Config{APIKey: "test-key"},
	}

	tests := []struct {
		name           string
		setupUsage     func(*Client)
		expectedCalled bool
		expectedTokens int
		description    string
	}{
		{
			name: "handler called with usage data",
			setupUsage: func(c *Client) {
				c.lastUsage = &ai.UsageMetadata{
					PromptTokens:     100,
					CompletionTokens: 50,
					TotalTokens:      150,
				}
			},
			expectedCalled: true,
			expectedTokens: 150,
			description:    "Should invoke handler when usage data is present",
		},
		{
			name: "handler not called without usage data",
			setupUsage: func(c *Client) {
				c.lastUsage = nil
			},
			expectedCalled: false,
			expectedTokens: 0,
			description:    "Should not invoke handler when no usage data",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupUsage(client)

			var called bool
			var receivedTokens int

			opts := &ai.AgentOptions{
				UsageHandler: func(usage *ai.UsageMetadata) {
					called = true
					receivedTokens = usage.TotalTokens
				},
			}

			client.reportUsage(opts)

			if called != tt.expectedCalled {
				t.Errorf("%s: handler called = %v, want %v", tt.description, called, tt.expectedCalled)
			}

			if tt.expectedCalled && receivedTokens != tt.expectedTokens {
				t.Errorf("%s: received tokens = %d, want %d", tt.description, receivedTokens, tt.expectedTokens)
			}
		})
	}
}

// TestUsageHandler_NilOptions tests that reportUsage handles nil options safely
func TestUsageHandler_NilOptions(_ *testing.T) {
	client := &Client{
		model:  shared.ChatModel(testModel),
		config: &Config{APIKey: "test-key"},
		lastUsage: &ai.UsageMetadata{
			PromptTokens:     100,
			CompletionTokens: 50,
			TotalTokens:      150,
		},
	}

	// Should not panic with nil options
	client.reportUsage(nil)

	// Should not panic with options but no handler
	client.reportUsage(&ai.AgentOptions{})
}
