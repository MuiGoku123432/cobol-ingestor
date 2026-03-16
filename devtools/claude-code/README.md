# Cobol-Graph MCP Server for Claude Code

## Overview

The cobol-graph MCP server exposes **55 tools** that let Claude Code query a COBOL knowledge graph stored in Neo4j. Once connected, Claude can explore program structures, trace call chains, analyze data flows, inspect copybooks, review JCL jobs, identify dead code, and plan modernization strategies -- all by conversing in natural language.

Tool categories:

- **Program Discovery** -- search and inspect COBOL programs
- **Call Chain and Impact Analysis** -- trace dependencies and blast radius
- **Data Items and Data Flow** -- field-level lineage and cross-program flow
- **Copybooks** -- usage, structure, and risk analysis
- **Business Domains** -- domain classification and reassignment
- **Program Details** -- conditions, parameters, error handlers, paragraph flow
- **SQL / CICS / IDMS** -- embedded database and transaction analysis
- **JCL** -- job and dataset exploration
- **Database** -- table usage and access patterns
- **External DB Mappings** -- COBOL-to-target-schema mapping and gap analysis
- **Dead Code Detection** -- unused paragraphs and summary reports
- **Modernization Planning** -- candidates, risk, effort estimates, migration sequencing
- **Reporting and Validation** -- dashboard statistics and validation reports

## Prerequisites

| Requirement | Notes |
|---|---|
| Go 1.22+ | Needed to build the MCP binary |
| Neo4j 5.x | Must be running and accessible via Bolt protocol |
| Ingested COBOL codebase | Run the ingestion pipeline first (`./bin/cobol-graph ingest --dir=<path>`) |

## Build

From the repository root:

```bash
go build -o bin/cobol-graph-mcp ./cmd/mcp
```

This produces the `bin/cobol-graph-mcp` binary that speaks MCP over stdio.

## Claude Code MCP Configuration

### Project-level configuration (`.mcp.json`)

Create a `.mcp.json` file in the project root:

```json
{
  "mcpServers": {
    "cobol-graph": {
      "command": "./bin/cobol-graph-mcp",
      "env": {
        "NEO4J_URI": "bolt://localhost:7687",
        "NEO4J_USER": "neo4j",
        "NEO4J_PASSWORD": "your-password"
      }
    }
  }
}
```

### Global configuration (`~/.claude/settings.json`)

To make the server available across all projects, add the same structure to your global settings:

```json
{
  "mcpServers": {
    "cobol-graph": {
      "command": "/absolute/path/to/bin/cobol-graph-mcp",
      "env": {
        "NEO4J_URI": "bolt://localhost:7687",
        "NEO4J_USER": "neo4j",
        "NEO4J_PASSWORD": "your-password"
      }
    }
  }
}
```

When using the global configuration, provide the absolute path to the binary.

## Required Environment Variables

| Variable | Description | Example |
|---|---|---|
| `NEO4J_URI` | Bolt connection URI for Neo4j | `bolt://localhost:7687` |
| `NEO4J_USER` | Neo4j username | `neo4j` |
| `NEO4J_PASSWORD` | Neo4j password | `your-password` |

## Quick Verification

After configuring the MCP server, verify that Claude Code can reach the graph by asking:

> How many programs are in the graph?

Claude should invoke the `get_dashboard_stats` tool and return a summary of the ingested codebase. If the tool call fails, check that Neo4j is running and the credentials are correct.

## Available Tools

### Program Discovery

| Tool | Description |
|---|---|
| `get_program` | Retrieve details for a specific program |
| `search_programs` | Full-text search across programs |
| `list_programs` | List all programs with optional filters |
| `get_program_source` | Retrieve the original COBOL source |

### Call Chain and Impact Analysis

| Tool | Description |
|---|---|
| `get_call_chain` | Trace upstream and downstream call paths |
| `get_impact_analysis` | Determine blast radius of a change |
| `list_bridge_programs` | Find programs that bridge multiple domains |

### Data Analysis

| Tool | Description |
|---|---|
| `get_data_items` | List data items for a program |
| `get_data_hierarchy` | Show nested data structure (01-level through 88-level) |
| `get_data_flow` | Trace MOVES and data transformations within a program |
| `get_type_mappings` | Get COBOL PIC to target-type mappings |
| `get_cross_program_data_flow` | Trace data flow across CALL boundaries |
| `trace_field_impact` | Follow a single field through all transformations |
| `get_shared_data_channels` | Identify shared files, queues, and DB tables between programs |

### Copybooks

| Tool | Description |
|---|---|
| `get_copybook_usage` | Show which programs COPY a given copybook |
| `get_copybook_structure` | Display the data layout inside a copybook |
| `list_copybooks` | List all copybooks in the graph |
| `list_copybook_risks` | Identify high-fan-out or orphaned copybooks |

### Business Domains

| Tool | Description |
|---|---|
| `list_business_domains` | List all classified business domains |
| `get_business_domain` | Get programs and details for a domain |
| `reassign_program_domain` | Move a program to a different domain |

### Program Details

| Tool | Description |
|---|---|
| `get_program_conditions` | List conditional logic (IF/EVALUATE) |
| `get_program_parameters` | Show LINKAGE SECTION parameters |
| `get_program_conditional_logic` | Detailed conditional flow analysis |
| `get_program_error_handlers` | List error handling patterns |
| `get_program_external_interfaces` | Show external system interfaces |
| `get_paragraph_flow` | Trace PERFORM execution order |

### SQL / CICS / IDMS

| Tool | Description |
|---|---|
| `get_program_sql` | List embedded SQL statements |
| `get_program_cics` | List CICS transaction calls |
| `get_idms_records` | Show IDMS record definitions |
| `get_idms_schema` | Display IDMS schema structure |
| `get_idms_impact` | Analyze impact of IDMS schema changes |
| `get_idms_areas` | List IDMS areas and their usage |

### JCL

| Tool | Description |
|---|---|
| `list_jcl_jobs` | List all JCL jobs |
| `get_jcl_job` | Get details for a specific job |
| `get_program_jcl` | Show JCL jobs that execute a program |
| `get_dataset_usage` | Trace dataset reads and writes across jobs |
| `get_file_accessors` | Find all programs that access a given file |

### Database

| Tool | Description |
|---|---|
| `list_db_tables` | List all database tables referenced in COBOL |
| `get_table_usage` | Show which programs access a table |
| `get_program_table_access` | List tables accessed by a program |

### External DB Mappings

| Tool | Description |
|---|---|
| `list_external_db_tables` | List tables in the external target schema |
| `get_external_db_mapping` | Show mapping between COBOL and external schema |
| `get_cobol_to_external_mappings` | List all COBOL-to-external field mappings |
| `get_gap_analysis` | Identify unmapped fields and coverage gaps |
| `get_data_flow_paths` | Trace data from COBOL source to external target |

### Dead Code Detection

| Tool | Description |
|---|---|
| `get_dead_paragraphs` | List paragraphs never reached by any PERFORM |
| `get_dead_code_summary` | Aggregate dead code statistics |

### Modernization Planning

| Tool | Description |
|---|---|
| `list_modernization_candidates` | Rank programs by modernization readiness |
| `list_risk_programs` | Identify high-risk programs |
| `list_volume_estimates` | Estimate lines of code and effort per program |
| `get_migration_sequence` | Suggest an optimal migration order based on dependencies |
| `get_effort_estimates` | Detailed effort breakdown for modernization |

### Reporting and Validation

| Tool | Description |
|---|---|
| `get_dashboard_stats` | High-level graph statistics |
| `get_validation_report` | Data quality and completeness report |

## Custom Commands and Multi-Agent Teams

This directory contains two subdirectories for extending Claude Code with COBOL-specific skills:

- **`commands/`** -- Slash commands for common migration and analysis tasks. Install by copying into `.claude/commands/` in your project root.
- **`teams/`** -- Multi-agent team definitions for coordinated analysis (e.g., a modernization team with structure, data flow, dependency, and business logic specialists).

To install a custom command:

```bash
mkdir -p .claude/commands
cp devtools/claude-code/commands/* .claude/commands/
```

Once installed, commands are available as `/command-name` in Claude Code sessions.
