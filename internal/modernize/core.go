package modernize

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"cobol-ingestor/internal/llm"

	"go.uber.org/zap"
)

// InputMessage is the exported form of chat input messages.
type InputMessage = chatInputMessage

// ChatParams holds all parameters for a headless chat invocation.
type ChatParams struct {
	Provider             *ProviderState
	MCPClient            *MCPClient
	SessionStore         SessionStore
	SessionID            string
	Messages             []InputMessage
	DiscoveryMode        bool
	GenerateDiagram      bool
	UnlimitedIterations  bool
	TargetLang           string
	Framework            string
	Integrations         string
	Codebase             string // used to scope glossary lookups (default "default")
	Emitter              EventEmitter
	Logger               *zap.Logger
	DiagramOutputDir     string
}

// RunChat executes the chat tool-use loop, emitting events via the Emitter.
// This is the transport-agnostic core extracted from ChatHandler.
func RunChat(ctx context.Context, p ChatParams) error {
	provider := p.Provider.Get()
	if provider == nil {
		return fmt.Errorf("not authenticated")
	}

	model := p.Provider.GetModel()
	maxTokens := p.Provider.GetMaxTokens()
	if maxTokens == 0 {
		maxTokens = 16384
	}

	var systemPrompt string
	var err error
	if p.DiscoveryMode {
		systemPrompt, err = BuildDiscoveryPrompt(p.GenerateDiagram)
	} else {
		systemPrompt, err = BuildSystemPrompt(p.TargetLang, p.Framework, p.Integrations, p.GenerateDiagram)
	}
	if err != nil {
		return fmt.Errorf("building system prompt: %w", err)
	}

	glossaryPreamble := fetchGlossaryPreamble(ctx, p.MCPClient, p.Codebase)
	var preamble string
	if p.DiscoveryMode {
		// In discovery mode there's no migration target, but glossary context is still useful.
		preamble = BuildContextPreamble("", "", "", glossaryPreamble)
	} else {
		preamble = BuildContextPreamble(p.TargetLang, p.Framework, p.Integrations, glossaryPreamble)
	}

	messages := make([]llm.ChatMessage, 0, len(p.Messages))
	for i, m := range p.Messages {
		text := m.Content
		if i == 0 && m.Role == "user" && preamble != "" {
			text = preamble + text
		}
		messages = append(messages, llm.ChatMessage{
			Role:    m.Role,
			Content: []llm.ContentBlock{llm.NewTextContent(text)},
		})
	}

	tools := p.MCPClient.BuildLLMToolDefinitions(ctx)

	for i := 0; p.UnlimitedIterations || i < maxToolIterations; i++ {
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
			return fmt.Errorf("LLM error: %w", err)
		}

		if text := resp.TextContent(); text != "" {
			p.Emitter.Emit("text", map[string]string{"content": text})
		}

		if resp.StopReason != "tool_use" {
			// Post-response diagram extraction
			if p.GenerateDiagram {
				results := ExtractAndRenderMermaidBlocks(resp.TextContent(), p.DiagramOutputDir)
				for _, dr := range results {
					if dr.Error == "" {
						p.Emitter.Emit("diagram_generated", map[string]string{
							"filePath": dr.FilePath,
							"svgData":  dr.SvgData,
						})
					}
				}
			}

			// Persist session
			if p.SessionStore != nil {
				assistantText := resp.TextContent()
				allMessages := make([]chatInputMessage, len(p.Messages))
				copy(allMessages, p.Messages)
				if assistantText != "" {
					allMessages = append(allMessages, chatInputMessage{Role: "assistant", Content: assistantText})
				}

				if p.SessionID == "" {
					title := p.Messages[0].Content
					if len(title) > 50 {
						title = title[:50]
					}
					session, err := p.SessionStore.Create(title)
					if err == nil {
						_ = p.SessionStore.Update(session.ID, allMessages, "")
						p.Emitter.Emit("session_created", map[string]string{"id": session.ID, "title": session.Title})
					}
				} else {
					_ = p.SessionStore.Update(p.SessionID, allMessages, "")
				}
			}
			p.Emitter.Emit("done", map[string]string{})
			return nil
		}

		// Process tool calls
		toolUseBlocks := resp.ToolUseBlocks()
		messages = append(messages, llm.ChatMessage{
			Role:    llm.RoleAssistant,
			Content: resp.Content,
		})

		var toolResults []llm.ContentBlock
		for _, block := range toolUseBlocks {
			p.Emitter.Emit("tool_start", map[string]string{
				"name":  block.Name,
				"id":    block.ID,
				"input": truncate(string(block.Input), 200),
			})

			var args map[string]any
			if err := json.Unmarshal(block.Input, &args); err != nil {
				args = map[string]any{}
			}

			result, err := p.MCPClient.CallTool(ctx, block.Name, args)
			if err != nil {
				toolResults = append(toolResults, llm.NewToolResultContent(block.ID, fmt.Sprintf("Error: %v", err), true))
				p.Emitter.Emit("tool_result", map[string]string{
					"name":  block.Name,
					"id":    block.ID,
					"error": err.Error(),
				})
			} else {
				toolResults = append(toolResults, llm.NewToolResultContent(block.ID, result, false))
				p.Emitter.Emit("tool_result", map[string]string{
					"name":   block.Name,
					"id":     block.ID,
					"result": truncate(result, 500),
				})
			}
		}

		messages = append(messages, llm.ChatMessage{
			Role:    llm.RoleUser,
			Content: toolResults,
		})
	}

	p.Emitter.Emit("error", map[string]string{"error": "max tool iterations reached"})
	p.Emitter.Emit("done", map[string]string{})
	return nil
}

// SwarmParams holds all parameters for a headless swarm invocation.
type SwarmParams struct {
	Provider             *ProviderState
	MCPClient            *MCPClient
	SessionStore         SessionStore
	SessionID            string
	Messages             []InputMessage
	DiscoveryMode        bool
	MultiRound           bool
	GenerateDiagram      bool
	UnlimitedIterations  bool
	TargetLang           string
	Framework            string
	Integrations         string
	Codebase             string // used to scope glossary lookups (default "default")
	Emitter              EventEmitter
	Logger               *zap.Logger
	DiagramOutputDir     string
}

// RunSwarm executes the full swarm agent orchestration, emitting events via the Emitter.
// It runs 4 specialist agents in parallel, coordinates multi-round follow-ups,
// and produces a final synthesis — all transport-agnostic via EventEmitter.
func RunSwarm(ctx context.Context, p SwarmParams) error {
	provider := p.Provider.Get()
	if provider == nil {
		return fmt.Errorf("not authenticated")
	}

	model := p.Provider.GetModel()
	maxTokens := p.Provider.GetMaxTokens()
	if maxTokens == 0 {
		maxTokens = 16384
	}

	// Extract the user's latest question
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

	targetLanguage := p.TargetLang
	framework := p.Framework
	integrations := p.Integrations
	if p.DiscoveryMode {
		targetLanguage = ""
		framework = ""
		integrations = ""
	}

	glossaryPreamble := fetchGlossaryPreamble(ctx, p.MCPClient, p.Codebase)

	promptData := swarmPromptData{
		TargetLanguage:  targetLanguage,
		Framework:       framework,
		Integrations:    integrations,
		Glossary:        glossaryPreamble,
		GenerateDiagram: p.GenerateDiagram,
	}

	maxRounds := 1
	if p.MultiRound {
		maxRounds = 3
	}

	tools := p.MCPClient.BuildLLMToolDefinitions(ctx)
	cache := newToolCache()
	var allRounds []roundSummary
	var followUpQueries map[string]string

	for round := 1; round <= maxRounds; round++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		// Emit round_start (multi-round only)
		if maxRounds > 1 {
			p.Emitter.Emit("round_start", map[string]any{
				"round":     round,
				"maxRounds": maxRounds,
			})
		}

		// Determine which agents to run
		agentsToRun, agentFollowUps := selectAgents(agentRoles, followUpQueries, round)

		// Run agents in parallel
		results := make([]agentResult, len(agentsToRun))
		var wg sync.WaitGroup

		for i, role := range agentsToRun {
			wg.Add(1)
			go func(idx int, role agentRole) {
				defer wg.Done()

				// Build per-agent prompt data with round context
				agentPromptData := promptData
				agentPromptData.Round = round
				if round > 1 {
					agentPromptData.PriorRounds = truncateRoundSummaries(allRounds, maxContextCharsPrior)
				}
				if q, ok := agentFollowUps[role.ID]; ok {
					agentPromptData.FollowUpQuery = q
				}

				// Use round-aware agent ID for events
				sseID := role.ID
				if maxRounds > 1 {
					sseID = fmt.Sprintf("%s-round-%d", role.ID, round)
				}
				sseRole := agentRole{ID: sseID, Name: role.Name, Prompt: role.Prompt}

				summary, err := runAgent(ctx, sseRole, role.Prompt, userQuery, agentPromptData, provider, p.MCPClient, cache, tools, model, maxTokens, p.Emitter, round, maxRounds > 1, p.UnlimitedIterations)
				if err != nil {
					p.Emitter.Emit("agent_complete", map[string]string{
						"id":      sseID,
						"summary": fmt.Sprintf("Error: %v", err),
					})
					results[idx] = agentResult{Name: role.Name, Summary: fmt.Sprintf("Error: %v", err)}
					return
				}
				results[idx] = agentResult{Name: role.Name, Summary: summary}
			}(i, role)
		}

		wg.Wait()

		if ctx.Err() != nil {
			return ctx.Err()
		}

		allRounds = append(allRounds, roundSummary{Round: round, Results: results})

		// Emit round_complete (multi-round only)
		if maxRounds > 1 {
			p.Emitter.Emit("round_complete", map[string]any{"round": round})
		}

		// If this is the last allowed round, skip decision
		if round == maxRounds {
			break
		}

		// Coordinator decision: do we need more info?
		latestResults := allRounds[len(allRounds)-1].Results
		decision, err := runCoordinatorDecision(ctx, latestResults, userQuery, promptData, provider, model, p.Emitter, round, p.Logger)
		if err != nil || decision.Satisfied {
			break
		}
		followUpQueries = decision.FollowUps
	}

	if ctx.Err() != nil {
		return ctx.Err()
	}

	// Flatten all round results for final synthesis
	finalResults := flattenRounds(allRounds, maxContextCharsLatest, maxContextCharsPrior)

	// Final synthesis with tool access
	p.Emitter.Emit("synthesis_start", map[string]string{})

	promptData.UserQuery = userQuery
	promptData.AgentResults = finalResults

	coordText, err := runCoordinatorSynthesis(ctx, promptData, provider, p.MCPClient, cache, tools, model, maxTokens, p.Emitter, p.UnlimitedIterations)
	if err != nil {
		p.Emitter.Emit("error", map[string]string{"error": fmt.Sprintf("Coordinator error: %v", err)})
		p.Emitter.Emit("done", map[string]string{})
		return nil
	}

	// Post-response diagram extraction
	if p.GenerateDiagram {
		results := ExtractAndRenderMermaidBlocks(coordText, p.DiagramOutputDir)
		for _, dr := range results {
			if dr.Error == "" {
				p.Emitter.Emit("diagram_generated", map[string]string{
					"filePath": dr.FilePath,
					"svgData":  dr.SvgData,
				})
			}
		}
	}

	// Persist session
	if p.SessionStore != nil {
		allMessages := make([]chatInputMessage, len(p.Messages))
		copy(allMessages, p.Messages)
		if coordText != "" {
			allMessages = append(allMessages, chatInputMessage{Role: "assistant", Content: coordText})
		}

		if p.SessionID == "" {
			title := userQuery
			if len(title) > 50 {
				title = title[:50]
			}
			session, err := p.SessionStore.Create(title)
			if err == nil {
				_ = p.SessionStore.Update(session.ID, allMessages, "")
				p.Emitter.Emit("session_created", map[string]string{"id": session.ID, "title": session.Title})
			}
		} else {
			_ = p.SessionStore.Update(p.SessionID, allMessages, "")
		}
	}

	p.Emitter.Emit("done", map[string]string{})
	return nil
}

// fetchGlossaryPreamble calls the list_glossary_terms MCP tool and returns a formatted
// [Company Glossary] preamble string. Returns empty string on any error or empty glossary.
func fetchGlossaryPreamble(ctx context.Context, mcpClient *MCPClient, codebase string) string {
	if mcpClient == nil {
		return ""
	}
	cb := codebase
	if cb == "" {
		cb = "default"
	}
	result, err := mcpClient.CallTool(ctx, "list_glossary_terms", map[string]any{
		"codebase": cb,
		"pageSize": 200,
	})
	if err != nil {
		return ""
	}
	return BuildGlossaryPreamble(result)
}

// ListModels returns available models from the provider.
func (ps *ProviderState) ListModels() ([]ModelInfo, error) {
	provider := ps.Get()
	if provider == nil {
		return nil, fmt.Errorf("not authenticated")
	}

	if lister, ok := provider.(ModelLister); ok {
		return lister.ListModels(context.Background())
	}

	if cp, ok := provider.(*llm.CopilotProvider); ok {
		models, err := cp.GetCopilotProvider().GetModels(context.Background())
		if err != nil {
			return nil, err
		}
		result := make([]ModelInfo, 0, len(models))
		for _, m := range models {
			result = append(result, ModelInfo{
				ID:                  m.ID,
				Name:                m.Name,
				Description:         m.Description,
				MaxTokens:           m.MaxTokens,
				SupportsToolCalling: m.SupportsToolCalling,
			})
		}
		return result, nil
	}

	// Generic fallback
	model := ps.GetModel()
	if model == "" {
		return nil, nil
	}
	return []ModelInfo{{ID: model, Name: model}}, nil
}

