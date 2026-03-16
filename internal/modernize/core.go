package modernize

import (
	"context"
	"encoding/json"
	"fmt"

	"cobol-ingestor/internal/llm"

	"go.uber.org/zap"
)

// InputMessage is the exported form of chat input messages.
type InputMessage = chatInputMessage

// ChatParams holds all parameters for a headless chat invocation.
type ChatParams struct {
	Provider      *ProviderState
	MCPClient     *MCPClient
	SessionStore  SessionStore
	SessionID     string
	Messages      []InputMessage
	DiscoveryMode bool
	TargetLang    string
	Framework     string
	Integrations  string
	Emitter       EventEmitter
	Logger        *zap.Logger
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
		systemPrompt, err = BuildDiscoveryPrompt()
	} else {
		systemPrompt, err = BuildSystemPrompt(p.TargetLang, p.Framework, p.Integrations)
	}
	if err != nil {
		return fmt.Errorf("building system prompt: %w", err)
	}

	var preamble string
	if !p.DiscoveryMode {
		preamble = BuildContextPreamble(p.TargetLang, p.Framework, p.Integrations)
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

	tools := GetToolDefinitions()

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
			return fmt.Errorf("LLM error: %w", err)
		}

		if text := resp.TextContent(); text != "" {
			p.Emitter.Emit("text", map[string]string{"content": text})
		}

		if resp.StopReason != "tool_use" {
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
	return nil
}

// SwarmParams holds all parameters for a headless swarm invocation.
type SwarmParams struct {
	Provider      *ProviderState
	MCPClient     *MCPClient
	SessionStore  SessionStore
	SessionID     string
	Messages      []InputMessage
	DiscoveryMode bool
	MultiRound    bool
	TargetLang    string
	Framework     string
	Integrations  string
	Emitter       EventEmitter
	Logger        *zap.Logger
}

// RunSwarm executes the swarm agent orchestration, emitting events via the Emitter.
// For now this delegates to the chat path; the full swarm extraction is a follow-up.
// TODO: Extract full swarm logic (parallel agents, coordinator, multi-round) from SwarmHandler.
func RunSwarm(ctx context.Context, p SwarmParams) error {
	// Reuse the chat path as a simplified swarm — single-agent mode.
	// The full swarm refactoring with parallel agents will follow.
	return RunChat(ctx, ChatParams{
		Provider:      p.Provider,
		MCPClient:     p.MCPClient,
		SessionStore:  p.SessionStore,
		SessionID:     p.SessionID,
		Messages:      p.Messages,
		DiscoveryMode: p.DiscoveryMode,
		TargetLang:    p.TargetLang,
		Framework:     p.Framework,
		Integrations:  p.Integrations,
		Emitter:       p.Emitter,
		Logger:        p.Logger,
	})
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
