# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**cobol-graph** — A Go CLI pipeline that ingests COBOL monoliths, uses Claude (Opus 4.6 / Sonnet 4.5) as the semantic parser, and persists extracted code relationships into Neo4j. Targets enterprise-scale COBOL codebases (500+ programs, 1M+ LOC).

## Build & Development Commands

```bash
# Build both binaries
go build -o bin/cobol-graph ./cmd/ingest
go build -o bin/cobol-graph-api ./cmd/server

# Run ingestion
./bin/cobol-graph ingest --dir=<path>

# Run API server
./bin/cobol-graph-api

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

### Three-Pass Pipeline

1. **Pass 1 (Structural)** — Sonnet 4.5: fast/cheap skeleton extraction (program IDs, COPY refs, CALL targets, paragraphs, FDs). 10-20 workers.
2. **Pass 2 (Deep Semantic)** — Opus 4.6: full relationship extraction with inlined copybooks and Neo4j context. 3-5 workers. PROCEDURE DIVISION chunked at paragraph boundaries.
3. **Pass 3 (Cross-Cutting)** — Opus 4.6: business domain classification, dead code detection, risk analysis from graph pattern queries.

### Data Flow

Scanner (filesystem walk + classify .cbl/.cpy/.jcl) → Cache Check (SQLite SHA-256, skip unchanged) → Chunker (division split, copybook inline, token estimation) → Worker Pool (bounded goroutines, channel backpressure) → Claude API → Response Parser (JSON → domain model) → Neo4j Batch Writer (UNWIND bulk inserts, nodes then relationships)

### Package Layout (`internal/`)

| Package | Responsibility |
|---------|---------------|
| `scanner` | Filesystem walker, file classifier, hashing |
| `chunker` | Division splitter, copybook inliner, token estimator |
| `claude` | Claude API client, prompt templates, retry/rate-limiting |
| `parser` | JSON response → internal graph model, validation |
| `graph` | Domain model: nodes, relationships, deduplication |
| `neo4j` | Batch writer, query helpers, schema management |
| `cache` | SQLite chunk hash cache for incremental runs |
| `pool` | Worker pool with backpressure (uses `sourcegraph/conc`) |
| `config` | Viper + .env loading |

### Entrypoints

- `cmd/ingest/` — CLI that runs the scan→analyze→write pipeline
- `cmd/server/` — Gin REST API serving graph data + triggering analysis

### API Layer (`api/`)

Gin HTTP framework with Swagger annotations (swaggo). Key endpoints: program listing/detail, call chains, copybook usage, impact analysis, business domain clusters, full-text search.

## Key Dependencies

- `github.com/anthropics/anthropic-sdk-go` — Claude API
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

Via `.env` file (see `plan.md` section 6 for all vars). Key settings: `ANTHROPIC_API_KEY`, `NEO4J_URI`/`NEO4J_USER`/`NEO4J_PASSWORD`, `CLAUDE_MAX_WORKERS`, `INGEST_TOKEN_LIMIT`.

## Infrastructure

Neo4j 5.x Community with APOC plugin, run via Docker Compose. Bolt protocol on port 7687, browser on 7474.
