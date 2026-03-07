package modernize

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

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
