package ollama

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ollama/ollama/api"

	"github.com/calque-ai/go-calque/pkg/calque"
	"github.com/calque-ai/go-calque/pkg/middleware/ai"
	"github.com/calque-ai/go-calque/pkg/middleware/tools"
	"github.com/calque-ai/go-calque/pkg/utils"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name        string
		model       string
		opts        []Option
		wantModel   string
		expectError bool
	}{
		{
			name:      "default model",
			model:     "",
			wantModel: "llama3.2",
		},
		{
			name:      "custom model",
			model:     "mistral",
			wantModel: "mistral",
		},
		{
			name:  "custom config",
			model: "llama3.2",
			opts: []Option{
				WithConfig(&Config{
					Temperature: utils.Float32Ptr(0.8),
					MaxTokens:   utils.IntPtr(1000),
				}),
			},
			wantModel: "llama3.2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := New(tt.model, tt.opts...)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error, got none")
				}
				return
			}

			if err != nil {
				t.Errorf("New() error = %v", err)
				return
			}

			if client.model != tt.wantModel {
				t.Errorf("New() model = %v, want %v", client.model, tt.wantModel)
			}

			if client.config == nil {
				t.Error("New() config should not be nil")
			}
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config == nil {
		t.Fatal("DefaultConfig() should not return nil")
	}

	if config.Temperature == nil || *config.Temperature != 0.7 {
		t.Error("DefaultConfig() should set temperature to 0.7")
	}

	if config.KeepAlive != "5m" {
		t.Errorf("DefaultConfig() KeepAlive = %v, want 5m", config.KeepAlive)
	}

	if config.Stream == nil || !*config.Stream {
		t.Error("DefaultConfig() should enable streaming by default")
	}
}

func TestWithConfig(t *testing.T) {
	customConfig := &Config{
		Temperature: utils.Float32Ptr(0.9),
		MaxTokens:   utils.IntPtr(2000),
	}

	option := WithConfig(customConfig)

	// Test applying the option
	config := &Config{}
	option.Apply(config)

	if config.Temperature == nil || *config.Temperature != 0.9 {
		t.Error("WithConfig() should apply custom temperature")
	}

	if config.MaxTokens == nil || *config.MaxTokens != 2000 {
		t.Error("WithConfig() should apply custom MaxTokens")
	}
}

func TestInputToChatRequest(t *testing.T) {
	client := &Client{
		model:  "test-model",
		config: DefaultConfig(),
	}

	tests := []struct {
		name        string
		input       *ai.ClassifiedInput
		expectError bool
		checkFunc   func(*api.ChatRequest) error
	}{
		{
			name: "text input",
			input: &ai.ClassifiedInput{
				Type: ai.TextInput,
				Text: "Hello, world!",
			},
			checkFunc: func(req *api.ChatRequest) error {
				if req.Model != "test-model" {
					return fmt.Errorf("model = %v, want test-model", req.Model)
				}
				if len(req.Messages) != 1 {
					return fmt.Errorf("messages length = %v, want 1", len(req.Messages))
				}
				if req.Messages[0].Role != "user" {
					return fmt.Errorf("message role = %v, want user", req.Messages[0].Role)
				}
				if req.Messages[0].Content != "Hello, world!" {
					return fmt.Errorf("message content = %v, want 'Hello, world!'", req.Messages[0].Content)
				}
				return nil
			},
		},
		{
			name: "multimodal input with text",
			input: &ai.ClassifiedInput{
				Type: ai.MultimodalJSONInput,
				Multimodal: &ai.MultimodalInput{
					Parts: []ai.ContentPart{
						{Type: "text", Text: "What's in this image?"},
					},
				},
			},
			checkFunc: func(req *api.ChatRequest) error {
				if len(req.Messages) != 1 {
					return fmt.Errorf("messages length = %v, want 1", len(req.Messages))
				}
				if req.Messages[0].Content != "What's in this image?" {
					return fmt.Errorf("message content = %v, want 'What's in this image?'", req.Messages[0].Content)
				}
				return nil
			},
		},
		{
			name: "multimodal input with image",
			input: &ai.ClassifiedInput{
				Type: ai.MultimodalJSONInput,
				Multimodal: &ai.MultimodalInput{
					Parts: []ai.ContentPart{
						{Type: "text", Text: "Analyze this image"},
						{Type: "image", Data: []byte("fake-image-data"), MimeType: "image/jpeg"},
					},
				},
			},
			checkFunc: func(req *api.ChatRequest) error {
				if len(req.Messages) != 1 {
					return fmt.Errorf("messages length = %v, want 1", len(req.Messages))
				}
				if req.Messages[0].Content != "Analyze this image" {
					return fmt.Errorf("message content = %v, want 'Analyze this image'", req.Messages[0].Content)
				}
				if len(req.Messages[0].Images) != 1 {
					return fmt.Errorf("images length = %v, want 1", len(req.Messages[0].Images))
				}
				if string(req.Messages[0].Images[0]) != "fake-image-data" {
					return fmt.Errorf("image data = %v, want 'fake-image-data'", string(req.Messages[0].Images[0]))
				}
				return nil
			},
		},
		{
			name: "unsupported audio content",
			input: &ai.ClassifiedInput{
				Type: ai.MultimodalJSONInput,
				Multimodal: &ai.MultimodalInput{
					Parts: []ai.ContentPart{
						{Type: "audio", Data: []byte("fake-audio-data"), MimeType: "audio/wav"},
					},
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := client.inputToChatRequest(tt.input)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error, got none")
				}
				return
			}

			if err != nil {
				t.Errorf("inputToChatRequest() error = %v", err)
				return
			}

			if tt.checkFunc != nil {
				if err := tt.checkFunc(req); err != nil {
					t.Errorf("inputToChatRequest() %v", err)
				}
			}
		})
	}
}

func TestApplyChatConfig(t *testing.T) {
	tests := []struct {
		name   string
		config *Config
		schema *ai.ResponseFormat
		check  func(*api.ChatRequest) error
	}{
		{
			name: "basic config",
			config: &Config{
				Temperature: utils.Float32Ptr(0.8),
				TopP:        utils.Float32Ptr(0.9),
				MaxTokens:   utils.IntPtr(1500),
				Stop:        []string{"END", "STOP"},
				KeepAlive:   "10m",
				Stream:      utils.BoolPtr(false),
			},
			check: func(req *api.ChatRequest) error {
				if temp, ok := req.Options["temperature"]; !ok || temp != float32(0.8) {
					return fmt.Errorf("temperature = %v, want 0.8", temp)
				}
				if topP, ok := req.Options["top_p"]; !ok || topP != float32(0.9) {
					return fmt.Errorf("top_p = %v, want 0.9", topP)
				}
				if maxTokens, ok := req.Options["num_predict"]; !ok || maxTokens != 1500 {
					return fmt.Errorf("num_predict = %v, want 1500", maxTokens)
				}
				if stop, ok := req.Options["stop"]; !ok {
					return fmt.Errorf("stop should be set")
				} else if len(stop.([]string)) != 2 {
					return fmt.Errorf("stop length = %v, want 2", len(stop.([]string)))
				}
				if keepAlive, ok := req.Options["keep_alive"]; !ok || keepAlive != "10m" {
					return fmt.Errorf("keep_alive = %v, want 10m", keepAlive)
				}
				if req.Stream == nil || *req.Stream {
					return fmt.Errorf("stream = %v, want false", req.Stream)
				}
				return nil
			},
		},
		{
			name: "json_object schema",
			schema: &ai.ResponseFormat{
				Type: "json_object",
			},
			check: func(req *api.ChatRequest) error {
				if req.Format == nil {
					return fmt.Errorf("format should be set for json_object")
				}
				expected := json.RawMessage(`"json"`)
				if string(req.Format) != string(expected) {
					return fmt.Errorf("format = %v, want %v", string(req.Format), string(expected))
				}
				return nil
			},
		},
		{
			name: "custom options override",
			config: &Config{
				Temperature: utils.Float32Ptr(0.7),
				Options: map[string]any{
					"temperature":   float32(0.9), // Should override the Temperature field
					"custom_option": "test",
				},
			},
			check: func(req *api.ChatRequest) error {
				if temp, ok := req.Options["temperature"]; !ok || temp != float32(0.9) {
					return fmt.Errorf("temperature = %v, want 0.9 (from Options)", temp)
				}
				if custom, ok := req.Options["custom_option"]; !ok || custom != "test" {
					return fmt.Errorf("custom_option = %v, want test", custom)
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

			req := &api.ChatRequest{
				Options: make(map[string]any),
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

func TestConvertToOllamaTools(t *testing.T) {
	// Create a simple mock tool
	tool := tools.Simple("calculator", "Performs calculations", func(_ string) string {
		return "result"
	})

	client := &Client{}
	ollamaTools := client.convertToOllamaTools([]tools.Tool{tool})

	if len(ollamaTools) != 1 {
		t.Fatalf("convertToOllamaTools() returned %d tools, want 1", len(ollamaTools))
	}

	ollamaTool := ollamaTools[0]
	if ollamaTool.Type != "function" {
		t.Errorf("tool type = %v, want function", ollamaTool.Type)
	}

	if ollamaTool.Function.Name != "calculator" {
		t.Errorf("tool name = %v, want calculator", ollamaTool.Function.Name)
	}

	if ollamaTool.Function.Description != "Performs calculations" {
		t.Errorf("tool description = %v, want 'Performs calculations'", ollamaTool.Function.Description)
	}

	if ollamaTool.Function.Parameters.Type != "object" {
		t.Errorf("parameters type = %v, want object", ollamaTool.Function.Parameters.Type)
	}
}

func TestWriteOllamaToolCalls(t *testing.T) {
	client := &Client{}

	toolCalls := []api.ToolCall{
		{
			Function: api.ToolCallFunction{
				Name: "calculator",
				Arguments: map[string]any{
					"input": "2+2",
				},
			},
		},
	}

	var response strings.Builder
	w := calque.NewResponse(&response)

	err := client.writeOllamaToolCalls(toolCalls, w)
	if err != nil {
		t.Errorf("writeOllamaToolCalls() error = %v", err)
		return
	}

	result := response.String()

	// Should be valid JSON
	var jsonResult map[string]any
	if err := json.Unmarshal([]byte(result), &jsonResult); err != nil {
		t.Errorf("writeOllamaToolCalls() produced invalid JSON: %v", err)
		return
	}

	// Check structure
	if _, ok := jsonResult["tool_calls"]; !ok {
		t.Error("writeOllamaToolCalls() should include tool_calls field")
	}

	// Verify it contains expected tool call structure
	if !strings.Contains(result, "calculator") {
		t.Error("writeOllamaToolCalls() should contain calculator tool name")
	}

	if !strings.Contains(result, "2+2") {
		t.Error("writeOllamaToolCalls() should contain tool arguments")
	}
}

func TestCleanFullJSONResponse(t *testing.T) {
	client := &Client{}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "clean JSON",
			input:    `{"result": "success"}`,
			expected: `{"result": "success"}`,
		},
		{
			name:     "JSON with markdown",
			input:    "```json\n{\"result\": \"success\"}\n```",
			expected: `{"result": "success"}`,
		},
		{
			name:     "JSON with explanation",
			input:    `{"result": "success"} Analysis: This is the result`,
			expected: `{"result": "success"}`,
		},
		{
			name:     "JSON with whitespace",
			input:    "  \n  {\"result\": \"success\"}  \n  ",
			expected: `{"result": "success"}`,
		},
		{
			name:     "complex JSON with trailing content",
			input:    "```json\n{\"name\": \"test\", \"value\": 42}\n```\nThis explains the output",
			expected: `{"name": "test", "value": 42}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := client.cleanFullJSONResponse(tt.input)
			if result != tt.expected {
				t.Errorf("cleanFullJSONResponse() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// Mock HTTP server for integration testing
func createMockOllamaServer(t *testing.T, responses map[string]string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/chat" {
			// Parse request
			var req api.ChatRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Errorf("Failed to decode request: %v", err)
				http.Error(w, "Bad request", 400)
				return
			}

			// Get response based on message content
			var responseContent string
			if len(req.Messages) > 0 {
				responseContent = responses[req.Messages[0].Content]
			}
			if responseContent == "" {
				responseContent = "Mock response"
			}

			// Send streaming response
			w.Header().Set("Content-Type", "application/x-ndjson")
			response := api.ChatResponse{
				Message: api.Message{
					Role:    "assistant",
					Content: responseContent,
				},
			}
			json.NewEncoder(w).Encode(response)
		} else {
			http.Error(w, "Not found", 404)
		}
	}))
}

func TestChatIntegration(t *testing.T) {
	// Create mock server
	responses := map[string]string{
		"Hello":        "Hi there!",
		"What is 2+2?": "4",
	}
	server := createMockOllamaServer(t, responses)
	defer server.Close()

	// Create client with mock server
	client, err := New("test-model", WithConfig(&Config{
		Host: server.URL,
	}))
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple text",
			input:    "Hello",
			expected: "Hi there!",
		},
		{
			name:     "question",
			input:    "What is 2+2?",
			expected: "4",
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
