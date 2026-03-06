package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// CompleteChat implements ChatProvider for the Copilot backend.
// Copilot uses OpenAI-compatible tool_calls format.
func (p *CopilotProvider) CompleteChat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	model := req.Model
	if model == "" {
		model = p.model
	}

	// Convert messages
	msgs := make([]types.ChatMessage, 0, len(req.Messages))
	for _, m := range req.Messages {
		switch {
		case m.Role == RoleSystem:
			msgs = append(msgs, types.ChatMessage{Role: "system", Content: contentBlocksToText(m.Content)})
		case hasToolResult(m.Content):
			// Tool result messages: one per tool_result block with role "tool"
			for _, b := range m.Content {
				if b.Type == "tool_result" {
					msgs = append(msgs, types.ChatMessage{
						Role:       "tool",
						ToolCallID: b.ToolUseID,
						Content:    b.Content,
					})
				}
			}
		case hasToolUse(m.Content):
			// Assistant message with tool_calls
			var toolCalls []types.ToolCall
			var textParts []string
			for _, b := range m.Content {
				switch b.Type {
				case "text":
					if b.Text != "" {
						textParts = append(textParts, b.Text)
					}
				case "tool_use":
					toolCalls = append(toolCalls, types.ToolCall{
						ID:   b.ID,
						Type: "function",
						Function: types.ToolCallFunction{
							Name:      b.Name,
							Arguments: string(b.Input),
						},
					})
				}
			}
			msgs = append(msgs, types.ChatMessage{
				Role:      "assistant",
				Content:   strings.Join(textParts, "\n"),
				ToolCalls: toolCalls,
			})
		default:
			msgs = append(msgs, types.ChatMessage{
				Role:    m.Role,
				Content: contentBlocksToText(m.Content),
			})
		}
	}

	// Convert tools
	var toolDefs []types.Tool
	for _, t := range req.Tools {
		toolDefs = append(toolDefs, types.Tool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.InputSchema,
		})
	}

	opts := types.GenerateOptions{
		Model:     model,
		Messages:  msgs,
		MaxTokens: req.MaxTokens,
		Stream:    true,
		Tools:     toolDefs,
	}

	stream, err := p.provider.GenerateChatCompletion(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("copilot chat completion: %w", err)
	}
	defer stream.Close()

	var (
		textContent  string
		toolCalls    = map[int]*ContentBlock{} // indexed by choice delta index
		finishReason string
		usage        types.Usage
	)

	for {
		chunk, err := stream.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("reading copilot chat stream: %w", err)
		}

		textContent += chunk.Content
		usage = chunk.Usage

		for _, choice := range chunk.Choices {
			if choice.FinishReason != "" {
				finishReason = choice.FinishReason
			}
			// Accumulate tool calls from delta
			for _, tc := range choice.Delta.ToolCalls {
				idx := 0 // Use tool call ID as key if available
				for i, existing := range toolCalls {
					if existing.ID == tc.ID && tc.ID != "" {
						idx = i
						break
					}
				}
				if tc.ID != "" {
					// First chunk for this tool call - has ID and name
					toolCalls[len(toolCalls)] = &ContentBlock{
						Type:  "tool_use",
						ID:    tc.ID,
						Name:  tc.Function.Name,
						Input: json.RawMessage(tc.Function.Arguments),
					}
				} else if tc.Function.Arguments != "" {
					// Subsequent chunks - append arguments
					if existing, ok := toolCalls[idx]; ok {
						existing.Input = json.RawMessage(string(existing.Input) + tc.Function.Arguments)
					}
				}
			}
		}
	}

	// Build response content blocks
	var blocks []ContentBlock
	if textContent != "" {
		blocks = append(blocks, ContentBlock{Type: "text", Text: textContent})
	}
	for i := 0; i < len(toolCalls); i++ {
		if tc := toolCalls[i]; tc != nil {
			if len(tc.Input) == 0 {
				tc.Input = json.RawMessage("{}")
			}
			blocks = append(blocks, *tc)
		}
	}

	// Normalize stop reason
	stopReason := finishReason
	switch finishReason {
	case "tool_calls":
		stopReason = "tool_use"
	case "stop":
		stopReason = "end_turn"
	case "length":
		stopReason = "max_tokens"
	}

	return &ChatResponse{
		Content:      blocks,
		StopReason:   stopReason,
		PromptTokens: usage.PromptTokens,
		OutputTokens: usage.CompletionTokens,
	}, nil
}

func contentBlocksToText(blocks []ContentBlock) string {
	var parts []string
	for _, b := range blocks {
		if b.Type == "text" && b.Text != "" {
			parts = append(parts, b.Text)
		}
	}
	return strings.Join(parts, "\n")
}

func hasToolUse(blocks []ContentBlock) bool {
	for _, b := range blocks {
		if b.Type == "tool_use" {
			return true
		}
	}
	return false
}

func hasToolResult(blocks []ContentBlock) bool {
	for _, b := range blocks {
		if b.Type == "tool_result" {
			return true
		}
	}
	return false
}
