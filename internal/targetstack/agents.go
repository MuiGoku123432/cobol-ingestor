package targetstack

// gapAgentDef defines a specialist gap analysis agent.
type gapAgentDef struct {
	ID      string
	Name    string
	Mission string
}

// gapAgents defines the 5 specialist agents for gap analysis.
var gapAgents = []gapAgentDef{
	{
		ID:   "rule_gap",
		Name: "Business Rule Gap Finder",
		Mission: `Identify business rules and validations present in the COBOL mainframe that are missing or incomplete in the target stack.

Use cobol_* tools to:
- Query paragraph annotations for VALIDATION, CALCULATION, and PROCESSING categories
- Retrieve conditional logic (IF/EVALUATE decision points) from key programs
- Look at business domains to understand what rules should exist
- Check modernization candidates for programs with complex business logic

Use target_* tools to:
- List all target services and their business rules
- Check rule categories (VALIDATION, CALCULATION, AUTHORIZATION, WORKFLOW, TRANSFORMATION)
- Compare coverage against COBOL logic

Focus on: validation rules, calculation formulas, business workflow conditions, authorization checks.
Identify: rules in COBOL with no equivalent in the target stack (COBOL_ONLY), rules in the target stack with no COBOL equivalent (TARGET_ONLY), and partial/mismatched implementations.`,
	},
	{
		ID:   "data_model",
		Name: "Data Model Coverage Analyst",
		Mission: `Identify data model gaps between COBOL data structures (copybooks, working storage, DB2 tables) and target stack data models.

Use cobol_* tools to:
- Get data hierarchy for key programs (CHILD_OF relationships, copybook structures)
- List DB tables accessed by COBOL programs (get_program_table_access)
- Check copybook structures for shared data definitions
- Look at type mappings (COBOL PIC to Java/SQL types)

Use target_* tools to:
- List target data models and their fields
- Check for ORM-mapped table names
- Compare field coverage against COBOL data items

Focus on: missing entities, missing fields in existing entities, type mismatches, unmapped DB2 tables.
Flag critical gaps where COBOL DB2 tables have no corresponding target data model.`,
	},
	{
		ID:   "integration",
		Name: "Integration Gap Mapper",
		Mission: `Identify integration points in the COBOL mainframe that haven't been re-implemented in the target stack.

Use cobol_* tools to:
- List external interfaces (MQ queues, CICS commands, file I/O) per program
- Check CICS transactions and their purpose
- Look at cross-program data flow channels (get_shared_data_channels)
- Query file accessors for shared files between programs

Use target_* tools to:
- List target integrations by type (MESSAGE_QUEUE, FILE_IO, REST_CLIENT, etc.)
- Compare queue names, file names, and API endpoints
- Check which services expose or consume external interfaces

Focus on: MQ queues without consumers, CICS transactions without REST equivalents, shared files without API replacements, database connections without corresponding target integrations.`,
	},
	{
		ID:   "error_handling",
		Name: "Error Handling Analyst",
		Mission: `Compare error handling patterns between COBOL mainframe programs and the target stack.

Use cobol_* tools to:
- Get error handlers per program (get_program_error_handlers)
- Check FILE-STATUS handling, SQLCODE checks, CICS-RESP patterns
- Look at programs with complex error handling (risk programs with error_handling type)
- Review conditional logic for error condition branches

Use target_* tools to:
- List error handlers by service (TRY_CATCH, CIRCUIT_BREAKER, RETRY, FALLBACK patterns)
- Compare coverage of critical error scenarios
- Check if retry/fallback patterns exist for external integrations

Focus on: COBOL programs with sophisticated error handling (DB2 SQLCODE, file status checks) that the target stack handles simplistically, missing circuit breakers for external calls, absent retry logic.`,
	},
	{
		ID:   "batch",
		Name: "Batch Processing Analyst",
		Mission: `Identify COBOL batch processes (JCL jobs) that haven't been re-implemented in the target stack.

Use cobol_* tools to:
- List all JCL jobs and their steps (list_jcl_jobs, get_jcl_job)
- Check which COBOL programs are batch (executionMode=BATCH)
- Review volume estimates for high-volume batch programs
- Look at sequential file processing patterns

Use target_* tools to:
- List services of type BATCH_JOB
- Compare batch coverage by business domain
- Check for scheduling/orchestration integrations (Quartz, Spring Batch, Kafka Streams, etc.)

Focus on: JCL jobs with no target equivalent, batch programs processing critical data with no replacement, missing scheduling/orchestration infrastructure, high-volume batch processes that need careful capacity planning.`,
	},
}
