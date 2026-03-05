package mcp

import (
	"context"
	"encoding/json"

	n4j "cobol-ingestor/internal/neo4j"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func registerAllTools(s *mcp.Server, reader n4j.Reader) {
	registerGetProgram(s, reader)
	registerSearchPrograms(s, reader)
	registerListPrograms(s, reader)
	registerGetCallChain(s, reader)
	registerGetImpactAnalysis(s, reader)
	registerGetCopybookUsage(s, reader)
	registerGetDataItems(s, reader)
	registerListBusinessDomains(s, reader)
	registerGetBusinessDomain(s, reader)
	registerGetDashboardStats(s, reader)
}

func toolError(msg string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: msg}},
		IsError: true,
	}
}

func registerGetProgram(s *mcp.Server, reader n4j.Reader) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_program",
		Description: "Get full details of a COBOL program including callers, callees, copybooks, paragraphs, data items, sections, file definitions, and business domains.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input GetProgramInput) (*mcp.CallToolResult, *n4j.ProgramDetail, error) {
		detail, err := reader.GetProgram(ctx, input.ProgramID)
		if err != nil {
			return nil, nil, err
		}
		if detail == nil {
			return toolError("program not found: " + input.ProgramID), nil, nil
		}
		return nil, detail, nil
	})
}

func registerSearchPrograms(s *mcp.Server, reader n4j.Reader) {
	type output struct {
		Results []n4j.SearchResult `json:"results"`
	}
	mcp.AddTool(s, &mcp.Tool{
		Name:        "search_programs",
		Description: "Full-text search across programs and paragraphs. Returns scored results.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input SearchProgramsInput) (*mcp.CallToolResult, *output, error) {
		limit := input.Limit
		if limit <= 0 {
			limit = 20
		}
		results, err := reader.SearchFullText(ctx, input.Query, limit)
		if err != nil {
			return nil, nil, err
		}
		return nil, &output{Results: results}, nil
	})
}

func registerListPrograms(s *mcp.Server, reader n4j.Reader) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_programs",
		Description: "List all COBOL programs with pagination. Optionally filter by program ID substring.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input ListProgramsInput) (*mcp.CallToolResult, *n4j.PagedResponse, error) {
		page := input.Page
		if page <= 0 {
			page = 1
		}
		pageSize := input.PageSize
		if pageSize <= 0 {
			pageSize = 20
		}
		result, err := reader.ListPrograms(ctx, n4j.Filter{Search: input.Search}, page, pageSize)
		if err != nil {
			return nil, nil, err
		}
		return nil, result, nil
	})
}

func registerGetCallChain(s *mcp.Server, reader n4j.Reader) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_call_chain",
		Description: "Trace the call chain for a program. Use direction=downstream to see what it calls, direction=upstream to see what calls it.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input GetCallChainInput) (*mcp.CallToolResult, any, error) {
		direction := input.Direction
		if direction == "" {
			direction = "downstream"
		}
		depth := input.Depth
		if depth <= 0 {
			depth = 3
		}
		nodes, err := reader.GetCallChain(ctx, input.ProgramID, direction, depth)
		if err != nil {
			return nil, nil, err
		}
		b, err := json.Marshal(nodes)
		if err != nil {
			return nil, nil, err
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: string(b)}},
		}, nil, nil
	})
}

func registerGetImpactAnalysis(s *mcp.Server, reader n4j.Reader) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_impact_analysis",
		Description: "Analyze the blast radius of changing a program: upstream/downstream dependencies, shared copybooks, and shared files.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input GetImpactAnalysisInput) (*mcp.CallToolResult, *n4j.ImpactResult, error) {
		result, err := reader.GetImpactAnalysis(ctx, input.ProgramID)
		if err != nil {
			return nil, nil, err
		}
		return nil, result, nil
	})
}

func registerGetCopybookUsage(s *mcp.Server, reader n4j.Reader) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_copybook_usage",
		Description: "Find which programs include a given copybook.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input GetCopybookUsageInput) (*mcp.CallToolResult, *n4j.CopybookUsage, error) {
		usage, err := reader.GetCopybookUsage(ctx, input.Name)
		if err != nil {
			return nil, nil, err
		}
		return nil, usage, nil
	})
}

func registerGetDataItems(s *mcp.Server, reader n4j.Reader) {
	type output struct {
		Items []n4j.DataItemInfo `json:"items"`
	}
	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_data_items",
		Description: "List all data items (variables) defined in a program with their levels, FQNs, and PIC clauses.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input GetDataItemsInput) (*mcp.CallToolResult, *output, error) {
		items, err := reader.GetDataItems(ctx, input.ProgramID)
		if err != nil {
			return nil, nil, err
		}
		return nil, &output{Items: items}, nil
	})
}

func registerListBusinessDomains(s *mcp.Server, reader n4j.Reader) {
	type emptyInput struct{}
	type output struct {
		Domains []n4j.BusinessDomainSummary `json:"domains"`
	}
	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_business_domains",
		Description: "List all business domains with their program counts.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input emptyInput) (*mcp.CallToolResult, *output, error) {
		domains, err := reader.ListBusinessDomains(ctx)
		if err != nil {
			return nil, nil, err
		}
		return nil, &output{Domains: domains}, nil
	})
}

func registerGetBusinessDomain(s *mcp.Server, reader n4j.Reader) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_business_domain",
		Description: "Get details of a business domain including all member programs.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input GetBusinessDomainInput) (*mcp.CallToolResult, *n4j.BusinessDomainDetail, error) {
		detail, err := reader.GetBusinessDomain(ctx, input.Name)
		if err != nil {
			return nil, nil, err
		}
		if detail == nil {
			return toolError("domain not found: " + input.Name), nil, nil
		}
		return nil, detail, nil
	})
}

func registerGetDashboardStats(s *mcp.Server, reader n4j.Reader) {
	type emptyInput struct{}
	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_dashboard_stats",
		Description: "Get aggregate statistics: program count, copybook count, paragraph count, data items, relationships, orphans, and domain count.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input emptyInput) (*mcp.CallToolResult, *n4j.DashboardStats, error) {
		stats, err := reader.GetDashboardStats(ctx)
		if err != nil {
			return nil, nil, err
		}
		return nil, stats, nil
	})
}
