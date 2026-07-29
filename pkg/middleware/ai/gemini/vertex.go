package gemini

import (
	"context"
	"os"

	"google.golang.org/genai"

	"github.com/calque-ai/go-calque/pkg/calque"
	"github.com/calque-ai/go-calque/pkg/helpers"
	"github.com/calque-ai/go-calque/pkg/middleware/ai"
	"github.com/calque-ai/go-calque/pkg/middleware/ai/config"
)

const defaultVertexLocation = "us-central1"

// VertexConfig holds Vertex AI-specific configuration.
//
// Configures authentication via GCP project and location (ADC credentials are
// picked up automatically from the environment). All model-tuning fields are
// identical to Config.
//
// Example:
//
//	cfg := &gemini.VertexConfig{
//		Project:     "my-gcp-project",
//		Location:    "us-central1",
//		Temperature: helpers.PtrOf(float32(0.8)),
//	}
type VertexConfig struct {
	// Required. GCP project ID. Fallback: GOOGLE_CLOUD_PROJECT env var.
	Project string

	// Optional. GCP region. Fallback: GOOGLE_CLOUD_LOCATION env var. Default: "us-central1".
	Location string

	// Optional. Controls randomness in token selection (0.0-2.0)
	Temperature *float32

	// Optional. Nucleus sampling parameter (0.0-1.0)
	TopP *float32

	// Optional. Top-k sampling - select from k highest probability tokens
	TopK *float32

	// Optional. Maximum number of tokens in the response
	MaxTokens *int

	// Optional. Strings that stop text generation when encountered
	Stop []string

	// Optional. System instructions to steer model behavior
	SystemInstruction string

	// Optional. Penalize tokens that already appear in generated text (-2.0 to 2.0)
	PresencePenalty *float32

	// Optional. Penalize frequently repeated tokens (-2.0 to 2.0)
	FrequencyPenalty *float32

	// Optional. Fixed seed for reproducible responses
	Seed *int32

	// Optional. Number of response variations to generate
	CandidateCount *int32

	// Optional. Response format configuration (JSON schema, etc.)
	ResponseFormat *ai.ResponseFormat

	// Optional. Safety settings to block unsafe content
	SafetySettings []*genai.SafetySetting

	// Optional. Enable/disable streaming of responses (disabled automatically when tools are present)
	Stream *bool
}

// VertexOption is the functional option interface for VertexConfig.
type VertexOption interface {
	ApplyVertex(*VertexConfig)
}

type vertexConfigOption struct{ config *VertexConfig }

func (o vertexConfigOption) ApplyVertex(target *VertexConfig) {
	config.Merge(target, o.config)
}

// WithVertexConfig sets custom Vertex AI configuration.
//
// Example:
//
//	cfg := &gemini.VertexConfig{Project: "my-project", Temperature: helpers.PtrOf(float32(0.9))}
//	client, _ := gemini.NewVertex("gemini-3.6-flash", gemini.WithVertexConfig(cfg))
func WithVertexConfig(cfg *VertexConfig) VertexOption {
	return vertexConfigOption{config: cfg}
}

// DefaultVertexConfig returns sensible defaults for Vertex AI.
//
// Reads GOOGLE_CLOUD_PROJECT and GOOGLE_CLOUD_LOCATION from the environment.
// Location defaults to "us-central1" when the env var is unset.
func DefaultVertexConfig() *VertexConfig {
	location := os.Getenv("GOOGLE_CLOUD_LOCATION")
	if location == "" {
		location = defaultVertexLocation
	}
	return &VertexConfig{
		Project:     os.Getenv("GOOGLE_CLOUD_PROJECT"),
		Location:    location,
		Temperature: helpers.PtrOf(float32(0.7)),
	}
}

// NewVertex creates a new Gemini client authenticated via Vertex AI (ADC).
//
// Input: model name string, optional VertexOption config options
// Output: *Client, error
// Behavior: Initializes a genai client using BackendVertexAI and application
// default credentials — no API key required.
//
// Requires GOOGLE_CLOUD_PROJECT (or config.Project) and valid ADC credentials
// (service account JSON, gcloud auth application-default login, or GCE metadata).
//
// Example:
//
//	client, err := gemini.NewVertex("gemini-3.6-flash")
//	if err != nil { log.Fatal(err) }
func NewVertex(model string, opts ...VertexOption) (*Client, error) {
	ctx := context.Background()
	if model == "" {
		return nil, calque.NewErr(ctx, "model name is required")
	}

	cfg := DefaultVertexConfig()
	for _, opt := range opts {
		opt.ApplyVertex(cfg)
	}

	if cfg.Project == "" {
		return nil, calque.NewErr(ctx, "GOOGLE_CLOUD_PROJECT environment variable not set or provided in config")
	}

	clientConfig := &genai.ClientConfig{
		Backend:  genai.BackendVertexAI,
		Project:  cfg.Project,
		Location: cfg.Location,
	}

	genaiClient, err := genai.NewClient(ctx, clientConfig)
	if err != nil {
		return nil, calque.WrapErr(ctx, err, "failed to create vertex ai client")
	}

	geminiCfg := &Config{}
	config.Merge(geminiCfg, cfg)

	return &Client{
		client: genaiClient,
		model:  model,
		config: geminiCfg,
	}, nil
}
