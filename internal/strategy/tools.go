package strategy

import (
	"context"
	"fmt"

	"cobol-ingestor/internal/llm"
	"cobol-ingestor/internal/modernize"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// DiscoverTools dynamically discovers all available tools from the MCP server
// at runtime, replacing the previous hardcoded tool definitions.
func DiscoverTools(ctx context.Context, mcpClient *modernize.MCPClient) ([]llm.ToolDefinition, error) {
	if mcpClient == nil {
		return nil, fmt.Errorf("MCP client is nil")
	}

	mcpTools, err := mcpClient.ListTools(ctx)
	if err != nil {
		return nil, fmt.Errorf("discovering MCP tools: %w", err)
	}

	tools := make([]llm.ToolDefinition, 0, len(mcpTools))
	for _, t := range mcpTools {
		tools = append(tools, llm.ToolDefinition{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: schemaToMap(t.InputSchema),
		})
	}

	return tools, nil
}

// schemaToMap converts the MCP tool's InputSchema (any) to a map[string]any.
func schemaToMap(v any) map[string]any {
	if v == nil {
		return map[string]any{"type": "object", "properties": map[string]any{}}
	}
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return map[string]any{"type": "object", "properties": map[string]any{}}
}

// Ensure mcp package is used (the Tool type comes from it via MCPClient.ListTools).
var _ *mcp.Tool
