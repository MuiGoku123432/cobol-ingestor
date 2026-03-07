package modernize

import (
	"encoding/json"
	"fmt"
	"net/http"

	"cobol-ingestor/internal/llm"

	"github.com/gin-gonic/gin"
)

const maxToolIterations = 10

// ChatHandler creates a Gin handler for the chat endpoint.
// Accepts *ProviderState for deferred provider initialization (Copilot auth flow).
// The defaultModel and defaultMaxTokens are used as fallbacks when no model has been selected via the model picker.
func ChatHandler(ps *ProviderState, mcpClient *MCPClient, defaultModel string, defaultMaxTokens int, sessionStore SessionStore) gin.HandlerFunc {
	tools := GetToolDefinitions()

	return func(c *gin.Context) {
		provider := ps.Get()
		if provider == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
			return
		}

		// Use model from ProviderState if selected, otherwise fall back to defaults
		model := ps.GetModel()
		if model == "" {
			model = defaultModel
		}
		maxTokens := ps.GetMaxTokens()
		if maxTokens == 0 {
			maxTokens = defaultMaxTokens
		}
		var req struct {
			Messages       []chatInputMessage `json:"messages"`
			TargetLanguage string             `json:"targetLanguage"`
			Framework      string             `json:"framework"`
			SessionID      string             `json:"sessionId"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		systemPrompt, err := BuildSystemPrompt(req.TargetLanguage, req.Framework)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to build system prompt"})
			return
		}

		// Convert input messages to ChatMessage format
		messages := make([]llm.ChatMessage, 0, len(req.Messages))
		for _, m := range req.Messages {
			messages = append(messages, llm.ChatMessage{
				Role:    m.Role,
				Content: []llm.ContentBlock{llm.NewTextContent(m.Content)},
			})
		}

		// Set SSE headers
		c.Writer.Header().Set("Content-Type", "text/event-stream")
		c.Writer.Header().Set("Cache-Control", "no-cache")
		c.Writer.Header().Set("Connection", "keep-alive")
		c.Writer.Header().Set("X-Accel-Buffering", "no")

		ctx := c.Request.Context()

		for i := 0; i < maxToolIterations; i++ {
			chatReq := llm.ChatRequest{
				Model:       model,
				System:      systemPrompt,
				Messages:    messages,
				Tools:       tools,
				MaxTokens:   maxTokens,
				Temperature: 0.3,
			}

			resp, err := provider.CompleteChat(ctx, chatReq)
			if err != nil {
				SendSSEJSON(c.Writer, "error", map[string]string{"error": fmt.Sprintf("LLM error: %v", err)})
				return
			}

			// Send any text content
			if text := resp.TextContent(); text != "" {
				SendSSEJSON(c.Writer, "text", map[string]string{"content": text})
			}

			if resp.StopReason != "tool_use" {
				// Persist session
				if sessionStore != nil {
					assistantText := resp.TextContent()
					allMessages := make([]chatInputMessage, len(req.Messages))
					copy(allMessages, req.Messages)
					if assistantText != "" {
						allMessages = append(allMessages, chatInputMessage{Role: "assistant", Content: assistantText})
					}

					if req.SessionID == "" {
						title := req.Messages[0].Content
						if len(title) > 50 {
							title = title[:50]
						}
						session, err := sessionStore.Create(title)
						if err == nil {
							_ = sessionStore.Update(session.ID, allMessages, "")
							SendSSEJSON(c.Writer, "session_created", map[string]string{"id": session.ID, "title": session.Title})
						}
					} else {
						_ = sessionStore.Update(req.SessionID, allMessages, "")
					}
				}
				SendSSE(c.Writer, "done", "{}")
				return
			}

			// Process tool calls
			toolUseBlocks := resp.ToolUseBlocks()

			// Add assistant response to message history
			messages = append(messages, llm.ChatMessage{
				Role:    llm.RoleAssistant,
				Content: resp.Content,
			})

			// Execute each tool and collect results
			var toolResults []llm.ContentBlock
			for _, block := range toolUseBlocks {
				SendSSEJSON(c.Writer, "tool_start", map[string]string{
					"name":  block.Name,
					"id":    block.ID,
					"input": truncate(string(block.Input), 200),
				})

				var args map[string]any
				if err := json.Unmarshal(block.Input, &args); err != nil {
					args = map[string]any{}
				}

				result, err := mcpClient.CallTool(ctx, block.Name, args)
				if err != nil {
					toolResults = append(toolResults, llm.NewToolResultContent(block.ID, fmt.Sprintf("Error: %v", err), true))
					SendSSEJSON(c.Writer, "tool_result", map[string]string{
						"name":   block.Name,
						"id":     block.ID,
						"error":  err.Error(),
						"result": "",
					})
				} else {
					toolResults = append(toolResults, llm.NewToolResultContent(block.ID, result, false))
					SendSSEJSON(c.Writer, "tool_result", map[string]string{
						"name":   block.Name,
						"id":     block.ID,
						"result": truncate(result, 500),
					})
				}
			}

			// Add tool results as a user message
			messages = append(messages, llm.ChatMessage{
				Role:    llm.RoleUser,
				Content: toolResults,
			})
		}

		SendSSEJSON(c.Writer, "error", map[string]string{"error": "max tool iterations reached"})
	}
}

type chatInputMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
