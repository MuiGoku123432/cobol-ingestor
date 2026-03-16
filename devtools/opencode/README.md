# OpenCode MCP Integration for cobol-graph

## Overview

The cobol-graph MCP server exposes 55 tools for querying a COBOL knowledge graph stored in Neo4j. OpenCode can use these tools via its native MCP support to explore program structures, trace call chains, analyze data flow, and plan modernization efforts across enterprise-scale COBOL codebases.

## Prerequisites

- OpenCode CLI installed and configured
- Go 1.22+ (to build the MCP server binary)
- Neo4j 5.x running with APOC plugin enabled
- COBOL codebase already ingested into Neo4j via `cobol-graph ingest`

## Build the MCP Server

```bash
go build -o bin/cobol-graph-mcp ./cmd/mcp
```

## OpenCode MCP Configuration

Add the following to `opencode.json` in your project root:

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

## Required Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `NEO4J_URI` | Bolt URI for Neo4j instance | `bolt://localhost:7687` |
| `NEO4J_USER` | Neo4j username | `neo4j` |
| `NEO4J_PASSWORD` | Neo4j password | (none) |

## Verification

Start OpenCode and ask a simple question to confirm the MCP server is connected:

> How many programs are in the COBOL graph?

OpenCode should invoke the `get_dashboard_stats` tool and return a count of ingested programs.

## Available Tool Categories

The MCP server provides tools organized into the following categories:

- **Program Discovery** -- search, list, and filter programs by various criteria
- **Call Chain and Impact** -- trace upstream/downstream call relationships and blast radius
- **Data Analysis** -- data items, hierarchies, type mappings, cross-program data flow
- **Copybooks** -- usage tracking, structure analysis, shared definition risks
- **Business Domains** -- domain classification, domain membership, domain-level statistics
- **Program Details** -- source code, paragraph flow, conditional logic, annotations
- **SQL/CICS/IDMS** -- embedded SQL statements, CICS transactions, IDMS database operations
- **JCL** -- job and step analysis, dataset references
- **Database** -- table access patterns, shared data channels
- **External DB Mappings** -- external database interface mappings and analysis
- **Dead Code** -- dead paragraph detection, unreachable code summaries
- **Modernization** -- migration candidates, effort estimates, migration sequencing, risk assessment
- **Reporting** -- dashboard statistics, validation reports, volume estimates

## Related Resources

- `prompts/` -- Agent prompts for common analysis and migration tasks
- `../../prompts/` -- Go-embedded prompt templates used by the ingestion pipeline
