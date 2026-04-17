package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	n4j "cobol-ingestor/internal/neo4j"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func registerCypherTools(s *mcp.Server, client *n4j.Client) {
	registerRunCypherReadonly(s, client)
	registerRunNamedQuery(s, client)
	registerListNamedQueries(s)
}

func registerRunCypherReadonly(s *mcp.Server, client *n4j.Client) {
	mcp.AddTool(s, &mcp.Tool{
		Name: "run_cypher_readonly",
		Description: `Executes an arbitrary READ-ONLY Cypher query against the cobol-graph Neo4j database.
Write keywords (CREATE, DELETE, MERGE, SET, REMOVE, DROP, etc.) are rejected before execution.
A LIMIT clause is auto-appended when absent. Queries run under a 30-second timeout.

Schema quick-reference — Node labels:
  Program, Paragraph, Section, Copybook, DataItem, File, SQLStatement, CICSTransaction,
  JCLJob, JCLStep, BusinessDomain, IDMSRecord, IDMSSet, ExternalDatabase, TargetService

Key relationships:
  CALLS, INCLUDES, READS, WRITES, PERFORMS, PERFORMS_THRU, BELONGS_TO, CHILD_OF, REDEFINES,
  MOVES_TO, RUNS, EXECUTES_SQL, EXECUTES_CICS, LINKAGE_MAPS_TO, DATA_FLOWS_TO, MAPS_TO_EXT_DB

Use list_named_queries / run_named_query for common patterns before writing custom Cypher.`,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input RunCypherReadonlyInput) (*mcp.CallToolResult, any, error) {
		if input.Query == "" {
			return toolError("query is required"), nil, nil
		}
		result, err := client.RunQueryReadOnly(ctx, input.Query, input.Params, input.Limit)
		if err != nil {
			return toolError(fmt.Sprintf("cypher error: %v", err)), nil, nil
		}
		return marshalResult(result)
	})
}

func registerRunNamedQuery(s *mcp.Server, client *n4j.Client) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "run_named_query",
		Description: "Runs a named, parameterized Cypher query from the cobol-graph library. All queries are read-only. Use list_named_queries to discover available names and their parameters.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input RunNamedQueryInput) (*mcp.CallToolResult, any, error) {
		if input.Name == "" {
			return toolError("name is required"), nil, nil
		}

		var q *namedQuery
		for i := range namedQueryRegistry {
			if namedQueryRegistry[i].Name == input.Name {
				q = &namedQueryRegistry[i]
				break
			}
		}
		if q == nil {
			return toolError(fmt.Sprintf("unknown named query '%s'; use list_named_queries to see available queries", input.Name)), nil, nil
		}

		// Merge user params, ensuring nil params for unset optional values
		params := map[string]any{
			"codebase":  nil,
			"limit":     nil,
			"programId": nil,
			"from":      nil,
			"to":        nil,
			"maxHops":   nil,
			"table":     nil,
			"copybook":  nil,
			"pattern":   nil,
			"database":  nil,
		}
		for k, v := range input.Params {
			params[k] = v
		}

		result, err := client.RunQueryReadOnly(ctx, q.Cypher, params, input.Limit)
		if err != nil {
			return toolError(fmt.Sprintf("named query '%s' error: %v", input.Name, err)), nil, nil
		}
		return marshalResult(result)
	})
}

func registerListNamedQueries(s *mcp.Server) {
	type queryCatalogEntry struct {
		Name        string            `json:"name"`
		Description string            `json:"description"`
		ParamDocs   map[string]string `json:"paramDocs,omitempty"`
	}

	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_named_queries",
		Description: "Returns the catalog of curated, parameterized Cypher queries built into cobol-graph. Use run_named_query to execute one.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, _ ListNamedQueriesInput) (*mcp.CallToolResult, any, error) {
		catalog := make([]queryCatalogEntry, len(namedQueryRegistry))
		for i, q := range namedQueryRegistry {
			catalog[i] = queryCatalogEntry{
				Name:        q.Name,
				Description: q.Description,
				ParamDocs:   q.ParamDocs,
			}
		}
		b, err := json.Marshal(catalog)
		if err != nil {
			return toolError("failed to marshal catalog"), nil, nil
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: string(b)}},
		}, nil, nil
	})
}
