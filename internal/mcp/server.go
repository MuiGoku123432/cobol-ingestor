package mcp

import (
	n4j "cobol-ingestor/internal/neo4j"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// NewServer creates an MCP server with all COBOL graph tools registered.
func NewServer(reader n4j.Reader) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{
		Name:    "cobol-graph",
		Version: "1.0.0",
	}, nil)
	registerAllTools(s, reader)
	return s
}
