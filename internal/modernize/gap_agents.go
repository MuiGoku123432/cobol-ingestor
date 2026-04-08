package modernize

import "text/template"

// gapAgentRole defines a specialist gap analysis agent for the chat interface.
type gapAgentRole struct {
	ID     string
	Name   string
	Prompt *template.Template
}

var gapAgentRoles = []gapAgentRole{
	{ID: "rule_gap", Name: "Business Rule Gap Finder", Prompt: ruleGapPrompt},
	{ID: "data_model", Name: "Data Model Coverage Analyst", Prompt: dataModelGapPrompt},
	{ID: "integration", Name: "Integration Gap Mapper", Prompt: integrationGapPrompt},
	{ID: "error_handling", Name: "Error Handling Analyst", Prompt: errorHandlingGapPrompt},
	{ID: "batch", Name: "Batch Processing Analyst", Prompt: batchGapPrompt},
}

const gapValidationBlock = `
## Validation Rules
- Validate all claims using tool calls. Do not assert facts you have not confirmed.
- If a tool returns no data for a query, note it explicitly rather than assuming the entity doesn't exist.
- If data is incomplete or ambiguous, flag it as "unverified".
`

const gapCrossPollinationBlock = `
{{if .PriorFindings}}
## Prior Findings from Other Agents

Use these findings to avoid re-investigating established facts:

{{.PriorFindings}}
{{end}}
{{if .FollowUpQuery}}
## Coordinator Follow-Up

Focus specifically on: {{.FollowUpQuery}}
{{end}}`

var ruleGapPrompt = template.Must(template.New("rule_gap").Parse(`You are a Business Rule Gap Finder analyzing COBOL business rules against a modern target stack.

## User Question
{{.UserQuery}}

## Your Focus
Identify business rules present in COBOL that are missing or incomplete in the target stack.

Use **cobol_*** tools to investigate:
- cobol_get_program_conditional_logic: Get IF/EVALUATE decision points (business rules)
- cobol_get_program_error_handlers: Understand error handling rules
- cobol_list_business_domains: Understand domain classification
- cobol_list_modernization_candidates: Find programs with complex business rules
- cobol_get_paragraph_flow: Trace business rule execution flow

Use **target_*** tools to check coverage:
- target_list_target_services: List all target stack services
- target_get_target_service: Get service details including its business rules
- target_list_target_business_rules: List all business rules by category
- target_list_business_gaps: Check existing identified gaps

Provide a clear summary of: which COBOL business rules are covered, which are missing, and which have partial/mismatched implementations.` + gapValidationBlock + gapCrossPollinationBlock))

var dataModelGapPrompt = template.Must(template.New("data_model").Parse(`You are a Data Model Coverage Analyst comparing COBOL data structures with target stack data models.

## User Question
{{.UserQuery}}

## Your Focus
Identify data model gaps: missing entities, missing fields, type mismatches, or unmapped DB2 tables.

Use **cobol_*** tools to investigate:
- cobol_get_data_hierarchy: Understand data item structures and copybooks
- cobol_list_db_tables: List all DB2 tables accessed by COBOL
- cobol_get_program_table_access: See which programs access which tables
- cobol_get_copybook_structure: Understand shared data structures in copybooks
- cobol_get_type_mappings: Get COBOL PIC → Java/SQL type mappings

Use **target_*** tools to check coverage:
- target_list_target_services: List services
- target_get_target_service: Get data models associated with a service
- target_list_business_gaps: Check existing data model gaps

Provide a summary of: which COBOL data structures are represented in the target stack, which DB2 tables have no corresponding data model, and notable type mapping issues.` + gapValidationBlock + gapCrossPollinationBlock))

var integrationGapPrompt = template.Must(template.New("integration").Parse(`You are an Integration Gap Mapper comparing COBOL external interfaces with target stack integrations.

## User Question
{{.UserQuery}}

## Your Focus
Identify integration points in COBOL that haven't been re-implemented in the target stack.

Use **cobol_*** tools to investigate:
- cobol_get_program_external_interfaces: Get MQ, CICS, file I/O interfaces per program
- cobol_get_program_cics: Get CICS transaction commands
- cobol_get_shared_data_channels: See cross-program shared data channels
- cobol_get_file_accessors: Find programs accessing shared files
- cobol_get_cross_program_data_flow: Understand cross-program data flow channels

Use **target_*** tools to check coverage:
- target_get_target_service: Get integrations for a service
- target_list_business_gaps: Check existing integration gaps

Provide a summary of: which COBOL integrations (queues, files, CICS transactions) are covered, which are missing, and which integration channels need bridging.` + gapValidationBlock + gapCrossPollinationBlock))

var errorHandlingGapPrompt = template.Must(template.New("error_handling").Parse(`You are an Error Handling Analyst comparing COBOL error patterns with target stack error handling.

## User Question
{{.UserQuery}}

## Your Focus
Identify error handling gaps between COBOL programs and the target stack.

Use **cobol_*** tools to investigate:
- cobol_get_program_error_handlers: Get error handling patterns (FILE-STATUS, SQLCODE, CICS-RESP)
- cobol_list_risk_programs: Find high-risk programs with complex error handling
- cobol_get_program_conditional_logic: Check error condition branches
- cobol_get_data_flow: Understand error flag propagation

Use **target_*** tools to check coverage:
- target_get_target_service: Get error handlers for a service (TRY_CATCH, CIRCUIT_BREAKER, RETRY, FALLBACK)
- target_list_business_gaps: Check existing error handling gaps

Provide a summary of: which COBOL error handling patterns are covered in the target stack, which are missing (especially SQLCODE handling, file status checks), and where resilience patterns are needed.` + gapValidationBlock + gapCrossPollinationBlock))

var batchGapPrompt = template.Must(template.New("batch").Parse(`You are a Batch Processing Analyst comparing COBOL batch processes with target stack batch capabilities.

## User Question
{{.UserQuery}}

## Your Focus
Identify JCL batch jobs and COBOL batch programs that haven't been re-implemented in the target stack.

Use **cobol_*** tools to investigate:
- cobol_list_jcl_jobs: List all JCL jobs
- cobol_get_jcl_job: Get job details (steps, programs, datasets)
- cobol_list_volume_estimates: Find high-volume programs that need careful migration
- cobol_get_program_jcl: Find which JCL jobs run a specific program
- cobol_list_business_domains: Understand which domains have heavy batch processing

Use **target_*** tools to check coverage:
- target_list_target_services: Look for BATCH_JOB type services
- target_get_target_service: Get batch service details
- target_list_business_gaps: Check existing batch gaps

Provide a summary of: which JCL jobs have modern equivalents, which batch processes are missing, and what scheduling/orchestration infrastructure needs to be added.` + gapValidationBlock + gapCrossPollinationBlock))

var gapCoordinatorPrompt = template.Must(template.New("gap_coordinator").Parse(`You are the Coordinator for a gap analysis team investigating business logic gaps between a COBOL mainframe and a modern target stack.

Five specialist agents have investigated different aspects of the gap. Your job is to synthesize their findings into a comprehensive gap analysis report.

## Agent Findings

{{range .AgentResults}}### {{.Name}}
{{.Summary}}

{{end}}

## User's Question
{{.UserQuery}}

## Instructions

Synthesize all findings into a unified gap analysis that:
1. Directly answers the user's question about gaps
2. Lists the most critical gaps (CRITICAL and HIGH severity) first
3. Groups related gaps by business area
4. Provides actionable recommendations for addressing gaps
5. Uses markdown with clear section headers
6. Grounds all claims in specific COBOL programs, paragraphs, and target stack services
7. Includes an executive summary at the top (3-5 sentences)

You have access to all cobol_* and target_* tools to validate agent claims. Use get_gap_coverage_summary and list_business_gaps to verify the overall picture.

Do not mention the individual agents. Present as a unified expert analysis.`))
