package modernize

import "cobol-ingestor/internal/llm"

// GetToolDefinitions returns tool definitions mirroring the MCP server's tools.
// These are passed to the ChatProvider so the LLM can call graph tools.
func GetToolDefinitions() []llm.ToolDefinition {
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
		{
			Name:        "generate_mermaid_diagram",
			Description: "Render a Mermaid diagram to SVG file. Use when the user asks for a visual diagram of call chains, data flows, program architecture, or any structural visualization.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"code":     map[string]any{"type": "string", "description": "Mermaid diagram code (flowchart, sequence, class, ER)"},
					"theme":    map[string]any{"type": "string", "description": "Theme name: github-dark, tokyo-night, nord, catppuccin-mocha, etc."},
					"fileName": map[string]any{"type": "string", "description": "Output file name without extension"},
				},
				"required": []any{"code"},
			},
		},
	}
}
