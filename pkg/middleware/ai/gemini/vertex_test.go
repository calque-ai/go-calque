package gemini

import (
	"os"
	"strings"
	"testing"

	"github.com/calque-ai/go-calque/pkg/middleware/ai"
)

// requireADC skips tests that construct a real Vertex AI client, which calls
// out to Application Default Credentials.
func requireADC(t *testing.T) {
	t.Helper()
	if os.Getenv("GOOGLE_APPLICATION_CREDENTIALS") == "" {
		t.Skip("GOOGLE_APPLICATION_CREDENTIALS not set, skipping test requiring Vertex AI ADC")
	}
}

func TestNewVertex(t *testing.T) {
	tests := []struct {
		name          string
		model         string
		opts          []VertexOption
		setupEnv      func(*testing.T)
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
			name:  "no project",
			model: "gemini-3.6-flash",
			setupEnv: func(t *testing.T) {
				t.Setenv("GOOGLE_CLOUD_PROJECT", "")
			},
			expectError:   true,
			errorContains: "GOOGLE_CLOUD_PROJECT environment variable not set",
		},
		{
			name:  "valid model with env project",
			model: "gemini-3.6-flash",
			setupEnv: func(t *testing.T) {
				t.Setenv("GOOGLE_CLOUD_PROJECT", "test-project")
				t.Setenv("GOOGLE_CLOUD_LOCATION", "us-central1")
			},
		},
		{
			name:  "valid model with config option",
			model: "gemini-3.6-flash",
			opts: []VertexOption{
				WithVertexConfig(&VertexConfig{
					Project:     "config-project",
					Location:    "europe-west1",
					Temperature: new(float32(0.5)),
				}),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !tt.expectError {
				requireADC(t)
			}

			if tt.setupEnv != nil {
				tt.setupEnv(t)
			}

			client, err := NewVertex(tt.model, tt.opts...)

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
				t.Errorf("NewVertex() error = %v", err)
				return
			}

			if client.model != tt.model {
				t.Errorf("NewVertex() model = %v, want %v", client.model, tt.model)
			}

			if client.config == nil {
				t.Error("NewVertex() config should not be nil")
			}

			if client.client == nil {
				t.Error("NewVertex() genai client should not be nil")
			}

			if client.config.APIKey != "" {
				t.Error("NewVertex() config.APIKey should be empty (mutually exclusive with Project)")
			}
		})
	}
}

func TestDefaultVertexConfig(t *testing.T) {
	t.Run("defaults when env vars unset", func(t *testing.T) {
		t.Setenv("GOOGLE_CLOUD_PROJECT", "")
		t.Setenv("GOOGLE_CLOUD_LOCATION", "")

		cfg := DefaultVertexConfig()

		if cfg == nil {
			t.Fatal("DefaultVertexConfig() should not return nil")
		}
		if cfg.Project != "" {
			t.Errorf("DefaultVertexConfig() Project = %q, want empty when env unset", cfg.Project)
		}
		if cfg.Location != "us-central1" {
			t.Errorf("DefaultVertexConfig() Location = %q, want %q", cfg.Location, "us-central1")
		}
		if cfg.Temperature == nil || *cfg.Temperature != 0.7 {
			t.Error("DefaultVertexConfig() should set Temperature to 0.7")
		}
	})

	t.Run("reads project from env", func(t *testing.T) {
		t.Setenv("GOOGLE_CLOUD_PROJECT", "env-project")

		cfg := DefaultVertexConfig()
		if cfg.Project != "env-project" {
			t.Errorf("DefaultVertexConfig() Project = %q, want %q", cfg.Project, "env-project")
		}
	})

	t.Run("reads location from env", func(t *testing.T) {
		t.Setenv("GOOGLE_CLOUD_LOCATION", "europe-west1")

		cfg := DefaultVertexConfig()
		if cfg.Location != "europe-west1" {
			t.Errorf("DefaultVertexConfig() Location = %q, want %q", cfg.Location, "europe-west1")
		}
	})
}

func TestWithVertexConfig(t *testing.T) {
	custom := &VertexConfig{
		Project:     "custom-project",
		Location:    "asia-east1",
		Temperature: new(float32(0.9)),
		MaxTokens:   new(2000),
		TopP:        new(float32(0.95)),
	}

	opt := WithVertexConfig(custom)

	target := &VertexConfig{}
	opt.ApplyVertex(target)

	if target.Project != "custom-project" {
		t.Errorf("WithVertexConfig() Project = %q, want %q", target.Project, "custom-project")
	}
	if target.Location != "asia-east1" {
		t.Errorf("WithVertexConfig() Location = %q, want %q", target.Location, "asia-east1")
	}
	if target.Temperature == nil || *target.Temperature != 0.9 {
		t.Error("WithVertexConfig() should apply Temperature")
	}
	if target.MaxTokens == nil || *target.MaxTokens != 2000 {
		t.Error("WithVertexConfig() should apply MaxTokens")
	}
	if target.TopP == nil || *target.TopP != 0.95 {
		t.Error("WithVertexConfig() should apply TopP")
	}
}

func TestWithVertexConfig_MergePreservesExisting(t *testing.T) {
	base := DefaultVertexConfig()
	base.Project = "base-project"

	opt := WithVertexConfig(&VertexConfig{
		Location: "us-east1",
	})
	opt.ApplyVertex(base)

	if base.Project != "base-project" {
		t.Errorf("ApplyVertex() should preserve existing Project, got %q", base.Project)
	}
	if base.Location != "us-east1" {
		t.Errorf("ApplyVertex() should set Location to %q, got %q", "us-east1", base.Location)
	}
	if base.Temperature == nil || *base.Temperature != 0.7 {
		t.Error("ApplyVertex() should preserve default Temperature")
	}
}

func TestVertexConfig_TuningFieldsPassThrough(t *testing.T) {
	requireADC(t)

	client, err := NewVertex("gemini-3.6-flash", WithVertexConfig(&VertexConfig{
		Project:           "test-project",
		Location:          "us-central1",
		Temperature:       new(float32(0.3)),
		MaxTokens:         new(500),
		SystemInstruction: "Be concise",
		Stream:            new(false),
	}))
	if err != nil {
		t.Fatalf("NewVertex() error = %v", err)
	}

	if client.config.Temperature == nil || *client.config.Temperature != 0.3 {
		t.Errorf("config.Temperature = %v, want 0.3", client.config.Temperature)
	}
	if client.config.MaxTokens == nil || *client.config.MaxTokens != 500 {
		t.Errorf("config.MaxTokens = %v, want 500", client.config.MaxTokens)
	}
	if client.config.SystemInstruction != "Be concise" {
		t.Errorf("config.SystemInstruction = %q, want %q", client.config.SystemInstruction, "Be concise")
	}
	if client.config.Stream == nil || *client.config.Stream {
		t.Errorf("config.Stream = %v, want false", client.config.Stream)
	}
}

func TestVertexClientInterfaceCompliance(_ *testing.T) {
	var _ ai.Client = (*Client)(nil)
}
