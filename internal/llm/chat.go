package llm

import (
	"context"
	"encoding/json"
)

// ToolDefinition describes a tool the model can call.
type ToolDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"input_schema"`
}

// ContentBlock is a single block in a chat response.
type ContentBlock struct {
	Type  string          `json:"type"`            // "text", "tool_use", or "tool_result"
	Text  string          `json:"text,omitempty"`
	ID    string          `json:"id,omitempty"`    // tool use ID
	Name  string          `json:"name,omitempty"`  // tool name
	Input json.RawMessage `json:"input,omitempty"` // tool input JSON

	// tool_result fields
	ToolUseID string `json:"tool_use_id,omitempty"`
	Content   string `json:"content,omitempty"` // tool result text
	IsError   bool   `json:"is_error,omitempty"`
}

// ChatMessage has structured content blocks (not just a string).
type ChatMessage struct {
	Role    string         `json:"role"`
	Content []ContentBlock `json:"content"`
}

// ChatRequest holds parameters for a tool-use chat completion.
type ChatRequest struct {
	Model       string
	System      string
	Messages    []ChatMessage
	Tools       []ToolDefinition
	MaxTokens   int
	Temperature float64
}

// ChatResponse holds the result of a tool-use chat completion.
type ChatResponse struct {
	Content      []ContentBlock
	StopReason   string // "end_turn", "tool_use", "max_tokens"
	PromptTokens int
	OutputTokens int
}

// HasToolUse returns true if any content block is a tool_use.
func (r *ChatResponse) HasToolUse() bool {
	for _, b := range r.Content {
		if b.Type == "tool_use" {
			return true
		}
	}
	return false
}

// TextContent concatenates all text blocks in the response.
func (r *ChatResponse) TextContent() string {
	var s string
	for _, b := range r.Content {
		if b.Type == "text" {
			s += b.Text
		}
	}
	return s
}

// ToolUseBlocks returns only the tool_use content blocks.
func (r *ChatResponse) ToolUseBlocks() []ContentBlock {
	var blocks []ContentBlock
	for _, b := range r.Content {
		if b.Type == "tool_use" {
			blocks = append(blocks, b)
		}
	}
	return blocks
}

// NewTextContent creates a text content block.
func NewTextContent(text string) ContentBlock {
	return ContentBlock{Type: "text", Text: text}
}

// NewToolUseContent creates a tool_use content block.
func NewToolUseContent(id, name string, input json.RawMessage) ContentBlock {
	return ContentBlock{Type: "tool_use", ID: id, Name: name, Input: input}
}

// NewToolResultContent creates a tool_result content block.
func NewToolResultContent(toolUseID, content string, isError bool) ContentBlock {
	return ContentBlock{Type: "tool_result", ToolUseID: toolUseID, Content: content, IsError: isError}
}

// ChatProvider adds tool-use completions on top of Provider.
type ChatProvider interface {
	Provider
	CompleteChat(ctx context.Context, req ChatRequest) (*ChatResponse, error)
}
