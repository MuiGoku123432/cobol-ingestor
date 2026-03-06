package mcp

import (
	"context"

	n4j "cobol-ingestor/internal/neo4j"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Phase 1: Dead paragraph detection tools

func registerGetDeadParagraphs(s *mcp.Server, reader n4j.Reader) {
	type output struct {
		Paragraphs []n4j.DeadParagraphInfo `json:"paragraphs"`
	}
	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_dead_paragraphs",
		Description: "List unreachable (dead) paragraphs for a program. These are paragraphs not reachable from any entry paragraph via PERFORMS/PERFORMS_THRU control flow.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input GetProgramInput) (*mcp.CallToolResult, *output, error) {
		items, err := reader.GetDeadParagraphs(ctx, input.ProgramID)
		if err != nil {
			return nil, nil, err
		}
		return nil, &output{Paragraphs: items}, nil
	})
}

func registerGetDeadCodeSummary(s *mcp.Server, reader n4j.Reader) {
	type emptyInput struct{}
	type output struct {
		Programs []n4j.DeadCodeSummaryInfo `json:"programs"`
	}
	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_dead_code_summary",
		Description: "Get aggregate dead paragraph counts per program. Shows total vs dead (unreachable) paragraphs for each program that has dead code.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input emptyInput) (*mcp.CallToolResult, *output, error) {
		items, err := reader.GetDeadCodeSummary(ctx)
		if err != nil {
			return nil, nil, err
		}
		return nil, &output{Programs: items}, nil
	})
}

// Phase 2: JCL analysis tools

func registerListJCLJobs(s *mcp.Server, reader n4j.Reader) {
	type emptyInput struct{}
	type output struct {
		Jobs []n4j.JCLJobInfo `json:"jobs"`
	}
	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_jcl_jobs",
		Description: "List all JCL jobs with step counts.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input emptyInput) (*mcp.CallToolResult, *output, error) {
		items, err := reader.ListJCLJobs(ctx)
		if err != nil {
			return nil, nil, err
		}
		return nil, &output{Jobs: items}, nil
	})
}

func registerGetJCLJob(s *mcp.Server, reader n4j.Reader) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_jcl_job",
		Description: "Get full JCL job detail: steps, programs invoked, and datasets (DD cards).",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input GetJCLJobInput) (*mcp.CallToolResult, *n4j.JCLJobDetail, error) {
		detail, err := reader.GetJCLJob(ctx, input.JobName)
		if err != nil {
			return nil, nil, err
		}
		if detail == nil {
			return toolError("JCL job not found: " + input.JobName), nil, nil
		}
		return nil, detail, nil
	})
}

func registerGetProgramJCL(s *mcp.Server, reader n4j.Reader) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_program_jcl",
		Description: "Reverse lookup: which JCL jobs and steps invoke a given program.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input GetProgramInput) (*mcp.CallToolResult, *n4j.ProgramJCLInfo, error) {
		info, err := reader.GetProgramJCL(ctx, input.ProgramID)
		if err != nil {
			return nil, nil, err
		}
		return nil, info, nil
	})
}

func registerGetDatasetUsage(s *mcp.Server, reader n4j.Reader) {
	type output struct {
		Datasets []n4j.DatasetUsageInfo `json:"datasets"`
	}
	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_dataset_usage",
		Description: "Find which JCL jobs read/write a dataset (searches by dataset name substring).",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input GetDatasetUsageInput) (*mcp.CallToolResult, *output, error) {
		items, err := reader.GetDatasetUsage(ctx, input.DSName)
		if err != nil {
			return nil, nil, err
		}
		return nil, &output{Datasets: items}, nil
	})
}

// Phase 3: DB table access tools

func registerListDBTables(s *mcp.Server, reader n4j.Reader) {
	type emptyInput struct{}
	type output struct {
		Tables []n4j.DBTableInfo `json:"tables"`
	}
	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_db_tables",
		Description: "List all database tables discovered from SQL statements, with program access counts.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input emptyInput) (*mcp.CallToolResult, *output, error) {
		items, err := reader.ListDBTables(ctx)
		if err != nil {
			return nil, nil, err
		}
		return nil, &output{Tables: items}, nil
	})
}

func registerGetTableUsage(s *mcp.Server, reader n4j.Reader) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_table_usage",
		Description: "Find which programs access a database table, with their operations (SELECT/INSERT/UPDATE/DELETE) and columns.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input GetTableUsageInput) (*mcp.CallToolResult, *n4j.TableUsageInfo, error) {
		usage, err := reader.GetTableUsage(ctx, input.TableName)
		if err != nil {
			return nil, nil, err
		}
		return nil, usage, nil
	})
}

func registerGetProgramTableAccess(s *mcp.Server, reader n4j.Reader) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_program_table_access",
		Description: "List all database tables accessed by a program with operations and columns.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input GetProgramInput) (*mcp.CallToolResult, *n4j.ProgramTableAccessInfo, error) {
		info, err := reader.GetProgramTableAccess(ctx, input.ProgramID)
		if err != nil {
			return nil, nil, err
		}
		return nil, info, nil
	})
}

// Phase 4: Cross-program data flow tools

func registerGetCrossProgramDataFlow(s *mcp.Server, reader n4j.Reader) {
	type output struct {
		Flows []n4j.CrossProgramFlowInfo `json:"flows"`
	}
	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_cross_program_data_flow",
		Description: "Get all cross-program data flows for a program (inbound and outbound). Shows flows through shared files, DB2 tables, and LINKAGE parameter passing.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input GetProgramInput) (*mcp.CallToolResult, *output, error) {
		items, err := reader.GetCrossProgramDataFlow(ctx, input.ProgramID)
		if err != nil {
			return nil, nil, err
		}
		return nil, &output{Flows: items}, nil
	})
}

func registerTraceFieldImpact(s *mcp.Server, reader n4j.Reader) {
	type output struct {
		Impact []n4j.FieldImpactInfo `json:"impact"`
	}
	mcp.AddTool(s, &mcp.Tool{
		Name:        "trace_field_impact",
		Description: "Trace a field downstream through intra-program MOVES_TO and cross-program CALLS + LINKAGE data flows.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input TraceFieldImpactInput) (*mcp.CallToolResult, *output, error) {
		items, err := reader.TraceFieldImpact(ctx, input.ProgramID, input.FieldName)
		if err != nil {
			return nil, nil, err
		}
		return nil, &output{Impact: items}, nil
	})
}

func registerGetSharedDataChannels(s *mcp.Server, reader n4j.Reader) {
	type emptyInput struct{}
	type output struct {
		Channels []n4j.SharedDataChannelInfo `json:"channels"`
	}
	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_shared_data_channels",
		Description: "List all shared files and database tables with the programs that write to and read from them.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input emptyInput) (*mcp.CallToolResult, *output, error) {
		items, err := reader.GetSharedDataChannels(ctx)
		if err != nil {
			return nil, nil, err
		}
		return nil, &output{Channels: items}, nil
	})
}
