package modernize

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"text/template"

	"cobol-ingestor/internal/llm"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

const (
	maxSwarmToolIterations       = 25
	maxCoordinatorToolIterations = 10
	maxContextCharsLatest        = 3000
	maxContextCharsPrior         = 1500
)

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

// toolCache provides a thread-safe cache for MCP tool call results.
type toolCache struct {
	mu    sync.RWMutex
	store map[string]string
}

func newToolCache() *toolCache {
	return &toolCache{store: make(map[string]string)}
}

// Key builds a deterministic cache key from tool name and arguments.
func (tc *toolCache) Key(name string, args map[string]any) string {
	// Sort keys for determinism
	keys := make([]string, 0, len(args))
	for k := range args {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	ordered := make(map[string]any, len(args))
	for _, k := range keys {
		ordered[k] = args[k]
	}
	b, _ := json.Marshal(map[string]any{"tool": name, "args": ordered})
	return string(b)
}

func (tc *toolCache) Get(key string) (string, bool) {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	v, ok := tc.store[key]
	return v, ok
}

func (tc *toolCache) Set(key, value string) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.store[key] = value
}

// truncateForContext truncates a string to maxChars, appending "..." if truncated.
func truncateForContext(s string, maxChars int) string {
	if len(s) <= maxChars {
		return s
	}
	return s[:maxChars] + "..."
}

// summarizeForCoordinator extracts key findings from agent output to reduce noise.
// It pulls structured lines (bullets, headers, numbered items) and COBOL artifact names,
// dropping verbose prose paragraphs. Falls back to truncation if extraction yields nothing.
func summarizeForCoordinator(s string, maxChars int) string {
	if len(s) <= maxChars {
		return s
	}

	var keyLines []string
	for _, line := range strings.Split(s, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		// Keep structured lines: bullets, numbered items, headers, lines with COBOL artifacts
		if strings.HasPrefix(trimmed, "-") ||
			strings.HasPrefix(trimmed, "*") ||
			strings.HasPrefix(trimmed, "#") ||
			(len(trimmed) > 0 && trimmed[0] >= '1' && trimmed[0] <= '9' && strings.Contains(trimmed[:min(3, len(trimmed))], ".")) ||
			strings.Contains(trimmed, "PROGRAM-ID") ||
			strings.Contains(trimmed, "COPY ") ||
			strings.Contains(trimmed, "CALL ") ||
			strings.Contains(trimmed, "PERFORM ") ||
			strings.ContainsAny(trimmed, "→←") {
			keyLines = append(keyLines, trimmed)
		}
	}

	if len(keyLines) == 0 {
		return truncateForContext(s, maxChars)
	}

	result := strings.Join(keyLines, "\n")
	return truncateForContext(result, maxChars)
}

// coordinatorDecision represents the coordinator's assessment between rounds.
type coordinatorDecision struct {
	Satisfied bool              `json:"satisfied"`
	FollowUps map[string]string `json:"follow_ups"`
	Reasoning string            `json:"reasoning"`
}

// SwarmHandler creates a Gin handler for the agent swarm endpoint.
// This is HTTP glue that delegates to RunSwarm.
func SwarmHandler(ps *ProviderState, mcpClient *MCPClient, defaultModel string, defaultMaxTokens int, sessionStore SessionStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		if ps.Get() == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
			return
		}

		var req struct {
			Messages       []chatInputMessage `json:"messages"`
			TargetLanguage string             `json:"targetLanguage"`
			Framework      string             `json:"framework"`
			Integrations   string             `json:"integrations"`
			DiscoveryMode  bool               `json:"discoveryMode"`
			SessionID      string             `json:"sessionId"`
			MultiRound     bool               `json:"multiRound"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		SetSSEHeaders(c.Writer)

		emitter := &SSEEmitter{W: c.Writer, Mu: &sync.Mutex{}}

		err := RunSwarm(c.Request.Context(), SwarmParams{
			Provider:      ps,
			MCPClient:     mcpClient,
			SessionStore:  sessionStore,
			SessionID:     req.SessionID,
			Messages:      req.Messages,
			DiscoveryMode: req.DiscoveryMode,
			MultiRound:    req.MultiRound,
			TargetLang:    req.TargetLanguage,
			Framework:     req.Framework,
			Integrations:  req.Integrations,
			Emitter:       emitter,
			Logger:        nil,
		})
		if err != nil {
			emitter.Emit("error", map[string]string{"error": err.Error()})
		}
	}
}

// selectAgents determines which agents to run for a given round.
// Round 1: all agents. Round 2+: only those in followUpQueries (or all if nil/empty).
func selectAgents(roles []agentRole, followUpQueries map[string]string, round int) ([]agentRole, map[string]string) {
	if round == 1 || len(followUpQueries) == 0 {
		return roles, followUpQueries
	}
	var selected []agentRole
	for _, role := range roles {
		if _, ok := followUpQueries[role.ID]; ok {
			selected = append(selected, role)
		}
	}
	if len(selected) == 0 {
		return roles, followUpQueries
	}
	return selected, followUpQueries
}

// truncateRoundSummaries creates a copy of round summaries with truncated agent summaries.
func truncateRoundSummaries(rounds []roundSummary, maxChars int) []roundSummary {
	out := make([]roundSummary, len(rounds))
	for i, r := range rounds {
		results := make([]agentResult, len(r.Results))
		for j, ar := range r.Results {
			results[j] = agentResult{
				Name:    ar.Name,
				Summary: truncateForContext(ar.Summary, maxChars),
			}
		}
		out[i] = roundSummary{Round: r.Round, Results: results}
	}
	return out
}

// flattenRounds creates a flat list of agent results for the coordinator.
// The latest round gets higher char limits; prior rounds get lower limits.
// Uses summarizeForCoordinator to extract key findings rather than naive truncation.
func flattenRounds(rounds []roundSummary, latestMax, priorMax int) []agentResult {
	if len(rounds) == 0 {
		return nil
	}
	if len(rounds) == 1 {
		// Single round: use latest max for all
		results := make([]agentResult, len(rounds[0].Results))
		for i, ar := range rounds[0].Results {
			results[i] = agentResult{
				Name:    ar.Name,
				Summary: summarizeForCoordinator(ar.Summary, latestMax),
			}
		}
		return results
	}

	var all []agentResult
	for i, r := range rounds {
		maxChars := priorMax
		if i == len(rounds)-1 {
			maxChars = latestMax
		}
		for _, ar := range r.Results {
			all = append(all, agentResult{
				Name:    fmt.Sprintf("%s (Round %d)", ar.Name, r.Round),
				Summary: summarizeForCoordinator(ar.Summary, maxChars),
			})
		}
	}
	return all
}

func runAgent(
	ctx context.Context,
	sseRole agentRole, // role with potentially round-qualified ID for events
	promptTmpl *template.Template,
	userQuery string,
	promptData swarmPromptData,
	provider llm.ChatProvider,
	mcpClient *MCPClient,
	cache *toolCache,
	tools []llm.ToolDefinition,
	model string,
	maxTokens int,
	emitter EventEmitter,
	round int,
	isMultiRound bool,
) (string, error) {
	// Send agent_start event
	startEvent := map[string]any{
		"id":   sseRole.ID,
		"name": sseRole.Name,
	}
	if isMultiRound {
		startEvent["round"] = round
	}
	emitter.Emit("agent_start", startEvent)

	systemPrompt, err := buildSwarmPrompt(promptTmpl, promptData)
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
			emitter.Emit("agent_progress", map[string]string{
				"id":      sseRole.ID,
				"content": text,
			})
		}

		if resp.StopReason != "tool_use" {
			emitter.Emit("agent_complete", map[string]string{
				"id":      sseRole.ID,
				"summary": truncate(fullText, 500),
			})
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
			emitter.Emit("agent_tool_start", map[string]string{
				"id":       sseRole.ID,
				"toolName": block.Name,
				"toolId":   block.ID,
			})

			var args map[string]any
			if err := json.Unmarshal(block.Input, &args); err != nil {
				args = map[string]any{}
			}

			cacheKey := cache.Key(block.Name, args)
			if cached, ok := cache.Get(cacheKey); ok {
				toolResults = append(toolResults, llm.NewToolResultContent(block.ID, cached, false))
				emitter.Emit("agent_tool_result", map[string]string{
					"id":     sseRole.ID,
					"toolId": block.ID,
					"result": truncate(cached, 300),
					"cached": "true",
				})
			} else {
				result, callErr := mcpClient.CallTool(ctx, block.Name, args)
				if callErr != nil {
					toolResults = append(toolResults, llm.NewToolResultContent(block.ID, fmt.Sprintf("Error: %v", callErr), true))
					emitter.Emit("agent_tool_result", map[string]string{
						"id":     sseRole.ID,
						"toolId": block.ID,
						"result": callErr.Error(),
						"cached": "false",
					})
				} else {
					cache.Set(cacheKey, result)
					toolResults = append(toolResults, llm.NewToolResultContent(block.ID, result, false))
					emitter.Emit("agent_tool_result", map[string]string{
						"id":     sseRole.ID,
						"toolId": block.ID,
						"result": truncate(result, 300),
						"cached": "false",
					})
				}
			}
		}

		messages = append(messages, llm.ChatMessage{
			Role:    llm.RoleUser,
			Content: toolResults,
		})
	}

	// Max iterations reached — return what we have
	emitter.Emit("agent_complete", map[string]string{
		"id":      sseRole.ID,
		"summary": truncate(fullText, 500),
	})
	return fullText, nil
}

// coordinatorDecisionTool is the tool definition used to enforce structured coordinator decisions.
var coordinatorDecisionTool = llm.ToolDefinition{
	Name:        "submit_decision",
	Description: "Submit your coordinator decision about whether the agent findings sufficiently answer the user's question.",
	InputSchema: map[string]any{
		"type": "object",
		"properties": map[string]any{
			"satisfied": map[string]any{
				"type":        "boolean",
				"description": "Whether the findings sufficiently answer the user's question",
			},
			"reasoning": map[string]any{
				"type":        "string",
				"description": "Brief explanation of your decision",
			},
			"follow_ups": map[string]any{
				"type":        "object",
				"description": "Map of agentId to follow-up question. Valid IDs: structure, dataflow, dependency, business",
				"additionalProperties": map[string]any{"type": "string"},
			},
		},
		"required": []any{"satisfied", "reasoning"},
	},
}

// runCoordinatorDecision asks the coordinator if the current findings are sufficient.
// Uses a submit_decision tool call to enforce structured JSON output, with text-based
// JSON parsing as a fallback. Defaults to Satisfied: false on parse failure so that
// a retry round is triggered rather than silently ending investigation.
func runCoordinatorDecision(
	ctx context.Context,
	results []agentResult,
	userQuery string,
	promptData swarmPromptData,
	provider llm.ChatProvider,
	model string,
	emitter EventEmitter,
	round int,
	logger *zap.Logger,
) (*coordinatorDecision, error) {
	decisionPromptData := swarmPromptData{
		TargetLanguage: promptData.TargetLanguage,
		Framework:      promptData.Framework,
		Integrations:   promptData.Integrations,
		UserQuery:      userQuery,
		AgentResults:   results,
	}

	systemPrompt, err := buildSwarmPrompt(coordinatorDecisionSystemPrompt, decisionPromptData)
	if err != nil {
		return &coordinatorDecision{Satisfied: false, Reasoning: "failed to build system prompt"}, err
	}

	userContent, err := buildSwarmPrompt(coordinatorDecisionUserPrompt, decisionPromptData)
	if err != nil {
		return &coordinatorDecision{Satisfied: false, Reasoning: "failed to build user prompt"}, err
	}

	messages := []llm.ChatMessage{
		{
			Role:    llm.RoleUser,
			Content: []llm.ContentBlock{llm.NewTextContent(userContent)},
		},
	}

	chatReq := llm.ChatRequest{
		Model:       model,
		System:      systemPrompt,
		Messages:    messages,
		Tools:       []llm.ToolDefinition{coordinatorDecisionTool},
		MaxTokens:   1024,
		Temperature: 0.2,
	}

	resp, err := provider.CompleteChat(ctx, chatReq)
	if err != nil {
		return &coordinatorDecision{Satisfied: false, Reasoning: "LLM request failed"}, err
	}

	var decision coordinatorDecision
	parsed := false

	// Primary path: extract decision from tool call
	if resp.StopReason == "tool_use" {
		if blocks := resp.ToolUseBlocks(); len(blocks) > 0 {
			if err := json.Unmarshal(blocks[0].Input, &decision); err == nil {
				parsed = true
			} else if logger != nil {
				logger.Warn("coordinator tool call JSON parse failed",
					zap.String("raw_input", string(blocks[0].Input)),
					zap.Error(err),
				)
			}
		}
	}

	// Fallback: try text-based JSON parsing (existing logic)
	if !parsed {
		text := resp.TextContent()
		if err := json.Unmarshal([]byte(text), &decision); err != nil {
			decision = coordinatorDecision{Satisfied: false, Reasoning: "Could not parse decision response"}
			if extracted := extractJSON(text); extracted != "" {
				if jsonErr := json.Unmarshal([]byte(extracted), &decision); jsonErr != nil {
					// Extraction found JSON but it didn't match our schema — keep default
					decision = coordinatorDecision{Satisfied: false, Reasoning: "Could not parse decision response"}
				}
			}
			if decision.Reasoning == "Could not parse decision response" && logger != nil {
				logger.Warn("coordinator decision parse failed",
					zap.String("raw_response", truncate(text, 500)),
					zap.String("stop_reason", resp.StopReason),
					zap.Error(err),
				)
			}
		}
	}

	// Send coordinator_decision event
	emitter.Emit("coordinator_decision", map[string]any{
		"round":     round,
		"satisfied": decision.Satisfied,
		"reasoning": decision.Reasoning,
		"followUps": decision.FollowUps,
	})

	return &decision, nil
}

// extractJSON tries to extract a JSON object from text that may contain markdown code blocks.
// It correctly handles braces inside JSON string values.
func extractJSON(s string) string {
	start := strings.Index(s, "{")
	if start == -1 {
		return ""
	}
	depth := 0
	inString := false
	escaped := false
	for i := start; i < len(s); i++ {
		c := s[i]
		if escaped {
			escaped = false
			continue
		}
		if c == '\\' && inString {
			escaped = true
			continue
		}
		if c == '"' {
			inString = !inString
			continue
		}
		if inString {
			continue
		}
		switch c {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[start : i+1]
			}
		}
	}
	return ""
}

// runCoordinatorSynthesis runs the coordinator with tool access for final synthesis.
func runCoordinatorSynthesis(
	ctx context.Context,
	promptData swarmPromptData,
	provider llm.ChatProvider,
	mcpClient *MCPClient,
	cache *toolCache,
	tools []llm.ToolDefinition,
	model string,
	maxTokens int,
	emitter EventEmitter,
) (string, error) {
	coordSystemPrompt, err := buildSwarmPrompt(coordinatorPrompt, promptData)
	if err != nil {
		return "", fmt.Errorf("build coordinator prompt: %w", err)
	}

	coordPreamble := BuildContextPreamble(promptData.TargetLanguage, promptData.Framework, promptData.Integrations)
	messages := []llm.ChatMessage{
		{
			Role:    llm.RoleUser,
			Content: []llm.ContentBlock{llm.NewTextContent(coordPreamble + promptData.UserQuery)},
		},
	}

	var fullText string

	for i := 0; i < maxCoordinatorToolIterations; i++ {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}

		chatReq := llm.ChatRequest{
			Model:       model,
			System:      coordSystemPrompt,
			Messages:    messages,
			Tools:       tools,
			MaxTokens:   maxTokens,
			Temperature: 0.3,
		}

		resp, err := provider.CompleteChat(ctx, chatReq)
		if err != nil {
			return "", fmt.Errorf("coordinator LLM error: %w", err)
		}

		if text := resp.TextContent(); text != "" {
			fullText += text
			emitter.Emit("text", map[string]string{"content": text})
		}

		if resp.StopReason != "tool_use" {
			return fullText, nil
		}

		// Process coordinator tool calls
		toolUseBlocks := resp.ToolUseBlocks()

		messages = append(messages, llm.ChatMessage{
			Role:    llm.RoleAssistant,
			Content: resp.Content,
		})

		var toolResults []llm.ContentBlock
		for _, block := range toolUseBlocks {
			emitter.Emit("coordinator_tool_start", map[string]string{
				"toolName": block.Name,
				"toolId":   block.ID,
			})

			var args map[string]any
			if err := json.Unmarshal(block.Input, &args); err != nil {
				args = map[string]any{}
			}

			cacheKey := cache.Key(block.Name, args)
			if cached, ok := cache.Get(cacheKey); ok {
				toolResults = append(toolResults, llm.NewToolResultContent(block.ID, cached, false))
				emitter.Emit("coordinator_tool_result", map[string]string{
					"toolId": block.ID,
					"result": truncate(cached, 300),
					"cached": "true",
				})
			} else {
				result, callErr := mcpClient.CallTool(ctx, block.Name, args)
				if callErr != nil {
					toolResults = append(toolResults, llm.NewToolResultContent(block.ID, fmt.Sprintf("Error: %v", callErr), true))
					emitter.Emit("coordinator_tool_result", map[string]string{
						"toolId": block.ID,
						"result": callErr.Error(),
						"cached": "false",
					})
				} else {
					cache.Set(cacheKey, result)
					toolResults = append(toolResults, llm.NewToolResultContent(block.ID, result, false))
					emitter.Emit("coordinator_tool_result", map[string]string{
						"toolId": block.ID,
						"result": truncate(result, 300),
						"cached": "false",
					})
				}
			}
		}

		messages = append(messages, llm.ChatMessage{
			Role:    llm.RoleUser,
			Content: toolResults,
		})
	}

	return fullText, nil
}
