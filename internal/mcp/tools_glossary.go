package mcp

import (
	"context"

	n4j "cobol-ingestor/internal/neo4j"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func registerLookupGlossaryTerm(s *mcp.Server, reader n4j.Reader) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "lookup_term",
		Description: "Look up a specific company glossary term or acronym by its exact name. Returns the definition and any known aliases.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input LookupGlossaryTermInput) (*mcp.CallToolResult, *n4j.GlossaryTermDetail, error) {
		cb := input.Codebase
		if cb == "" {
			cb = "default"
		}
		detail, err := reader.GetGlossaryTerm(ctx, cb, input.Term)
		if err != nil {
			return nil, nil, err
		}
		if detail == nil {
			return toolError("glossary term not found: " + input.Term), nil, nil
		}
		return nil, detail, nil
	})
}

func registerSearchGlossary(s *mcp.Server, reader n4j.Reader) {
	type output struct {
		Results []n4j.GlossaryTermDetail `json:"results"`
	}
	mcp.AddTool(s, &mcp.Tool{
		Name:        "search_glossary",
		Description: "Full-text search across company glossary terms and definitions. Useful for finding acronym expansions, domain vocabulary, or business concepts.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input SearchGlossaryInput) (*mcp.CallToolResult, *output, error) {
		cb := input.Codebase
		if cb == "" {
			cb = "default"
		}
		limit := input.Limit
		if limit <= 0 {
			limit = 20
		}
		results, err := reader.SearchGlossaryTerms(ctx, cb, input.Query, limit)
		if err != nil {
			return nil, nil, err
		}
		return nil, &output{Results: results}, nil
	})
}

func registerListGlossaryTerms(s *mcp.Server, reader n4j.Reader) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_glossary_terms",
		Description: "List all company glossary terms with pagination. Optionally filter by codebase.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input ListGlossaryTermsInput) (*mcp.CallToolResult, any, error) {
		cb := input.Codebase
		if cb == "" {
			cb = "default"
		}
		page := input.Page
		if page <= 0 {
			page = 1
		}
		pageSize := input.PageSize
		if pageSize <= 0 {
			pageSize = 50
		}
		result, err := reader.ListGlossaryTerms(ctx, cb, page, pageSize)
		if err != nil {
			return nil, nil, err
		}
		return marshalResult(result)
	})
}
