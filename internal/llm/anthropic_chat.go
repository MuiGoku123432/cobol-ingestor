package llm

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"
)

// CompleteChat implements ChatProvider for the Anthropic backend.
// Uses streaming internally but returns a complete response with content blocks.
func (p *AnthropicProvider) CompleteChat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	model := req.Model
	if model == "" {
		model = p.opusModel
	}

	maxTokens := int64(req.MaxTokens)
	if maxTokens == 0 {
		maxTokens = 16384
	}

	// Convert messages
	msgs := make([]anthropic.MessageParam, 0, len(req.Messages))
	for _, m := range req.Messages {
		blocks := make([]anthropic.ContentBlockParamUnion, 0, len(m.Content))
		for _, b := range m.Content {
			switch b.Type {
			case "text":
				blocks = append(blocks, anthropic.NewTextBlock(b.Text))
			case "tool_use":
				var input any
				if len(b.Input) > 0 {
					_ = json.Unmarshal(b.Input, &input)
				}
				blocks = append(blocks, anthropic.NewToolUseBlock(b.ID, input, b.Name))
			case "tool_result":
				blocks = append(blocks, anthropic.NewToolResultBlock(b.ToolUseID, b.Content, b.IsError))
			}
		}
		switch m.Role {
		case RoleUser:
			msgs = append(msgs, anthropic.MessageParam{Role: anthropic.MessageParamRoleUser, Content: blocks})
		case RoleAssistant:
			msgs = append(msgs, anthropic.MessageParam{Role: anthropic.MessageParamRoleAssistant, Content: blocks})
		}
	}

	// Convert tools
	tools := make([]anthropic.ToolUnionParam, 0, len(req.Tools))
	for _, t := range req.Tools {
		props, _ := t.InputSchema["properties"]
		required, _ := t.InputSchema["required"].([]any)
		var reqStrings []string
		for _, r := range required {
			if s, ok := r.(string); ok {
				reqStrings = append(reqStrings, s)
			}
		}
		tools = append(tools, anthropic.ToolUnionParam{
			OfTool: &anthropic.ToolParam{
				Name:        t.Name,
				Description: anthropic.String(t.Description),
				InputSchema: anthropic.ToolInputSchemaParam{
					Properties: props,
					Required:   reqStrings,
				},
			},
		})
	}

	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(model),
		MaxTokens: maxTokens,
		Messages:  msgs,
	}

	if req.System != "" {
		params.System = []anthropic.TextBlockParam{
			{Text: req.System},
		}
	}

	if req.Temperature > 0 {
		params.Temperature = anthropic.Float(req.Temperature)
	}

	if len(tools) > 0 {
		params.Tools = tools
	}

	stream := p.client.Messages.NewStreaming(ctx, params)
	defer stream.Close()

	var (
		contentBlocks []ContentBlock
		inputTokens   int
		outputTokens  int
		stopReason    string
		// Track tool_use blocks being built from stream events
		currentToolUse *ContentBlock
		partialJSON    string
	)

	for stream.Next() {
		event := stream.Current()
		switch event.Type {
		case "message_start":
			inputTokens = int(event.Message.Usage.InputTokens)

		case "content_block_start":
			cb := event.ContentBlock
			switch cb.Type {
			case "text":
				contentBlocks = append(contentBlocks, ContentBlock{Type: "text"})
			case "tool_use":
				currentToolUse = &ContentBlock{
					Type: "tool_use",
					ID:   cb.ID,
					Name: cb.Name,
				}
				partialJSON = ""
			}

		case "content_block_delta":
			if event.Delta.Text != "" && len(contentBlocks) > 0 {
				last := &contentBlocks[len(contentBlocks)-1]
				if last.Type == "text" {
					last.Text += event.Delta.Text
				}
			}
			if event.Delta.PartialJSON != "" && currentToolUse != nil {
				partialJSON += event.Delta.PartialJSON
			}

		case "content_block_stop":
			if currentToolUse != nil {
				if partialJSON != "" {
					currentToolUse.Input = json.RawMessage(partialJSON)
				} else {
					currentToolUse.Input = json.RawMessage("{}")
				}
				contentBlocks = append(contentBlocks, *currentToolUse)
				currentToolUse = nil
				partialJSON = ""
			}

		case "message_delta":
			stopReason = string(event.Delta.StopReason)
			outputTokens = int(event.Usage.OutputTokens)
		}
	}
	if err := stream.Err(); err != nil {
		return nil, fmt.Errorf("anthropic chat streaming: %w", err)
	}

	return &ChatResponse{
		Content:      contentBlocks,
		StopReason:   stopReason,
		PromptTokens: inputTokens,
		OutputTokens: outputTokens,
	}, nil
}
