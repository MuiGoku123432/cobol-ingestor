package mcp

// namedQuery describes a curated, parameterized read-only Cypher query.
type namedQuery struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	ParamDocs   map[string]string `json:"paramDocs,omitempty"` // param name → description
	Cypher      string            `json:"-"`
}

// namedQueryRegistry is the catalog of built-in named queries.
// All queries are read-only and safe for LLM invocation.
var namedQueryRegistry = []namedQuery{
	{
		Name:        "top_programs_by_risk",
		Description: "List programs with the highest riskScore, including their call-target count and copybook count. Useful for prioritising modernisation.",
		ParamDocs: map[string]string{
			"codebase": "(optional) Filter to a specific codebase name",
			"limit":    "(optional) Max programs to return (default 20)",
		},
		Cypher: `
MATCH (p:Program)
WHERE ($codebase IS NULL OR p.codebase = $codebase)
OPTIONAL MATCH (p)-[:INCLUDES]->(cb:Copybook)
WITH p, count(cb) AS copybookCount
RETURN p.programId AS programId,
       p.riskScore AS riskScore,
       p.callTargetCount AS callTargetCount,
       p.deadCode AS deadCode,
       copybookCount
ORDER BY riskScore DESC
`,
	},
	{
		Name:        "programs_with_most_copybooks",
		Description: "Programs ranked by the number of copybooks they INCLUDE — good for identifying high-fan-in copybook candidates.",
		ParamDocs: map[string]string{
			"codebase": "(optional) Filter to a specific codebase",
			"limit":    "(optional) Max programs to return (default 20)",
		},
		Cypher: `
MATCH (p:Program)-[:INCLUDES]->(cb:Copybook)
WHERE ($codebase IS NULL OR p.codebase = $codebase)
WITH p, count(DISTINCT cb) AS copybookCount
RETURN p.programId AS programId, copybookCount
ORDER BY copybookCount DESC
`,
	},
	{
		Name:        "copybooks_by_field_count",
		Description: "Copybooks ranked by their DataItem count — reveals the heaviest shared structures.",
		ParamDocs: map[string]string{
			"limit": "(optional) Max copybooks to return (default 20)",
		},
		Cypher: `
MATCH (d:DataItem)-[:DEFINED_IN]->(cb:Copybook)
WITH cb.name AS copybook, count(d) AS fieldCount
RETURN copybook, fieldCount
ORDER BY fieldCount DESC
`,
	},
	{
		Name:        "paragraphs_by_fanin",
		Description: "Paragraphs ranked by how many other paragraphs PERFORM them — the hottest subroutines in the codebase.",
		ParamDocs: map[string]string{
			"codebase": "(optional) Filter to a specific codebase",
			"limit":    "(optional) Max paragraphs to return (default 25)",
		},
		Cypher: `
MATCH (caller:Paragraph)-[:PERFORMS]->(callee:Paragraph)
WHERE ($codebase IS NULL OR callee.codebase = $codebase)
WITH callee.name AS paragraph, callee.programId AS programId, count(DISTINCT caller) AS fanIn
RETURN paragraph, programId, fanIn
ORDER BY fanIn DESC
`,
	},
	{
		Name:        "cross_domain_calls",
		Description: "CALLS edges that cross BusinessDomain boundaries — highlights integration seams and potential decomposition points.",
		ParamDocs: map[string]string{
			"codebase": "(optional) Filter to a specific codebase",
		},
		Cypher: `
MATCH (caller:Program)-[:BELONGS_TO]->(callerDomain:BusinessDomain)
MATCH (callee:Program)-[:BELONGS_TO]->(calleeDomain:BusinessDomain)
MATCH (caller)-[:CALLS]->(callee)
WHERE callerDomain.name <> calleeDomain.name
  AND ($codebase IS NULL OR caller.codebase = $codebase)
RETURN caller.programId AS callerProgram,
       callerDomain.name AS callerDomain,
       callee.programId AS calleeProgram,
       calleeDomain.name AS calleeDomain
ORDER BY callerDomain, callerProgram
`,
	},
	{
		Name:        "programs_touching_table",
		Description: "Programs that READ or WRITE a given SQL/DB2 table via EXECUTES_SQL edges.",
		ParamDocs: map[string]string{
			"table":    "(required) SQL table name (case-insensitive substring match)",
			"codebase": "(optional) Filter to a specific codebase",
		},
		Cypher: `
MATCH (p:Program)-[:EXECUTES_SQL]->(s:SQLStatement)
WHERE toUpper(s.tableName) CONTAINS toUpper($table)
  AND ($codebase IS NULL OR p.codebase = $codebase)
RETURN DISTINCT p.programId AS programId, s.operation AS operation, s.tableName AS tableName
ORDER BY programId
`,
	},
	{
		Name:        "programs_using_copybook",
		Description: "All programs that INCLUDE a given copybook, with their codebase.",
		ParamDocs: map[string]string{
			"copybook": "(required) Copybook name (exact match)",
		},
		Cypher: `
MATCH (p:Program)-[:INCLUDES]->(cb:Copybook {name: $copybook})
RETURN p.programId AS programId, p.codebase AS codebase, p.businessDomain AS businessDomain
ORDER BY programId
`,
	},
	{
		Name:        "call_path_between",
		Description: "Any CALLS path between two programs (shortest found first, capped at maxHops). Useful for dependency tracing.",
		ParamDocs: map[string]string{
			"from":    "(required) Calling program ID",
			"to":      "(required) Called program ID",
			"maxHops": "(optional) Max hops to traverse (default 5)",
		},
		Cypher: `
MATCH path = shortestPath(
  (from:Program {programId: $from})-[:CALLS*1..5]->(to:Program {programId: $to})
)
RETURN [n IN nodes(path) | n.programId] AS callPath,
       length(path) AS hops
`,
	},
	{
		Name:        "dead_paragraphs_by_domain",
		Description: "Dead (unreachable) paragraphs grouped by BusinessDomain — shows which domains carry the most dead code.",
		ParamDocs: map[string]string{
			"codebase": "(optional) Filter to a specific codebase",
		},
		Cypher: `
MATCH (para:Paragraph {isReachable: false})-[:BELONGS_TO]->(prog:Program)-[:BELONGS_TO]->(d:BusinessDomain)
WHERE ($codebase IS NULL OR prog.codebase = $codebase)
WITH d.name AS domain, count(para) AS deadParagraphCount, collect(para.name)[..5] AS examples
RETURN domain, deadParagraphCount, examples
ORDER BY deadParagraphCount DESC
`,
	},
	{
		Name:        "jcl_programs_without_jcl",
		Description: "Programs with no JCL RUNS relationship — may be dead-code candidates or library-only programs.",
		ParamDocs: map[string]string{
			"codebase": "(optional) Filter to a specific codebase",
		},
		Cypher: `
MATCH (p:Program)
WHERE ($codebase IS NULL OR p.codebase = $codebase)
  AND NOT (p)<-[:RUNS]-(:JCLStep)
RETURN p.programId AS programId, p.codebase AS codebase
ORDER BY programId
`,
	},
	{
		Name:        "sql_statement_search",
		Description: "Full-text search across all SQLStatement nodes. Use APOC regex or CONTAINS to find specific table names, column references, or SQL patterns.",
		ParamDocs: map[string]string{
			"pattern": "(required) Case-insensitive substring to search in the SQL text",
			"limit":   "(optional) Max results (default 50)",
		},
		Cypher: `
MATCH (s:SQLStatement)
WHERE toUpper(s.text) CONTAINS toUpper($pattern)
RETURN s.programId AS programId, s.operation AS operation, s.tableName AS tableName, s.text AS sqlText
ORDER BY programId
`,
	},
	{
		Name:        "linkage_coverage",
		Description: "For a given program, list its LINKAGE parameters and whether each has a LINKAGE_MAPS_TO edge (matched) or not.",
		ParamDocs: map[string]string{
			"programId": "(required) The COBOL program ID",
		},
		Cypher: `
MATCH (p:Program {programId: $programId})-[:CALLS]->(callee:Program)
OPTIONAL MATCH (di:DataItem {programId: callee.programId})-[:LINKAGE_MAPS_TO]->(mapped)
WHERE di.level = 1
RETURN callee.programId AS calleeId,
       di.name AS linkageField,
       di.picture AS picture,
       mapped IS NOT NULL AS isMapped
ORDER BY calleeId, linkageField
`,
	},
	{
		Name:        "redefines_chains",
		Description: "DataItem REDEFINES relationships, optionally filtered to a program. Shows what each field redefines.",
		ParamDocs: map[string]string{
			"programId": "(optional) Filter to a specific program ID",
		},
		Cypher: `
MATCH (di:DataItem)-[:REDEFINES]->(target:DataItem)
WHERE ($programId IS NULL OR di.programId = $programId)
RETURN di.programId AS programId,
       di.name AS redefiningField,
       di.picture AS redefiningPic,
       target.name AS targetField,
       target.picture AS targetPic
ORDER BY programId, redefiningField
`,
	},
	{
		Name:        "external_db_fanout",
		Description: "ExternalDatabase nodes and the programs mapped to them via MAPS_TO_EXT_DB, optionally filtered by database name.",
		ParamDocs: map[string]string{
			"database": "(optional) Filter by ExternalDatabase name substring",
		},
		Cypher: `
MATCH (p:Program)-[:MAPS_TO_EXT_DB]->(ext:ExternalDatabase)
WHERE ($database IS NULL OR toUpper(ext.name) CONTAINS toUpper($database))
RETURN ext.name AS externalDb, ext.type AS dbType,
       collect(DISTINCT p.programId) AS programs,
       count(DISTINCT p) AS programCount
ORDER BY programCount DESC
`,
	},
}
