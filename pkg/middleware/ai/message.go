package ai

import "github.com/calque-ai/go-calque/pkg/middleware/tools"

// Role identifies who authored a Message in a conversation.
type Role string

// Role values for Message.Role.
const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
	RoleSystem    Role = "system"
)

// Message is one turn in a multi-shot tool-calling conversation.
//
// ToolCalls is set on an assistant message that requested tools; ToolCallID
// is set on a tool message and must match the ToolCall.ID it answers.
// Multimodal is set on a user message carrying non-text content (images,
// audio, video) alongside or instead of Content; io.Reader-backed parts are
// read once when the message is first sent to a provider, so Multimodal
// should only be set on the initial user turn, not reconstructed on
// subsequent loop iterations.
type Message struct {
	Role       Role
	Content    string
	ToolCalls  []tools.ToolCall
	ToolCallID string
	Multimodal *MultimodalInput
}
