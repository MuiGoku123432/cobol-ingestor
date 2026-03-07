package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/copilot"
	"github.com/google/uuid"
)

// CompleteChat implements ChatProvider for the Copilot backend.
// We bypass the library's GenerateChatCompletion because its prepareRequest()
// drops ToolCalls and ToolCallID fields, causing 400 errors from the OpenAI-compatible API.
func (p *CopilotProvider) CompleteChat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	model := req.Model
	if model == "" {
		model = p.model
	}

	// Build copilot.ChatMessage structs directly (preserves ToolCalls/ToolCallID)
	msgs := make([]copilot.ChatMessage, 0, len(req.Messages))
	for _, m := range req.Messages {
		switch {
		case m.Role == RoleSystem:
			msgs = append(msgs, copilot.ChatMessage{Role: "system", Content: contentBlocksToText(m.Content)})
		case hasToolResult(m.Content):
			for _, b := range m.Content {
				if b.Type == "tool_result" {
					msgs = append(msgs, copilot.ChatMessage{
						Role:       "tool",
						ToolCallID: b.ToolUseID,
						Content:    b.Content,
					})
				}
			}
		case hasToolUse(m.Content):
			var toolCalls []copilot.ToolCall
			var textParts []string
			for _, b := range m.Content {
				switch b.Type {
				case "text":
					if b.Text != "" {
						textParts = append(textParts, b.Text)
					}
				case "tool_use":
					toolCalls = append(toolCalls, copilot.ToolCall{
						ID:   b.ID,
						Type: "function",
						Function: copilot.ToolCallFunction{
							Name:      b.Name,
							Arguments: string(b.Input),
						},
					})
				}
			}
			msgs = append(msgs, copilot.ChatMessage{
				Role:      "assistant",
				Content:   strings.Join(textParts, "\n"),
				ToolCalls: toolCalls,
			})
		default:
			msgs = append(msgs, copilot.ChatMessage{
				Role:    m.Role,
				Content: contentBlocksToText(m.Content),
			})
		}
	}

	// Convert tools to copilot format
	var tools []copilot.Tool
	for _, t := range req.Tools {
		tools = append(tools, copilot.Tool{
			Type: "function",
			Function: copilot.ToolFunction{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.InputSchema,
			},
		})
	}

	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = copilot.DefaultMaxTokens
	}

	apiReq := &copilot.ChatCompletionRequest{
		Model:     model,
		Messages:  msgs,
		MaxTokens: maxTokens,
		Stream:    true,
		Tools:     tools,
	}

	// Get auth token
	token, err := p.provider.GetCopilotToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("copilot chat: get token: %w", err)
	}

	// Make streaming HTTP request directly
	jsonBody, err := json.Marshal(apiReq)
	if err != nil {
		return nil, fmt.Errorf("copilot chat: marshal request: %w", err)
	}

	url := p.provider.GetBaseURL() + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("copilot chat: create request: %w", err)
	}

	// Set required headers (mirrors library's setCopilotHeaders)
	httpReq.Header.Set("Authorization", "Bearer "+token)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("copilot-integration-id", copilot.CopilotIntegrationID)
	httpReq.Header.Set("editor-version", "vscode/"+copilot.VSCodeVersion)
	httpReq.Header.Set("editor-plugin-version", copilot.EditorPluginVersion)
	httpReq.Header.Set("user-agent", copilot.UserAgent)
	httpReq.Header.Set("openai-intent", copilot.OpenAIIntent)
	httpReq.Header.Set("x-github-api-version", copilot.GitHubAPIVersion)
	httpReq.Header.Set("x-request-id", uuid.New().String())

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("copilot chat: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("copilot chat: API error %d: %s", resp.StatusCode, string(body))
	}

	// Parse SSE stream using the library's exported stream parser
	stream := copilot.NewCopilotStream(resp)
	// Don't defer stream.Close() — it would close resp.Body which we already defer

	var (
		textContent  string
		toolCalls    = map[int]*ContentBlock{}
		finishReason string
		promptTokens int
		outputTokens int
	)

	for {
		chunk, err := stream.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("copilot chat: reading stream: %w", err)
		}

		if chunk.Done {
			break
		}

		textContent += chunk.Content

		if chunk.Usage.PromptTokens > 0 {
			promptTokens = chunk.Usage.PromptTokens
		}
		if chunk.Usage.CompletionTokens > 0 {
			outputTokens = chunk.Usage.CompletionTokens
		}

		for _, choice := range chunk.Choices {
			if choice.FinishReason != "" {
				finishReason = choice.FinishReason
			}
			for _, tc := range choice.Delta.ToolCalls {
				if tc.ID != "" {
					toolCalls[len(toolCalls)] = &ContentBlock{
						Type:  "tool_use",
						ID:    tc.ID,
						Name:  tc.Function.Name,
						Input: json.RawMessage(tc.Function.Arguments),
					}
				} else if tc.Function.Arguments != "" {
					// Find the last tool call to append arguments
					idx := len(toolCalls) - 1
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
		PromptTokens: promptTokens,
		OutputTokens: outputTokens,
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
