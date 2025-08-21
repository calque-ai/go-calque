package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/invopop/jsonschema"
	"github.com/sashabaranov/go-openai"
	orderedmap "github.com/wk8/go-ordered-map/v2"

	"github.com/calque-ai/go-calque/pkg/calque"
	"github.com/calque-ai/go-calque/pkg/middleware/ai"
	"github.com/calque-ai/go-calque/pkg/middleware/tools"
)

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

func (m *mockTool) Execute(input string) (string, error) {
	return "mock result", nil
}

func (m *mockTool) ServeFlow(r *calque.Request, w *calque.Response) error {
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
			model: "gpt-3.5-turbo",
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

			if client.model != tt.model {
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

	if config.Temperature == nil || *config.Temperature != 0.7 {
		t.Errorf("expected temperature 0.7, got %v", config.Temperature)
	}

	if config.Stream == nil || !*config.Stream {
		t.Errorf("expected stream to be true, got %v", config.Stream)
	}
}

func TestWithConfig(t *testing.T) {
	config := &Config{
		APIKey:      "test-key",
		Temperature: ai.Float32Ptr(0.9),
		MaxTokens:   ai.IntPtr(1000),
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
		model:  "gpt-3.5-turbo",
		config: DefaultConfig(),
	}

	openaiTools := client.convertToOpenAITools([]tools.Tool{mockTool})

	if len(openaiTools) != 1 {
		t.Errorf("expected 1 tool, got %d", len(openaiTools))
		return
	}

	tool := openaiTools[0]
	if tool.Function.Name != "test_tool" {
		t.Errorf("expected tool name 'test_tool', got '%s'", tool.Function.Name)
	}

	if tool.Function.Description != "A test tool" {
		t.Errorf("expected tool description 'A test tool', got '%s'", tool.Function.Description)
	}
}

// Integration test that requires a real API key
func TestChatIntegration(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY not set, skipping integration test")
	}

	client, err := New("gpt-3.5-turbo")
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
		model:  "gpt-3.5-turbo",
		config: DefaultConfig(),
	}

	tests := []struct {
		name        string
		input       *ai.ClassifiedInput
		expectError bool
		checkFunc   func([]openai.ChatCompletionMessage) error
	}{
		{
			name: "text input",
			input: &ai.ClassifiedInput{
				Type: ai.TextInput,
				Text: "Hello, world!",
			},
			checkFunc: func(messages []openai.ChatCompletionMessage) error {
				if len(messages) != 1 {
					return fmt.Errorf("expected 1 message, got %d", len(messages))
				}
				if messages[0].Role != openai.ChatMessageRoleUser {
					return fmt.Errorf("expected user role, got %s", messages[0].Role)
				}
				if messages[0].Content != "Hello, world!" {
					return fmt.Errorf("expected 'Hello, world!', got %s", messages[0].Content)
				}
				return nil
			},
		},
		{
			name: "multimodal input with text and image",
			input: &ai.ClassifiedInput{
				Type: ai.MultimodalJSONInput,
				Multimodal: &ai.MultimodalInput{
					Parts: []ai.ContentPart{
						{Type: "text", Text: "What's in this image?"},
						{Type: "image", Data: []byte("fake-image-data"), MimeType: "image/jpeg"},
					},
				},
			},
			checkFunc: func(messages []openai.ChatCompletionMessage) error {
				if len(messages) != 1 {
					return fmt.Errorf("expected 1 message, got %d", len(messages))
				}
				if len(messages[0].MultiContent) != 2 {
					return fmt.Errorf("expected 2 content parts, got %d", len(messages[0].MultiContent))
				}
				if messages[0].MultiContent[0].Type != openai.ChatMessagePartTypeText {
					return fmt.Errorf("expected text part, got %s", messages[0].MultiContent[0].Type)
				}
				if messages[0].MultiContent[1].Type != openai.ChatMessagePartTypeImageURL {
					return fmt.Errorf("expected image URL part, got %s", messages[0].MultiContent[1].Type)
				}
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
		check  func(*openai.ChatCompletionRequest) error
	}{
		{
			name: "basic config",
			config: &Config{
				Temperature:      ai.Float32Ptr(0.8),
				TopP:             ai.Float32Ptr(0.9),
				MaxTokens:        ai.IntPtr(1500),
				N:                ai.IntPtr(2),
				Stop:             []string{"END", "STOP"},
				PresencePenalty:  ai.Float32Ptr(0.5),
				FrequencyPenalty: ai.Float32Ptr(0.3),
				User:             "test-user",
				Seed:             ai.IntPtr(42),
			},
			check: func(req *openai.ChatCompletionRequest) error {
				if req.Temperature != 0.8 {
					return fmt.Errorf("temperature = %v, want 0.8", req.Temperature)
				}
				if req.TopP != 0.9 {
					return fmt.Errorf("topP = %v, want 0.9", req.TopP)
				}
				if req.MaxTokens != 1500 {
					return fmt.Errorf("maxTokens = %v, want 1500", req.MaxTokens)
				}
				if req.N != 2 {
					return fmt.Errorf("n = %v, want 2", req.N)
				}
				if len(req.Stop) != 2 {
					return fmt.Errorf("stop length = %v, want 2", len(req.Stop))
				}
				if req.PresencePenalty != 0.5 {
					return fmt.Errorf("presencePenalty = %v, want 0.5", req.PresencePenalty)
				}
				if req.FrequencyPenalty != 0.3 {
					return fmt.Errorf("frequencyPenalty = %v, want 0.3", req.FrequencyPenalty)
				}
				if req.User != "test-user" {
					return fmt.Errorf("user = %v, want test-user", req.User)
				}
				if req.Seed == nil || *req.Seed != 42 {
					return fmt.Errorf("seed = %v, want 42", req.Seed)
				}
				return nil
			},
		},
		{
			name: "json_object schema",
			config: &Config{
				ResponseFormat: &ai.ResponseFormat{
					Type: "json_object",
				},
			},
			check: func(req *openai.ChatCompletionRequest) error {
				if req.ResponseFormat == nil {
					return fmt.Errorf("responseFormat should be set")
				}
				if req.ResponseFormat.Type != openai.ChatCompletionResponseFormatTypeJSONObject {
					return fmt.Errorf("responseFormat type = %v, want json_object", req.ResponseFormat.Type)
				}
				return nil
			},
		},
		{
			name: "json_schema with schema override",
			config: &Config{
				ResponseFormat: &ai.ResponseFormat{
					Type: "json_object",
				},
			},
			schema: &ai.ResponseFormat{
				Type: "json_schema",
				Schema: &jsonschema.Schema{
					Type:       "object",
					Properties: orderedmap.New[string, *jsonschema.Schema](),
				},
			},
			check: func(req *openai.ChatCompletionRequest) error {
				if req.ResponseFormat == nil {
					return fmt.Errorf("responseFormat should be set")
				}
				if req.ResponseFormat.Type != openai.ChatCompletionResponseFormatTypeJSONSchema {
					return fmt.Errorf("responseFormat type = %v, want json_schema", req.ResponseFormat.Type)
				}
				if req.ResponseFormat.JSONSchema == nil {
					return fmt.Errorf("JSONSchema should be set")
				}
				return nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &Client{
				model:  "gpt-3.5-turbo",
				config: tt.config,
			}

			req := &openai.ChatCompletionRequest{
				Model: client.model,
			}

			client.applyChatConfig(req, tt.schema)

			if tt.check != nil {
				if err := tt.check(req); err != nil {
					t.Errorf("applyChatConfig() %v", err)
				}
			}
		})
	}
}

// TestBuildRequestConfig tests the request configuration building
func TestBuildRequestConfig(t *testing.T) {
	client := &Client{
		model: "gpt-3.5-turbo",
		config: &Config{
			Temperature: ai.Float32Ptr(0.7),
			MaxTokens:   ai.IntPtr(100),
		},
	}

	tests := []struct {
		name        string
		input       *ai.ClassifiedInput
		schema      *ai.ResponseFormat
		tools       []tools.Tool
		expectError bool
		checkFunc   func(*RequestConfig) error
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
			checkFunc: func(config *RequestConfig) error {
				if len(config.ChatRequest.Tools) != 1 {
					return fmt.Errorf("expected 1 tool, got %d", len(config.ChatRequest.Tools))
				}
				if config.ChatRequest.ToolChoice != "auto" {
					return fmt.Errorf("expected tool choice 'auto', got %v", config.ChatRequest.ToolChoice)
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
			checkFunc: func(config *RequestConfig) error {
				if config.ChatRequest.ResponseFormat == nil {
					return fmt.Errorf("responseFormat should be set")
				}
				return nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := client.buildRequestConfig(tt.input, tt.schema, tt.tools)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error, got none")
				}
				return
			}

			if err != nil {
				t.Errorf("buildRequestConfig() error = %v", err)
				return
			}

			if tt.checkFunc != nil {
				if err := tt.checkFunc(config); err != nil {
					t.Errorf("buildRequestConfig() %v", err)
				}
			}
		})
	}
}

// TestMultimodalToMessages tests multimodal input conversion
func TestMultimodalToMessages(t *testing.T) {
	client := &Client{
		model:  "gpt-4-vision-preview",
		config: DefaultConfig(),
	}

	tests := []struct {
		name        string
		multimodal  *ai.MultimodalInput
		expectError bool
		checkFunc   func([]openai.ChatCompletionMessage) error
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
			checkFunc: func(messages []openai.ChatCompletionMessage) error {
				if len(messages) != 1 {
					return fmt.Errorf("expected 1 message, got %d", len(messages))
				}
				if len(messages[0].MultiContent) != 1 {
					return fmt.Errorf("expected 1 content part, got %d", len(messages[0].MultiContent))
				}
				if messages[0].MultiContent[0].Type != openai.ChatMessagePartTypeText {
					return fmt.Errorf("expected text part")
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
			checkFunc: func(messages []openai.ChatCompletionMessage) error {
				if len(messages[0].MultiContent) != 1 {
					return fmt.Errorf("expected 1 content part, got %d", len(messages[0].MultiContent))
				}
				if messages[0].MultiContent[0].Type != openai.ChatMessagePartTypeImageURL {
					return fmt.Errorf("expected image URL part")
				}
				if messages[0].MultiContent[0].ImageURL == nil {
					return fmt.Errorf("image URL should not be nil")
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
			checkFunc: func(messages []openai.ChatCompletionMessage) error {
				if len(messages[0].MultiContent) != 1 {
					return fmt.Errorf("expected 1 content part, got %d", len(messages[0].MultiContent))
				}
				if messages[0].MultiContent[0].Type != openai.ChatMessagePartTypeImageURL {
					return fmt.Errorf("expected image URL part")
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
		model:  "gpt-3.5-turbo",
		config: DefaultConfig(),
	}

	tests := []struct {
		name      string
		tools     []tools.Tool
		checkFunc func([]openai.Tool) error
	}{
		{
			name:  "empty tools",
			tools: []tools.Tool{},
			checkFunc: func(tools []openai.Tool) error {
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
			checkFunc: func(tools []openai.Tool) error {
				if len(tools) != 1 {
					return fmt.Errorf("expected 1 tool, got %d", len(tools))
				}
				tool := tools[0]
				if tool.Type != openai.ToolTypeFunction {
					return fmt.Errorf("expected function tool type")
				}
				if tool.Function.Name != "calculator" {
					return fmt.Errorf("expected calculator name, got %s", tool.Function.Name)
				}
				if tool.Function.Description != "Performs calculations" {
					return fmt.Errorf("expected description 'Performs calculations', got %s", tool.Function.Description)
				}
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
			checkFunc: func(tools []openai.Tool) error {
				if len(tools) != 2 {
					return fmt.Errorf("expected 2 tools, got %d", len(tools))
				}
				return nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			openaiTools := client.convertToOpenAITools(tt.tools)

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
		model:  "gpt-3.5-turbo",
		config: DefaultConfig(),
	}

	tests := []struct {
		name      string
		toolCalls []openai.ToolCall
		checkFunc func(string) error
	}{
		{
			name: "single tool call",
			toolCalls: []openai.ToolCall{
				{
					Type: "function",
					Function: openai.FunctionCall{
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
			name: "multiple tool calls",
			toolCalls: []openai.ToolCall{
				{
					Type: "function",
					Function: openai.FunctionCall{
						Name:      "tool1",
						Arguments: `{"input":"test1"}`,
					},
				},
				{
					Type: "function",
					Function: openai.FunctionCall{
						Name:      "tool2",
						Arguments: `{"input":"test2"}`,
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
				if len(calls) != 2 {
					return fmt.Errorf("expected 2 tool calls, got %d", len(calls))
				}
				return nil
			},
		},
		{
			name:      "empty tool calls",
			toolCalls: []openai.ToolCall{},
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

// Mock HTTP server for integration testing
func createMockOpenAIServer(t *testing.T, responses map[string]string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/chat/completions" {
			// Parse request
			var req openai.ChatCompletionRequest
			body, _ := io.ReadAll(r.Body)
			if err := json.Unmarshal(body, &req); err != nil {
				t.Errorf("Failed to decode request: %v", err)
				http.Error(w, "Bad request", 400)
				return
			}

			// Get response based on message content
			var responseContent string
			if len(req.Messages) > 0 {
				if req.Messages[0].Content != "" {
					responseContent = responses[req.Messages[0].Content]
				}
			}
			if responseContent == "" {
				responseContent = "Mock response"
			}

			// Check if streaming is requested
			if req.Stream {
				// Send streaming response
				w.Header().Set("Content-Type", "text/event-stream")
				response := openai.ChatCompletionStreamResponse{
					Choices: []openai.ChatCompletionStreamChoice{
						{
							Delta: openai.ChatCompletionStreamChoiceDelta{
								Content: responseContent,
							},
						},
					},
				}
				data, _ := json.Marshal(response)
				fmt.Fprintf(w, "data: %s\n\n", data)
				fmt.Fprintf(w, "data: [DONE]\n\n")
			} else {
				// Send non-streaming response
				w.Header().Set("Content-Type", "application/json")
				response := openai.ChatCompletionResponse{
					Choices: []openai.ChatCompletionChoice{
						{
							Message: openai.ChatCompletionMessage{
								Role:    openai.ChatMessageRoleAssistant,
								Content: responseContent,
							},
						},
					},
				}
				json.NewEncoder(w).Encode(response)
			}
		} else {
			http.Error(w, "Not found", 404)
		}
	}))
}

// TestChatWithMockServer tests the Chat method with a mock server
func TestChatWithMockServer(t *testing.T) {
	// Create mock server
	responses := map[string]string{
		"Hello":        "Hi there!",
		"What is 2+2?": "The answer is 4.",
	}
	server := createMockOpenAIServer(t, responses)
	defer server.Close()

	// Create client with mock server
	config := &Config{
		APIKey:  "test-key",
		BaseURL: server.URL + "/v1",
		Stream:  ai.BoolPtr(false), // Test non-streaming first
	}
	client, err := New("gpt-3.5-turbo", WithConfig(config))
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple greeting",
			input:    "Hello",
			expected: "Hi there!",
		},
		{
			name:     "math question",
			input:    "What is 2+2?",
			expected: "The answer is 4.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.input)
			req := calque.NewRequest(context.Background(), reader)

			var response strings.Builder
			res := calque.NewResponse(&response)

			err := client.Chat(req, res, nil)
			if err != nil {
				t.Errorf("Chat() error = %v", err)
				return
			}

			result := response.String()
			if result != tt.expected {
				t.Errorf("Chat() = %q, want %q", result, tt.expected)
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
					model:  "gpt-4",
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
