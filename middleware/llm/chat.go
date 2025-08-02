package llm

import (
	"github.com/calque-ai/calque-pipe/core"
	"github.com/calque-ai/calque-pipe/middleware/tools"
)

// LLMProvider interface - streaming by default
type LLMProvider interface {
	Chat(r *core.Request, w *core.Response) error
	ChatWithTools(r *core.Request, w *core.Response, tools ...tools.Tool) error
}

// Chat middleware - works with any llm provider
func Chat(provider LLMProvider) core.Handler {
	return core.HandlerFunc(func(r *core.Request, w *core.Response) error {
		return provider.Chat(r, w)
	})
}

func ChatWithTools(provider LLMProvider, tools ...tools.Tool) core.Handler {
	return core.HandlerFunc(func(r *core.Request, w *core.Response) error {
		return provider.ChatWithTools(r, w, tools...)
	})
}
