package memory

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/calque-ai/go-calque/pkg/calque"
)

func TestMessage(t *testing.T) {
	tests := []struct {
		name           string
		role           string
		content        []byte
		expectedText   string
		expectedString string
	}{
		{
			name:           "simple text message",
			role:           "user",
			content:        []byte("Hello world"),
			expectedText:   "Hello world",
			expectedString: "user: Hello world",
		},
		{
			name:           "assistant message",
			role:           "assistant",
			content:        []byte("How can I help you?"),
			expectedText:   "How can I help you?",
			expectedString: "assistant: How can I help you?",
		},
		{
			name:           "empty message",
			role:           "system",
			content:        []byte(""),
			expectedText:   "",
			expectedString: "system: ",
		},
		{
			name:           "binary content",
			role:           "user",
			content:        []byte{0x48, 0x65, 0x6c, 0x6c, 0x6f}, // "Hello"
			expectedText:   "Hello",
			expectedString: "user: Hello",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := Message{
				Role:    tt.role,
				Content: tt.content,
			}

			if got := msg.Text(); got != tt.expectedText {
				t.Errorf("Message.Text() = %q, want %q", got, tt.expectedText)
			}

			if got := msg.String(); got != tt.expectedString {
				t.Errorf("Message.String() = %q, want %q", got, tt.expectedString)
			}
		})
	}
}

func TestNewConversation(t *testing.T) {
	conv := NewConversation()

	if conv == nil { //nolint:staticcheck
		t.Error("NewConversation() returned nil")
	}

	if conv.store == nil { //nolint:staticcheck
		t.Error("NewConversation() store is nil")
	}
}

func TestNewConversationWithStore(t *testing.T) {
	store := NewInMemoryStore()
	conv := NewConversationWithStore(store)

	if conv == nil { //nolint:staticcheck
		t.Error("NewConversationWithStore() returned nil")
	}

	if conv.store != store { //nolint:staticcheck
		t.Error("NewConversationWithStore() did not use provided store")
	}
}

func TestConversationMemoryGetConversation(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(*ConversationMemory)
		key      string
		expected []Message
		wantErr  bool
	}{
		{
			name:     "empty conversation",
			key:      "empty",
			expected: []Message{},
			wantErr:  false,
		},
		{
			name: "existing conversation",
			setup: func(cm *ConversationMemory) {
				messages := []Message{
					{Role: "user", Content: []byte("Hello")},
					{Role: "assistant", Content: []byte("Hi there!")},
				}
				cm.saveConversation(context.Background(), "existing", messages)
			},
			key: "existing",
			expected: []Message{
				{Role: "user", Content: []byte("Hello")},
				{Role: "assistant", Content: []byte("Hi there!")},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conv := NewConversation()

			if tt.setup != nil {
				tt.setup(conv)
			}

			ctx := context.Background()
			got, err := conv.getConversation(ctx, tt.key)

			if (err != nil) != tt.wantErr {
				t.Errorf("getConversation() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if len(got) != len(tt.expected) {
				t.Errorf("getConversation() length = %d, want %d", len(got), len(tt.expected))
				return
			}

			for i, msg := range got {
				if msg.Role != tt.expected[i].Role {
					t.Errorf("getConversation()[%d].Role = %q, want %q", i, msg.Role, tt.expected[i].Role)
				}
				if !bytes.Equal(msg.Content, tt.expected[i].Content) {
					t.Errorf("getConversation()[%d].Content = %v, want %v", i, msg.Content, tt.expected[i].Content)
				}
			}
		})
	}
}

func TestConversationMemorySaveConversation(t *testing.T) {
	conv := NewConversation()

	messages := []Message{
		{Role: "user", Content: []byte("Hello")},
		{Role: "assistant", Content: []byte("Hi!")},
	}

	ctx := context.Background()
	err := conv.saveConversation(ctx, "test", messages)
	if err != nil {
		t.Errorf("saveConversation() error = %v", err)
	}

	// Verify it was saved
	retrieved, err := conv.getConversation(ctx, "test")
	if err != nil {
		t.Errorf("getConversation() after save error = %v", err)
	}

	if len(retrieved) != len(messages) {
		t.Errorf("Retrieved conversation length = %d, want %d", len(retrieved), len(messages))
	}
}

func TestConversationMemoryInput(t *testing.T) {
	tests := []struct {
		name           string
		setup          func(*ConversationMemory)
		key            string
		input          string
		expectedOutput string
		wantErr        bool
		errMsg         string
	}{
		{
			name:           "first message in conversation",
			key:            "first",
			input:          "Hello",
			expectedOutput: "user: Hello",
			wantErr:        false,
		},
		{
			name: "message with existing history",
			setup: func(cm *ConversationMemory) {
				messages := []Message{
					{Role: "user", Content: []byte("Previous message")},
					{Role: "assistant", Content: []byte("Previous response")},
				}
				cm.saveConversation(context.Background(), "history", messages)
			},
			key:            "history",
			input:          "New message",
			expectedOutput: "user: Previous message\nassistant: Previous response\nuser: New message",
			wantErr:        false,
		},
		{
			name:    "empty input",
			key:     "empty",
			input:   "",
			wantErr: true,
			errMsg:  "empty input for conversation",
		},
		{
			name:    "whitespace only input",
			key:     "whitespace",
			input:   "   \n\t  ",
			wantErr: true,
			errMsg:  "empty input for conversation",
		},
		{
			name:           "input with leading/trailing whitespace",
			key:            "trimmed",
			input:          "  Hello world  \n",
			expectedOutput: "user: Hello world",
			wantErr:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conv := NewConversation()

			if tt.setup != nil {
				tt.setup(conv)
			}

			handler := conv.Input(tt.key)
			var buf bytes.Buffer
			reader := strings.NewReader(tt.input)

			req := calque.NewRequest(context.Background(), reader)
			res := calque.NewResponse(&buf)
			err := handler.ServeFlow(req, res)

			if (err != nil) != tt.wantErr {
				t.Errorf("Input() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.errMsg != "" {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("Input() error = %v, want error containing %q", err, tt.errMsg)
				}
				return
			}

			if got := buf.String(); got != tt.expectedOutput {
				t.Errorf("Input() output = %q, want %q", got, tt.expectedOutput)
			}

			// Verify message was stored
			if !tt.wantErr {
				ctx := context.Background()
				history, _ := conv.getConversation(ctx, tt.key)
				lastMessage := history[len(history)-1]
				if lastMessage.Role != "user" {
					t.Errorf("Last stored message role = %q, want %q", lastMessage.Role, "user")
				}
				expectedContent := strings.TrimSpace(tt.input)
				if lastMessage.Text() != expectedContent {
					t.Errorf("Last stored message content = %q, want %q", lastMessage.Text(), expectedContent)
				}
			}
		})
	}
}

func TestConversationMemoryOutput(t *testing.T) {
	tests := []struct {
		name           string
		setup          func(*ConversationMemory)
		key            string
		input          string
		expectedOutput string
		wantErr        bool
	}{
		{
			name:           "capture assistant response",
			key:            "response",
			input:          "Hello, how can I help?",
			expectedOutput: "Hello, how can I help?",
			wantErr:        false,
		},
		{
			name: "capture response with existing history",
			setup: func(cm *ConversationMemory) {
				messages := []Message{
					{Role: "user", Content: []byte("Question")},
				}
				cm.saveConversation(context.Background(), "existing", messages)
			},
			key:            "existing",
			input:          "Answer to question",
			expectedOutput: "Answer to question",
			wantErr:        false,
		},
		{
			name:           "empty response",
			key:            "empty",
			input:          "",
			expectedOutput: "",
			wantErr:        false,
		},
		{
			name:           "binary response",
			key:            "binary",
			input:          string([]byte{0x48, 0x65, 0x6c, 0x6c, 0x6f}), // "Hello"
			expectedOutput: "Hello",
			wantErr:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conv := NewConversation()

			if tt.setup != nil {
				tt.setup(conv)
			}

			handler := conv.Output(tt.key)
			var buf bytes.Buffer
			reader := strings.NewReader(tt.input)

			req := calque.NewRequest(context.Background(), reader)
			res := calque.NewResponse(&buf)
			err := handler.ServeFlow(req, res)

			if (err != nil) != tt.wantErr {
				t.Errorf("Output() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if got := buf.String(); got != tt.expectedOutput {
				t.Errorf("Output() output = %q, want %q", got, tt.expectedOutput)
			}

			// Verify response was stored (only if non-empty)
			if !tt.wantErr && tt.input != "" {
				ctx := context.Background()
				history, _ := conv.getConversation(ctx, tt.key)
				if len(history) == 0 {
					t.Error("Expected assistant message to be stored, but history is empty")
					return
				}
				lastMessage := history[len(history)-1]
				if lastMessage.Role != "assistant" {
					t.Errorf("Last stored message role = %q, want %q", lastMessage.Role, "assistant")
				}
				if lastMessage.Text() != tt.input {
					t.Errorf("Last stored message content = %q, want %q", lastMessage.Text(), tt.input)
				}
			}
		})
	}
}

func TestConversationMemoryClear(t *testing.T) {
	conv := NewConversation()

	// Add some conversation history
	messages := []Message{
		{Role: "user", Content: []byte("Hello")},
		{Role: "assistant", Content: []byte("Hi!")},
	}
	ctx := context.Background()
	conv.saveConversation(ctx, "test", messages)

	// Verify it exists
	history, _ := conv.getConversation(ctx, "test")
	if len(history) != 2 {
		t.Errorf("Expected 2 messages before clear, got %d", len(history))
	}

	// Clear the conversation
	err := conv.Clear("test")
	if err != nil {
		t.Errorf("Clear() error = %v", err)
	}

	// Verify it's gone
	history, _ = conv.getConversation(ctx, "test")
	if len(history) != 0 {
		t.Errorf("Expected 0 messages after clear, got %d", len(history))
	}
}

func TestConversationMemoryInfo(t *testing.T) {
	tests := []struct {
		name           string
		setup          func(*ConversationMemory)
		key            string
		expectedCount  int
		expectedExists bool
		wantErr        bool
	}{
		{
			name:           "non-existent conversation",
			key:            "missing",
			expectedCount:  0,
			expectedExists: false,
			wantErr:        false,
		},
		{
			name: "existing conversation",
			setup: func(cm *ConversationMemory) {
				messages := []Message{
					{Role: "user", Content: []byte("Hello")},
					{Role: "assistant", Content: []byte("Hi!")},
					{Role: "user", Content: []byte("How are you?")},
				}
				cm.saveConversation(context.Background(), "existing", messages)
			},
			key:            "existing",
			expectedCount:  3,
			expectedExists: true,
			wantErr:        false,
		},
		{
			name: "empty conversation",
			setup: func(cm *ConversationMemory) {
				cm.saveConversation(context.Background(), "empty", []Message{})
			},
			key:            "empty",
			expectedCount:  0,
			expectedExists: true,
			wantErr:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conv := NewConversation()

			if tt.setup != nil {
				tt.setup(conv)
			}

			ctx := context.Background()
			count, exists, err := conv.Info(ctx, tt.key)

			if (err != nil) != tt.wantErr {
				t.Errorf("Info() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if count != tt.expectedCount {
				t.Errorf("Info() count = %d, want %d", count, tt.expectedCount)
			}

			if exists != tt.expectedExists {
				t.Errorf("Info() exists = %v, want %v", exists, tt.expectedExists)
			}
		})
	}
}

func TestConversationMemoryListKeys(t *testing.T) {
	conv := NewConversation()

	// Initially empty
	keys := conv.ListKeys()
	if len(keys) != 0 {
		t.Errorf("Expected 0 keys initially, got %d", len(keys))
	}

	// Add some conversations
	ctx := context.Background()
	conv.saveConversation(ctx, "conv1", []Message{{Role: "user", Content: []byte("Hello")}})
	conv.saveConversation(ctx, "conv2", []Message{{Role: "user", Content: []byte("Hi")}})
	conv.saveConversation(ctx, "conv3", []Message{{Role: "user", Content: []byte("Hey")}})

	keys = conv.ListKeys()
	if len(keys) != 3 {
		t.Errorf("Expected 3 keys, got %d", len(keys))
	}

	// Check all expected keys are present
	keyMap := make(map[string]bool)
	for _, key := range keys {
		keyMap[key] = true
	}

	expectedKeys := []string{"conv1", "conv2", "conv3"}
	for _, expectedKey := range expectedKeys {
		if !keyMap[expectedKey] {
			t.Errorf("Expected key %q not found in keys list", expectedKey)
		}
	}
}

func TestConversationMemoryFullWorkflow(t *testing.T) {
	conv := NewConversation()
	key := "workflow-test"

	// First user input
	inputHandler := conv.Input(key)
	var buf1 bytes.Buffer
	reader1 := strings.NewReader("Hello, I need help")

	req1 := calque.NewRequest(context.Background(), reader1)
	res1 := calque.NewResponse(&buf1)
	err := inputHandler.ServeFlow(req1, res1)
	if err != nil {
		t.Errorf("First input error = %v", err)
	}

	expected1 := "user: Hello, I need help"
	if got := buf1.String(); got != expected1 {
		t.Errorf("First input output = %q, want %q", got, expected1)
	}

	// Assistant response
	outputHandler := conv.Output(key)
	var buf2 bytes.Buffer
	reader2 := strings.NewReader("Sure, I can help you with that!")

	req2 := calque.NewRequest(context.Background(), reader2)
	res2 := calque.NewResponse(&buf2)
	err = outputHandler.ServeFlow(req2, res2)
	if err != nil {
		t.Errorf("Output error = %v", err)
	}

	expected2 := "Sure, I can help you with that!"
	if got := buf2.String(); got != expected2 {
		t.Errorf("Output = %q, want %q", got, expected2)
	}

	// Second user input should include history
	var buf3 bytes.Buffer
	reader3 := strings.NewReader("Thank you!")

	req3 := calque.NewRequest(context.Background(), reader3)
	res3 := calque.NewResponse(&buf3)
	err = inputHandler.ServeFlow(req3, res3)
	if err != nil {
		t.Errorf("Second input error = %v", err)
	}

	expected3 := "user: Hello, I need help\nassistant: Sure, I can help you with that!\nuser: Thank you!"
	if got := buf3.String(); got != expected3 {
		t.Errorf("Second input output = %q, want %q", got, expected3)
	}

	// Verify conversation state
	ctx := context.Background()
	count, exists, err := conv.Info(ctx, key)
	if err != nil {
		t.Errorf("Info error = %v", err)
	}
	if !exists {
		t.Error("Expected conversation to exist")
	}
	if count != 3 { // 2 user messages + 1 assistant message
		t.Errorf("Expected 3 messages, got %d", count)
	}
}

// Mock store for testing error conditions
type errorStore struct {
	getError    error
	setError    error
	deleteError error
}

func (es *errorStore) Get(_ string) ([]byte, error) {
	return nil, es.getError
}

func (es *errorStore) Set(_ string, _ []byte) error {
	return es.setError
}

func (es *errorStore) Delete(_ string) error {
	return es.deleteError
}

func (es *errorStore) List() []string {
	return []string{}
}

func (es *errorStore) Exists(_ string) bool {
	return false
}

func TestConversationMemoryErrorHandling(t *testing.T) {
	t.Run("input with store get error", func(t *testing.T) {
		conv := NewConversationWithStore(&errorStore{getError: errors.New("get failed")})
		handler := conv.Input("test")

		var buf bytes.Buffer
		reader := strings.NewReader("Hello")

		req := calque.NewRequest(context.Background(), reader)
		res := calque.NewResponse(&buf)
		err := handler.ServeFlow(req, res)
		if err == nil {
			t.Error("Expected error from store get failure")
		}
		if !strings.Contains(err.Error(), "failed to get conversation") {
			t.Errorf("Expected get conversation error, got %v", err)
		}
	})

	t.Run("input with store set error", func(t *testing.T) {
		conv := NewConversationWithStore(&errorStore{setError: errors.New("set failed")})
		handler := conv.Input("test")

		var buf bytes.Buffer
		reader := strings.NewReader("Hello")

		req := calque.NewRequest(context.Background(), reader)
		res := calque.NewResponse(&buf)
		err := handler.ServeFlow(req, res)
		if err == nil {
			t.Error("Expected error from store set failure")
		}
		if !strings.Contains(err.Error(), "failed to save conversation") {
			t.Errorf("Expected save conversation error, got %v", err)
		}
	})

	t.Run("output with store get error", func(t *testing.T) {
		conv := NewConversationWithStore(&errorStore{getError: errors.New("get failed")})
		handler := conv.Output("test")

		var buf bytes.Buffer
		reader := strings.NewReader("Response")

		req := calque.NewRequest(context.Background(), reader)
		res := calque.NewResponse(&buf)
		err := handler.ServeFlow(req, res)
		if err == nil {
			t.Error("Expected error from store get failure")
		}
		if !strings.Contains(err.Error(), "failed to get conversation") {
			t.Errorf("Expected get conversation error, got %v", err)
		}
	})

	t.Run("malformed JSON in store", func(t *testing.T) {
		store := NewInMemoryStore()
		store.Set("bad-json", []byte("invalid json"))

		conv := NewConversationWithStore(store)
		ctx := context.Background()
		_, err := conv.getConversation(ctx, "bad-json")

		if err == nil {
			t.Error("Expected error from malformed JSON")
		}
		if !strings.Contains(err.Error(), "failed to unmarshal conversation") {
			t.Errorf("Expected unmarshal error, got %v", err)
		}
	})
}
