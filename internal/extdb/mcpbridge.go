package extdb

import (
	"context"
	"fmt"
	"strings"

	"cobol-ingestor/internal/llm"
	"cobol-ingestor/internal/modernize"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	prefixCobol = "cobol_"
	prefixExtDB = "extdb_"
)

// MCPBridge wraps two MCP clients (COBOL graph + external DB) and merges their
// tool namespaces so a single LLM can explore both systems.
type MCPBridge struct {
	graphClient *modernize.MCPClient
	dbClient    *modernize.MCPClient
}

// NewMCPBridge creates a bridge between COBOL graph and external DB MCP clients.
func NewMCPBridge(graphClient, dbClient *modernize.MCPClient) *MCPBridge {
	return &MCPBridge{
		graphClient: graphClient,
		dbClient:    dbClient,
	}
}

// ListAllTools discovers tools from both MCP servers and returns them with
// namespaced prefixes (cobol_ / extdb_).
func (b *MCPBridge) ListAllTools(ctx context.Context) ([]llm.ToolDefinition, error) {
	var graphTools, dbTools []*mcp.Tool

	if b.graphClient != nil {
		tools, err := b.graphClient.ListTools(ctx)
		if err != nil {
			return nil, fmt.Errorf("listing graph tools: %w", err)
		}
		graphTools = tools
	}

	if b.dbClient != nil {
		tools, err := b.dbClient.ListTools(ctx)
		if err != nil {
			return nil, fmt.Errorf("listing extdb tools: %w", err)
		}
		dbTools = tools
	}

	var tools []llm.ToolDefinition

	for _, t := range graphTools {
		schema := schemaToMap(t.InputSchema)
		tools = append(tools, llm.ToolDefinition{
			Name:        prefixCobol + t.Name,
			Description: "[COBOL Graph] " + t.Description,
			InputSchema: schema,
		})
	}

	for _, t := range dbTools {
		schema := schemaToMap(t.InputSchema)
		tools = append(tools, llm.ToolDefinition{
			Name:        prefixExtDB + t.Name,
			Description: "[External DB] " + t.Description,
			InputSchema: schema,
		})
	}

	return tools, nil
}

// CallTool strips the namespace prefix and routes the call to the correct MCP client.
func (b *MCPBridge) CallTool(ctx context.Context, name string, args map[string]any) (string, error) {
	switch {
	case strings.HasPrefix(name, prefixCobol):
		if b.graphClient == nil {
			return "", fmt.Errorf("graph MCP client not configured")
		}
		return b.graphClient.CallTool(ctx, strings.TrimPrefix(name, prefixCobol), args)
	case strings.HasPrefix(name, prefixExtDB):
		if b.dbClient == nil {
			return "", fmt.Errorf("external DB MCP client not configured")
		}
		return b.dbClient.CallTool(ctx, strings.TrimPrefix(name, prefixExtDB), args)
	default:
		return "", fmt.Errorf("unknown tool prefix for %q: expected %q or %q prefix", name, prefixCobol, prefixExtDB)
	}
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
