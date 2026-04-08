package targetstack

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"text/template"

	"cobol-ingestor/internal/graph"
	"cobol-ingestor/internal/llm"
	"cobol-ingestor/internal/modernize"
	"cobol-ingestor/prompts"

	"go.uber.org/zap"
)

const (
	maxGapAgentIter = 25
	maxGapRounds    = 2
)

// GapBridge wraps the COBOL graph MCP client and the target stack MCP client,
// namespacing tools as cobol_* and target_* so a single LLM can query both graphs.
type GapBridge struct {
	cobolClient  *modernize.MCPClient
	targetClient *modernize.MCPClient
}

// NewGapBridge creates a GapBridge from two MCP clients.
func NewGapBridge(cobolClient, targetClient *modernize.MCPClient) *GapBridge {
	return &GapBridge{cobolClient: cobolClient, targetClient: targetClient}
}

// ListAllTools discovers tools from both MCP servers with namespace prefixes.
func (b *GapBridge) ListAllTools(ctx context.Context) ([]llm.ToolDefinition, error) {
	var tools []llm.ToolDefinition

	if b.cobolClient != nil {
		cobolTools, err := b.cobolClient.ListTools(ctx)
		if err != nil {
			return nil, fmt.Errorf("listing COBOL tools: %w", err)
		}
		for _, t := range cobolTools {
			tools = append(tools, llm.ToolDefinition{
				Name:        "cobol_" + t.Name,
				Description: "[COBOL Graph] " + t.Description,
				InputSchema: schemaToMap(t.InputSchema),
			})
		}
	}

	if b.targetClient != nil {
		targetTools, err := b.targetClient.ListTools(ctx)
		if err != nil {
			return nil, fmt.Errorf("listing target stack tools: %w", err)
		}
		for _, t := range targetTools {
			tools = append(tools, llm.ToolDefinition{
				Name:        "target_" + t.Name,
				Description: "[Target Stack] " + t.Description,
				InputSchema: schemaToMap(t.InputSchema),
			})
		}
	}

	return tools, nil
}

// CallTool strips the namespace prefix and routes to the correct MCP client.
func (b *GapBridge) CallTool(ctx context.Context, name string, args map[string]any) (string, error) {
	switch {
	case strings.HasPrefix(name, "cobol_"):
		if b.cobolClient == nil {
			return "", fmt.Errorf("COBOL client not available")
		}
		return b.cobolClient.CallTool(ctx, strings.TrimPrefix(name, "cobol_"), args)
	case strings.HasPrefix(name, "target_"):
		if b.targetClient == nil {
			return "", fmt.Errorf("target stack client not available")
		}
		return b.targetClient.CallTool(ctx, strings.TrimPrefix(name, "target_"), args)
	default:
		return "", fmt.Errorf("unknown tool namespace in: %s", name)
	}
}

// schemaToMap converts an MCP tool input schema to the map format the LLM layer expects.
func schemaToMap(schema any) map[string]any {
	if schema == nil {
		return map[string]any{"type": "object", "properties": map[string]any{}}
	}
	b, err := json.Marshal(schema)
	if err != nil {
		return map[string]any{"type": "object", "properties": map[string]any{}}
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return map[string]any{"type": "object", "properties": map[string]any{}}
	}
	return m
}

// GapSwarmResult holds the output of a gap analysis swarm run.
type GapSwarmResult struct {
	Gaps         []graph.BusinessGap
	Requirements []graph.BusinessRequirement
	Summary      string
	AgentOutputs map[string]string
}

// RunGapSwarm runs the full gap analysis swarm: 5 specialist agents in parallel,
// coordinator synthesizes, writes results. Follows the modernize/swarm.go pattern.
func RunGapSwarm(ctx context.Context, bridge *GapBridge, provider llm.ChatProvider, model string, maxTokens int, logger *zap.Logger) (*GapSwarmResult, error) {
	agentTpl, err := template.New("gap_agent").Parse(prompts.TSGapAgents)
	if err != nil {
		return nil, fmt.Errorf("parsing gap agent template: %w", err)
	}
	coordTpl, err := template.New("gap_coord").Parse(prompts.TSGapCoordinator)
	if err != nil {
		return nil, fmt.Errorf("parsing gap coordinator template: %w", err)
	}

	tools, err := bridge.ListAllTools(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing tools: %w", err)
	}
	logger.Info("gap swarm starting", zap.Int("tools", len(tools)), zap.Int("agents", len(gapAgents)))

	// Shared tool cache to avoid redundant MCP calls across agents
	toolCache := &sync.Map{}

	// ---- Round 1: all agents in parallel ----
	agentOutputs := make(map[string]string, len(gapAgents))
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, agent := range gapAgents {
		wg.Add(1)
		go func(ag gapAgentDef) {
			defer wg.Done()

			output, runErr := runGapAgent(ctx, ag, bridge, provider, model, maxTokens, tools, toolCache, "", "", agentTpl, logger)
			mu.Lock()
			if runErr != nil {
				logger.Error("gap agent failed", zap.String("agent", ag.ID), zap.Error(runErr))
				agentOutputs[ag.ID] = fmt.Sprintf("Agent failed: %v", runErr)
			} else {
				agentOutputs[ag.ID] = output
			}
			mu.Unlock()
		}(agent)
	}
	wg.Wait()

	// ---- Coordinator synthesis ----
	summary, gaps, reqs, err := runGapCoordinator(ctx, agentOutputs, provider, model, maxTokens, bridge, tools, coordTpl, logger)
	if err != nil {
		return nil, fmt.Errorf("coordinator synthesis: %w", err)
	}

	return &GapSwarmResult{
		Gaps:         gaps,
		Requirements: reqs,
		Summary:      summary,
		AgentOutputs: agentOutputs,
	}, nil
}

// runGapAgent runs one specialist agent's investigation loop.
func runGapAgent(
	ctx context.Context,
	ag gapAgentDef,
	bridge *GapBridge,
	provider llm.ChatProvider,
	model string,
	maxTokens int,
	tools []llm.ToolDefinition,
	toolCache *sync.Map,
	priorFindings string,
	followUp string,
	tpl *template.Template,
	logger *zap.Logger,
) (string, error) {
	var promptBuf bytes.Buffer
	if err := tpl.Execute(&promptBuf, map[string]string{
		"AgentName":     ag.Name,
		"AgentMission":  ag.Mission,
		"PriorFindings": priorFindings,
		"FollowUpQuery": followUp,
	}); err != nil {
		return "", fmt.Errorf("rendering agent template: %w", err)
	}

	messages := []llm.ChatMessage{
		{
			Role:    llm.RoleUser,
			Content: []llm.ContentBlock{llm.NewTextContent("Begin your gap analysis investigation.")},
		},
	}

	for i := range maxGapAgentIter {
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

		// Process tool calls
		messages = append(messages, llm.ChatMessage{
			Role:    llm.RoleAssistant,
			Content: resp.Content,
		})

		var toolResults []llm.ContentBlock
		for _, block := range resp.ToolUseBlocks() {
			var args map[string]any
			_ = json.Unmarshal(block.Input, &args)

			// Check tool cache
			cacheKey := block.Name + "|" + string(block.Input)
			if cached, ok := toolCache.Load(cacheKey); ok {
				toolResults = append(toolResults, llm.NewToolResultContent(block.ID, cached.(string), false))
				continue
			}

			result, callErr := bridge.CallTool(ctx, block.Name, args)
			if callErr != nil {
				logger.Debug("tool call error", zap.String("tool", block.Name), zap.Error(callErr))
				toolResults = append(toolResults, llm.NewToolResultContent(block.ID, fmt.Sprintf("Error: %v", callErr), true))
			} else {
				toolCache.Store(cacheKey, result)
				toolResults = append(toolResults, llm.NewToolResultContent(block.ID, result, false))
			}
		}

		messages = append(messages, llm.ChatMessage{
			Role:    llm.RoleUser,
			Content: toolResults,
		})
	}

	return "", fmt.Errorf("max iterations reached for agent %s", ag.ID)
}

// runGapCoordinator synthesizes all agent findings into gaps and requirements.
func runGapCoordinator(
	ctx context.Context,
	agentOutputs map[string]string,
	provider llm.ChatProvider,
	model string,
	maxTokens int,
	bridge *GapBridge,
	tools []llm.ToolDefinition,
	tpl *template.Template,
	logger *zap.Logger,
) (string, []graph.BusinessGap, []graph.BusinessRequirement, error) {
	// Build findings summary
	var findingsParts []string
	for _, ag := range gapAgents {
		output := agentOutputs[ag.ID]
		if output == "" {
			output = "(no findings)"
		}
		if len(output) > 3000 {
			output = output[:3000] + "...(truncated)"
		}
		findingsParts = append(findingsParts, fmt.Sprintf("### %s\n\n%s", ag.Name, output))
	}

	var coordBuf bytes.Buffer
	if err := tpl.Execute(&coordBuf, map[string]any{
		"AgentCount":   len(gapAgents),
		"AgentFindings": strings.Join(findingsParts, "\n\n---\n\n"),
	}); err != nil {
		return "", nil, nil, fmt.Errorf("rendering coordinator template: %w", err)
	}

	messages := []llm.ChatMessage{
		{
			Role:    llm.RoleUser,
			Content: []llm.ContentBlock{llm.NewTextContent("Synthesize all agent findings and generate business requirements.")},
		},
	}

	for i := range maxGapAgentIter {
		if ctx.Err() != nil {
			return "", nil, nil, ctx.Err()
		}

		resp, err := provider.CompleteChat(ctx, llm.ChatRequest{
			Model:     model,
			System:    coordBuf.String(),
			MaxTokens: maxTokens,
			Tools:     tools,
			Messages:  messages,
		})
		if err != nil {
			return "", nil, nil, fmt.Errorf("coordinator iteration %d: %w", i+1, err)
		}

		if resp.StopReason != "tool_use" {
			text := resp.TextContent()
			gaps := extractGapsFromAgents(agentOutputs)
			reqs := extractRequirementsFromCoordinator(text)
			logger.Info("coordinator synthesis complete",
				zap.Int("gaps", len(gaps)),
				zap.Int("requirements", len(reqs)),
			)
			return text, gaps, reqs, nil
		}

		// Tool calls — coordinator may validate agent claims
		messages = append(messages, llm.ChatMessage{
			Role:    llm.RoleAssistant,
			Content: resp.Content,
		})
		var toolResults []llm.ContentBlock
		for _, block := range resp.ToolUseBlocks() {
			var args map[string]any
			_ = json.Unmarshal(block.Input, &args)
			result, callErr := bridge.CallTool(ctx, block.Name, args)
			if callErr != nil {
				toolResults = append(toolResults, llm.NewToolResultContent(block.ID, fmt.Sprintf("Error: %v", callErr), true))
			} else {
				toolResults = append(toolResults, llm.NewToolResultContent(block.ID, result, false))
			}
		}
		messages = append(messages, llm.ChatMessage{Role: llm.RoleUser, Content: toolResults})
	}

	return "", nil, nil, fmt.Errorf("coordinator max iterations reached")
}

// extractGapsFromAgents parses <gaps> blocks from all agent outputs.
func extractGapsFromAgents(agentOutputs map[string]string) []graph.BusinessGap {
	gapsBlockRe := regexp.MustCompile(`(?s)<gaps>(.*?)</gaps>`)
	var allGaps []graph.BusinessGap
	seen := make(map[string]bool)

	for _, output := range agentOutputs {
		match := gapsBlockRe.FindStringSubmatch(output)
		if len(match) < 2 {
			continue
		}
		jsonStr := strings.TrimSpace(match[1])

		var rawGaps []struct {
			ID           string  `json:"id"`
			GapType      string  `json:"gapType"`
			Category     string  `json:"category"`
			Description  string  `json:"description"`
			Severity     string  `json:"severity"`
			CobolSource  string  `json:"cobolSource"`
			TargetSource string  `json:"targetSource"`
			Confidence   float64 `json:"confidence"`
		}
		if err := json.Unmarshal([]byte(jsonStr), &rawGaps); err != nil {
			continue
		}

		for _, g := range rawGaps {
			if g.ID == "" || seen[g.ID] {
				continue
			}
			seen[g.ID] = true
			allGaps = append(allGaps, graph.BusinessGap{
				ID:           g.ID,
				GapType:      g.GapType,
				Category:     g.Category,
				Description:  g.Description,
				Severity:     g.Severity,
				CobolSource:  g.CobolSource,
				TargetSource: g.TargetSource,
				Confidence:   g.Confidence,
			})
		}
	}
	return allGaps
}

// extractRequirementsFromCoordinator parses <requirements> blocks from coordinator output.
func extractRequirementsFromCoordinator(text string) []graph.BusinessRequirement {
	reqBlockRe := regexp.MustCompile(`(?s)<requirements>(.*?)</requirements>`)
	match := reqBlockRe.FindStringSubmatch(text)
	if len(match) < 2 {
		return nil
	}
	jsonStr := strings.TrimSpace(match[1])

	var rawReqs []struct {
		ID                 string `json:"id"`
		Title              string `json:"title"`
		Description        string `json:"description"`
		Priority           string `json:"priority"`
		Category           string `json:"category"`
		AcceptanceCriteria string `json:"acceptanceCriteria"`
		EstimatedEffort    string `json:"estimatedEffort"`
		GapID              string `json:"gapId"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &rawReqs); err != nil {
		return nil
	}

	var reqs []graph.BusinessRequirement
	for _, r := range rawReqs {
		if r.ID == "" || r.Title == "" {
			continue
		}
		reqs = append(reqs, graph.BusinessRequirement{
			ID:                 r.ID,
			Title:              r.Title,
			Description:        r.Description,
			Priority:           r.Priority,
			Category:           r.Category,
			AcceptanceCriteria: r.AcceptanceCriteria,
			EstimatedEffort:    r.EstimatedEffort,
			GapID:              r.GapID,
		})
	}
	return reqs
}
