package llm

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/document"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
)

// CompleteChat implements ChatProvider for the Bedrock backend.
// Uses ConverseStream internally but returns a complete response with content blocks.
func (p *BedrockProvider) CompleteChat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	model := req.Model
	if model == "" {
		model = p.opusModel
	}

	maxTokens := int32(req.MaxTokens)
	if maxTokens == 0 {
		maxTokens = 16384
	}

	// Convert messages to Bedrock format.
	var msgs []types.Message
	var systemBlocks []types.SystemContentBlock
	if req.System != "" {
		systemBlocks = append(systemBlocks, &types.SystemContentBlockMemberText{
			Value: req.System,
		})
	}

	for _, m := range req.Messages {
		var blocks []types.ContentBlock
		for _, b := range m.Content {
			switch b.Type {
			case "text":
				blocks = append(blocks, &types.ContentBlockMemberText{Value: b.Text})
			case "tool_use":
				var inputMap any
				if len(b.Input) > 0 {
					_ = json.Unmarshal(b.Input, &inputMap)
				}
				if inputMap == nil {
					inputMap = map[string]any{}
				}
				blocks = append(blocks, &types.ContentBlockMemberToolUse{
					Value: types.ToolUseBlock{
						ToolUseId: aws.String(b.ID),
						Name:      aws.String(b.Name),
						Input:     document.NewLazyDocument(inputMap),
					},
				})
			case "tool_result":
				status := types.ToolResultStatusSuccess
				if b.IsError {
					status = types.ToolResultStatusError
				}
				blocks = append(blocks, &types.ContentBlockMemberToolResult{
					Value: types.ToolResultBlock{
						ToolUseId: aws.String(b.ToolUseID),
						Content: []types.ToolResultContentBlock{
							&types.ToolResultContentBlockMemberText{Value: b.Content},
						},
						Status: status,
					},
				})
			}
		}

		var role types.ConversationRole
		switch m.Role {
		case RoleUser:
			role = types.ConversationRoleUser
		case RoleAssistant:
			role = types.ConversationRoleAssistant
		default:
			continue
		}
		msgs = append(msgs, types.Message{Role: role, Content: blocks})
	}

	// Convert tools to Bedrock format.
	var toolConfig *types.ToolConfiguration
	if len(req.Tools) > 0 {
		var tools []types.Tool
		for _, t := range req.Tools {
			tools = append(tools, &types.ToolMemberToolSpec{
				Value: types.ToolSpecification{
					Name:        aws.String(t.Name),
					Description: aws.String(t.Description),
					InputSchema: &types.ToolInputSchemaMemberJson{
						Value: mapToDocument(t.InputSchema),
					},
				},
			})
		}
		toolConfig = &types.ToolConfiguration{Tools: tools}
	}

	input := &bedrockruntime.ConverseStreamInput{
		ModelId:  aws.String(bedrockModelID(model, "")),
		Messages: msgs,
		InferenceConfig: &types.InferenceConfiguration{
			MaxTokens: aws.Int32(maxTokens),
		},
	}
	if len(systemBlocks) > 0 {
		input.System = systemBlocks
	}
	if toolConfig != nil {
		input.ToolConfig = toolConfig
	}

	output, err := p.client.ConverseStream(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("bedrock chat: %w", err)
	}

	stream := output.GetStream()
	defer stream.Close()

	var (
		contentBlocks  []ContentBlock
		currentToolUse *ContentBlock
		partialJSON    string
		stopReason     string
		promptTokens   int
		outputTokens   int
	)

	for event := range stream.Events() {
		switch ev := event.(type) {
		case *types.ConverseStreamOutputMemberContentBlockStart:
			start := ev.Value.Start
			switch s := start.(type) {
			case *types.ContentBlockStartMemberToolUse:
				currentToolUse = &ContentBlock{
					Type: "tool_use",
					ID:   aws.ToString(s.Value.ToolUseId),
					Name: aws.ToString(s.Value.Name),
				}
				partialJSON = ""
			default:
				// Text block start — add placeholder.
				contentBlocks = append(contentBlocks, ContentBlock{Type: "text"})
			}

		case *types.ConverseStreamOutputMemberContentBlockDelta:
			delta := ev.Value.Delta
			switch d := delta.(type) {
			case *types.ContentBlockDeltaMemberText:
				if len(contentBlocks) > 0 {
					last := &contentBlocks[len(contentBlocks)-1]
					if last.Type == "text" {
						last.Text += d.Value
					}
				}
			case *types.ContentBlockDeltaMemberToolUse:
				if currentToolUse != nil && d.Value.Input != nil {
					partialJSON += *d.Value.Input
				}
			}

		case *types.ConverseStreamOutputMemberContentBlockStop:
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

		case *types.ConverseStreamOutputMemberMessageStop:
			stopReason = string(ev.Value.StopReason)

		case *types.ConverseStreamOutputMemberMetadata:
			if ev.Value.Usage != nil {
				promptTokens = int(aws.ToInt32(ev.Value.Usage.InputTokens))
				outputTokens = int(aws.ToInt32(ev.Value.Usage.OutputTokens))
			}
		}
	}

	if err := stream.Err(); err != nil {
		return nil, fmt.Errorf("bedrock chat streaming: %w", err)
	}

	return &ChatResponse{
		Content:      contentBlocks,
		StopReason:   stopReason,
		PromptTokens: promptTokens,
		OutputTokens: outputTokens,
	}, nil
}
