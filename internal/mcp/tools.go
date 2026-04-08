package mcp

import (
	"context"
	"encoding/json"

	n4j "cobol-ingestor/internal/neo4j"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func registerAllTools(s *mcp.Server, reader n4j.Reader, writer *n4j.BatchWriter, client *n4j.Client) {
	registerGetProgram(s, reader)
	registerSearchPrograms(s, reader)
	registerListPrograms(s, reader)
	registerListCopybooks(s, reader)
	registerGetCallChain(s, reader)
	registerGetImpactAnalysis(s, reader)
	registerGetCopybookUsage(s, reader)
	registerGetDataItems(s, reader)
	registerListBusinessDomains(s, reader)
	registerGetBusinessDomain(s, reader)
	registerGetDashboardStats(s, reader)
	registerGetProgramConditions(s, reader)
	registerGetProgramParameters(s, reader)
	registerGetProgramConditionalLogic(s, reader)
	registerGetProgramErrorHandlers(s, reader)
	registerGetProgramExternalInterfaces(s, reader)
	registerListBridgePrograms(s, reader)
	registerListCopybookRisks(s, reader)
	registerListModernizationCandidates(s, reader)
	registerListRiskPrograms(s, reader)
	registerListVolumeEstimates(s, reader)
	registerGetProgramSQL(s, reader)
	registerGetProgramCICS(s, reader)
	registerGetParagraphFlow(s, reader)
	registerGetDataFlow(s, reader)
	registerGetDataHierarchy(s, reader)
	// Phase 1: Dead paragraph detection
	registerGetDeadParagraphs(s, reader)
	registerGetDeadCodeSummary(s, reader)
	// Phase 2: JCL analysis
	registerListJCLJobs(s, reader)
	registerGetJCLJob(s, reader)
	registerGetProgramJCL(s, reader)
	registerGetDatasetUsage(s, reader)
	// Phase 3: DB table access
	registerListDBTables(s, reader)
	registerGetTableUsage(s, reader)
	registerGetProgramTableAccess(s, reader)
	// Phase 4: Cross-program data flow
	registerGetCrossProgramDataFlow(s, reader)
	registerTraceFieldImpact(s, reader)
	registerGetSharedDataChannels(s, reader)
	// Phase 5: Validation report
	registerGetValidationReport(s, reader)
	// Copybook structure & type mappings
	registerGetCopybookStructure(s, reader)
	registerGetTypeMappings(s, reader)
	// Source retrieval, migration, file accessors, effort estimates
	registerGetProgramSource(s, reader)
	registerGetMigrationSequence(s, reader)
	registerGetFileAccessors(s, reader)
	registerGetEffortEstimates(s, reader)
	// IDMS tools
	registerGetIDMSRecords(s, reader)
	registerGetIDMSSchema(s, reader)
	registerGetIDMSImpact(s, reader)
	registerGetIDMSAreas(s, reader)
	// External DB gap analysis tools
	registerExternalDBTools(s, reader)
	// Target stack gap analysis tools
	registerTargetStackTools(s, reader)
	// Codebase filtering tools
	registerListCodebases(s, client)
	registerGetCrossCodebaseCalls(s, client)
	// Write tools (require writer)
	if writer != nil {
		registerReassignProgramDomain(s, writer)
	}
}

func toolError(msg string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: msg}},
		IsError: true,
	}
}

// marshalResult serializes v to JSON and returns it as a TextContent result.
// Use this for output types that can't be schema-inferred (any, recursive types).
func marshalResult(v any) (*mcp.CallToolResult, any, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, nil, err
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(b)}},
	}, nil, nil
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
	}, func(ctx context.Context, req *mcp.CallToolRequest, input ListProgramsInput) (*mcp.CallToolResult, any, error) {
		page := input.Page
		if page <= 0 {
			page = 1
		}
		pageSize := input.PageSize
		if pageSize <= 0 {
			pageSize = 20
		}
		result, err := reader.ListPrograms(ctx, n4j.Filter{Search: input.Search, Codebase: input.Codebase}, page, pageSize)
		if err != nil {
			return nil, nil, err
		}
		return marshalResult(result)
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
		return marshalResult(nodes)
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

func registerGetProgramConditions(s *mcp.Server, reader n4j.Reader) {
	type output struct {
		Conditions []n4j.ConditionInfo `json:"conditions"`
	}
	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_program_conditions",
		Description: "List 88-level condition variables defined in a program.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input GetProgramInput) (*mcp.CallToolResult, *output, error) {
		items, err := reader.GetProgramConditions(ctx, input.ProgramID)
		if err != nil {
			return nil, nil, err
		}
		return nil, &output{Conditions: items}, nil
	})
}

func registerGetProgramParameters(s *mcp.Server, reader n4j.Reader) {
	type output struct {
		Parameters []n4j.ParameterInfo `json:"parameters"`
	}
	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_program_parameters",
		Description: "List LINKAGE SECTION parameters for a program with direction (IN/OUT/INOUT).",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input GetProgramInput) (*mcp.CallToolResult, *output, error) {
		items, err := reader.GetProgramParameters(ctx, input.ProgramID)
		if err != nil {
			return nil, nil, err
		}
		return nil, &output{Parameters: items}, nil
	})
}

func registerGetProgramConditionalLogic(s *mcp.Server, reader n4j.Reader) {
	type output struct {
		Logic []n4j.ConditionalLogicInfo `json:"conditionalLogic"`
	}
	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_program_conditional_logic",
		Description: "List IF/EVALUATE decision points in a program's paragraphs.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input GetProgramInput) (*mcp.CallToolResult, *output, error) {
		items, err := reader.GetProgramConditionalLogic(ctx, input.ProgramID)
		if err != nil {
			return nil, nil, err
		}
		return nil, &output{Logic: items}, nil
	})
}

func registerGetProgramErrorHandlers(s *mcp.Server, reader n4j.Reader) {
	type output struct {
		Handlers []n4j.ErrorHandlerInfo `json:"errorHandlers"`
	}
	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_program_error_handlers",
		Description: "List error handling patterns found in a program's paragraphs.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input GetProgramInput) (*mcp.CallToolResult, *output, error) {
		items, err := reader.GetProgramErrorHandlers(ctx, input.ProgramID)
		if err != nil {
			return nil, nil, err
		}
		return nil, &output{Handlers: items}, nil
	})
}

func registerGetProgramExternalInterfaces(s *mcp.Server, reader n4j.Reader) {
	type output struct {
		Interfaces []n4j.ExternalInterfaceInfo `json:"externalInterfaces"`
	}
	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_program_external_interfaces",
		Description: "List external integration points (MQ, CICS LINK/XCTL/TS/TD/START/FILE/ENQ, IMS, IDMS, ADABAS, SORT, batch utilities, TCP, file transfers) for a program.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input GetProgramInput) (*mcp.CallToolResult, *output, error) {
		items, err := reader.GetProgramExternalInterfaces(ctx, input.ProgramID)
		if err != nil {
			return nil, nil, err
		}
		return nil, &output{Interfaces: items}, nil
	})
}

func registerListBridgePrograms(s *mcp.Server, reader n4j.Reader) {
	type emptyInput struct{}
	type output struct {
		Programs []n4j.BridgeProgramInfo `json:"programs"`
	}
	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_bridge_programs",
		Description: "List programs that connect multiple business domains.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input emptyInput) (*mcp.CallToolResult, *output, error) {
		items, err := reader.ListBridgePrograms(ctx)
		if err != nil {
			return nil, nil, err
		}
		return nil, &output{Programs: items}, nil
	})
}

func registerListCopybookRisks(s *mcp.Server, reader n4j.Reader) {
	type emptyInput struct{}
	type output struct {
		Risks []n4j.CopybookRiskInfo `json:"risks"`
	}
	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_copybook_risks",
		Description: "List copybooks with risk assessments based on usage count and cross-domain spread.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input emptyInput) (*mcp.CallToolResult, *output, error) {
		items, err := reader.ListCopybookRisks(ctx)
		if err != nil {
			return nil, nil, err
		}
		return nil, &output{Risks: items}, nil
	})
}

func registerListModernizationCandidates(s *mcp.Server, reader n4j.Reader) {
	type emptyInput struct{}
	type output struct {
		Candidates []n4j.ModernizationCandidateInfo `json:"candidates"`
	}
	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_modernization_candidates",
		Description: "List programs scored as modernization candidates with recommended approach.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input emptyInput) (*mcp.CallToolResult, *output, error) {
		items, err := reader.ListModernizationCandidates(ctx)
		if err != nil {
			return nil, nil, err
		}
		return nil, &output{Candidates: items}, nil
	})
}

func registerListRiskPrograms(s *mcp.Server, reader n4j.Reader) {
	type output struct {
		Programs []n4j.RiskProgramInfo `json:"programs"`
	}
	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_risk_programs",
		Description: "List programs with risk scores at or above a minimum threshold.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input ListRiskProgramsInput) (*mcp.CallToolResult, *output, error) {
		minScore := input.MinScore
		if minScore <= 0 {
			minScore = 0.5
		}
		items, err := reader.ListRiskPrograms(ctx, minScore)
		if err != nil {
			return nil, nil, err
		}
		return nil, &output{Programs: items}, nil
	})
}

func registerListVolumeEstimates(s *mcp.Server, reader n4j.Reader) {
	type emptyInput struct{}
	type output struct {
		Estimates []n4j.VolumeEstimateInfo `json:"estimates"`
	}
	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_volume_estimates",
		Description: "List transaction volume estimates (HIGH/MEDIUM/LOW) for all analyzed programs.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input emptyInput) (*mcp.CallToolResult, *output, error) {
		items, err := reader.ListVolumeEstimates(ctx)
		if err != nil {
			return nil, nil, err
		}
		return nil, &output{Estimates: items}, nil
	})
}

func registerListCopybooks(s *mcp.Server, reader n4j.Reader) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_copybooks",
		Description: "List all copybooks with pagination and usage counts. Optionally filter by name substring.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input ListProgramsInput) (*mcp.CallToolResult, any, error) {
		page := input.Page
		if page <= 0 {
			page = 1
		}
		pageSize := input.PageSize
		if pageSize <= 0 {
			pageSize = 20
		}
		result, err := reader.ListCopybooks(ctx, n4j.Filter{Search: input.Search, Codebase: input.Codebase}, page, pageSize)
		if err != nil {
			return nil, nil, err
		}
		return marshalResult(result)
	})
}

func registerGetProgramSQL(s *mcp.Server, reader n4j.Reader) {
	type output struct {
		Statements []n4j.SQLStatementInfo `json:"statements"`
	}
	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_program_sql",
		Description: "List SQL statements (SELECT, INSERT, UPDATE, DELETE, etc.) embedded in a COBOL program.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input GetProgramInput) (*mcp.CallToolResult, *output, error) {
		items, err := reader.GetProgramSQL(ctx, input.ProgramID)
		if err != nil {
			return nil, nil, err
		}
		return nil, &output{Statements: items}, nil
	})
}

func registerGetProgramCICS(s *mcp.Server, reader n4j.Reader) {
	type output struct {
		Transactions []n4j.CICSTransactionInfo `json:"transactions"`
	}
	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_program_cics",
		Description: "List CICS transaction commands (SEND, RECEIVE, READ, WRITE, LINK, XCTL, etc.) used by a program.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input GetProgramInput) (*mcp.CallToolResult, *output, error) {
		items, err := reader.GetProgramCICS(ctx, input.ProgramID)
		if err != nil {
			return nil, nil, err
		}
		return nil, &output{Transactions: items}, nil
	})
}

func registerGetParagraphFlow(s *mcp.Server, reader n4j.Reader) {
	type output struct {
		Flow []n4j.ParagraphFlowInfo `json:"flow"`
	}
	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_paragraph_flow",
		Description: "Get PERFORMS and PERFORMS THRU control flow between paragraphs in a program. Shows which paragraphs call which, with loop and condition info.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input GetProgramInput) (*mcp.CallToolResult, *output, error) {
		items, err := reader.GetParagraphFlow(ctx, input.ProgramID)
		if err != nil {
			return nil, nil, err
		}
		return nil, &output{Flow: items}, nil
	})
}

func registerGetDataFlow(s *mcp.Server, reader n4j.Reader) {
	type output struct {
		Flows []n4j.DataFlowInfo `json:"flows"`
	}
	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_data_flow",
		Description: "Get MOVES_TO data flow relationships between data items in a program. Shows how data moves between variables.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input GetProgramInput) (*mcp.CallToolResult, *output, error) {
		items, err := reader.GetDataFlow(ctx, input.ProgramID)
		if err != nil {
			return nil, nil, err
		}
		return nil, &output{Flows: items}, nil
	})
}

func registerGetDataHierarchy(s *mcp.Server, reader n4j.Reader) {
	type output struct {
		Hierarchy []n4j.DataHierarchyInfo `json:"hierarchy"`
	}
	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_data_hierarchy",
		Description: "Get data item hierarchy (CHILD_OF parent-child relationships) and REDEFINES relationships for a program.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input GetProgramInput) (*mcp.CallToolResult, *output, error) {
		items, err := reader.GetDataHierarchy(ctx, input.ProgramID)
		if err != nil {
			return nil, nil, err
		}
		return nil, &output{Hierarchy: items}, nil
	})
}

func registerGetIDMSRecords(s *mcp.Server, reader n4j.Reader) {
	type output struct {
		Records []n4j.IDMSRecordInfo `json:"records"`
	}
	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_idms_records",
		Description: "List IDMS database records referenced by a program, including area associations.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input GetIDMSRecordsInput) (*mcp.CallToolResult, *output, error) {
		items, err := reader.GetIDMSRecords(ctx, input.ProgramID)
		if err != nil {
			return nil, nil, err
		}
		return nil, &output{Records: items}, nil
	})
}

func registerGetIDMSSchema(s *mcp.Server, reader n4j.Reader) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_idms_schema",
		Description: "Get the IDMS schema/subschema binding and protocol mode for a program.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input GetIDMSSchemaInput) (*mcp.CallToolResult, *n4j.IDMSSchemaInfo, error) {
		info, err := reader.GetIDMSSchema(ctx, input.ProgramID)
		if err != nil {
			return nil, nil, err
		}
		if info == nil {
			return toolError("no IDMS schema found for program: " + input.ProgramID), nil, nil
		}
		return nil, info, nil
	})
}

func registerGetIDMSImpact(s *mcp.Server, reader n4j.Reader) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_idms_impact",
		Description: "Analyze which programs navigate, store, modify, or erase a given IDMS record. Shows the blast radius of changing an IDMS record definition.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input GetIDMSImpactInput) (*mcp.CallToolResult, *n4j.IDMSImpactInfo, error) {
		info, err := reader.GetIDMSImpact(ctx, input.RecordName)
		if err != nil {
			return nil, nil, err
		}
		return nil, info, nil
	})
}

func registerGetIDMSAreas(s *mcp.Server, reader n4j.Reader) {
	type output struct {
		Areas []n4j.IDMSAreaInfo `json:"areas"`
	}
	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_idms_areas",
		Description: "List IDMS database areas readied by a program with their usage modes (RETRIEVAL/UPDATE).",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input GetIDMSAreasInput) (*mcp.CallToolResult, *output, error) {
		items, err := reader.GetIDMSAreas(ctx, input.ProgramID)
		if err != nil {
			return nil, nil, err
		}
		return nil, &output{Areas: items}, nil
	})
}

func registerListCodebases(s *mcp.Server, client *n4j.Client) {
	type output struct {
		Codebases []string `json:"codebases"`
	}
	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_codebases",
		Description: "List all codebase identifiers present in the graph.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input ListCodebasesInput) (*mcp.CallToolResult, *output, error) {
		codebases, err := client.ListCodebases(ctx)
		if err != nil {
			return nil, nil, err
		}
		return nil, &output{Codebases: codebases}, nil
	})
}

func registerGetCrossCodebaseCalls(s *mcp.Server, client *n4j.Client) {
	type callInfo struct {
		CallerID       string `json:"callerId"`
		CallerCodebase string `json:"callerCodebase"`
		CalleeID       string `json:"calleeId"`
		CalleeCodebase string `json:"calleeCodebase"`
	}
	type output struct {
		Calls []callInfo `json:"calls"`
	}
	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_cross_codebase_calls",
		Description: "Find CALLS relationships where caller and callee belong to different codebases.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input GetCrossCodebaseCallsInput) (*mcp.CallToolResult, *output, error) {
		calls, err := client.GetCrossCodebaseCalls(ctx)
		if err != nil {
			return nil, nil, err
		}
		var result []callInfo
		for _, c := range calls {
			result = append(result, callInfo{
				CallerID:       c.CallerID,
				CallerCodebase: c.CallerCodebase,
				CalleeID:       c.CalleeID,
				CalleeCodebase: c.CalleeCodebase,
			})
		}
		return nil, &output{Calls: result}, nil
	})
}
