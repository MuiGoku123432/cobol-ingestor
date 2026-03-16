package extdb

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"text/template"

	"cobol-ingestor/internal/llm"
	"cobol-ingestor/internal/parser"
	"cobol-ingestor/prompts"

	"go.uber.org/zap"
)

// Analyzer orchestrates LLM-driven external DB gap analysis using two MCP servers.
type Analyzer struct {
	bridge   *MCPBridge
	provider llm.ChatProvider
	model    string
	maxIter  int
	maxToks  int
	dbName   string
	dbType   string
	logger   *zap.Logger
}

// NewAnalyzer creates an Analyzer.
func NewAnalyzer(bridge *MCPBridge, provider llm.ChatProvider, model string, maxIter, maxToks int, dbName, dbType string, logger *zap.Logger) *Analyzer {
	return &Analyzer{
		bridge:   bridge,
		provider: provider,
		model:    model,
		maxIter:  maxIter,
		maxToks:  maxToks,
		dbName:   dbName,
		dbType:   dbType,
		logger:   logger,
	}
}

// Run executes the agentic analysis loop and returns structured results.
func (a *Analyzer) Run(ctx context.Context) (*parser.ExternalDBParsedResult, error) {
	tools, err := a.bridge.ListAllTools(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing tools: %w", err)
	}
	a.logger.Info("discovered tools", zap.Int("count", len(tools)))

	systemPrompt, err := a.buildSystemPrompt()
	if err != nil {
		return nil, fmt.Errorf("building system prompt: %w", err)
	}

	messages := []llm.ChatMessage{
		{
			Role:    llm.RoleUser,
			Content: []llm.ContentBlock{llm.NewTextContent("Analyze the external database and map to the COBOL DB2 tables. Return your findings as structured JSON.")},
		},
	}

	for i := 0; i < a.maxIter; i++ {
		a.logger.Debug("analysis iteration", zap.Int("iteration", i+1))

		chatReq := llm.ChatRequest{
			Model:       a.model,
			System:      systemPrompt,
			Messages:    messages,
			Tools:       tools,
			MaxTokens:   a.maxToks,
			Temperature: 0.2,
		}

		resp, err := a.provider.CompleteChat(ctx, chatReq)
		if err != nil {
			return nil, fmt.Errorf("LLM completion (iteration %d): %w", i+1, err)
		}

		if resp.StopReason != "tool_use" {
			// Final response — extract and parse JSON
			text := resp.TextContent()
			a.logger.Info("analysis complete",
				zap.Int("iterations", i+1),
				zap.Int("prompt_tokens", resp.PromptTokens),
				zap.Int("output_tokens", resp.OutputTokens),
			)
			return parser.ParseExternalDBResponse(text, a.dbName, a.dbType)
		}

		// Process tool calls
		toolUseBlocks := resp.ToolUseBlocks()
		messages = append(messages, llm.ChatMessage{
			Role:    llm.RoleAssistant,
			Content: resp.Content,
		})

		var toolResults []llm.ContentBlock
		for _, block := range toolUseBlocks {
			a.logger.Debug("tool call", zap.String("name", block.Name))

			var args map[string]any
			if err := json.Unmarshal(block.Input, &args); err != nil {
				args = map[string]any{}
			}

			result, err := a.bridge.CallTool(ctx, block.Name, args)
			if err != nil {
				toolResults = append(toolResults, llm.NewToolResultContent(block.ID, fmt.Sprintf("Error: %v", err), true))
			} else {
				toolResults = append(toolResults, llm.NewToolResultContent(block.ID, result, false))
			}
		}

		messages = append(messages, llm.ChatMessage{
			Role:    llm.RoleUser,
			Content: toolResults,
		})
	}

	return nil, fmt.Errorf("max iterations (%d) reached without final response", a.maxIter)
}

func (a *Analyzer) buildSystemPrompt() (string, error) {
	tmpl, err := template.New("extdb").Parse(prompts.ExtDBAnalysis)
	if err != nil {
		return "", fmt.Errorf("parsing template: %w", err)
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, map[string]string{
		"DatabaseName": a.dbName,
		"DatabaseType": a.dbType,
	})
	if err != nil {
		return "", fmt.Errorf("executing template: %w", err)
	}
	return buf.String(), nil
}
