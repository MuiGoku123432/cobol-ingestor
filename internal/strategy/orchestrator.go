package strategy

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"

	"cobol-ingestor/internal/llm"
)

const (
	maxAgentToolIterations       = 10
	maxCoordinatorToolIterations = 5
)

// toolCache provides a thread-safe cache for MCP tool call results.
type toolCache struct {
	mu    sync.RWMutex
	store map[string]string
}

func newToolCache() *toolCache {
	return &toolCache{store: make(map[string]string)}
}

func (tc *toolCache) Key(name string, args map[string]any) string {
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

// RunStrategy executes the full strategy analysis pipeline.
func RunStrategy(ctx context.Context, p StrategyParams) (*StrategyPlan, error) {
	provider := p.Provider.Get()
	if provider == nil {
		return nil, fmt.Errorf("not authenticated")
	}

	model := p.Provider.GetModel()
	maxTokens := p.Provider.GetMaxTokens()
	if maxTokens == 0 {
		maxTokens = 16384
	}

	// Build prompt data from strategy context
	promptData := strategyPromptData{
		Strategy:          p.Context.Strategy,
		HybridStrategies:  p.Context.HybridStrategies,
		TargetLang:        p.Context.TargetLang,
		TargetFramework:   p.Context.TargetFramework,
		TargetPlatform:    p.Context.TargetPlatform,
		Timeline:          p.Context.Timeline,
		TeamSize:          p.Context.TeamSize,
		TeamSkills:        p.Context.TeamSkills,
		Compliance:        p.Context.Compliance,
		Integrations:      p.Context.Integrations,
		PriorityCriteria:  p.Context.PriorityCriteria,
		PhasingApproach:   p.Context.PhasingApproach,
		Criticality:       p.Context.Criticality,
		DowntimeTolerance: p.Context.DowntimeTolerance,
		AdditionalNotes:   p.Context.AdditionalNotes,
	}

	tools := GetToolDefinitions()
	cache := newToolCache()

	// Run all 5 agents in parallel
	results := make([]agentResult, len(strategyAgents))
	var wg sync.WaitGroup

	for i, agent := range strategyAgents {
		wg.Add(1)
		go func(idx int, agent strategyAgent) {
			defer wg.Done()

			p.Emitter.Emit("agent_start", map[string]string{
				"agentId":   agent.ID,
				"agentName": agent.Name,
			})

			summary, err := runStrategyAgent(ctx, agent, promptData, provider, p.MCPClient, cache, tools, model, maxTokens, p.Emitter)
			if err != nil {
				p.Emitter.Emit("agent_complete", map[string]string{
					"agentId": agent.ID,
				})
				results[idx] = agentResult{Name: agent.Name, Summary: fmt.Sprintf("Error: %v", err)}
				return
			}
			results[idx] = agentResult{Name: agent.Name, Summary: summary}
			p.Emitter.Emit("agent_complete", map[string]string{
				"agentId": agent.ID,
			})
		}(i, agent)
	}

	wg.Wait()

	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	// Collect agent results into map
	agentResultMap := make(map[string]string, len(results))
	for i, r := range results {
		agentResultMap[strategyAgents[i].ID] = r.Summary
	}

	// Run coordinator synthesis
	p.Emitter.Emit("synthesis_start", map[string]string{})

	coordPromptData := promptData
	coordPromptData.AgentResults = results

	synthesis, err := runCoordinatorSynthesis(ctx, coordPromptData, provider, p.MCPClient, cache, tools, model, maxTokens, p.Emitter)
	if err != nil {
		return nil, fmt.Errorf("coordinator synthesis: %w", err)
	}

	return &StrategyPlan{
		Context:      p.Context,
		AgentResults: agentResultMap,
		Synthesis:    synthesis,
	}, nil
}

func runStrategyAgent(
	ctx context.Context,
	agent strategyAgent,
	promptData strategyPromptData,
	provider llm.ChatProvider,
	mcpClient interface{ CallTool(context.Context, string, map[string]any) (string, error) },
	cache *toolCache,
	tools []llm.ToolDefinition,
	model string,
	maxTokens int,
	emitter interface{ Emit(string, any) },
) (string, error) {
	systemPrompt, err := buildStrategyPrompt(agent.Prompt, promptData)
	if err != nil {
		return "", fmt.Errorf("build prompt: %w", err)
	}

	messages := []llm.ChatMessage{
		{
			Role:    llm.RoleUser,
			Content: []llm.ContentBlock{llm.NewTextContent("Analyze the COBOL codebase and produce your assessment for the migration strategy.")},
		},
	}

	var fullText string

	for i := 0; i < maxAgentToolIterations; i++ {
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
				"agentId": agent.ID,
				"content": text,
			})
		}

		if resp.StopReason != "tool_use" {
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
			var args map[string]any
			if err := json.Unmarshal(block.Input, &args); err != nil {
				args = map[string]any{}
			}

			cacheKey := cache.Key(block.Name, args)
			if cached, ok := cache.Get(cacheKey); ok {
				toolResults = append(toolResults, llm.NewToolResultContent(block.ID, cached, false))
			} else {
				result, callErr := mcpClient.CallTool(ctx, block.Name, args)
				if callErr != nil {
					toolResults = append(toolResults, llm.NewToolResultContent(block.ID, fmt.Sprintf("Error: %v", callErr), true))
				} else {
					cache.Set(cacheKey, result)
					toolResults = append(toolResults, llm.NewToolResultContent(block.ID, result, false))
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

func runCoordinatorSynthesis(
	ctx context.Context,
	promptData strategyPromptData,
	provider llm.ChatProvider,
	mcpClient interface{ CallTool(context.Context, string, map[string]any) (string, error) },
	cache *toolCache,
	tools []llm.ToolDefinition,
	model string,
	maxTokens int,
	emitter interface{ Emit(string, any) },
) (string, error) {
	systemPrompt, err := buildStrategyPrompt(coordinatorSynthesisPrompt, promptData)
	if err != nil {
		return "", fmt.Errorf("build coordinator prompt: %w", err)
	}

	messages := []llm.ChatMessage{
		{
			Role:    llm.RoleUser,
			Content: []llm.ContentBlock{llm.NewTextContent("Synthesize the agent findings into a comprehensive migration strategy document.")},
		},
	}

	var fullText string

	for i := 0; i < maxCoordinatorToolIterations; i++ {
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
			return "", fmt.Errorf("coordinator LLM error: %w", err)
		}

		if text := resp.TextContent(); text != "" {
			fullText += text
			emitter.Emit("synthesis_progress", map[string]string{"content": text})
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
			var args map[string]any
			if err := json.Unmarshal(block.Input, &args); err != nil {
				args = map[string]any{}
			}

			cacheKey := cache.Key(block.Name, args)
			if cached, ok := cache.Get(cacheKey); ok {
				toolResults = append(toolResults, llm.NewToolResultContent(block.ID, cached, false))
			} else {
				result, callErr := mcpClient.CallTool(ctx, block.Name, args)
				if callErr != nil {
					toolResults = append(toolResults, llm.NewToolResultContent(block.ID, fmt.Sprintf("Error: %v", callErr), true))
				} else {
					cache.Set(cacheKey, result)
					toolResults = append(toolResults, llm.NewToolResultContent(block.ID, result, false))
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
