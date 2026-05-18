package mcp

import (
	n4j "cobol-ingestor/internal/neo4j"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ServerOptions holds optional configuration for the MCP server.
type ServerOptions struct {
	DiagramOutputDir string // non-empty enables the generate_mermaid_diagram tool
}

// NewServer creates an MCP server with all COBOL graph tools registered.
// writer is optional — pass nil if write operations (e.g. domain reassignment) are not needed.
// opts is optional — pass nil for defaults.
func NewServer(client *n4j.Client, writer *n4j.BatchWriter, opts *ServerOptions) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{
		Name:    "cobol-graph",
		Version: "1.0.0",
	}, nil)
	registerAllTools(s, client, writer, client)
	if opts != nil && opts.DiagramOutputDir != "" {
		registerGenerateMermaidDiagram(s, opts.DiagramOutputDir)
	}
	return s
}
