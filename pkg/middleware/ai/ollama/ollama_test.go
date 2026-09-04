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
					Temperature: new(float32(0.8)),
					MaxTokens:   new(1000),
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
		Temperature: new(float32(0.9)),
		MaxTokens:   new(2000),
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
			ctx := context.Background()
			req, err := client.inputToChatRequest(ctx, tt.input, nil)

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

// TestHistoryToMessages pins how each ai.Message role maps to Ollama's
// native []api.Message representation, since this is the mechanism that
// lets the multi-shot tool-calling loop in ai.runAgentLoop drive Ollama.
func TestHistoryToMessages(t *testing.T) {
	client := &Client{}

	tests := []struct {
		name      string
		history   []ai.Message
		expectErr bool
		checkFunc func(t *testing.T, messages []api.Message)
	}{
		{
			name:      "user message",
			history:   []ai.Message{{Role: ai.RoleUser, Content: "What's the weather?"}},
			checkFunc: checkUserTextMessage,
		},
		{
			name: "user message with multimodal image",
			history: []ai.Message{
				{
					Role: ai.RoleUser,
					Multimodal: &ai.MultimodalInput{
						Parts: []ai.ContentPart{
							{Type: "text", Text: "What's in this image?"},
							{Type: "image", Data: []byte("test-image-data"), MimeType: "image/png"},
						},
					},
				},
			},
			checkFunc: checkUserMultimodalMessage,
		},
		{
			name:      "system message",
			history:   []ai.Message{{Role: ai.RoleSystem, Content: "Answer concisely."}},
			checkFunc: checkSystemMessage,
		},
		{
			name:      "assistant message without tool calls",
			history:   []ai.Message{{Role: ai.RoleAssistant, Content: "Sure, one moment."}},
			checkFunc: checkAssistantTextMessage,
		},
		{
			name: "assistant message with tool calls",
			history: []ai.Message{
				{
					Role:      ai.RoleAssistant,
					ToolCalls: []tools.ToolCall{{ID: "call_1", Name: "get_weather", Arguments: `{"city":"nyc"}`}},
				},
			},
			checkFunc: checkAssistantToolCallMessage,
		},
		{
			name:      "tool result message",
			history:   []ai.Message{{Role: ai.RoleTool, ToolCallID: "call_1", ToolName: "get_weather", Content: "72F and sunny"}},
			checkFunc: checkToolResultMessage,
		},
		{
			name: "full tool-calling round trip",
			history: []ai.Message{
				{Role: ai.RoleUser, Content: "What's the weather in NYC?"},
				{Role: ai.RoleAssistant, ToolCalls: []tools.ToolCall{{ID: "call_1", Name: "get_weather", Arguments: `{"city":"nyc"}`}}},
				{Role: ai.RoleTool, ToolCallID: "call_1", ToolName: "get_weather", Content: "72F and sunny"},
			},
			checkFunc: checkFullToolRoundTrip,
		},
		{
			name: "parallel tool calls stay one message per result",
			history: []ai.Message{
				{
					Role: ai.RoleAssistant,
					ToolCalls: []tools.ToolCall{
						{ID: "call_1", Name: "get_status", Arguments: `{}`},
						{ID: "call_2", Name: "get_sensors", Arguments: `{}`},
						{ID: "call_3", Name: "get_video", Arguments: `{}`},
					},
				},
				{Role: ai.RoleTool, ToolCallID: "call_1", ToolName: "get_status", Content: "online"},
				{Role: ai.RoleTool, ToolCallID: "call_2", ToolName: "get_sensors", Content: "nominal"},
				{Role: ai.RoleTool, ToolCallID: "call_3", ToolName: "get_video", Content: "streaming"},
			},
			checkFunc: checkParallelToolCalls,
		},
		{
			name:      "unsupported role",
			history:   []ai.Message{{Role: ai.Role("bogus"), Content: "x"}},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			messages, err := client.historyToMessages(context.Background(), tt.history)

			if tt.expectErr {
				if err == nil {
					t.Fatal("expected error, got none")
				}
				return
			}
			if err != nil {
				t.Fatalf("historyToMessages() error = %v", err)
			}
			if tt.checkFunc != nil {
				tt.checkFunc(t, messages)
			}
		})
	}
}

func checkUserTextMessage(t *testing.T, messages []api.Message) {
	t.Helper()
	if len(messages) != 1 {
		t.Fatalf("len(messages) = %d, want 1", len(messages))
	}
	if messages[0].Role != "user" {
		t.Errorf("role = %v, want user", messages[0].Role)
	}
	if messages[0].Content != "What's the weather?" {
		t.Errorf("content = %v, want \"What's the weather?\"", messages[0].Content)
	}
}

func checkUserMultimodalMessage(t *testing.T, messages []api.Message) {
	t.Helper()
	if len(messages) != 1 {
		t.Fatalf("len(messages) = %d, want 1", len(messages))
	}
	if messages[0].Content != "What's in this image?" {
		t.Errorf("content = %v, want \"What's in this image?\"", messages[0].Content)
	}
	if len(messages[0].Images) != 1 {
		t.Fatalf("len(images) = %d, want 1", len(messages[0].Images))
	}
}

func checkSystemMessage(t *testing.T, messages []api.Message) {
	t.Helper()
	if messages[0].Role != "system" {
		t.Errorf("role = %v, want system", messages[0].Role)
	}
	if messages[0].Content != "Answer concisely." {
		t.Errorf("content = %v, want 'Answer concisely.'", messages[0].Content)
	}
}

func checkAssistantTextMessage(t *testing.T, messages []api.Message) {
	t.Helper()
	if messages[0].Role != "assistant" {
		t.Errorf("role = %v, want assistant", messages[0].Role)
	}
	if messages[0].Content != "Sure, one moment." {
		t.Errorf("content = %v, want 'Sure, one moment.'", messages[0].Content)
	}
	if len(messages[0].ToolCalls) != 0 {
		t.Errorf("ToolCalls = %+v, want none", messages[0].ToolCalls)
	}
}

func checkAssistantToolCallMessage(t *testing.T, messages []api.Message) {
	t.Helper()
	if messages[0].Role != "assistant" {
		t.Errorf("role = %v, want assistant", messages[0].Role)
	}
	if len(messages[0].ToolCalls) != 1 {
		t.Fatalf("len(ToolCalls) = %d, want 1", len(messages[0].ToolCalls))
	}
	call := messages[0].ToolCalls[0]
	if call.ID != "call_1" || call.Function.Name != "get_weather" {
		t.Errorf("ToolCall = %+v, want ID=call_1 Name=get_weather", call)
	}
	city, ok := call.Function.Arguments.Get("city")
	if !ok || city != "nyc" {
		t.Errorf("Arguments[city] = %v, ok=%v, want nyc, true", city, ok)
	}
}

func checkToolResultMessage(t *testing.T, messages []api.Message) {
	t.Helper()
	if messages[0].Role != "tool" {
		t.Errorf("role = %v, want tool", messages[0].Role)
	}
	if messages[0].ToolCallID != "call_1" {
		t.Errorf("ToolCallID = %v, want call_1", messages[0].ToolCallID)
	}
	if messages[0].ToolName != "get_weather" {
		t.Errorf("ToolName = %v, want get_weather", messages[0].ToolName)
	}
	if messages[0].Content != "72F and sunny" {
		t.Errorf("content = %v, want '72F and sunny'", messages[0].Content)
	}
}

// checkParallelToolCalls pins that Ollama's []api.Message list format -
// unlike Gemini's, which requires all FunctionResponse parts for a
// parallel-call turn batched into a single Content (see
// gemini.checkParallelToolResultsFold) - accepts one tool-role message per
// result with no folding: historyToMessages stays a strict 1:1
// index-preserving conversion, matched by each message's own ToolCallID.
func checkParallelToolCalls(t *testing.T, messages []api.Message) {
	t.Helper()
	if len(messages) != 4 {
		t.Fatalf("len(messages) = %d, want 4 (1 assistant turn + 3 tool-result messages)", len(messages))
	}
	if messages[0].Role != "assistant" || len(messages[0].ToolCalls) != 3 {
		t.Fatalf("messages[0] = role=%v ToolCalls=%d, want assistant with 3 ToolCalls", messages[0].Role, len(messages[0].ToolCalls))
	}

	wantIDs := []string{"call_1", "call_2", "call_3"}
	wantNames := []string{"get_status", "get_sensors", "get_video"}
	wantContent := []string{"online", "nominal", "streaming"}
	for i, wantID := range wantIDs {
		msg := messages[i+1]
		if msg.Role != "tool" {
			t.Errorf("messages[%d].Role = %v, want tool", i+1, msg.Role)
		}
		if msg.ToolCallID != wantID || msg.ToolName != wantNames[i] {
			t.Errorf("messages[%d] = ToolCallID=%v ToolName=%v, want ID=%s Name=%s", i+1, msg.ToolCallID, msg.ToolName, wantID, wantNames[i])
		}
		if msg.Content != wantContent[i] {
			t.Errorf("messages[%d].Content = %v, want %s", i+1, msg.Content, wantContent[i])
		}
	}
}

func checkFullToolRoundTrip(t *testing.T, messages []api.Message) {
	t.Helper()
	if len(messages) != 3 {
		t.Fatalf("len(messages) = %d, want 3", len(messages))
	}
	wantRoles := []string{"user", "assistant", "tool"}
	for i, want := range wantRoles {
		if messages[i].Role != want {
			t.Errorf("messages[%d].Role = %v, want %v", i, messages[i].Role, want)
		}
	}
}

// TestAssistantMessage tests the standalone assistantMessage helper directly.
func TestAssistantMessage(t *testing.T) {
	tests := []struct {
		name      string
		msg       ai.Message
		checkFunc func(t *testing.T, m api.Message)
	}{
		{
			name: "no tool calls",
			msg:  ai.Message{Role: ai.RoleAssistant, Content: "hello"},
			checkFunc: func(t *testing.T, m api.Message) {
				t.Helper()
				if m.Content != "hello" {
					t.Errorf("content = %v, want hello", m.Content)
				}
				if len(m.ToolCalls) != 0 {
					t.Errorf("ToolCalls = %+v, want none", m.ToolCalls)
				}
			},
		},
		{
			name: "single tool call",
			msg: ai.Message{
				Role:      ai.RoleAssistant,
				ToolCalls: []tools.ToolCall{{ID: "call_1", Name: "calculator", Arguments: `{"input":"2+2"}`}},
			},
			checkFunc: func(t *testing.T, m api.Message) {
				t.Helper()
				if len(m.ToolCalls) != 1 {
					t.Fatalf("len(ToolCalls) = %d, want 1", len(m.ToolCalls))
				}
				input, ok := m.ToolCalls[0].Function.Arguments.Get("input")
				if !ok || input != "2+2" {
					t.Errorf("Arguments[input] = %v, ok=%v, want 2+2, true", input, ok)
				}
			},
		},
		{
			name: "multiple tool calls preserve order",
			msg: ai.Message{
				Role: ai.RoleAssistant,
				ToolCalls: []tools.ToolCall{
					{ID: "call_1", Name: "first", Arguments: `{}`},
					{ID: "call_2", Name: "second", Arguments: `{}`},
				},
			},
			checkFunc: func(t *testing.T, m api.Message) {
				t.Helper()
				if len(m.ToolCalls) != 2 {
					t.Fatalf("len(ToolCalls) = %d, want 2", len(m.ToolCalls))
				}
				if m.ToolCalls[0].Function.Name != "first" || m.ToolCalls[1].Function.Name != "second" {
					t.Errorf("ToolCalls order = %+v, want [first, second]", m.ToolCalls)
				}
			},
		},
		{
			name: "malformed arguments JSON does not panic",
			msg: ai.Message{
				Role:      ai.RoleAssistant,
				ToolCalls: []tools.ToolCall{{ID: "call_1", Name: "broken", Arguments: `not json`}},
			},
			checkFunc: func(t *testing.T, m api.Message) {
				t.Helper()
				if len(m.ToolCalls) != 1 {
					t.Fatalf("len(ToolCalls) = %d, want 1", len(m.ToolCalls))
				}
				if m.ToolCalls[0].Function.Arguments.Len() != 0 {
					t.Errorf("Arguments.Len() = %d, want 0 for malformed JSON", m.ToolCalls[0].Function.Arguments.Len())
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.checkFunc(t, assistantMessage(tt.msg))
		})
	}
}

// TestBuildRequestConfig pins that ToolsDisabled omits Tools entirely -
// Ollama has no OpenAI-style tool_choice/Gemini-style
// FunctionCallingConfigModeNone mechanism to keep tools declared but
// disabled, confirmed via SDK source and official docs, so omitting Tools
// is the only option available.
func TestBuildRequestConfig(t *testing.T) {
	client := &Client{
		model:  "test-model",
		config: DefaultConfig(),
	}

	tool := tools.Simple("calculator", "Performs calculations", func(_ string) string { return "42" })
	input := &ai.ClassifiedInput{Type: ai.TextInput, Text: "Hello"}
	ctx := context.Background()

	t.Run("tools enabled", func(t *testing.T) {
		config, err := client.buildRequestConfig(ctx, input, nil, []tools.Tool{tool}, nil, false)
		if err != nil {
			t.Fatalf("buildRequestConfig() error = %v", err)
		}
		if len(config.ChatRequest.Tools) != 1 {
			t.Errorf("Tools = %d entries, want 1", len(config.ChatRequest.Tools))
		}
	})

	t.Run("tools disabled omits Tools entirely (Ollama has no declared-but-disabled mechanism)", func(t *testing.T) {
		config, err := client.buildRequestConfig(ctx, input, nil, []tools.Tool{tool}, nil, true)
		if err != nil {
			t.Fatalf("buildRequestConfig() error = %v", err)
		}
		if len(config.ChatRequest.Tools) != 0 {
			t.Errorf("Tools = %d entries, want 0", len(config.ChatRequest.Tools))
		}
	})

	t.Run("no tools and disabled", func(t *testing.T) {
		config, err := client.buildRequestConfig(ctx, input, nil, nil, nil, true)
		if err != nil {
			t.Fatalf("buildRequestConfig() error = %v", err)
		}
		if len(config.ChatRequest.Tools) != 0 {
			t.Errorf("Tools = %d entries, want 0", len(config.ChatRequest.Tools))
		}
	})

	t.Run("schema applies", func(t *testing.T) {
		schema := &ai.ResponseFormat{Type: "json_object"}
		config, err := client.buildRequestConfig(ctx, input, schema, nil, nil, false)
		if err != nil {
			t.Fatalf("buildRequestConfig() error = %v", err)
		}
		if config.ChatRequest.Format == nil {
			t.Error("Format should be set from schema")
		}
	})

	t.Run("history builds messages and tools still apply", func(t *testing.T) {
		history := []ai.Message{{Role: ai.RoleUser, Content: "hi"}}
		config, err := client.buildRequestConfig(ctx, input, nil, []tools.Tool{tool}, history, false)
		if err != nil {
			t.Fatalf("buildRequestConfig() error = %v", err)
		}
		if len(config.ChatRequest.Messages) != 1 || config.ChatRequest.Messages[0].Content != "hi" {
			t.Errorf("Messages = %+v, want built from history", config.ChatRequest.Messages)
		}
		if len(config.ChatRequest.Tools) != 1 {
			t.Errorf("Tools = %d entries, want 1", len(config.ChatRequest.Tools))
		}
	})
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
				Temperature: new(float32(0.8)),
				TopP:        new(float32(0.9)),
				MaxTokens:   new(1500),
				Stop:        []string{"END", "STOP"},
				KeepAlive:   "10m",
				Stream:      new(false),
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
				Temperature: new(float32(0.7)),
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
	ctx := context.Background()
	ollamaTools := client.convertToOllamaTools(ctx, []tools.Tool{tool})

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

func makeToolArgs(m map[string]any) api.ToolCallFunctionArguments {
	args := api.NewToolCallFunctionArguments()
	for k, v := range m {
		args.Set(k, v)
	}
	return args
}

func TestWriteOllamaToolCalls(t *testing.T) {
	client := &Client{}

	toolCalls := []api.ToolCall{
		{
			ID: "call_abc",
			Function: api.ToolCallFunction{
				Name:      "calculator",
				Arguments: makeToolArgs(map[string]any{"input": "2+2"}),
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
	toolCallsRaw, ok := jsonResult["tool_calls"].([]any)
	if !ok || len(toolCallsRaw) != 1 {
		t.Fatalf("writeOllamaToolCalls() tool_calls = %v, want a 1-element array", jsonResult["tool_calls"])
	}
	entry, ok := toolCallsRaw[0].(map[string]any)
	if !ok {
		t.Fatalf("tool_calls[0] = %v, want an object", toolCallsRaw[0])
	}
	if entry["id"] != "call_abc" {
		t.Errorf("tool_calls[0][\"id\"] = %v, want call_abc", entry["id"])
	}

	// Verify it contains expected tool call structure
	if !strings.Contains(result, "calculator") {
		t.Error("writeOllamaToolCalls() should contain calculator tool name")
	}

	if !strings.Contains(result, "2+2") {
		t.Error("writeOllamaToolCalls() should contain tool arguments")
	}
}

// TestWriteOllamaToolCallsEmptyID pins that an unset ToolCall.ID round-trips
// as an empty string without erroring - the shared tools.parseJSONToolCalls
// generates a real ID downstream in that case, this function's job is just
// to pass through whatever Ollama supplied.
func TestWriteOllamaToolCallsEmptyID(t *testing.T) {
	client := &Client{}

	toolCalls := []api.ToolCall{
		{
			Function: api.ToolCallFunction{
				Name:      "calculator",
				Arguments: makeToolArgs(map[string]any{"input": "2+2"}),
			},
		},
	}

	var response strings.Builder
	w := calque.NewResponse(&response)

	if err := client.writeOllamaToolCalls(toolCalls, w); err != nil {
		t.Fatalf("writeOllamaToolCalls() error = %v", err)
	}

	var jsonResult map[string]any
	if err := json.Unmarshal([]byte(response.String()), &jsonResult); err != nil {
		t.Fatalf("writeOllamaToolCalls() produced invalid JSON: %v", err)
	}

	toolCallsRaw := jsonResult["tool_calls"].([]any)
	entry := toolCallsRaw[0].(map[string]any)
	if entry["id"] != "" {
		t.Errorf("tool_calls[0][\"id\"] = %v, want empty string", entry["id"])
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

// TestChatIntegrationWithHistory pins that AgentOptions.History reaches
// Ollama as the full message list, end-to-end through Chat - not just
// historyToMessages in isolation. Uses a dedicated mock server that captures
// the decoded request, since createMockOllamaServer's response lookup is
// keyed on a single-turn input and doesn't fit a history-shape assertion.
func TestChatIntegrationWithHistory(t *testing.T) {
	var captured api.ChatRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Errorf("failed to decode request: %v", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/x-ndjson")
		_ = json.NewEncoder(w).Encode(api.ChatResponse{
			Message: api.Message{Role: "assistant", Content: "72F and sunny"},
		})
	}))
	defer server.Close()

	client, err := New("test-model", WithConfig(&Config{Host: server.URL}))
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	history := []ai.Message{
		{Role: ai.RoleUser, Content: "What's the weather in NYC?"},
		{Role: ai.RoleAssistant, ToolCalls: []tools.ToolCall{{ID: "call_1", Name: "get_weather", Arguments: `{"city":"nyc"}`}}},
		{Role: ai.RoleTool, ToolCallID: "call_1", ToolName: "get_weather", Content: "72F and sunny"},
	}

	req := calque.NewRequest(context.Background(), strings.NewReader(""))
	var response strings.Builder
	res := calque.NewResponse(&response)

	if err := client.Chat(req, res, &ai.AgentOptions{History: history}); err != nil {
		t.Fatalf("Chat() error = %v", err)
	}

	if len(captured.Messages) != len(history) {
		t.Fatalf("server received %d messages, want %d", len(captured.Messages), len(history))
	}
	wantRoles := []string{"user", "assistant", "tool"}
	for i, want := range wantRoles {
		if captured.Messages[i].Role != want {
			t.Errorf("Messages[%d].Role = %v, want %v", i, captured.Messages[i].Role, want)
		}
	}
}

// TestExecuteRequestScenarios tests different response scenarios
func TestExecuteRequestScenarios(t *testing.T) {
	tests := []struct {
		name           string
		toolCalls      []api.ToolCall
		bufferedText   string
		hasTools       bool
		hasFormat      bool
		expectedOutput string
		expectJSON     bool
		description    string
	}{
		{
			name:           "just text, no tools",
			toolCalls:      nil,
			bufferedText:   "",
			hasTools:       false,
			hasFormat:      false,
			expectedOutput: "",
			expectJSON:     false,
			description:    "Plain text response with no tools should stream directly",
		},
		{
			name:           "tools available but not used - text response",
			toolCalls:      nil,
			bufferedText:   "This is a plain text answer",
			hasTools:       true,
			hasFormat:      false,
			expectedOutput: "This is a plain text answer",
			expectJSON:     false,
			description:    "When tools are available but not called, buffered text should be written",
		},
		{
			name: "tool calls only",
			toolCalls: []api.ToolCall{
				{
					Function: api.ToolCallFunction{
						Name:      "calculator",
						Arguments: makeToolArgs(map[string]any{"input": "2+2"}),
					},
				},
			},
			bufferedText:   "",
			hasTools:       true,
			hasFormat:      false,
			expectedOutput: "",
			expectJSON:     true,
			description:    "Tool calls should be formatted as JSON",
		},
		{
			name: "tool calls with buffered text (text ignored)",
			toolCalls: []api.ToolCall{
				{
					Function: api.ToolCallFunction{
						Name:      "search",
						Arguments: makeToolArgs(map[string]any{"query": "golang"}),
					},
				},
			},
			bufferedText:   "Let me search for that...",
			hasTools:       true,
			hasFormat:      false,
			expectedOutput: "",
			expectJSON:     true,
			description:    "Tool calls take precedence, buffered text ignored",
		},
		{
			name:           "JSON format response",
			toolCalls:      nil,
			bufferedText:   `{"result": "success"}`,
			hasTools:       false,
			hasFormat:      true,
			expectedOutput: `{"result": "success"}`,
			expectJSON:     false,
			description:    "JSON format should clean and write response",
		},
		{
			name:           "JSON format with markdown",
			toolCalls:      nil,
			bufferedText:   "```json\n{\"result\": \"success\"}\n```",
			hasTools:       false,
			hasFormat:      true,
			expectedOutput: `{"result": "success"}`,
			expectJSON:     false,
			description:    "JSON format should clean markdown from response",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &Client{}
			var response strings.Builder
			w := calque.NewResponse(&response)

			// Simulate the finalization logic from executeRequest
			switch {
			case len(tt.toolCalls) > 0:
				err := client.writeOllamaToolCalls(tt.toolCalls, w)
				if err != nil {
					t.Errorf("%s: writeOllamaToolCalls() error = %v", tt.description, err)
					return
				}
			case tt.hasFormat && tt.bufferedText != "":
				cleaned := client.cleanFullJSONResponse(tt.bufferedText)
				_, err := w.Data.Write([]byte(cleaned))
				if err != nil {
					t.Errorf("%s: write error = %v", tt.description, err)
					return
				}
			case tt.bufferedText != "":
				_, err := w.Data.Write([]byte(tt.bufferedText))
				if err != nil {
					t.Errorf("%s: write error = %v", tt.description, err)
					return
				}
			}

			output := response.String()

			if tt.expectJSON {
				// Should be valid JSON with tool_calls
				var result struct {
					ToolCalls []any `json:"tool_calls"`
				}
				if err := json.Unmarshal([]byte(output), &result); err != nil {
					t.Errorf("%s: output is not valid JSON: %v\nOutput: %s", tt.description, err, output)
					return
				}
				if len(result.ToolCalls) == 0 {
					t.Errorf("%s: expected tool calls in JSON output", tt.description)
				}
			} else if tt.expectedOutput != "" {
				// Check text content
				if output != tt.expectedOutput {
					t.Errorf("%s: output = %q, want %q", tt.description, output, tt.expectedOutput)
				}
			}
		})
	}
}

// TestBufferingBehavior tests the buffering decision logic
func TestBufferingBehavior(t *testing.T) {
	tests := []struct {
		name         string
		hasTools     bool
		hasFormat    bool
		shouldBuffer bool
		description  string
	}{
		{
			name:         "no tools, no format",
			hasTools:     false,
			hasFormat:    false,
			shouldBuffer: false,
			description:  "Should stream when no tools or format",
		},
		{
			name:         "tools present",
			hasTools:     true,
			hasFormat:    false,
			shouldBuffer: true,
			description:  "Should buffer when tools are available",
		},
		{
			name:         "format present",
			hasTools:     false,
			hasFormat:    true,
			shouldBuffer: true,
			description:  "Should buffer when JSON format requested",
		},
		{
			name:         "both tools and format",
			hasTools:     true,
			hasFormat:    true,
			shouldBuffer: true,
			description:  "Should buffer when both present",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate the buffering logic from executeRequest
			chatRequest := &api.ChatRequest{
				Tools: []api.Tool{},
			}

			if tt.hasTools {
				chatRequest.Tools = []api.Tool{
					{
						Type: "function",
						Function: api.ToolFunction{
							Name: "test_tool",
						},
					},
				}
			}

			if tt.hasFormat {
				format := json.RawMessage(`"json"`)
				chatRequest.Format = format
			}

			shouldBuffer := len(chatRequest.Tools) > 0 || chatRequest.Format != nil

			if shouldBuffer != tt.shouldBuffer {
				t.Errorf("%s: shouldBuffer = %v, want %v", tt.description, shouldBuffer, tt.shouldBuffer)
			}
		})
	}
}

// TestToolCallPriority tests that tool calls take priority over text
func TestToolCallPriority(t *testing.T) {
	client := &Client{}

	// Simulate scenario where both text and tool calls are present
	toolCalls := []api.ToolCall{
		{
			Function: api.ToolCallFunction{
				Name:      "calculator",
				Arguments: makeToolArgs(map[string]any{"expression": "2+2"}),
			},
		},
	}

	var response strings.Builder
	w := calque.NewResponse(&response)

	// Write tool calls (this should happen, text should be ignored)
	err := client.writeOllamaToolCalls(toolCalls, w)
	if err != nil {
		t.Fatalf("writeOllamaToolCalls() error = %v", err)
	}

	output := response.String()

	// Verify it's JSON tool calls
	var result struct {
		ToolCalls []any `json:"tool_calls"`
	}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("Output should be valid JSON tool calls: %v\nOutput: %s", err, output)
	}

	// Verify no text content mixed in
	if strings.Contains(output, "Let me") || strings.Contains(output, "thinking") {
		t.Error("Tool calls output should not contain text content")
	}
}
