package modernize

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"text/template"

	"cobol-ingestor/internal/llm"

	"github.com/gin-gonic/gin"
)

const maxSwarmToolIterations = 10

type agentRole struct {
	ID     string
	Name   string
	Prompt *template.Template
}

var agentRoles = []agentRole{
	{ID: "structure", Name: "Structure Analyzer", Prompt: structureAnalyzerPrompt},
	{ID: "dataflow", Name: "Data Flow Analyst", Prompt: dataFlowAnalystPrompt},
	{ID: "dependency", Name: "Dependency Mapper", Prompt: dependencyMapperPrompt},
	{ID: "business", Name: "Business Logic Extractor", Prompt: businessLogicExtractorPrompt},
}

// SwarmHandler creates a Gin handler for the agent swarm endpoint.
func SwarmHandler(ps *ProviderState, mcpClient *MCPClient, defaultModel string, defaultMaxTokens int, sessionStore SessionStore) gin.HandlerFunc {
	tools := GetToolDefinitions()

	return func(c *gin.Context) {
		provider := ps.Get()
		if provider == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
			return
		}

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
			Integrations   string             `json:"integrations"`
			SessionID      string             `json:"sessionId"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Extract the user's latest question
		userQuery := ""
		for i := len(req.Messages) - 1; i >= 0; i-- {
			if req.Messages[i].Role == "user" {
				userQuery = req.Messages[i].Content
				break
			}
		}
		if userQuery == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "no user message found"})
			return
		}

		// Set SSE headers
		c.Writer.Header().Set("Content-Type", "text/event-stream")
		c.Writer.Header().Set("Cache-Control", "no-cache")
		c.Writer.Header().Set("Connection", "keep-alive")
		c.Writer.Header().Set("X-Accel-Buffering", "no")

		ctx := c.Request.Context()
		w := c.Writer
		var mu sync.Mutex

		promptData := swarmPromptData{
			TargetLanguage: req.TargetLanguage,
			Framework:      req.Framework,
			Integrations:   req.Integrations,
		}

		// Run 4 agents in parallel
		results := make([]agentResult, len(agentRoles))
		var wg sync.WaitGroup

		for i, role := range agentRoles {
			wg.Add(1)
			go func(idx int, role agentRole) {
				defer wg.Done()

				summary, err := runAgent(ctx, role, userQuery, promptData, provider, mcpClient, tools, model, maxTokens, w, &mu)
				if err != nil {
					mu.Lock()
					SendSSEJSON(w, "agent_complete", map[string]string{
						"id":      role.ID,
						"summary": fmt.Sprintf("Error: %v", err),
					})
					mu.Unlock()
					results[idx] = agentResult{Name: role.Name, Summary: fmt.Sprintf("Error: %v", err)}
					return
				}
				results[idx] = agentResult{Name: role.Name, Summary: summary}
			}(i, role)
		}

		wg.Wait()

		// Check if client disconnected
		if ctx.Err() != nil {
			return
		}

		// Run Coordinator
		mu.Lock()
		SendSSEJSON(w, "synthesis_start", map[string]string{})
		mu.Unlock()

		promptData.UserQuery = userQuery
		promptData.AgentResults = results

		coordSystemPrompt, err := buildSwarmPrompt(coordinatorPrompt, promptData)
		if err != nil {
			mu.Lock()
			SendSSEJSON(w, "error", map[string]string{"error": "failed to build coordinator prompt"})
			mu.Unlock()
			return
		}

		coordPreamble := BuildContextPreamble(req.TargetLanguage, req.Framework, req.Integrations)
		coordMessages := []llm.ChatMessage{
			{
				Role:    llm.RoleUser,
				Content: []llm.ContentBlock{llm.NewTextContent(coordPreamble + userQuery)},
			},
		}

		chatReq := llm.ChatRequest{
			Model:       model,
			System:      coordSystemPrompt,
			Messages:    coordMessages,
			MaxTokens:   maxTokens,
			Temperature: 0.3,
		}

		resp, err := provider.CompleteChat(ctx, chatReq)
		if err != nil {
			mu.Lock()
			SendSSEJSON(w, "error", map[string]string{"error": fmt.Sprintf("Coordinator error: %v", err)})
			mu.Unlock()
			return
		}

		coordText := resp.TextContent()
		if coordText != "" {
			mu.Lock()
			SendSSEJSON(w, "text", map[string]string{"content": coordText})
			mu.Unlock()
		}

		// Persist session
		if sessionStore != nil {
			allMessages := make([]chatInputMessage, len(req.Messages))
			copy(allMessages, req.Messages)
			if coordText != "" {
				allMessages = append(allMessages, chatInputMessage{Role: "assistant", Content: coordText})
			}

			if req.SessionID == "" {
				title := userQuery
				if len(title) > 50 {
					title = title[:50]
				}
				session, err := sessionStore.Create(title)
				if err == nil {
					_ = sessionStore.Update(session.ID, allMessages, "")
					mu.Lock()
					SendSSEJSON(w, "session_created", map[string]string{"id": session.ID, "title": session.Title})
					mu.Unlock()
				}
			} else {
				_ = sessionStore.Update(req.SessionID, allMessages, "")
			}
		}

		mu.Lock()
		SendSSE(w, "done", "{}")
		mu.Unlock()
	}
}

func runAgent(
	ctx context.Context,
	role agentRole,
	userQuery string,
	promptData swarmPromptData,
	provider llm.ChatProvider,
	mcpClient *MCPClient,
	tools []llm.ToolDefinition,
	model string,
	maxTokens int,
	w http.ResponseWriter,
	mu *sync.Mutex,
) (string, error) {
	// Send agent_start event
	mu.Lock()
	SendSSEJSON(w, "agent_start", map[string]string{
		"id":   role.ID,
		"name": role.Name,
	})
	mu.Unlock()

	systemPrompt, err := buildSwarmPrompt(role.Prompt, promptData)
	if err != nil {
		return "", fmt.Errorf("build prompt: %w", err)
	}

	preamble := BuildContextPreamble(promptData.TargetLanguage, promptData.Framework, promptData.Integrations)
	messages := []llm.ChatMessage{
		{
			Role:    llm.RoleUser,
			Content: []llm.ContentBlock{llm.NewTextContent(preamble + userQuery)},
		},
	}

	var fullText string

	for i := 0; i < maxSwarmToolIterations; i++ {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}

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
			return "", fmt.Errorf("LLM error: %w", err)
		}

		if text := resp.TextContent(); text != "" {
			fullText += text
			mu.Lock()
			SendSSEJSON(w, "agent_progress", map[string]string{
				"id":      role.ID,
				"content": text,
			})
			mu.Unlock()
		}

		if resp.StopReason != "tool_use" {
			mu.Lock()
			SendSSEJSON(w, "agent_complete", map[string]string{
				"id":      role.ID,
				"summary": truncate(fullText, 500),
			})
			mu.Unlock()
			return fullText, nil
		}

		// Process tool calls
		toolUseBlocks := resp.ToolUseBlocks()

		messages = append(messages, llm.ChatMessage{
			Role:    llm.RoleAssistant,
			Content: resp.Content,
		})

		var toolResults []llm.ContentBlock
		for _, block := range toolUseBlocks {
			mu.Lock()
			SendSSEJSON(w, "agent_tool_start", map[string]string{
				"id":       role.ID,
				"toolName": block.Name,
				"toolId":   block.ID,
			})
			mu.Unlock()

			var args map[string]any
			if err := json.Unmarshal(block.Input, &args); err != nil {
				args = map[string]any{}
			}

			result, err := mcpClient.CallTool(ctx, block.Name, args)
			if err != nil {
				toolResults = append(toolResults, llm.NewToolResultContent(block.ID, fmt.Sprintf("Error: %v", err), true))
				mu.Lock()
				SendSSEJSON(w, "agent_tool_result", map[string]string{
					"id":     role.ID,
					"toolId": block.ID,
					"result": err.Error(),
				})
				mu.Unlock()
			} else {
				toolResults = append(toolResults, llm.NewToolResultContent(block.ID, result, false))
				mu.Lock()
				SendSSEJSON(w, "agent_tool_result", map[string]string{
					"id":     role.ID,
					"toolId": block.ID,
					"result": truncate(result, 300),
				})
				mu.Unlock()
			}
		}

		messages = append(messages, llm.ChatMessage{
			Role:    llm.RoleUser,
			Content: toolResults,
		})
	}

	// Max iterations reached — return what we have
	mu.Lock()
	SendSSEJSON(w, "agent_complete", map[string]string{
		"id":      role.ID,
		"summary": truncate(fullText, 500),
	})
	mu.Unlock()
	return fullText, nil
}
