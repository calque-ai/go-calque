package llm

import (
	"context"
	"io"

	"github.com/calque-ai/calque-pipe/core"
)

// LLMProvider interface - streaming by default
type LLMProvider interface {
	Chat(ctx context.Context, input io.Reader, output io.Writer) error
}

// Chat middleware - works with any llm provider
func Chat(provider LLMProvider) core.Handler {
	return core.HandlerFunc(func(ctx context.Context, r io.Reader, w io.Writer) error {
		return provider.Chat(ctx, r, w)
	})
}
