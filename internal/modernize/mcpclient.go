package modernize

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"cobol-ingestor/internal/llm"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// MCPClient wraps the MCP SDK client for calling graph tools.
type MCPClient struct {
	client  *mcp.Client
	session *mcp.ClientSession
}

// NewMCPClient creates an MCP client using command (subprocess) transport.
func NewMCPClient(ctx context.Context, serverBin string, env []string) (*MCPClient, error) {
	client := mcp.NewClient(&mcp.Implementation{
		Name:    "cobol-modernize",
		Version: "1.0.0",
	}, nil)

	cmd := exec.Command(serverBin)
	if len(env) > 0 {
		cmd.Env = env
	}

	transport := &mcp.CommandTransport{Command: cmd}
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, fmt.Errorf("mcp command connect: %w", err)
	}

	return &MCPClient{client: client, session: session}, nil
}

// NewMCPClientCommand creates an MCP client using a pre-built exec.Cmd.
func NewMCPClientCommand(ctx context.Context, cmd *exec.Cmd, env []string) (*MCPClient, error) {
	client := mcp.NewClient(&mcp.Implementation{
		Name:    "cobol-modernize",
		Version: "1.0.0",
	}, nil)

	if len(env) > 0 {
		cmd.Env = env
	}

	transport := &mcp.CommandTransport{Command: cmd}
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, fmt.Errorf("mcp command connect: %w", err)
	}

	return &MCPClient{client: client, session: session}, nil
}

// NewMCPClientHTTP creates an MCP client using Streamable HTTP transport.
func NewMCPClientHTTP(ctx context.Context, serverURL string) (*MCPClient, error) {
	client := mcp.NewClient(&mcp.Implementation{
		Name:    "cobol-modernize",
		Version: "1.0.0",
	}, nil)

	transport := &mcp.StreamableClientTransport{
		Endpoint: serverURL,
	}
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, fmt.Errorf("mcp http connect: %w", err)
	}

	return &MCPClient{client: client, session: session}, nil
}

// ListTools returns the tools available on this MCP server.
func (c *MCPClient) ListTools(ctx context.Context) ([]*mcp.Tool, error) {
	result, err := c.session.ListTools(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("mcp list tools: %w", err)
	}
	return result.Tools, nil
}

// BuildLLMToolDefinitions queries the MCP server for its full tool catalog and
// converts each entry to an llm.ToolDefinition for use in chat/swarm prompts.
// Falls back to the hardcoded GetToolDefinitions() list if discovery fails.
func (c *MCPClient) BuildLLMToolDefinitions(ctx context.Context) []llm.ToolDefinition {
	tools, err := c.ListTools(ctx)
	if err != nil || len(tools) == 0 {
		return GetToolDefinitions()
	}

	defs := make([]llm.ToolDefinition, 0, len(tools))
	for _, t := range tools {
		schema, _ := t.InputSchema.(map[string]any)
		if schema == nil {
			schema = map[string]any{"type": "object", "properties": map[string]any{}}
		}
		defs = append(defs, llm.ToolDefinition{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: schema,
		})
	}
	return defs
}

// CallTool calls an MCP tool and returns the text content from the result.
func (c *MCPClient) CallTool(ctx context.Context, name string, args map[string]any) (string, error) {
	result, err := c.session.CallTool(ctx, &mcp.CallToolParams{
		Name:      name,
		Arguments: args,
	})
	if err != nil {
		return "", fmt.Errorf("mcp call %s: %w", name, err)
	}

	if result.IsError {
		return "", fmt.Errorf("mcp tool %s error: %s", name, extractText(result))
	}

	return extractText(result), nil
}

// NewMCPClientInProcess creates an MCP client connected to a server in the same
// process via in-memory transport. No subprocess or network needed.
func NewMCPClientInProcess(ctx context.Context, server *mcp.Server) (*MCPClient, error) {
	client := mcp.NewClient(&mcp.Implementation{
		Name:    "cobol-modernize",
		Version: "1.0.0",
	}, nil)

	clientTransport, serverTransport := mcp.NewInMemoryTransports()

	// Start the server in a background goroutine.
	go func() {
		_ = server.Run(ctx, serverTransport)
	}()

	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		return nil, fmt.Errorf("mcp in-process connect: %w", err)
	}

	return &MCPClient{client: client, session: session}, nil
}

// Close shuts down the MCP session.
func (c *MCPClient) Close() error {
	if c.session != nil {
		return c.session.Close()
	}
	return nil
}

func extractText(result *mcp.CallToolResult) string {
	var parts []string
	for _, content := range result.Content {
		if tc, ok := content.(*mcp.TextContent); ok {
			parts = append(parts, tc.Text)
		}
	}
	return strings.Join(parts, "\n")
}
