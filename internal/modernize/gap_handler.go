package modernize

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"cobol-ingestor/internal/llm"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// GapAnalysisParams holds all parameters for a gap analysis invocation.
type GapAnalysisParams struct {
	Provider     *ProviderState
	MCPClient    *MCPClient // COBOL graph MCP client
	SessionStore SessionStore
	SessionID    string
	Messages     []InputMessage
	MultiRound   bool
	TargetRepos  []string // optional filter to specific repo URLs
	Emitter      EventEmitter
	Logger       *zap.Logger
}

// GapAnalysisHandler creates a Gin handler for the /api/gap-analysis endpoint.
// It follows the same pattern as SwarmHandler but uses gap-specific agents that
// have access to both cobol_* and target_* tools via the MCPBridge.
func GapAnalysisHandler(ps *ProviderState, mcpClient *MCPClient, defaultModel string, defaultMaxTokens int, sessionStore SessionStore, logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		if ps.Get() == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
			return
		}

		var req struct {
			Messages    []chatInputMessage `json:"messages"`
			SessionID   string             `json:"sessionId"`
			MultiRound  bool               `json:"multiRound"`
			TargetRepos []string           `json:"targetRepos"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		SetSSEHeaders(c.Writer)

		emitter := &SSEEmitter{W: c.Writer, Mu: &sync.Mutex{}}

		gLogger := logger
		if gLogger == nil {
			gLogger, _ = zap.NewProduction()
		}

		err := RunGapAnalysis(c.Request.Context(), GapAnalysisParams{
			Provider:     ps,
			MCPClient:    mcpClient,
			SessionStore: sessionStore,
			SessionID:    req.SessionID,
			Messages:     req.Messages,
			MultiRound:   req.MultiRound,
			TargetRepos:  req.TargetRepos,
			Emitter:      emitter,
			Logger:       gLogger,
		})
		if err != nil {
			emitter.Emit("error", map[string]string{"error": err.Error()})
		}
	}
}

// RunGapAnalysis runs the gap analysis swarm: 5 specialist agents in parallel
// with access to both COBOL and target stack tools, coordinator synthesizes.
// Follows RunSwarm pattern from swarm.go.
func RunGapAnalysis(ctx context.Context, p GapAnalysisParams) error {
	provider := p.Provider.Get()
	if provider == nil {
		return fmt.Errorf("not authenticated")
	}

	model := p.Provider.GetModel()
	maxTokens := p.Provider.GetMaxTokens()
	if maxTokens == 0 {
		maxTokens = 16384
	}

	userQuery := ""
	for i := len(p.Messages) - 1; i >= 0; i-- {
		if p.Messages[i].Role == "user" {
			userQuery = p.Messages[i].Content
			break
		}
	}
	if userQuery == "" {
		return fmt.Errorf("no user message found")
	}

	// Get tools from MCP client (both COBOL and target stack tools are in the same MCP server)
	tools, err := getGapTools(ctx, p.MCPClient)
	if err != nil {
		return fmt.Errorf("listing gap tools: %w", err)
	}

	p.Logger.Info("gap analysis starting", zap.Int("tools", len(tools)), zap.String("query", truncate(userQuery, 100)))

	toolCache := newToolCache()

	// Run 5 gap agents in parallel
	type agentOut struct {
		role   gapAgentRole
		output string
		err    error
	}

	results := make([]agentOut, len(gapAgentRoles))
	var wg sync.WaitGroup

	for i, role := range gapAgentRoles {
		wg.Add(1)
		go func(idx int, r gapAgentRole) {
			defer wg.Done()

			p.Emitter.Emit("agent_start", map[string]string{"agent": r.Name})

			output, agentErr := runGapAgentChat(ctx, r, p.MCPClient, provider, model, maxTokens, tools, toolCache, userQuery, "", p.Emitter, p.Logger)
			results[idx] = agentOut{role: r, output: output, err: agentErr}

			if agentErr != nil {
				p.Emitter.Emit("agent_complete", map[string]string{"agent": r.Name, "status": "error"})
			} else {
				p.Emitter.Emit("agent_complete", map[string]string{"agent": r.Name, "status": "ok"})
			}
		}(i, role)
	}
	wg.Wait()

	// Build agent results for coordinator
	agentResults := make([]agentResult, 0, len(results))
	for _, res := range results {
		summary := res.output
		if res.err != nil {
			summary = fmt.Sprintf("Error: %v", res.err)
		}
		agentResults = append(agentResults, agentResult{Name: res.role.Name, Summary: summarizeForCoordinator(summary, maxContextCharsLatest)})
	}

	// Coordinator synthesis
	p.Emitter.Emit("coordinator_start", map[string]string{})
	synthesis, coordErr := runGapCoordinatorChat(ctx, agentResults, userQuery, p.MCPClient, provider, model, maxTokens, tools, toolCache, p.Emitter, p.Logger)
	if coordErr != nil {
		return fmt.Errorf("gap coordinator: %w", coordErr)
	}

	p.Emitter.Emit("content", map[string]string{"text": synthesis})
	p.Emitter.Emit("done", map[string]string{})
	return nil
}

// runGapAgentChat runs one gap specialist agent's investigation loop.
func runGapAgentChat(
	ctx context.Context,
	role gapAgentRole,
	mcpClient *MCPClient,
	provider llm.ChatProvider,
	model string,
	maxTokens int,
	tools []llm.ToolDefinition,
	cache *toolCache,
	userQuery, followUp string,
	emitter EventEmitter,
	logger *zap.Logger,
) (string, error) {
	var promptBuf bytes.Buffer
	if err := role.Prompt.Execute(&promptBuf, map[string]string{
		"UserQuery":     userQuery,
		"PriorFindings": "",
		"FollowUpQuery": followUp,
	}); err != nil {
		return "", fmt.Errorf("rendering prompt for %s: %w", role.ID, err)
	}

	messages := []llm.ChatMessage{
		{
			Role:    llm.RoleUser,
			Content: []llm.ContentBlock{llm.NewTextContent("Begin your gap analysis investigation.")},
		},
	}

	for i := range maxSwarmToolIterations {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}

		resp, err := provider.CompleteChat(ctx, llm.ChatRequest{
			Model:     model,
			System:    promptBuf.String(),
			MaxTokens: maxTokens,
			Tools:     tools,
			Messages:  messages,
		})
		if err != nil {
			return "", fmt.Errorf("iteration %d: %w", i+1, err)
		}

		if resp.StopReason != "tool_use" {
			return resp.TextContent(), nil
		}

		messages = append(messages, llm.ChatMessage{
			Role:    llm.RoleAssistant,
			Content: resp.Content,
		})

		var toolResults []llm.ContentBlock
		for _, block := range resp.ToolUseBlocks() {
			emitter.Emit("agent_tool_start", map[string]string{"agent": role.Name, "tool": block.Name})

			cacheKey := cache.Key(block.Name, map[string]any{})
			if len(block.Input) > 0 {
				var args map[string]any
				_ = json.Unmarshal(block.Input, &args)
				cacheKey = cache.Key(block.Name, args)
			}

			if cached, ok := cache.Get(cacheKey); ok {
				toolResults = append(toolResults, llm.NewToolResultContent(block.ID, cached, false))
				emitter.Emit("agent_tool_result", map[string]string{"agent": role.Name, "tool": block.Name, "cached": "true"})
				continue
			}

			var args map[string]any
			_ = json.Unmarshal(block.Input, &args)
			result, callErr := mcpClient.CallTool(ctx, block.Name, args)
			if callErr != nil {
				logger.Debug("tool error", zap.String("tool", block.Name), zap.Error(callErr))
				toolResults = append(toolResults, llm.NewToolResultContent(block.ID, fmt.Sprintf("Error: %v", callErr), true))
			} else {
				cache.Set(cacheKey, result)
				toolResults = append(toolResults, llm.NewToolResultContent(block.ID, result, false))
			}
			emitter.Emit("agent_tool_result", map[string]string{"agent": role.Name, "tool": block.Name})
		}

		messages = append(messages, llm.ChatMessage{
			Role:    llm.RoleUser,
			Content: toolResults,
		})
	}

	return "", fmt.Errorf("max iterations reached for gap agent %s", role.ID)
}

// runGapCoordinatorChat synthesizes gap agent findings using the coordinator prompt.
func runGapCoordinatorChat(
	ctx context.Context,
	agentResults []agentResult,
	userQuery string,
	mcpClient *MCPClient,
	provider llm.ChatProvider,
	model string,
	maxTokens int,
	tools []llm.ToolDefinition,
	cache *toolCache,
	emitter EventEmitter,
	logger *zap.Logger,
) (string, error) {
	var promptBuf bytes.Buffer
	if err := gapCoordinatorPrompt.Execute(&promptBuf, swarmPromptData{
		AgentResults: agentResults,
		UserQuery:    userQuery,
	}); err != nil {
		return "", fmt.Errorf("rendering gap coordinator prompt: %w", err)
	}

	messages := []llm.ChatMessage{
		{
			Role:    llm.RoleUser,
			Content: []llm.ContentBlock{llm.NewTextContent("Synthesize the agent findings into a comprehensive gap analysis.")},
		},
	}

	for i := range maxCoordinatorToolIterations {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}

		resp, err := provider.CompleteChat(ctx, llm.ChatRequest{
			Model:     model,
			System:    promptBuf.String(),
			MaxTokens: maxTokens,
			Tools:     tools,
			Messages:  messages,
		})
		if err != nil {
			return "", fmt.Errorf("coordinator iteration %d: %w", i+1, err)
		}

		if resp.StopReason != "tool_use" {
			return resp.TextContent(), nil
		}

		messages = append(messages, llm.ChatMessage{
			Role:    llm.RoleAssistant,
			Content: resp.Content,
		})

		var toolResults []llm.ContentBlock
		for _, block := range resp.ToolUseBlocks() {
			var args map[string]any
			_ = json.Unmarshal(block.Input, &args)
			result, callErr := mcpClient.CallTool(ctx, block.Name, args)
			if callErr != nil {
				toolResults = append(toolResults, llm.NewToolResultContent(block.ID, fmt.Sprintf("Error: %v", callErr), true))
			} else {
				toolResults = append(toolResults, llm.NewToolResultContent(block.ID, result, false))
			}
		}
		messages = append(messages, llm.ChatMessage{Role: llm.RoleUser, Content: toolResults})
	}

	return "", fmt.Errorf("gap coordinator max iterations reached")
}

// getGapTools retrieves tool definitions from the MCP client, prefixing target stack
// tools that come from the same server (they are already registered as list_target_*,
// list_business_gaps, etc. in the MCP server).
func getGapTools(ctx context.Context, mcpClient *MCPClient) ([]llm.ToolDefinition, error) {
	if mcpClient == nil {
		return GetToolDefinitions(), nil
	}
	mcpTools, err := mcpClient.ListTools(ctx)
	if err != nil {
		// Fall back to static definitions
		return GetToolDefinitions(), nil
	}
	var tools []llm.ToolDefinition
	for _, t := range mcpTools {
		schema := make(map[string]any)
		if b, marshalErr := json.Marshal(t.InputSchema); marshalErr == nil {
			_ = json.Unmarshal(b, &schema)
		}
		if len(schema) == 0 {
			schema = map[string]any{"type": "object", "properties": map[string]any{}}
		}
		tools = append(tools, llm.ToolDefinition{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: schema,
		})
	}
	return tools, nil
}

