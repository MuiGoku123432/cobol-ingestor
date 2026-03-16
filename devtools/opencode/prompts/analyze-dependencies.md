# Analyze Program Dependencies

Perform a comprehensive dependency analysis for a COBOL program, mapping all upstream and downstream relationships.

## Input

- **Program ID**: The COBOL program identifier to analyze

## Agent Instructions

Use the cobol-graph MCP server tools to trace every dependency relationship for the target program and produce a structured dependency report.

### Step 1: Get Program Overview

Call `get_program` with the program ID to retrieve metadata including complexity rating, business domain, LOC count, and current annotations.

### Step 2: Trace Call Chains

Call `get_call_chain` with direction "upstream" to find all programs that call this one. Call `get_call_chain` with direction "downstream" to find all programs this one calls. Note the full transitive closure in both directions.

### Step 3: Assess Impact

Call `get_impact_analysis` to determine the blast radius. Record the number of directly and transitively affected programs.

### Step 4: Analyze Shared Copybooks

Call `get_copybook_usage` to list all copybooks included by this program. Call `list_copybook_risks` to identify copybooks shared across many programs that represent coupling risks.

### Step 5: Trace Cross-Program Data Flow

Call `get_cross_program_data_flow` to map how data moves between this program and its callers/callees through linkage parameters. Call `get_shared_data_channels` to identify shared files, queues, or database tables used for inter-program communication.

### Step 6: Check Database Dependencies

Call `get_program_table_access` to list all DB2 tables read or written by this program. Call `get_table_usage` to find other programs that access the same tables, revealing implicit dependencies.

### Step 7: Map External Interfaces

Call `get_program_external_interfaces` to identify calls to external systems, MQ queues, or service endpoints. Call `get_program_jcl` to find JCL jobs and steps that execute this program, revealing batch scheduling dependencies.

### Step 8: Assess Migration Risk

Call `get_effort_estimates` to get complexity-weighted migration estimates. Call `get_migration_sequence` to understand where this program falls in the recommended migration order and what must be migrated first.

### Step 9: Generate Dependency Report

Compile findings into a structured report with the following sections:

- **Program Summary** -- ID, domain, complexity, LOC
- **Call Graph** -- upstream callers, downstream callees, transitive depth
- **Impact Radius** -- direct and indirect affected programs
- **Shared Copybooks** -- list with risk ratings for highly shared ones
- **Data Flow** -- linkage parameter mappings, shared data channels
- **Database Dependencies** -- tables accessed, other programs sharing those tables
- **External Interfaces** -- MQ, services, files, other external systems
- **JCL References** -- jobs and steps that invoke this program
- **Migration Considerations** -- effort estimate, sequencing constraints, risk factors
