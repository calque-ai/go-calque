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
type Message struct {
	Role       Role
	Content    string
	ToolCalls  []tools.ToolCall
	ToolCallID string
}
