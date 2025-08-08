package prompt

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"text/template"

	"github.com/calque-ai/calque-pipe/pkg/core"
)

func TestTemplate(t *testing.T) {
	tests := []struct {
		name        string
		template    string
		data        map[string]any
		input       string
		expected    string
		wantErr     bool
		errorSubstr string
	}{
		{
			name:     "basic template with input",
			template: "Question: {{.Input}}",
			input:    "How do I sort an array?",
			expected: "Question: How do I sort an array?",
			wantErr:  false,
		},
		{
			name:     "template with additional data",
			template: "Role: {{.Role}}\nQuestion: {{.Input}}",
			data:     map[string]any{"Role": "coding expert"},
			input:    "How do I sort an array?",
			expected: "Role: coding expert\nQuestion: How do I sort an array?",
			wantErr:  false,
		},
		{
			name:     "assistant prompt template",
			template: "You are a helpful assistant. User: {{.Input}}",
			input:    "Hello world",
			expected: "You are a helpful assistant. User: Hello world",
			wantErr:  false,
		},
		{
			name:     "empty input",
			template: "Query: {{.Input}}",
			input:    "",
			expected: "Query: ",
			wantErr:  false,
		},
		{
			name:     "multiple variables",
			template: "Context: {{.Context}}, Task: {{.Task}}, Input: {{.Input}}",
			data:     map[string]any{"Context": "programming", "Task": "explain"},
			input:    "recursion",
			expected: "Context: programming, Task: explain, Input: recursion",
			wantErr:  false,
		},
		{
			name:        "invalid template syntax",
			template:    "Invalid {{.Input",
			input:       "test",
			expected:    "",
			wantErr:     true,
			errorSubstr: "template parse error",
		},
		{
			name:     "undefined variable in template",
			template: "Value: {{.UndefinedVar}}",
			input:    "test",
			expected: "Value: <no value>",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var handler core.Handler
			if tt.data != nil {
				handler = Template(tt.template, tt.data)
			} else {
				handler = Template(tt.template)
			}

			var buf bytes.Buffer
			reader := strings.NewReader(tt.input)

			req := core.NewRequest(context.Background(), reader)
			res := core.NewResponse(&buf)
			err := handler.ServeFlow(req, res)

			if (err != nil) != tt.wantErr {
				t.Errorf("Template() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.errorSubstr != "" {
				if !strings.Contains(err.Error(), tt.errorSubstr) {
					t.Errorf("Template() error = %v, want error containing %q", err, tt.errorSubstr)
				}
				return
			}

			if got := buf.String(); got != tt.expected {
				t.Errorf("Template() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestSystem(t *testing.T) {
	tests := []struct {
		name          string
		systemMessage string
		input         string
		expected      string
	}{
		{
			name:          "basic system message",
			systemMessage: "You are a helpful coding assistant.",
			input:         "How do I sort an array?",
			expected:      "You are a helpful coding assistant.\n\nHow do I sort an array?",
		},
		{
			name:          "empty system message",
			systemMessage: "",
			input:         "Hello",
			expected:      "\n\nHello",
		},
		{
			name:          "empty input",
			systemMessage: "You are helpful.",
			input:         "",
			expected:      "You are helpful.\n\n",
		},
		{
			name:          "multiline system message",
			systemMessage: "You are a helpful assistant.\nYou should be concise.",
			input:         "Explain recursion",
			expected:      "You are a helpful assistant.\nYou should be concise.\n\nExplain recursion",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := System(tt.systemMessage)

			var buf bytes.Buffer
			reader := strings.NewReader(tt.input)

			req := core.NewRequest(context.Background(), reader)
			res := core.NewResponse(&buf)
			err := handler.ServeFlow(req, res)
			if err != nil {
				t.Errorf("System() error = %v", err)
				return
			}

			if got := buf.String(); got != tt.expected {
				t.Errorf("System() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestSystemUser(t *testing.T) {
	tests := []struct {
		name          string
		systemMessage string
		input         string
		expected      string
	}{
		{
			name:          "basic system user format",
			systemMessage: "You are a helpful coding assistant.",
			input:         "How do I sort an array?",
			expected:      "System: You are a helpful coding assistant.\n\nUser: How do I sort an array?",
		},
		{
			name:          "empty system message",
			systemMessage: "",
			input:         "Hello",
			expected:      "System: \n\nUser: Hello",
		},
		{
			name:          "empty input",
			systemMessage: "You are helpful.",
			input:         "",
			expected:      "System: You are helpful.\n\nUser: ",
		},
		{
			name:          "long system message",
			systemMessage: "You are a highly skilled programming assistant with expertise in multiple languages.",
			input:         "Debug this code",
			expected:      "System: You are a highly skilled programming assistant with expertise in multiple languages.\n\nUser: Debug this code",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := SystemUser(tt.systemMessage)

			var buf bytes.Buffer
			reader := strings.NewReader(tt.input)

			req := core.NewRequest(context.Background(), reader)
			res := core.NewResponse(&buf)
			err := handler.ServeFlow(req, res)
			if err != nil {
				t.Errorf("SystemUser() error = %v", err)
				return
			}

			if got := buf.String(); got != tt.expected {
				t.Errorf("SystemUser() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestChat(t *testing.T) {
	tests := []struct {
		name           string
		role           string
		initialMessage []string
		input          string
		expected       string
	}{
		{
			name:     "simple role message",
			role:     "user",
			input:    "Hello",
			expected: "user: Hello",
		},
		{
			name:     "assistant role",
			role:     "assistant",
			input:    "How can I help?",
			expected: "assistant: How can I help?",
		},
		{
			name:           "chat with initial message",
			role:           "assistant",
			initialMessage: []string{"I'm here to help!"},
			input:          "Hello",
			expected:       "assistant: I'm here to help!\nuser: Hello",
		},
		{
			name:           "system role with initial message",
			role:           "system",
			initialMessage: []string{"You are in debug mode."},
			input:          "Show logs",
			expected:       "system: You are in debug mode.\nuser: Show logs",
		},
		{
			name:     "empty input simple role",
			role:     "user",
			input:    "",
			expected: "user: ",
		},
		{
			name:           "empty input with initial message",
			role:           "assistant",
			initialMessage: []string{"Ready to assist!"},
			input:          "",
			expected:       "assistant: Ready to assist!\nuser: ",
		},
		{
			name:           "multiple initial messages uses first",
			role:           "assistant",
			initialMessage: []string{"First message", "Second message"},
			input:          "Hi",
			expected:       "assistant: First message\nuser: Hi",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var handler core.Handler
			if len(tt.initialMessage) > 0 {
				handler = Chat(tt.role, tt.initialMessage...)
			} else {
				handler = Chat(tt.role)
			}

			var buf bytes.Buffer
			reader := strings.NewReader(tt.input)

			req := core.NewRequest(context.Background(), reader)
			res := core.NewResponse(&buf)
			err := handler.ServeFlow(req, res)
			if err != nil {
				t.Errorf("Chat() error = %v", err)
				return
			}

			if got := buf.String(); got != tt.expected {
				t.Errorf("Chat() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestInstruct(t *testing.T) {
	tests := []struct {
		name        string
		instruction string
		input       string
		expected    string
	}{
		{
			name:        "translate instruction",
			instruction: "Translate to French",
			input:       "Hello world",
			expected:    "### Instruction: Translate to French\n### Input: Hello world\n### Response:",
		},
		{
			name:        "code generation instruction",
			instruction: "Write a Python function",
			input:       "that sorts a list",
			expected:    "### Instruction: Write a Python function\n### Input: that sorts a list\n### Response:",
		},
		{
			name:        "empty instruction",
			instruction: "",
			input:       "some input",
			expected:    "### Instruction: \n### Input: some input\n### Response:",
		},
		{
			name:        "empty input",
			instruction: "Process this",
			input:       "",
			expected:    "### Instruction: Process this\n### Input: \n### Response:",
		},
		{
			name:        "multiline instruction",
			instruction: "First, analyze the code.\nThen, suggest improvements.",
			input:       "def hello(): print('hi')",
			expected:    "### Instruction: First, analyze the code.\nThen, suggest improvements.\n### Input: def hello(): print('hi')\n### Response:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := Instruct(tt.instruction)

			var buf bytes.Buffer
			reader := strings.NewReader(tt.input)

			req := core.NewRequest(context.Background(), reader)
			res := core.NewResponse(&buf)
			err := handler.ServeFlow(req, res)
			if err != nil {
				t.Errorf("Instruct() error = %v", err)
				return
			}

			if got := buf.String(); got != tt.expected {
				t.Errorf("Instruct() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestFromTemplate(t *testing.T) {
	tests := []struct {
		name        string
		templateStr string
		data        map[string]any
		input       string
		expected    string
		wantErr     bool
		errorSubstr string
	}{
		{
			name:        "basic template execution",
			templateStr: "Input: {{.Input}}",
			input:       "test data",
			expected:    "Input: test data",
			wantErr:     false,
		},
		{
			name:        "template with additional data",
			templateStr: "Context: {{.Context}}, Input: {{.Input}}",
			data:        map[string]any{"Context": "testing"},
			input:       "sample",
			expected:    "Context: testing, Input: sample",
			wantErr:     false,
		},
		{
			name:        "complex template",
			templateStr: "{{if .Debug}}DEBUG: {{end}}{{.Prefix}}{{.Input}}{{.Suffix}}",
			data:        map[string]any{"Debug": true, "Prefix": "[", "Suffix": "]"},
			input:       "content",
			expected:    "DEBUG: [content]",
			wantErr:     false,
		},
		{
			name:        "template with range",
			templateStr: "Items: {{range .Items}}{{.}} {{end}}Input: {{.Input}}",
			data:        map[string]any{"Items": []string{"a", "b", "c"}},
			input:       "test",
			expected:    "Items: a b c Input: test",
			wantErr:     false,
		},
		{
			name:        "empty input with template",
			templateStr: "Result: [{{.Input}}]",
			input:       "",
			expected:    "Result: []",
			wantErr:     false,
		},
		{
			name:        "template execution error",
			templateStr: "{{.Input.NonExistentMethod}}",
			input:       "test",
			expected:    "",
			wantErr:     true,
			errorSubstr: "template execution error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl, err := template.New("test").Parse(tt.templateStr)
			if err != nil {
				t.Fatalf("Failed to parse template: %v", err)
			}

			var handler core.Handler
			if tt.data != nil {
				handler = FromTemplate(tmpl, tt.data)
			} else {
				handler = FromTemplate(tmpl)
			}

			var buf bytes.Buffer
			reader := strings.NewReader(tt.input)

			req := core.NewRequest(context.Background(), reader)
			res := core.NewResponse(&buf)
			err = handler.ServeFlow(req, res)

			if (err != nil) != tt.wantErr {
				t.Errorf("FromTemplate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.errorSubstr != "" {
				if !strings.Contains(err.Error(), tt.errorSubstr) {
					t.Errorf("FromTemplate() error = %v, want error containing %q", err, tt.errorSubstr)
				}
				return
			}

			if got := buf.String(); got != tt.expected {
				t.Errorf("FromTemplate() = %q, want %q", got, tt.expected)
			}
		})
	}
}
