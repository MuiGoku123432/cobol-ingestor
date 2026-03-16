---
description: Report overall COBOL migration status and planning
mode: agent
tools: ["cobol-graph"]
---

# COBOL Migration Status Report

Generate a comprehensive migration status report for the entire COBOL codebase.

## Instructions

Use the cobol-graph MCP tools to gather portfolio-wide metrics and produce an actionable status report.

### 1. Dashboard Overview

- Use `get_dashboard_stats` to get total counts for programs, copybooks, paragraphs, data items, and relationships

### 2. Business Domain Breakdown

- Use `list_business_domains` to get all identified domains
- Use `get_domain_programs` for each domain to understand the functional grouping

### 3. Modernization Candidates

- Use `list_modernization_candidates` to identify programs flagged for migration
- Note complexity levels, LOC, and readiness indicators

### 4. Risk Assessment

- Use `get_risk_assessment` to evaluate migration risk across the portfolio
- Identify high-risk programs with deep dependency chains or high complexity

### 5. Volume Estimates

- Summarize total lines of code, program count, copybook count
- Break down by domain and complexity tier (low, medium, high)

### 6. Effort Estimates

- Use `get_effort_estimate` for key programs to sample migration effort
- Extrapolate portfolio-wide effort based on complexity distribution

### 7. Migration Sequence

- Use `get_migration_sequence` to determine the recommended order of migration
- Identify leaf programs (no downstream dependents) as safe starting points
- Flag programs with high fan-in that should be migrated last

### 8. Dead Code Summary

- Use `get_dead_paragraphs` across programs to quantify unreachable code
- Calculate potential LOC reduction from dead code removal

### 9. Validation Status

- Use `get_validation_report` to check graph completeness and data quality
- Flag programs with missing relationships, unannotated paragraphs, or dangling calls

### 10. Compile Status Report

Produce a structured report with:

- **Executive summary** -- Total portfolio size, domain count, estimated effort range
- **Domain breakdown** -- Programs per domain with complexity distribution
- **Migration waves** -- Recommended sequencing with rationale
- **Risk register** -- High-risk programs and mitigation strategies
- **Quick wins** -- Low-complexity leaf programs suitable for pilot migration
- **Dead code** -- Volume of unreachable code and cleanup recommendations
- **Data quality** -- Graph completeness metrics and outstanding issues
- **Next steps** -- Prioritized action items for the migration team
