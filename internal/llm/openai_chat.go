package llm

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/shared"
)

// CompleteChat implements ChatProvider for the OpenAI backend.
// Uses streaming internally to avoid header timeouts on large responses.
func (p *OpenAIProvider) CompleteChat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	model := p.resolveModel(req.Model)

	// Convert messages.
	var msgs []openai.ChatCompletionMessageParamUnion
	if req.System != "" {
		msgs = append(msgs, openai.SystemMessage(req.System))
	}

	for _, m := range req.Messages {
		switch m.Role {
		case RoleUser:
			msgs = append(msgs, convertUserMessage(m))
		case RoleAssistant:
			msgs = append(msgs, convertAssistantMessage(m))
		}
	}

	// Convert tools.
	var tools []openai.ChatCompletionToolParam
	for _, t := range req.Tools {
		tools = append(tools, openai.ChatCompletionToolParam{
			Function: shared.FunctionDefinitionParam{
				Name:        t.Name,
				Description: openai.String(t.Description),
				Parameters:  shared.FunctionParameters(t.InputSchema),
			},
		})
	}

	params := openai.ChatCompletionNewParams{
		Model:    shared.ChatModel(model),
		Messages: msgs,
	}
	if req.MaxTokens > 0 {
		params.MaxCompletionTokens = openai.Int(int64(req.MaxTokens))
	}
	if len(tools) > 0 {
		params.Tools = tools
	}

	// Use streaming to handle long responses.
	stream := p.client.Chat.Completions.NewStreaming(ctx, params)
	defer stream.Close()

	var (
		contentBlocks []ContentBlock
		textContent   string
		// Track tool calls by index as deltas arrive.
		toolCalls    = map[int64]*pendingToolCall{}
		stopReason   string
		promptTokens int
		outputTokens int
	)

	for stream.Next() {
		chunk := stream.Current()

		if chunk.Usage.PromptTokens > 0 {
			promptTokens = int(chunk.Usage.PromptTokens)
		}
		if chunk.Usage.CompletionTokens > 0 {
			outputTokens = int(chunk.Usage.CompletionTokens)
		}

		for _, choice := range chunk.Choices {
			if choice.FinishReason != "" {
				stopReason = normalizeOpenAIStopReason(choice.FinishReason)
			}

			// Accumulate text content.
			if choice.Delta.Content != "" {
				textContent += choice.Delta.Content
			}

			// Accumulate tool call deltas.
			for _, tc := range choice.Delta.ToolCalls {
				ptc, ok := toolCalls[tc.Index]
				if !ok {
					ptc = &pendingToolCall{}
					toolCalls[tc.Index] = ptc
				}
				if tc.ID != "" {
					ptc.id = tc.ID
				}
				if tc.Function.Name != "" {
					ptc.name = tc.Function.Name
				}
				ptc.arguments += tc.Function.Arguments
			}
		}
	}

	if err := stream.Err(); err != nil {
		return nil, fmt.Errorf("openai chat streaming: %w", err)
	}

	// Build content blocks.
	if textContent != "" {
		contentBlocks = append(contentBlocks, ContentBlock{Type: "text", Text: textContent})
	}

	// Add accumulated tool calls as content blocks.
	for i := int64(0); i < int64(len(toolCalls)); i++ {
		ptc := toolCalls[i]
		if ptc == nil {
			continue
		}
		var input json.RawMessage
		if ptc.arguments != "" {
			input = json.RawMessage(ptc.arguments)
		} else {
			input = json.RawMessage("{}")
		}
		contentBlocks = append(contentBlocks, ContentBlock{
			Type:  "tool_use",
			ID:    ptc.id,
			Name:  ptc.name,
			Input: input,
		})
	}

	return &ChatResponse{
		Content:      contentBlocks,
		StopReason:   stopReason,
		PromptTokens: promptTokens,
		OutputTokens: outputTokens,
	}, nil
}

// pendingToolCall accumulates streamed tool call deltas.
type pendingToolCall struct {
	id        string
	name      string
	arguments string
}

// convertUserMessage converts a ChatMessage with user role to OpenAI format.
// Handles both text and tool_result content blocks.
func convertUserMessage(m ChatMessage) openai.ChatCompletionMessageParamUnion {
	// Check if this message contains tool results.
	for _, b := range m.Content {
		if b.Type == "tool_result" {
			// For tool results, return as a tool message.
			// If there are multiple tool results, we only handle the first one here;
			// the caller should split them into separate messages if needed.
			// In practice, the chat handler sends one tool_result per message.
			return openai.ToolMessage(b.Content, b.ToolUseID)
		}
	}

	// Regular text message.
	var text string
	for _, b := range m.Content {
		if b.Type == "text" {
			text += b.Text
		}
	}
	return openai.UserMessage(text)
}

// convertAssistantMessage converts a ChatMessage with assistant role to OpenAI format.
// Handles both text content and tool_use blocks.
func convertAssistantMessage(m ChatMessage) openai.ChatCompletionMessageParamUnion {
	var textParts string
	var toolCalls []openai.ChatCompletionMessageToolCallParam

	for _, b := range m.Content {
		switch b.Type {
		case "text":
			textParts += b.Text
		case "tool_use":
			args := string(b.Input)
			if args == "" {
				args = "{}"
			}
			toolCalls = append(toolCalls, openai.ChatCompletionMessageToolCallParam{
				ID: b.ID,
				Function: openai.ChatCompletionMessageToolCallFunctionParam{
					Name:      b.Name,
					Arguments: args,
				},
			})
		}
	}

	msg := openai.ChatCompletionAssistantMessageParam{}
	if textParts != "" {
		msg.Content = openai.ChatCompletionAssistantMessageParamContentUnion{
			OfString: openai.String(textParts),
		}
	}
	if len(toolCalls) > 0 {
		msg.ToolCalls = toolCalls
	}

	return openai.ChatCompletionMessageParamUnion{
		OfAssistant: &msg,
	}
}
