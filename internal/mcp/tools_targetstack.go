package mcp

import (
	"context"

	n4j "cobol-ingestor/internal/neo4j"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func registerTargetStackTools(s *mcp.Server, reader n4j.Reader) {
	registerListTargetRepos(s, reader)
	registerListTargetServices(s, reader)
	registerGetTargetService(s, reader)
	registerListBusinessGaps(s, reader)
	registerListBusinessRequirements(s, reader)
	registerGetGapCoverageSummary(s, reader)
	registerGetTargetStackDashboard(s, reader)
}

func registerListTargetRepos(s *mcp.Server, reader n4j.Reader) {
	type emptyInput struct{}
	type output struct {
		Repos []n4j.TargetRepoInfo `json:"repos"`
	}
	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_target_repos",
		Description: "List all connected target stack repositories that have been analyzed for business logic gap analysis.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ emptyInput) (*mcp.CallToolResult, *output, error) {
		repos, err := reader.ListTargetRepos(ctx)
		if err != nil {
			return nil, nil, err
		}
		return nil, &output{Repos: repos}, nil
	})
}

type listTargetServicesInput struct {
	RepoURL string `json:"repoUrl,omitempty" jsonschema:"description=Filter by repository URL. Leave empty to list services from all repos."`
}

func registerListTargetServices(s *mcp.Server, reader n4j.Reader) {
	type output struct {
		Services []n4j.TargetServiceInfo `json:"services"`
	}
	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_target_services",
		Description: "List all services/modules extracted from the target stack repositories. Services represent logical components like REST APIs, message consumers, or batch jobs.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input listTargetServicesInput) (*mcp.CallToolResult, *output, error) {
		services, err := reader.ListTargetServices(ctx, input.RepoURL)
		if err != nil {
			return nil, nil, err
		}
		return nil, &output{Services: services}, nil
	})
}

type getTargetServiceInput struct {
	ServiceID string `json:"serviceId" jsonschema:"required,description=The service ID (format: repoURL::serviceName)"`
}

func registerGetTargetService(s *mcp.Server, reader n4j.Reader) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_target_service",
		Description: "Get full details for a target stack service including its endpoints, business rules, data models, integrations, and error handling patterns.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input getTargetServiceInput) (*mcp.CallToolResult, any, error) {
		detail, err := reader.GetTargetService(ctx, input.ServiceID)
		if err != nil {
			return nil, nil, err
		}
		if detail == nil {
			return toolError("service not found: " + input.ServiceID), nil, nil
		}
		return marshalResult(detail)
	})
}

type listBusinessGapsInput struct {
	GapType  string `json:"gapType,omitempty"  jsonschema:"description=Filter by gap type: COBOL_ONLY\\, TARGET_ONLY\\, PARTIAL_MATCH\\, or SEMANTIC_MISMATCH"`
	Severity string `json:"severity,omitempty" jsonschema:"description=Filter by severity: CRITICAL\\, HIGH\\, MEDIUM\\, or LOW"`
	Category string `json:"category,omitempty" jsonschema:"description=Filter by category: BUSINESS_RULE\\, DATA_MODEL\\, INTEGRATION\\, ERROR_HANDLING\\, or BATCH_PROCESSING"`
}

func registerListBusinessGaps(s *mcp.Server, reader n4j.Reader) {
	type output struct {
		Gaps []n4j.BusinessGapInfo `json:"gaps"`
	}
	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_business_gaps",
		Description: "List identified business logic gaps between the COBOL mainframe and the target stack. Gaps represent business rules, data models, integrations, or processes that exist in one system but not the other.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input listBusinessGapsInput) (*mcp.CallToolResult, *output, error) {
		gaps, err := reader.ListBusinessGaps(ctx, input.GapType, input.Severity, input.Category)
		if err != nil {
			return nil, nil, err
		}
		return nil, &output{Gaps: gaps}, nil
	})
}

type listBusinessRequirementsInput struct {
	Priority string `json:"priority,omitempty" jsonschema:"description=Filter by priority: P0\\, P1\\, P2\\, or P3"`
}

func registerListBusinessRequirements(s *mcp.Server, reader n4j.Reader) {
	type output struct {
		Requirements []n4j.BusinessRequirementInfo `json:"requirements"`
	}
	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_business_requirements",
		Description: "List generated business requirements derived from business logic gaps. Requirements include acceptance criteria and effort estimates.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input listBusinessRequirementsInput) (*mcp.CallToolResult, *output, error) {
		reqs, err := reader.ListBusinessRequirements(ctx, input.Priority)
		if err != nil {
			return nil, nil, err
		}
		return nil, &output{Requirements: reqs}, nil
	})
}

func registerGetGapCoverageSummary(s *mcp.Server, reader n4j.Reader) {
	type emptyInput struct{}
	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_gap_coverage_summary",
		Description: "Get a high-level summary of business logic coverage: how many COBOL business domains are covered vs uncovered in the target stack, and overall gap statistics.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ emptyInput) (*mcp.CallToolResult, any, error) {
		summary, err := reader.GetGapCoverageSummary(ctx)
		if err != nil {
			return nil, nil, err
		}
		return marshalResult(summary)
	})
}

func registerGetTargetStackDashboard(s *mcp.Server, reader n4j.Reader) {
	type emptyInput struct{}
	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_target_stack_dashboard",
		Description: "Get aggregate statistics about the connected target stack: number of repos, services, endpoints, business rules, data models, gaps, and requirements.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ emptyInput) (*mcp.CallToolResult, any, error) {
		stats, err := reader.GetTargetStackDashboard(ctx)
		if err != nil {
			return nil, nil, err
		}
		return marshalResult(stats)
	})
}
