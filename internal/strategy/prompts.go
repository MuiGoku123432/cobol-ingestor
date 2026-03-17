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
}

const strategyContextBlock = `
## Migration Context

- **Strategy**: {{.Strategy}}{{if .HybridStrategies}} (hybrid: {{range $i, $s := .HybridStrategies}}{{if $i}}, {{end}}{{$s}}{{end}}){{end}}
- **Target Language**: {{.TargetLang}}{{if .TargetFramework}} / {{.TargetFramework}}{{end}}
{{if .TargetPlatform}}- **Target Platform**: {{.TargetPlatform}}{{end}}
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

## Tools to Use
- ` + "`get_dashboard_stats`" + ` — aggregate codebase statistics
- ` + "`get_dead_code_summary`" + ` — dead paragraph counts
- ` + "`list_programs`" + ` — program inventory
- ` + "`list_business_domains`" + ` — domain classification
- ` + "`get_validation_report`" + ` — data quality gaps

Investigate thoroughly, then write a detailed assessment. Cite specific numbers and program names from tool results.`))

var sequencerPrompt = template.Must(template.New("sequencer").Parse(`You are a Migration Sequencer for a COBOL migration strategy analysis. Your job is to produce a dependency-ordered wave plan.
` + strategyContextBlock + `
## Your Task

Produce a migration wave plan:
1. **Dependency analysis**: identify leaf programs (no downstream calls) as Wave 1 candidates
2. **Tier assignments**: group programs into migration waves based on call chain dependencies
3. **Bridge programs**: identify programs spanning multiple domains that need special coordination
4. **Critical path**: identify the longest dependency chain that determines minimum migration duration

## Tools to Use
- ` + "`get_migration_sequence`" + ` — dependency-ordered tiers
- ` + "`list_bridge_programs`" + ` — multi-domain bridge programs
- ` + "`get_call_chain`" + ` — trace specific dependency chains
- ` + "`list_programs`" + ` — full program list

Investigate thoroughly, then write a detailed wave plan. Assign every program to a wave/tier and explain the ordering rationale.`))

var riskPrompt = template.Must(template.New("risk").Parse(`You are a Risk Analyst for a COBOL migration strategy analysis. Your job is to produce a risk matrix with mitigations.
` + strategyContextBlock + `
## Your Task

Produce a comprehensive risk assessment:
1. **Program-level risks**: complexity, dependency count, shared state
2. **Copybook risks**: widely-shared copybooks that are migration bottlenecks
3. **Integration risks**: programs with CICS, SQL, or file I/O that need special handling
4. **Data quality risks**: gaps in the knowledge graph that indicate analysis blind spots
5. **Mitigation strategies**: specific actions to reduce each identified risk

## Tools to Use
- ` + "`list_risk_programs`" + ` — risk-scored programs
- ` + "`list_copybook_risks`" + ` — risky shared copybooks
- ` + "`get_impact_analysis`" + ` — blast radius for high-risk programs
- ` + "`get_validation_report`" + ` — data quality gaps

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

## Tools to Use
- ` + "`get_effort_estimates`" + ` — complexity scores and sizing
- ` + "`list_modernization_candidates`" + ` — scored modernization recommendations
- ` + "`get_dashboard_stats`" + ` — aggregate statistics for capacity planning

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

## Tools to Use
- ` + "`list_db_tables`" + ` — database table inventory
- ` + "`get_table_usage`" + ` — per-table access details
- ` + "`get_cross_program_data_flow`" + ` — cross-program dependencies
- ` + "`get_shared_data_channels`" + ` — producer/consumer data
- ` + "`get_type_mappings`" + ` — COBOL to target type mappings

Investigate thoroughly, then write a detailed data migration and integration plan. Map every table and data channel to a migration approach.`))

var coordinatorSynthesisPrompt = template.Must(template.New("coordinator").Parse(`You are the Strategy Coordinator. Five specialist agents have analyzed different aspects of a COBOL migration. Your job is to synthesize their findings into a comprehensive migration strategy document.

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
