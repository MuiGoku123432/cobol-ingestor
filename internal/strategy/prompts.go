package strategy

import (
	"bytes"
	"text/template"
)

// strategyPromptData holds template data for strategy agent prompts.
type strategyPromptData struct {
	Strategy          string
	HybridStrategies  []string
	TargetLang        string
	TargetFramework   string
	TargetPlatform    string
	Timeline          string
	TeamSize          int
	TeamSkills        []string
	Compliance        []string
	Integrations      string
	PriorityCriteria  []string
	PhasingApproach   string
	Criticality       string
	DowntimeTolerance string
	AdditionalNotes   string

	// Current Stack
	CurrentDatabase      []string
	CurrentDatabaseOther string
	CurrentMiddleware    []string
	CurrentMiddlewareOther string
	CurrentBatch         string
	CurrentMonitoring    string

	// Target Stack (expanded)
	TargetDatabase       string
	TargetMessaging      string
	TargetAPIStyle       string
	TargetContainerization string
	TargetCICD           string

	// Coordinator fields
	AgentResults []agentResult
}

// agentResult mirrors the swarm pattern for template rendering.
type agentResult struct {
	Name    string
	Summary string
}

// strategyAgent defines a strategy analysis agent.
type strategyAgent struct {
	ID     string
	Name   string
	Prompt *template.Template
}

var strategyAgents = []strategyAgent{
	{ID: "assessor", Name: "Codebase Assessor", Prompt: assessorPrompt},
	{ID: "sequencer", Name: "Migration Sequencer", Prompt: sequencerPrompt},
	{ID: "risk", Name: "Risk Analyst", Prompt: riskPrompt},
	{ID: "effort", Name: "Effort Estimator", Prompt: effortPrompt},
	{ID: "data", Name: "Data & Integration Mapper", Prompt: dataPrompt},
	{ID: "architect", Name: "Architecture Designer", Prompt: architectPrompt},
}

const strategyContextBlock = `
## Migration Context

- **Strategy**: {{.Strategy}}{{if .HybridStrategies}} (hybrid: {{range $i, $s := .HybridStrategies}}{{if $i}}, {{end}}{{$s}}{{end}}){{end}}
- **Timeline**: {{.Timeline}}
{{if gt .TeamSize 0}}- **Team Size**: {{.TeamSize}} developers{{end}}
{{if .TeamSkills}}- **Team Skills**: {{range $i, $s := .TeamSkills}}{{if $i}}, {{end}}{{$s}}{{end}}{{end}}
{{if .Compliance}}- **Compliance**: {{range $i, $c := .Compliance}}{{if $i}}, {{end}}{{$c}}{{end}}{{end}}
{{if .Integrations}}- **Integrations**: {{.Integrations}}{{end}}
{{if .PriorityCriteria}}- **Priority Criteria**: {{range $i, $p := .PriorityCriteria}}{{if $i}}, {{end}}{{$p}}{{end}}{{end}}
{{if .PhasingApproach}}- **Phasing**: {{.PhasingApproach}}{{end}}
{{if .Criticality}}- **Criticality**: {{.Criticality}}{{end}}
{{if .DowntimeTolerance}}- **Downtime Tolerance**: {{.DowntimeTolerance}}{{end}}
{{if .AdditionalNotes}}- **Additional Notes**: {{.AdditionalNotes}}{{end}}

### Current Stack
{{if .CurrentDatabase}}- **Databases**: {{range $i, $d := .CurrentDatabase}}{{if $i}}, {{end}}{{$d}}{{end}}{{if .CurrentDatabaseOther}} ({{.CurrentDatabaseOther}}){{end}}{{end}}
{{if .CurrentMiddleware}}- **Middleware**: {{range $i, $m := .CurrentMiddleware}}{{if $i}}, {{end}}{{$m}}{{end}}{{if .CurrentMiddlewareOther}} ({{.CurrentMiddlewareOther}}){{end}}{{end}}
{{if .CurrentBatch}}- **Batch Scheduling**: {{.CurrentBatch}}{{end}}
{{if .CurrentMonitoring}}- **Monitoring**: {{.CurrentMonitoring}}{{end}}

### Target Stack
- **Language**: {{.TargetLang}}{{if .TargetFramework}} / {{.TargetFramework}}{{end}}
{{if .TargetPlatform}}- **Platform**: {{.TargetPlatform}}{{end}}
{{if .TargetDatabase}}- **Database**: {{.TargetDatabase}}{{end}}
{{if .TargetMessaging}}- **Messaging**: {{.TargetMessaging}}{{end}}
{{if .TargetAPIStyle}}- **API Style**: {{.TargetAPIStyle}}{{end}}
{{if .TargetContainerization}}- **Containerization**: {{.TargetContainerization}}{{end}}
{{if .TargetCICD}}- **CI/CD**: {{.TargetCICD}}{{end}}
`

var assessorPrompt = template.Must(template.New("assessor").Parse(`You are a Codebase Assessor for a COBOL migration strategy analysis. Your job is to produce a comprehensive profile of the COBOL codebase.
` + strategyContextBlock + `
## Your Task

Investigate the codebase and produce a detailed assessment covering:
1. **Codebase size**: total programs, copybooks, paragraphs, data items
2. **Complexity distribution**: simple vs. complex programs, average paragraph count
3. **Dead code volume**: unreachable paragraphs that can be eliminated
4. **Business domain coverage**: how programs map to business domains
5. **Overall health**: data quality gaps, unannotated programs
6. **External integration footprint**: DB access patterns, external interface density
7. **Batch landscape**: JCL job inventory and scheduling complexity

## Tools to Use

**Codebase Profiling:**
- ` + "`get_dashboard_stats`" + ` — aggregate codebase statistics (programs, copybooks, paragraphs, data items, relationships)
- ` + "`list_programs`" + ` — program inventory with pagination
- ` + "`list_copybooks`" + ` — copybook inventory and usage counts

**Domain & Business Analysis:**
- ` + "`list_business_domains`" + ` — domain classification with program counts
- ` + "`get_business_domain`" + ` — drill into specific domains for member programs and key data flows

**Code Quality & Gaps:**
- ` + "`get_dead_code_summary`" + ` — dead paragraph counts across all programs
- ` + "`get_validation_report`" + ` — data quality gaps, unannotated programs, missing relationships
- ` + "`get_gap_analysis`" + ` — external DB coverage gaps

**Infrastructure & Integration:**
- ` + "`list_jcl_jobs`" + ` — JCL job inventory to understand batch landscape
- ` + "`get_program_external_interfaces`" + ` — external integration points per program
- ` + "`list_volume_estimates`" + ` — transaction volume to gauge system load
- ` + "`get_program_table_access`" + ` — DB access patterns per program
- ` + "`list_db_tables`" + ` — database table landscape

Investigate thoroughly, then write a detailed assessment. Cite specific numbers and program names from tool results.`))

var sequencerPrompt = template.Must(template.New("sequencer").Parse(`You are a Migration Sequencer for a COBOL migration strategy analysis. Your job is to produce a dependency-ordered wave plan.
` + strategyContextBlock + `
## Your Task

Produce a migration wave plan:
1. **Dependency analysis**: identify leaf programs (no downstream calls) as Wave 1 candidates
2. **Tier assignments**: group programs into migration waves based on call chain dependencies
3. **Bridge programs**: identify programs spanning multiple domains that need special coordination
4. **Critical path**: identify the longest dependency chain that determines minimum migration duration
5. **Data dependency ordering**: ensure programs sharing data channels are sequenced correctly
6. **Batch job ordering**: factor in JCL job dependencies for batch program sequencing

## Tools to Use

**Migration Ordering:**
- ` + "`get_migration_sequence`" + ` — dependency-ordered tiers with tier assignments
- ` + "`list_bridge_programs`" + ` — multi-domain bridge programs requiring coordination
- ` + "`get_call_chain`" + ` — trace specific dependency chains (upstream/downstream)
- ` + "`list_programs`" + ` — full program list with metadata

**Data Dependencies:**
- ` + "`get_cross_program_data_flow`" + ` — data dependencies between programs (LINKAGE, shared files, DB)
- ` + "`get_shared_data_channels`" + ` — producer/consumer file/table relationships
- ` + "`get_copybook_usage`" + ` — shared copybook coupling between programs
- ` + "`get_file_accessors`" + ` — programs sharing file access (must migrate together or with bridges)
- ` + "`get_data_flow_paths`" + ` — end-to-end COBOL->DB2->external DB flows

**Batch & Integration:**
- ` + "`get_program_jcl`" + ` — which JCL jobs invoke a program (batch ordering constraints)
- ` + "`get_program_parameters`" + ` — LINKAGE parameter interfaces between caller/callee
- ` + "`trace_field_impact`" + ` — downstream field impact across programs

Investigate thoroughly, then write a detailed wave plan. Assign every program to a wave/tier and explain the ordering rationale.`))

var riskPrompt = template.Must(template.New("risk").Parse(`You are a Risk Analyst for a COBOL migration strategy analysis. Your job is to produce a risk matrix with mitigations.
` + strategyContextBlock + `
## Your Task

Produce a comprehensive risk assessment:
1. **Program-level risks**: complexity, dependency count, shared state
2. **Copybook risks**: widely-shared copybooks that are migration bottlenecks
3. **Integration risks**: programs with CICS, SQL, or file I/O that need special handling
4. **Data quality risks**: gaps in the knowledge graph that indicate analysis blind spots
5. **Data coupling risks**: cross-program data dependencies that create migration fragility
6. **Batch dependency risks**: JCL job chains that must be migrated atomically
7. **Mitigation strategies**: specific actions to reduce each identified risk

## Tools to Use

**Risk Scoring:**
- ` + "`list_risk_programs`" + ` — risk-scored programs by complexity/dependencies
- ` + "`list_copybook_risks`" + ` — risky shared copybooks (migration bottlenecks)
- ` + "`get_impact_analysis`" + ` — blast radius for high-risk programs (upstream/downstream/shared)

**Integration Complexity:**
- ` + "`get_program_external_interfaces`" + ` — external integration point count/complexity
- ` + "`get_program_error_handlers`" + ` — error handling coverage gaps
- ` + "`get_program_conditions`" + ` — 88-level condition density (business rule complexity)
- ` + "`get_program_sql`" + ` — SQL complexity per program
- ` + "`get_program_cics`" + ` — CICS transaction complexity per program

**Data & Dependency Risk:**
- ` + "`get_cross_program_data_flow`" + ` — data coupling risk between programs
- ` + "`get_shared_data_channels`" + ` — shared data contention risk (producer/consumer)
- ` + "`list_jcl_jobs`" + ` — batch job dependency risk

**Quality & Gaps:**
- ` + "`get_validation_report`" + ` — data quality gaps and analysis blind spots
- ` + "`get_gap_analysis`" + ` — external DB mapping gaps as migration risk

Investigate thoroughly, then write a risk matrix. Rate risks as High/Medium/Low and provide concrete mitigations.`))

var effortPrompt = template.Must(template.New("effort").Parse(`You are an Effort Estimator for a COBOL migration strategy analysis. Your job is to produce per-program effort estimates.
` + strategyContextBlock + `
## Your Task

Produce effort estimates calibrated to the team and timeline:
1. **Per-program sizing**: t-shirt sizes (S/M/L/XL) based on complexity
2. **Aggregate effort**: total story points or person-months by wave
3. **Team capacity**: whether the timeline is feasible given team size and skills
4. **Modernization approach per program**: which programs to rewrite, refactor, wrap, or retire
5. **Quick wins**: low-effort, high-value programs to migrate first
6. **Integration effort**: additional effort for DB, CICS, batch, and external interface migration
7. **Data migration effort**: effort for DB access pattern conversion and data migration tooling

## Tools to Use

**Complexity & Sizing:**
- ` + "`get_effort_estimates`" + ` — complexity scores and t-shirt sizing (S/M/L/XL)
- ` + "`list_modernization_candidates`" + ` — scored modernization recommendations per program
- ` + "`get_dashboard_stats`" + ` — aggregate statistics for capacity planning

**Integration Effort Drivers:**
- ` + "`get_program_external_interfaces`" + ` — integration points increase effort per program
- ` + "`get_program_sql`" + ` — SQL statement count drives data layer effort
- ` + "`get_program_cics`" + ` — CICS command count drives UI/API layer effort
- ` + "`get_program_parameters`" + ` — LINKAGE interfaces need API contract design effort

**Batch & Infrastructure Effort:**
- ` + "`list_jcl_jobs`" + ` — batch job count for batch modernization effort
- ` + "`get_program_jcl`" + ` — per-program JCL job associations
- ` + "`list_volume_estimates`" + ` — transaction volume affects performance testing effort

**Data Migration Effort:**
- ` + "`get_program_table_access`" + ` — DB access patterns drive data migration effort
- ` + "`get_gap_analysis`" + ` — external DB gaps add additional mapping effort

Investigate thoroughly, then write detailed effort estimates. Include both per-program and aggregate numbers.`))

var dataPrompt = template.Must(template.New("data").Parse(`You are a Data & Integration Mapper for a COBOL migration strategy analysis. Your job is to produce a data migration plan and integration mapping.
` + strategyContextBlock + `
## Your Task

Produce a comprehensive data and integration plan:
1. **Database tables**: all tables with access patterns (read/write/both) per program
2. **Cross-program data flow**: how data moves between programs via LINKAGE, files, and DB
3. **Shared data channels**: producer/consumer relationships
4. **Type mappings**: COBOL PIC clauses to target language types
5. **Integration points**: external systems, CICS transactions, file I/O to preserve
6. **External DB mapping**: how COBOL DB2 tables map to external/target database tables
7. **Data lineage**: end-to-end data flow from source through transformations to output
8. **Field-level impact**: downstream impact of field changes across programs

## Tools to Use

**Database & Table Analysis:**
- ` + "`list_db_tables`" + ` — database table inventory with access counts
- ` + "`get_table_usage`" + ` — per-table access details (programs, SQL operations, data items)
- ` + "`get_program_table_access`" + ` — per-program DB access patterns
- ` + "`get_program_sql`" + ` — SQL statements for data operation analysis

**Cross-Program Data Flow:**
- ` + "`get_cross_program_data_flow`" + ` — cross-program dependencies (LINKAGE, shared files, DB)
- ` + "`get_shared_data_channels`" + ` — producer/consumer data relationships
- ` + "`get_program_parameters`" + ` — LINKAGE parameter mapping between caller/callee
- ` + "`get_file_accessors`" + ` — file access patterns (programs sharing files)

**Data Lineage & Impact:**
- ` + "`trace_field_impact`" + ` — field-level downstream impact across programs
- ` + "`get_data_flow_paths`" + ` — end-to-end COBOL->DB2->external DB flows
- ` + "`get_copybook_structure`" + ` — shared data layout definitions

**External DB Integration:**
- ` + "`list_external_db_tables`" + ` — external DB table inventory
- ` + "`get_external_db_mapping`" + ` — COBOL DB2 -> external DB table mappings
- ` + "`get_cobol_to_external_mappings`" + ` — reverse mapping (COBOL perspective)
- ` + "`get_gap_analysis`" + ` — tables/columns with no external mapping

**Type Mappings & Integration:**
- ` + "`get_type_mappings`" + ` — COBOL PIC clause to target type mappings
- ` + "`get_program_external_interfaces`" + ` — full external integration inventory

Investigate thoroughly, then write a detailed data migration and integration plan. Map every table and data channel to a migration approach.`))

var architectPrompt = template.Must(template.New("architect").Parse(`You are an Architecture Designer for a COBOL migration strategy analysis. Your job is to design a detailed transition architecture tailored to the specific migration strategy and technology stack.
` + strategyContextBlock + `
## Your Task

Design a comprehensive transition architecture based on the migration strategy and tech stack. Your architecture must be specific to the chosen strategy and technologies — not generic.

{{if eq .Strategy "rewrite"}}
### Strategy-Specific Guidance: Clean-Room Rewrite
Design a clean target architecture. Map:
- COBOL programs -> microservices or modular monolith components
- VSAM files -> target database tables with appropriate indexes
- CICS transactions -> REST/gRPC API endpoints
- Batch JCL jobs -> scheduled jobs, workflows, or event-driven pipelines
- Copybook data structures -> target language DTOs/models
Focus on clean domain boundaries and modern patterns (DDD, hexagonal architecture).
Design a data access layer: repository pattern, data mappers, ORM configuration for each service/module. Define connection pooling, transaction management, and query patterns for the target stack.
{{else if eq .Strategy "replatform"}}
### Strategy-Specific Guidance: Replatform
Design cloud-native packaging for compiled COBOL:
- Container packaging strategy for COBOL runtimes
- Cloud database migration (DB2 -> target DB) with specific tooling (AWS DMS, Azure DMS, pgLoader)
- MQ Series -> cloud messaging replacement
- JCL -> cloud-native batch orchestration
- Monitoring and observability integration
- Connection string management and secrets rotation for migrated databases
- DB migration tooling: schema conversion, data validation, cutover strategy
{{else if eq .Strategy "integrate"}}
### Strategy-Specific Guidance: API Wrapping
Design integration layer architecture:
- CICS transaction -> REST API wrapper design
- MQ bridge adapters for async integration
- Screen scraper replacement with API contracts
- API gateway configuration and routing
- Authentication/authorization layer
- Data bridge patterns: read-through cache, write-behind, sync adapters for data operations
- Anti-corruption layer for data format translation between legacy and modern consumers
{{else if eq .Strategy "hybrid"}}
### Strategy-Specific Guidance: Hybrid
Design per-domain architecture decisions:
- Which domains get rewritten vs. wrapped vs. replatformed
- Inter-domain communication patterns during transition
- Shared data access patterns across strategy boundaries
- Gradual migration path per domain
- Per-domain data strategy: which domains keep existing DB, which migrate, which need dual-write during transition
- Domain-specific data access layer design based on chosen strategy per domain
{{else}}
### Strategy-Specific Guidance: Incremental / Strangler Fig
Design the strangler fig transition:
- Anti-corruption layer (ACL) between legacy and modern
- API gateway routing rules (legacy vs. modern endpoints)
- Dual-write patterns for data consistency during transition, including data sync strategy between legacy DB2 and target DB
- Feature toggle strategy for gradual traffic shifting
- Which programs to strangle first (leaf programs with low coupling)
- ACL for data operations: how legacy and modern services share data during the transition period
- Data synchronization strategy: CDC, ETL, or event-driven sync between legacy and target datastores
{{end}}

## Required Output

1. **Component Diagram**: A text-based Mermaid diagram showing source -> transition -> target architecture with all major components
2. **Technology Mapping Table**: Map each COBOL technology to its specific target replacement:
   - CICS SEND MAP -> target UI/API pattern
   - EXEC SQL -> target data access pattern
   - VSAM KSDS/RRDS/ESDS -> target storage with index strategy
   - COPY/INCLUDE -> target module/import pattern
   - CALL/PERFORM -> target service invocation pattern
   - LINKAGE SECTION -> target API contract pattern
3. **Data Migration Architecture**: Source DB -> ETL/CDC pipeline -> target DB with specific tool recommendations
4. **API Contract Design**: How each COBOL entry point becomes a target endpoint (with example signatures)
5. **Infrastructure Layout**: Container/deployment architecture with networking, service mesh, and observability
6. **Data Operations Integration Architecture**: Map how every data operation gets modernized:
   - **DB2 SQL -> target data access**: Map SQL patterns (SELECT/INSERT/UPDATE/DELETE with JOINs, cursors, host variables) to target ORM/query patterns with framework-specific examples
   - **VSAM file I/O -> target storage**: Map KSDS (keyed), RRDS (relative), ESDS (sequential) access patterns to target equivalents (indexed tables, key-value stores, append-only logs)
   - **CICS data operations -> target service layer**: Map CICS READ/WRITE/REWRITE/DELETE FILE, READQ/WRITEQ TS/TD to target patterns (cache operations, queue consumers/producers, service calls)
   - **Batch JCL data processing -> target batch**: Map JCL SORT/MERGE, multi-step data transformations to modern orchestration (Airflow, Step Functions, Spring Batch)
   - **Cross-program data flow -> service contracts**: Map LINKAGE SECTION parameter passing to API contracts, event schemas, or shared DTOs
   - **Shared files/queues -> messaging**: Map MQ Series, CICS TS/TD queues to target messaging with specific topic/queue design
   - **External DB integration**: Use gap analysis data to map COBOL DB2 tables -> external DB, identify CDC/ETL/dual-write sync patterns during transition
   - **Data lineage preservation**: Use ` + "`trace_field_impact`" + ` and ` + "`get_data_flow_paths`" + ` to ensure no data flow is lost in transition

## Tools to Use

**Program & Domain Analysis:**
- ` + "`get_program`" + ` — inspect specific program structure (callers, callees, copybooks, paragraphs)
- ` + "`list_business_domains`" + ` — domain classification for service boundary design
- ` + "`get_migration_sequence`" + ` — migration ordering for architecture phasing

**Data Operations:**
- ` + "`get_program_sql`" + ` — SQL statements in programs for data access layer design
- ` + "`get_program_cics`" + ` — CICS transactions in programs for service layer design
- ` + "`list_db_tables`" + ` — database table inventory
- ` + "`get_program_table_access`" + ` — per-program DB access operations
- ` + "`get_data_items`" + ` — data item definitions for schema design
- ` + "`get_type_mappings`" + ` — COBOL to target type mappings

**Cross-Program & Data Flow:**
- ` + "`get_program_parameters`" + ` — LINKAGE for API contract design
- ` + "`get_program_external_interfaces`" + ` — all external integrations for architecture mapping
- ` + "`get_shared_data_channels`" + ` — data channel architecture (producer/consumer)
- ` + "`get_cross_program_data_flow`" + ` — inter-service data flow design
- ` + "`get_file_accessors`" + ` — file I/O patterns to modernize
- ` + "`get_copybook_structure`" + ` — data structure layouts for schema design
- ` + "`trace_field_impact`" + ` — field tracing for data lineage preservation
- ` + "`get_data_flow_paths`" + ` — end-to-end data flow for pipeline design

**Batch & Infrastructure:**
- ` + "`get_program_jcl`" + ` — JCL job associations per program
- ` + "`list_jcl_jobs`" + ` — JCL job inventory for batch architecture
- ` + "`get_jcl_job`" + ` — JCL job details for batch mapping
- ` + "`list_volume_estimates`" + ` — capacity planning for infrastructure sizing

**External DB Integration:**
- ` + "`list_external_db_tables`" + ` — external DB table inventory
- ` + "`get_external_db_mapping`" + ` — COBOL DB2 -> external DB table mappings
- ` + "`get_cobol_to_external_mappings`" + ` — reverse mapping for migration planning
- ` + "`get_gap_analysis`" + ` — tables/columns with no external mapping

Investigate the codebase thoroughly, then produce a detailed, technology-specific transition architecture. Every recommendation must reference the specific source and target technologies from the migration context.`))

var coordinatorSynthesisPrompt = template.Must(template.New("coordinator").Parse(`You are the Strategy Coordinator. Six specialist agents have analyzed different aspects of a COBOL migration. Your job is to synthesize their findings into a comprehensive migration strategy document.

You have access to the same graph database tools the agents used. If you need to verify a claim or fill a gap, use the tools directly.
` + strategyContextBlock + `
## Agent Findings

{{range .AgentResults}}### {{.Name}}
{{.Summary}}

{{end}}

## Output Format

Structure your output with section markers so it can be split into separate documents. Use exactly these markers:

<!-- SECTION: executive-summary -->
# Executive Summary
[2-3 paragraph executive summary of the migration strategy, key findings, and recommendations]

<!-- SECTION: strategy-overview -->
# Strategy Overview
[Detailed rationale for the chosen strategy, how it maps to the codebase, and key architectural decisions]

<!-- SECTION: transition-architecture -->
# Transition Architecture
[Synthesize the Architecture Designer's output into a complete transition architecture:
- Component diagram (Mermaid) showing source -> transition -> target architecture
- Technology mapping table: each COBOL technology mapped to its specific target replacement, **including data operation mappings** (DB2 SQL -> target ORM/query patterns, VSAM -> target storage, CICS data ops -> target service patterns, JCL batch -> target orchestration)
- Migration pattern description (strangler fig, clean-room, API wrapping, etc.) with implementation details
- Data migration pipeline: source DB -> ETL/CDC -> target DB with tool recommendations, incorporating COBOL DB2 -> external DB flow paths from gap analysis
- Data operations integration: how each type of data operation (SQL queries, file I/O, CICS transactions, batch processing) transitions to the target stack with specific pattern mappings
- Data lineage diagram: field-level flow preservation showing how data flows are maintained through the transition
- Infrastructure layout: containers, orchestration, networking, observability]

<!-- SECTION: codebase-assessment -->
# Codebase Assessment
[Full codebase profile: size, complexity, domains, dead code, health metrics]

<!-- SECTION: wave-plan -->
# Wave Plan
[Dependency-ordered migration waves with program assignments, rationale, and dependencies between waves]

<!-- SECTION: risk-assessment -->
# Risk Assessment
[Risk matrix with severity ratings and mitigation strategies]

<!-- SECTION: effort-estimates -->
# Effort Estimates
[Per-program and aggregate effort estimates, team capacity analysis, timeline feasibility]

<!-- SECTION: data-migration -->
# Data Migration
[Database tables, data flow mapping, type conversions, data migration sequence]

<!-- SECTION: integration-strategy -->
# Integration Strategy
[External system preservation, API wrapping, CICS migration, file I/O modernization]

<!-- SECTION: testing-strategy -->
# Testing Strategy
[Testing approach for each wave: unit, integration, regression, parallel running, data validation]

<!-- SECTION: timeline-roadmap -->
# Timeline & Roadmap
[Month-by-month or quarter-by-quarter roadmap with milestones, gates, and decision points]

## Instructions

1. Synthesize all agent findings — do not simply concatenate them
2. Resolve any contradictions between agents
3. Ground all claims in specific COBOL artifact names from the agent findings
4. Make concrete recommendations, not generic advice
5. Calibrate all estimates to the team size, timeline, and constraints provided
6. Use markdown tables where appropriate (wave plans, risk matrices, effort estimates)
7. Do not mention the individual agents or that this was a multi-agent analysis`))

func buildStrategyPrompt(tmpl *template.Template, data strategyPromptData) (string, error) {
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}
