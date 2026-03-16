package mcp

import (
	"context"

	"cobol-ingestor/internal/graph"
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

// Domain reassignment write tool

func registerReassignProgramDomain(s *mcp.Server, writer *n4j.BatchWriter) {
	type output struct {
		Success   bool   `json:"success"`
		ProgramID string `json:"programId"`
		Domain    string `json:"domain"`
	}
	mcp.AddTool(s, &mcp.Tool{
		Name:        "reassign_program_domain",
		Description: "Move a program to a different business domain. Deletes existing BELONGS_TO edges and creates a new one with confidence 1.0 and source 'manual_override'.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input ReassignDomainInput) (*mcp.CallToolResult, *output, error) {
		if input.ProgramID == "" || input.Domain == "" {
			return toolError("both programId and domain are required"), nil, nil
		}
		if err := writer.ReassignProgramDomain(ctx, input.ProgramID, input.Domain); err != nil {
			return nil, nil, err
		}
		return nil, &output{
			Success:   true,
			ProgramID: input.ProgramID,
			Domain:    input.Domain,
		}, nil
	})
}

// Copybook structure tool

func registerGetCopybookStructure(s *mcp.Server, reader n4j.Reader) {
	type output struct {
		Items []n4j.DataItemInfo `json:"items"`
	}
	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_copybook_structure",
		Description: "Get the data structure defined in a copybook: all data items with levels, PIC clauses, and usage. Use to understand shared data layouts.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input GetCopybookStructureInput) (*mcp.CallToolResult, *output, error) {
		items, err := reader.GetCopybookStructure(ctx, input.Name)
		if err != nil {
			return nil, nil, err
		}
		if len(items) == 0 {
			return toolError("no data items found for copybook: " + input.Name), nil, nil
		}
		return nil, &output{Items: items}, nil
	})
}

// Type mappings tool

func registerGetTypeMappings(s *mcp.Server, reader n4j.Reader) {
	type mappedItem struct {
		Name    string            `json:"name"`
		Level   int               `json:"level"`
		Picture string            `json:"picture,omitempty"`
		Usage   string            `json:"usage,omitempty"`
		Mapping graph.TypeMapping `json:"mapping"`
	}
	type output struct {
		Items []mappedItem `json:"items"`
	}
	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_type_mappings",
		Description: "Get Java and SQL type mappings for data items in a program or copybook. Maps COBOL PIC clauses and USAGE to Java types, SQL types, storage format, and byte length.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input GetTypeMappingsInput) (*mcp.CallToolResult, *output, error) {
		var dataItems []n4j.DataItemInfo
		var err error
		if input.CopybookName != "" {
			dataItems, err = reader.GetCopybookStructure(ctx, input.CopybookName)
		} else if input.ProgramID != "" {
			dataItems, err = reader.GetDataItems(ctx, input.ProgramID)
		} else {
			return toolError("either programId or copybookName is required"), nil, nil
		}
		if err != nil {
			return nil, nil, err
		}

		var items []mappedItem
		for _, di := range dataItems {
			if di.Picture == "" {
				continue
			}
			items = append(items, mappedItem{
				Name:    di.Name,
				Level:   di.Level,
				Picture: di.Picture,
				Usage:   di.Usage,
				Mapping: graph.MapPICToTypes(di.Picture, di.Usage),
			})
		}
		if len(items) == 0 {
			return toolError("no data items with PIC clauses found"), nil, nil
		}
		return nil, &output{Items: items}, nil
	})
}

// Source code retrieval tool

func registerGetProgramSource(s *mcp.Server, reader n4j.Reader) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_program_source",
		Description: "Retrieve the raw COBOL source code for a program from disk. Returns the full source text with line count.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input GetProgramInput) (*mcp.CallToolResult, *n4j.ProgramSourceInfo, error) {
		info, err := reader.GetProgramSource(ctx, input.ProgramID)
		if err != nil {
			return nil, nil, err
		}
		if info == nil {
			return toolError("program not found or no source file: " + input.ProgramID), nil, nil
		}
		return nil, info, nil
	})
}

// Migration dependency ordering tool

func registerGetMigrationSequence(s *mcp.Server, reader n4j.Reader) {
	type emptyInput struct{}
	type output struct {
		Steps []n4j.MigrationStep `json:"steps"`
	}
	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_migration_sequence",
		Description: "Get dependency-ordered migration sequence. Programs are sorted so that dependencies (callees) are migrated before callers. Each step shows blockedBy dependencies, tier (LEAF/MIDDLE/ROOT), and T-shirt sizing.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input emptyInput) (*mcp.CallToolResult, *output, error) {
		steps, err := reader.GetMigrationSequence(ctx)
		if err != nil {
			return nil, nil, err
		}
		return nil, &output{Steps: steps}, nil
	})
}

// File accessor drill-down tool

func registerGetFileAccessors(s *mcp.Server, reader n4j.Reader) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_file_accessors",
		Description: "Get all programs that access a file (READS, WRITES, or both) and any JCL DD card mappings for the file.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input GetFileAccessorsInput) (*mcp.CallToolResult, *n4j.FileAccessInfo, error) {
		info, err := reader.GetFileAccessors(ctx, input.FileName)
		if err != nil {
			return nil, nil, err
		}
		return nil, info, nil
	})
}

// Effort estimation tool

func registerGetEffortEstimates(s *mcp.Server, reader n4j.Reader) {
	type emptyInput struct{}
	type output struct {
		Estimates []n4j.EffortEstimate `json:"estimates"`
	}
	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_effort_estimates",
		Description: "Get structural complexity and effort estimates for all programs. Includes paragraph/copybook/data item counts, external interfaces, SQL/CICS counts, complexity score, and T-shirt size (S/M/L/XL).",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input emptyInput) (*mcp.CallToolResult, *output, error) {
		items, err := reader.GetEffortEstimates(ctx)
		if err != nil {
			return nil, nil, err
		}
		return nil, &output{Estimates: items}, nil
	})
}

// Phase 5: Validation report tool

func registerGetValidationReport(s *mcp.Server, reader n4j.Reader) {
	type emptyInput struct{}
	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_validation_report",
		Description: "Run graph validation checks and return a report of detected gaps: missing Pass 3 analysis, missing CHILD_OF/MOVES_TO/CALLS relationships, unannotated paragraphs, unlinked DD cards, dangling calls, and orphan data items.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input emptyInput) (*mcp.CallToolResult, *n4j.ValidationResult, error) {
		result, err := reader.GetValidationReport(ctx)
		if err != nil {
			return nil, nil, err
		}
		return nil, result, nil
	})
}
