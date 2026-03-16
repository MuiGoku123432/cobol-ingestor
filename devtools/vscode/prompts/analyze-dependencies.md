---
description: Analyze dependencies and impact radius for a COBOL program
mode: agent
tools: ["cobol-graph"]
---

# Analyze COBOL Program Dependencies

Perform a comprehensive dependency and impact analysis for the COBOL program `{programId}`.

## Instructions

Use the cobol-graph MCP tools to trace all dependencies and produce an impact report.

### 1. Program Overview

- Use `get_program` to retrieve program metadata (complexity, domain, LOC, risk level)

### 2. Trace Call Chains

- Use `get_call_chain` with direction "up" to find all upstream callers
- Use `get_call_chain` with direction "down" to find all downstream callees
- Use `get_callers` and `get_callees` for direct neighbors

### 3. Run Impact Analysis

- Use `get_impact_analysis` to determine the full blast radius of changes to this program

### 4. Analyze Shared Copybooks

- Use `get_copybook_usage` to list all copybooks included by this program
- Use `get_copybook_structure` for each copybook to understand shared data layouts
- Identify other programs that include the same copybooks

### 5. Trace Cross-Program Data Flow

- Use `get_data_flow` for intra-program data movement
- Use `get_cross_program_data_flow` for data passed between programs via LINKAGE, files, or DB2

### 6. Check Database Dependencies

- Use `get_program_sql` for all embedded SQL statements
- Identify shared tables accessed by multiple programs

### 7. Map External Interfaces

- Use `get_program_external_interfaces` for external system calls
- Use `get_program_cics` for CICS transaction dependencies

### 8. Assess Migration Implications

- Use `get_effort_estimate` for estimated migration effort
- Use `get_migration_sequence` to determine safe ordering relative to dependencies

### 9. Generate Dependency Report

Compile a structured report including:

- Direct and transitive dependency counts
- Shared copybook matrix
- Cross-program data flow map
- Database table usage overlap
- Impact radius summary (programs affected by changes)
- Recommended migration order for this program and its dependencies
