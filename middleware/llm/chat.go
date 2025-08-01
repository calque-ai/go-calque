package llm

import (
	"github.com/calque-ai/calque-pipe/core"
)

// LLMProvider interface - streaming by default
type LLMProvider interface {
	Chat(r *core.Request, w *core.Response) error
}

// Chat middleware - works with any llm provider
func Chat(provider LLMProvider) core.Handler {
	return core.HandlerFunc(func(r *core.Request, w *core.Response) error {
		return provider.Chat(r, w)
	})
}
