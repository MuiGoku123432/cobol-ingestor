package strategy

import "cobol-ingestor/internal/llm"

// GetToolDefinitions returns the full tool set for strategy agents.
// It extends modernize.GetToolDefinitions() with strategy-specific tools.
func GetToolDefinitions() []llm.ToolDefinition {
	// Start with the base modernize tools — we import them inline to avoid
	// a circular dependency (modernize doesn't depend on strategy).
	tools := baseToolDefinitions()

	// Append strategy-specific tools
	tools = append(tools, strategyTools()...)
	return tools
}

// baseToolDefinitions returns the same tool set as modernize.GetToolDefinitions().
// Duplicated here to avoid importing modernize (which would create coupling).
// These mirror the MCP server's registered tools.
func baseToolDefinitions() []llm.ToolDefinition {
	return []llm.ToolDefinition{
		{
			Name:        "get_program",
			Description: "Get full details of a COBOL program including callers, callees, copybooks, paragraphs, data items, sections, file definitions, and business domains.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"programId": map[string]any{"type": "string", "description": "The COBOL program ID"},
				},
				"required": []any{"programId"},
			},
		},
		{
			Name:        "search_programs",
			Description: "Full-text search across programs and paragraphs. Returns scored results.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{"type": "string", "description": "Search query (supports fuzzy matching)"},
					"limit": map[string]any{"type": "integer", "description": "Max results (default 20)"},
				},
				"required": []any{"query"},
			},
		},
		{
			Name:        "list_programs",
			Description: "List all COBOL programs with pagination. Optionally filter by program ID substring.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"search":   map[string]any{"type": "string", "description": "Filter programs by ID substring"},
					"page":     map[string]any{"type": "integer", "description": "Page number (default 1)"},
					"pageSize": map[string]any{"type": "integer", "description": "Results per page (default 20)"},
				},
			},
		},
		{
			Name:        "get_call_chain",
			Description: "Trace the call chain for a program. Use direction=downstream to see what it calls, direction=upstream to see what calls it.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"programId": map[string]any{"type": "string", "description": "The program ID to trace"},
					"direction": map[string]any{"type": "string", "description": "upstream or downstream (default downstream)"},
					"depth":     map[string]any{"type": "integer", "description": "Traversal depth 1-10 (default 3)"},
				},
				"required": []any{"programId"},
			},
		},
		{
			Name:        "get_impact_analysis",
			Description: "Analyze the blast radius of changing a program: upstream/downstream dependencies, shared copybooks, and shared files.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"programId": map[string]any{"type": "string", "description": "The program ID to analyze"},
				},
				"required": []any{"programId"},
			},
		},
		{
			Name:        "get_copybook_usage",
			Description: "Find which programs include a given copybook.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{"type": "string", "description": "The copybook name"},
				},
				"required": []any{"name"},
			},
		},
		{
			Name:        "get_data_items",
			Description: "List all data items (variables) defined in a program with their levels, FQNs, and PIC clauses.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"programId": map[string]any{"type": "string", "description": "The program ID"},
				},
				"required": []any{"programId"},
			},
		},
		{
			Name:        "list_business_domains",
			Description: "List all business domains with their program counts.",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
		{
			Name:        "get_paragraph_flow",
			Description: "Get PERFORMS and PERFORMS THRU control flow between paragraphs in a program.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"programId": map[string]any{"type": "string", "description": "The program ID"},
				},
				"required": []any{"programId"},
			},
		},
		{
			Name:        "list_modernization_candidates",
			Description: "List programs scored as modernization candidates with recommended approach.",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
		{
			Name:        "get_program_sql",
			Description: "List SQL statements embedded in a COBOL program.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"programId": map[string]any{"type": "string", "description": "The program ID"},
				},
				"required": []any{"programId"},
			},
		},
		{
			Name:        "get_program_cics",
			Description: "List CICS transaction commands used by a program.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"programId": map[string]any{"type": "string", "description": "The program ID"},
				},
				"required": []any{"programId"},
			},
		},
		{
			Name:        "get_data_flow",
			Description: "Get MOVES_TO data flow relationships between data items in a program.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"programId": map[string]any{"type": "string", "description": "The program ID"},
				},
				"required": []any{"programId"},
			},
		},
		{
			Name:        "get_data_hierarchy",
			Description: "Get data item hierarchy and REDEFINES relationships for a program.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"programId": map[string]any{"type": "string", "description": "The program ID"},
				},
				"required": []any{"programId"},
			},
		},
		{
			Name:        "get_dead_paragraphs",
			Description: "List unreachable (dead) paragraphs for a program.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"programId": map[string]any{"type": "string", "description": "The program ID"},
				},
				"required": []any{"programId"},
			},
		},
		{
			Name:        "get_dashboard_stats",
			Description: "Get aggregate statistics: program count, copybook count, paragraph count, data items, relationships, orphans, and domain count.",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
	}
}

// strategyTools returns additional tools specific to the strategy workflow.
func strategyTools() []llm.ToolDefinition {
	return []llm.ToolDefinition{
		{
			Name:        "get_migration_sequence",
			Description: "Get dependency-ordered migration sequence with tier assignments (LEAF programs first, then their callers). Returns programs grouped by migration tier.",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
		{
			Name:        "get_effort_estimates",
			Description: "Get complexity scores and t-shirt sizing (S/M/L/XL) for all programs based on paragraph count, data items, dependencies, and SQL usage.",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
		{
			Name:        "list_risk_programs",
			Description: "List programs with risk scores based on complexity, dependency count, shared copybooks, and criticality.",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
		{
			Name:        "list_bridge_programs",
			Description: "List programs that span multiple business domains (bridge programs) which require special migration coordination.",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
		{
			Name:        "list_copybook_risks",
			Description: "List copybooks shared across many programs, ranked by risk (number of dependent programs). High-sharing copybooks are migration bottlenecks.",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
		{
			Name:        "get_dead_code_summary",
			Description: "Get summary of dead (unreachable) paragraphs across all programs. Returns per-program dead paragraph counts.",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
		{
			Name:        "get_shared_data_channels",
			Description: "Get producer/consumer relationships for shared data (files, DB tables, CICS queues) showing which programs write and which read.",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
		{
			Name:        "get_cross_program_data_flow",
			Description: "Get cross-program data dependencies including LINKAGE SECTION parameter passing between callers and callees.",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
		{
			Name:        "list_db_tables",
			Description: "List all database tables referenced by COBOL programs with access counts (read/write/both).",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
		{
			Name:        "get_table_usage",
			Description: "Get per-table details: which programs access it, what SQL operations they perform, and data items involved.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"tableName": map[string]any{"type": "string", "description": "The database table name"},
				},
				"required": []any{"tableName"},
			},
		},
		{
			Name:        "get_type_mappings",
			Description: "Get COBOL PIC clause to target language type mappings (e.g., PIC 9(5) -> int, PIC X(20) -> String).",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"targetLanguage": map[string]any{"type": "string", "description": "Target language (java, csharp, python, go, typescript, kotlin)"},
				},
				"required": []any{"targetLanguage"},
			},
		},
		{
			Name:        "get_validation_report",
			Description: "Get data quality report: missing relationships, unannotated paragraphs, programs without domains, and other graph gaps.",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
		{
			Name:        "get_business_domain",
			Description: "Get detailed information about a specific business domain including all programs, key data flows, and migration considerations.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"domain": map[string]any{"type": "string", "description": "The business domain name"},
				},
				"required": []any{"domain"},
			},
		},
	}
}
