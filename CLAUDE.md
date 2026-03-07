# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**cobol-graph** — A Go toolchain that ingests COBOL monoliths, uses Claude (Opus 4.6 / Sonnet 4.5) as the semantic parser, persists extracted code relationships into Neo4j, and provides an AI-powered chat interface for exploring and modernizing the codebase. Targets enterprise-scale COBOL codebases (500+ programs, 1M+ LOC).

## Build & Development Commands

```bash
# Build all binaries
go build -o bin/cobol-graph ./cmd/ingest
go build -o bin/cobol-graph-api ./cmd/server
go build -o bin/cobol-graph-mcp ./cmd/mcp
go build -o bin/cobol-graph-modernize ./cmd/modernize

# Run ingestion
./bin/cobol-graph ingest --dir=<path>

# Run API server
./bin/cobol-graph-api

# Run MCP server (graph tools)
./bin/cobol-graph-mcp

# Run modernize chat UI
./bin/cobol-graph-modernize

# Tests
go test ./... -v -race        # all tests
go test ./internal/scanner -v  # single package

# Lint
golangci-lint run ./...

# Swagger docs
swag init -g cmd/server/main.go -o api/docs

# Infrastructure
docker compose up -d           # Neo4j + API
```

## Architecture

### Five-Pass Pipeline

1. **Pass 1 (Structural)** — Sonnet 4.5: fast/cheap skeleton extraction (program IDs, COPY refs, CALL targets, paragraphs, FDs, JCL). 10-20 workers.
2. **Pass 2 (Deep Semantic)** — Opus 4.6: full relationship extraction with inlined copybooks and Neo4j context. 3-5 workers. PROCEDURE DIVISION chunked at paragraph boundaries.
3. **Pass 3 (Cross-Cutting)** — Opus 4.6: business domain classification, dead code detection, risk analysis from graph pattern queries.
4. **Pass 4 (Cross-Program Data Flow)** — Opus 4.6: LINKAGE parameter mapping between caller→callee, shared file/DB2 flow detection (graph-only).
5. **Pass 5 (Validation & Repair)** — Graph queries + LLM repair: fix missing CALLS, CHILD_OF, MOVES_TO, dangling calls, unannotated paragraphs; re-run Pass 3 for gaps; merge duplicate domains.

### Data Flow

Scanner (filesystem walk + classify .cbl/.cpy/.jcl) → Cache Check (SQLite SHA-256, skip unchanged) → Chunker (division split, copybook inline, token estimation) → Worker Pool (bounded goroutines, channel backpressure) → Claude API → Response Parser (JSON → domain model) → Neo4j Batch Writer (UNWIND bulk inserts, nodes then relationships)

### Package Layout (`internal/`)

| Package | Responsibility |
|---------|---------------|
| `scanner` | Filesystem walker, file classifier, hashing |
| `chunker` | Division splitter, copybook inliner, token estimator |
| `claude` | Claude API client, prompt templates, retry/rate-limiting |
| `parser` | JSON response → internal graph model, validation (passes 1-5) |
| `graph` | Domain model: nodes, relationships, deduplication, merge |
| `neo4j` | Batch writer, query helpers, schema management, validation, dead code detection |
| `cache` | SQLite chunk hash cache for incremental runs |
| `pool` | Worker pool with backpressure (uses `sourcegraph/conc`) |
| `config` | Viper + .env loading |
| `pipeline` | Five-pass pipeline orchestrator |
| `llm` | Multi-provider LLM abstraction (Anthropic + GitHub Copilot) |
| `auth` | GitHub OAuth device flow, persistent token store |
| `mcp` | MCP server implementation, 30+ tool registrations |
| `modernize` | Chat handler, multi-agent swarm, session management (SQLite), SSE streaming |

### Entrypoints

- `cmd/ingest/` — CLI that runs the five-pass scan→analyze→write pipeline
- `cmd/server/` — Gin REST API serving graph data + triggering analysis
- `cmd/mcp/` — MCP server exposing Neo4j graph as tools for any MCP client
- `cmd/modernize/` — Chat/modernization web app with swarm mode (port 8081)

### LLM Provider Abstraction (`internal/llm/`)

Pluggable LLM backend supporting:
- **Anthropic** — Direct Claude API via `anthropic-sdk-go`
- **GitHub Copilot** — Claude via Copilot proxy using `ai-provider-kit`, with browser-based OAuth device flow for auth

Both providers support tool-use chat completions with streaming.

### MCP Server (`internal/mcp/`)

Exposes the Neo4j COBOL knowledge graph as Model Context Protocol tools. 30+ tools including: `get_program`, `search_programs`, `get_call_chain`, `get_impact_analysis`, `get_copybook_usage`, `get_data_items`, `list_business_domains`, `get_paragraph_flow`, `list_modernization_candidates`, `get_data_flow`, `get_dead_paragraphs`, `get_dashboard_stats`, and more.

### Modernize Chat (`internal/modernize/`)

- **Chat handler** — Single/multi-turn chat with tool-use loop (max 10 iterations). Queries graph via MCP tools. SSE streaming.
- **Swarm handler** — 4 specialist agents (Structure, Data Flow, Dependencies, Business Logic) investigate in parallel, coordinator synthesizes.
- **Session management** — SQLite-backed persistence with CRUD HTTP handlers.
- **Auth** — Thread-safe provider state with deferred Copilot init via device flow.

### API Layer (`api/`)

Gin HTTP framework with Swagger annotations (swaggo). Handlers: programs, copybooks, search, analysis, dashboard, jobs. Middleware for logging and error handling.

### Web Assets (`web/`)

Embedded static files for the modernize chat UI (HTML + JS), served via Go `embed`.

## Key Dependencies

- `github.com/anthropics/anthropic-sdk-go` — Claude API
- `github.com/cecil-the-coder/ai-provider-kit` — GitHub Copilot LLM proxy
- `github.com/modelcontextprotocol/go-sdk` — MCP server/client
- `github.com/neo4j/neo4j-go-driver/v5` — Neo4j
- `github.com/spf13/cobra` + `viper` — CLI + config
- `github.com/gin-gonic/gin` — HTTP
- `github.com/sourcegraph/conc` — Worker pool
- `golang.org/x/time/rate` — Rate limiting for Claude API
- `modernc.org/sqlite` — Pure Go SQLite (no CGO)
- `go.uber.org/zap` — Structured logging
- `github.com/stretchr/testify` — Testing

## Neo4j Graph Model

Node labels: `Program`, `Paragraph`, `Section`, `Copybook`, `DataItem`, `File`, `SQLStatement`, `CICSTransaction`, `JCLJob`, `JCLStep`, `BusinessDomain`

Key relationships: `CALLS`, `INCLUDES`, `READS`, `WRITES`, `PERFORMS`, `PERFORMS_THRU`, `BELONGS_TO`, `CHILD_OF`, `REDEFINES`, `MOVES_TO`, `RUNS`, `EXECUTES_SQL`, `EXECUTES_CICS`

## Configuration

Via `.env` file (see `.env.example` for all vars). Key settings:

- **LLM**: `LLM_PROVIDER` (`anthropic`|`copilot`), `ANTHROPIC_API_KEY`, `COPILOT_GITHUB_TOKEN`, `COPILOT_ACCOUNT_TYPE`
- **Neo4j**: `NEO4J_URI`, `NEO4J_USER`, `NEO4J_PASSWORD`
- **Pipeline**: `CLAUDE_MAX_WORKERS`, `INGEST_TOKEN_LIMIT`, `INGEST_BATCH_SIZE`
- **Modernize**: `MODERNIZE_PORT`, `MODERNIZE_CHAT_MODEL`, `MODERNIZE_CHAT_MAX_TOKENS`, `MCP_TRANSPORT`, `MCP_SERVER_BIN`

## Infrastructure

Neo4j 5.x Community with APOC plugin, run via Docker Compose. Bolt protocol on port 7687, browser on 7474.
