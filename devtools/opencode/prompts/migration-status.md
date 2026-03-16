# Migration Status Report

Generate a comprehensive migration status report for the entire COBOL portfolio or a specific business domain.

## Input

- **Domain Name** (optional): A specific business domain to report on, or "all" for the full portfolio. Defaults to "all" if omitted.

## Agent Instructions

Use the cobol-graph MCP server tools to gather metrics across the codebase and compile a migration readiness report.

### Step 1: Get Portfolio Overview

Call `get_dashboard_stats` to retrieve high-level counts: total programs, copybooks, data items, relationships, and ingestion status.

### Step 2: List Business Domains

Call `list_business_domains` to get all classified domains with their program counts. If a specific domain was requested, filter subsequent queries to that domain.

### Step 3: Identify Modernization Candidates

Call `list_modernization_candidates` to get programs ranked by modernization priority. Note which programs are flagged as high-priority based on complexity, coupling, and business value.

### Step 4: Assess Risk

Call `list_risk_programs` to find programs with the highest risk scores. These are programs with high complexity, many dependencies, or critical business functions that require careful migration planning.

### Step 5: Estimate Volume

Call `list_volume_estimates` to get LOC counts and program counts by domain. This provides the scale dimension of the migration effort.

### Step 6: Get Effort Estimates

Call `get_effort_estimates` to retrieve complexity-weighted effort projections. Record estimates broken down by domain and complexity tier (low, medium, high, very high).

### Step 7: Determine Migration Sequence

Call `get_migration_sequence` to get the recommended order of migration based on dependency analysis. Programs with fewer dependencies should migrate first; highly depended-upon programs migrate last.

### Step 8: Check Dead Code

Call `get_dead_code_summary` to identify dead paragraphs and unreachable code. Dead code can be excluded from migration scope, reducing effort.

### Step 9: Run Validation

Call `get_validation_report` to check for graph quality issues: missing relationships, dangling calls, unannotated paragraphs, orphan nodes. These issues should be resolved before migration begins.

### Step 10: Compile Status Report

Produce a report with the following sections:

- **Scope** -- Total programs, LOC, copybooks, and domains. Breakdown by domain if reporting on the full portfolio.
- **Readiness** -- Number of programs fully analyzed vs. pending. Graph validation pass rate. Percentage of programs with complete annotations.
- **Risk Profile** -- Count of programs by risk tier (critical, high, medium, low). Top 10 highest-risk programs with their risk factors.
- **Dead Code** -- Total dead paragraphs found. Estimated LOC savings from excluding dead code. Programs with the most dead code.
- **Recommended Sequence** -- Migration waves in dependency order. Programs per wave with their domain and complexity.
- **Effort Summary** -- Total estimated effort by domain. Effort by complexity tier. Comparison of scope with and without dead code.
- **Blockers** -- Validation issues that must be resolved. Programs with unresolved dangling calls. Missing copybook definitions. Any gaps in the knowledge graph that could affect migration accuracy.
