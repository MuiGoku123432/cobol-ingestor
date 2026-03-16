package mcp

import (
	n4j "cobol-ingestor/internal/neo4j"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// NewServer creates an MCP server with all COBOL graph tools registered.
// writer is optional — pass nil if write operations (e.g. domain reassignment) are not needed.
func NewServer(reader n4j.Reader, writer *n4j.BatchWriter) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{
		Name:    "cobol-graph",
		Version: "1.0.0",
	}, nil)
	registerAllTools(s, reader, writer)
	return s
}
