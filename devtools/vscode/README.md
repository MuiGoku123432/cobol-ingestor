# cobol-graph MCP Server for VSCode + GitHub Copilot

## Overview

The cobol-graph MCP server exposes 55 tools for querying a COBOL knowledge graph stored in Neo4j. With VSCode's native MCP support, GitHub Copilot can call these tools directly from chat to explore program structures, trace dependencies, analyze impact, and plan modernization -- all without leaving the editor.

## Prerequisites

- **VSCode 1.99+** with MCP support enabled
- **GitHub Copilot** extension installed and authenticated
- **Go 1.22+** to build the MCP server binary
- **Neo4j 5.x** running with APOC plugin and ingested COBOL data

## Build

```bash
go build -o bin/cobol-graph-mcp ./cmd/mcp
```

## VSCode MCP Configuration

Add the following to your `.vscode/settings.json`:

```json
{
  "mcp": {
    "servers": {
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
}
```

## Required Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `NEO4J_URI` | Neo4j Bolt connection URI | `bolt://localhost:7687` |
| `NEO4J_USER` | Neo4j username | `neo4j` |
| `NEO4J_PASSWORD` | Neo4j password | (required) |

## Verification

Open Copilot Chat in agent mode and ask:

> How many programs are in the COBOL graph?

Copilot should invoke the `get_dashboard_stats` tool and return program counts from your Neo4j instance.

## Tool Categories

The MCP server provides tools across several categories:

- **Program exploration** -- `get_program`, `search_programs`, `get_program_source`, `list_programs`
- **Call chain analysis** -- `get_call_chain`, `get_impact_analysis`, `get_callers`, `get_callees`
- **Data structures** -- `get_data_items`, `get_data_hierarchy`, `get_type_mappings`
- **Program flow** -- `get_paragraph_flow`, `get_program_conditional_logic`
- **Copybook analysis** -- `get_copybook_usage`, `get_copybook_structure`, `list_copybooks`
- **Cross-program data flow** -- `get_data_flow`, `get_cross_program_data_flow`
- **Database and integration** -- `get_program_sql`, `get_program_cics`, `get_program_external_interfaces`
- **Business domains** -- `list_business_domains`, `get_domain_programs`
- **Modernization** -- `list_modernization_candidates`, `get_effort_estimate`, `get_migration_sequence`
- **Dead code and validation** -- `get_dead_paragraphs`, `get_validation_report`
- **Dashboard** -- `get_dashboard_stats`, `get_risk_assessment`

## Prompt Files

Ready-to-use Copilot prompt files are available in:

- **`prompts/`** -- Individual task prompts (migration, dependency analysis, test generation, etc.)
- **`teams/`** -- Team-oriented workflows and shared prompts

## Team Sharing

To share prompts across your team, copy them into `.github/copilot-prompts/` at the root of your repository. VSCode will automatically discover prompt files in that location and make them available to all team members using Copilot.
