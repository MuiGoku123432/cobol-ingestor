# COBOL Graph Ingestor — Tech Stack & Architecture

**Project:** `cobol-graph` (working name)
**Purpose:** Go-native CLI pipeline that ingests COBOL monoliths, uses Claude (Opus 4.6 / Sonnet 4.5) as the semantic parser, and persists all extracted code relationships into Neo4j.
**Target:** Enterprise-scale COBOL codebases (500+ programs, 1M+ LOC)

---

## 1. Tech Stack

### Core Language & Runtime

| Component | Choice | Notes |
|-----------|--------|-------|
| Language | **Go 1.22+** | Native concurrency, single binary deploy |
| Module path | `github.com/fancherholding/cobol-graph` | Adjust to your org |

### Go Dependencies (`go get`)

```bash
# Neo4j driver (official, bolt protocol)
go get github.com/neo4j/neo4j-go-driver/v5

# Anthropic Claude SDK (Go)
go get github.com/anthropics/anthropic-sdk-go

# CLI framework
go get github.com/spf13/cobra
go get github.com/spf13/viper

# Swagger / OpenAPI (REST API layer)
go get github.com/swaggo/swag/cmd/swag
go get github.com/swaggo/gin-swagger
go get github.com/swaggo/files
go get github.com/gin-gonic/gin

# Logging
go get go.uber.org/zap

# Configuration / env
go get github.com/joho/godotenv

# File hashing (incremental processing)
# stdlib crypto/sha256 — no external dep needed

# Worker pool
go get github.com/sourcegraph/conc

# Rate limiter (for Claude API calls)
go get golang.org/x/time/rate

# UUID generation (node IDs)
go get github.com/google/uuid

# Testing
go get github.com/stretchr/testify
```

### Infrastructure

| Component | Choice | Notes |
|-----------|--------|-------|
| Graph DB | **Neo4j 5.x** (Community or Enterprise) | Docker: `neo4j:5-community` |
| Cache layer | **SQLite** (via `modernc.org/sqlite`) | Local chunk/hash cache, zero-config |
| Containerization | **Docker + Docker Compose** | Neo4j + optional API server |
| CI/CD | GitHub Actions | Lint, test, build |

```bash
# SQLite (pure Go, no CGO)
go get modernc.org/sqlite
```

### AI / LLM

| Component | Choice | Notes |
|-----------|--------|-------|
| Deep analysis | **Claude Opus 4.6** | Semantic relationships, business logic |
| Structural pass | **Claude Sonnet 4.5** | Cheap/fast skeleton extraction |
| Interface | Anthropic API (Go SDK) | Structured JSON output via system prompt |
| Dev workflow | **Claude Code** or **GitHub Copilot Enterprise** | For building the tool itself |

### API & Documentation

| Component | Choice | Notes |
|-----------|--------|-------|
| HTTP framework | **Gin** | Lightweight, fast, middleware ecosystem |
| API docs | **Swag** (swaggo) | Auto-generates OpenAPI 3.0 from Go annotations |
| Spec format | OpenAPI 3.0 / Swagger UI | Served at `/swagger/index.html` |

---

## 2. Project Structure

```
cobol-graph/
├── cmd/
│   ├── ingest/          # CLI entrypoint — scans + analyzes + writes to Neo4j
│   └── server/          # REST API entrypoint — serves graph data + triggers analysis
├── internal/
│   ├── scanner/         # Filesystem walker, file classifier (.cbl, .cpy, .jcl)
│   ├── chunker/         # Division splitter, copybook inliner, token estimator
│   ├── claude/          # Claude API client wrapper, prompt templates, retry logic
│   ├── parser/          # JSON response parser → internal graph model
│   ├── graph/           # Domain model: nodes, relationships, deduplication
│   ├── neo4j/           # Neo4j batch writer, query helpers, schema management
│   ├── cache/           # SQLite chunk hash cache for incremental runs
│   ├── pool/            # Worker pool coordinator with backpressure
│   └── config/          # Viper config loading, env vars
├── api/
│   ├── handlers/        # Gin route handlers
│   ├── middleware/       # Auth, logging, CORS
│   ├── routes.go        # Route registration
│   └── docs/            # Auto-generated swagger docs (swag init)
├── prompts/
│   ├── pass1_structural.tmpl    # Sonnet prompt — skeleton extraction
│   ├── pass2_deep.tmpl          # Opus prompt — full semantic analysis
│   └── pass3_crosscutting.tmpl  # Opus prompt — cross-program analysis
├── migrations/
│   └── neo4j/           # Cypher scripts for constraints/indexes
├── docker-compose.yml
├── Makefile
├── .env.example
└── go.mod
```

---

## 3. Architecture

### High-Level Flow

```
                    ┌──────────────────────────────────────────────────┐
                    │                  CLI / REST API                   │
                    └──────────────┬───────────────────────────────────┘
                                   │
                    ┌──────────────▼───────────────────────────────────┐
                    │              SCANNER                              │
                    │  Walk filesystem, classify files, hash contents   │
                    └──────────────┬───────────────────────────────────┘
                                   │
                    ┌──────────────▼───────────────────────────────────┐
                    │           CACHE CHECK (SQLite)                    │
                    │  Skip unchanged files (SHA-256 match)            │
                    └──────────────┬───────────────────────────────────┘
                                   │
                    ┌──────────────▼───────────────────────────────────┐
                    │             CHUNKER                               │
                    │  Split by division, inline copybooks,            │
                    │  estimate tokens, enforce context limits         │
                    └──────────────┬───────────────────────────────────┘
                                   │
                    ┌──────────────▼───────────────────────────────────┐
                    │          WORKER POOL (bounded goroutines)         │
                    │  ┌─────────┐ ┌─────────┐ ┌─────────┐           │
                    │  │Worker 1 │ │Worker 2 │ │Worker N │           │
                    │  │ Claude  │ │ Claude  │ │ Claude  │           │
                    │  │  API    │ │  API    │ │  API    │           │
                    │  └────┬────┘ └────┬────┘ └────┬────┘           │
                    └───────┼──────────┼──────────┼───────────────────┘
                            │          │          │
                    ┌───────▼──────────▼──────────▼───────────────────┐
                    │          RESPONSE PARSER                         │
                    │  JSON → internal graph model, dedup, validate    │
                    └──────────────┬───────────────────────────────────┘
                                   │
                    ┌──────────────▼───────────────────────────────────┐
                    │        NEO4J BATCH WRITER                        │
                    │  UNWIND-based bulk inserts, 500-1000 per tx     │
                    │  Nodes first, relationships second               │
                    └──────────────┬───────────────────────────────────┘
                                   │
                    ┌──────────────▼───────────────────────────────────┐
                    │              NEO4J                                │
                    │  Persistent graph of all COBOL relationships     │
                    └──────────────────────────────────────────────────┘
```

### Concurrency Model

```
                   File Channel (buffered)
                   ┌──────────────────────┐
  Scanner ────────►│ file1 │ file2 │ ... │
                   └──────┬───────────────┘
                          │
            ┌─────────────┼─────────────┐
            ▼             ▼             ▼
       ┌─────────┐  ┌─────────┐  ┌─────────┐
       │Worker 1 │  │Worker 2 │  │Worker N │  (N = configurable, default 5)
       │ chunk   │  │ chunk   │  │ chunk   │
       │ call AI │  │ call AI │  │ call AI │
       │ parse   │  │ parse   │  │ parse   │
       └────┬────┘  └────┬────┘  └────┬────┘
            │             │             │
            ▼             ▼             ▼
                   Write Channel (buffered, backpressure)
                   ┌──────────────────────┐
                   │ batch1 │ batch2 │...│
                   └──────┬───────────────┘
                          │
                          ▼
                   Neo4j Batch Writer
                   (single goroutine, sequential txns)
```

### Multi-Pass Strategy

```
PASS 1 — STRUCTURAL SKELETON
├── Model: Sonnet 4.5 (cheap, fast)
├── Input: Raw file, no copybook inlining
├── Output: Program ID, COPY refs, CALL targets, paragraph names, FDs, 01-levels
├── Concurrency: 10-20 workers (lightweight calls)
├── Token budget: ~5K in / ~2K out per file
└── Result: Neo4j has full skeleton graph

PASS 2 — DEEP SEMANTIC ANALYSIS
├── Model: Opus 4.6 (expensive, smart)
├── Input: File + inlined copybooks + Neo4j context preamble
├── Output: Full relationship extraction — PERFORM graphs, data flow,
│           conditional logic, SQL, CICS, business logic annotations
├── Concurrency: 3-5 workers (large context, expensive)
├── Token budget: ~100K in / ~10K out per file
├── Chunking: DATA DIVISION always included, PROCEDURE DIVISION chunked at
│             paragraph/section boundaries with overlap
└── Result: Neo4j has rich semantic graph

PASS 3 — CROSS-CUTTING ANALYSIS
├── Model: Opus 4.6
├── Input: Curated graph slices assembled from Neo4j queries
├── Output: Business domain classification, subsystem boundaries,
│           data flow chains, dead code, inconsistencies, risk areas
├── Concurrency: 1-3 workers (highly targeted)
├── Token budget: ~50K in / ~5K out per query
├── Driven by: Graph pattern queries (community detection, PageRank, etc.)
└── Result: Neo4j enriched with high-level semantic layer
```

---

## 4. Neo4j Graph Model

### Node Labels

```
(:Program)          — COBOL program (PROGRAM-ID)
(:Paragraph)        — PROCEDURE DIVISION paragraph
(:Section)          — PROCEDURE DIVISION section
(:Copybook)         — COPY member (.cpy file)
(:DataItem)         — Data item (any level: 01, 05, 88, etc.)
(:File)             — FD / file definition
(:SQLStatement)     — Embedded DB2 SQL (EXEC SQL)
(:CICSTransaction)  — CICS command (EXEC CICS)
(:JCLJob)           — JCL job definition
(:JCLStep)          — JCL step (EXEC PGM=)
(:BusinessDomain)   — Pass 3: identified business domain/subsystem
```

### Relationship Types

```
(:Program)-[:CALLS]->(:Program)
(:Program)-[:INCLUDES]->(:Copybook)
(:Program)-[:READS]->(:File)
(:Program)-[:WRITES]->(:File)
(:Program)-[:EXECUTES_SQL]->(:SQLStatement)
(:Program)-[:EXECUTES_CICS]->(:CICSTransaction)
(:Paragraph)-[:PERFORMS]->(:Paragraph)
(:Paragraph)-[:PERFORMS_THRU]->(:Paragraph)
(:Paragraph)-[:BELONGS_TO]->(:Section)
(:Section)-[:BELONGS_TO]->(:Program)
(:DataItem)-[:CHILD_OF]->(:DataItem)
(:DataItem)-[:REDEFINES]->(:DataItem)
(:DataItem)-[:CONDITION_OF]->(:DataItem)       — 88-levels
(:DataItem)-[:DEFINED_IN]->(:Copybook)
(:DataItem)-[:MOVES_TO]->(:DataItem)           — data flow
(:JCLStep)-[:RUNS]->(:Program)
(:JCLStep)-[:BELONGS_TO]->(:JCLJob)
(:Program)-[:BELONGS_TO]->(:BusinessDomain)    — Pass 3
```

### Indexes & Constraints (migration script)

```cypher
CREATE CONSTRAINT program_id IF NOT EXISTS FOR (p:Program) REQUIRE p.programId IS UNIQUE;
CREATE CONSTRAINT copybook_name IF NOT EXISTS FOR (c:Copybook) REQUIRE c.name IS UNIQUE;
CREATE CONSTRAINT data_item_fqn IF NOT EXISTS FOR (d:DataItem) REQUIRE d.fqn IS UNIQUE;
CREATE INDEX program_file IF NOT EXISTS FOR (p:Program) ON (p.filePath);
CREATE INDEX data_item_level IF NOT EXISTS FOR (d:DataItem) ON (d.level);
CREATE INDEX paragraph_name IF NOT EXISTS FOR (p:Paragraph) ON (p.name);
```

---

## 5. REST API (Swagger)

### Endpoints

```
POST   /api/v1/ingest              — Trigger ingestion of a directory
GET    /api/v1/ingest/{jobId}      — Check ingestion job status
GET    /api/v1/programs             — List all programs
GET    /api/v1/programs/{id}        — Program detail + relationships
GET    /api/v1/programs/{id}/calls  — Call chain (depth-configurable)
GET    /api/v1/programs/{id}/data   — Data item hierarchy
GET    /api/v1/copybooks            — List all copybooks
GET    /api/v1/copybooks/{id}/usage — Programs that include this copybook
GET    /api/v1/search               — Full-text search across graph
GET    /api/v1/impact/{id}          — Impact analysis (what breaks if this changes?)
GET    /api/v1/domains              — Business domain clusters (Pass 3)
GET    /api/v1/stats                — Dashboard stats (program count, relationship count, etc.)
GET    /swagger/index.html          — Swagger UI
```

---

## 6. Configuration (`.env`)

```bash
# Claude
ANTHROPIC_API_KEY=sk-ant-...
CLAUDE_OPUS_MODEL=claude-opus-4-6
CLAUDE_SONNET_MODEL=claude-sonnet-4-5-20250929
CLAUDE_MAX_WORKERS=5
CLAUDE_MAX_RETRIES=3

# Neo4j
NEO4J_URI=bolt://localhost:7687
NEO4J_USER=neo4j
NEO4J_PASSWORD=changeme
NEO4J_DATABASE=cobol

# Ingestor
INGEST_ROOT_DIR=/path/to/cobol/source
INGEST_BATCH_SIZE=500
INGEST_CACHE_DB=./cache.sqlite
INGEST_TOKEN_LIMIT=150000

# API Server
API_PORT=8080
API_LOG_LEVEL=info
```

---

## 7. Docker Compose

```yaml
services:
  neo4j:
    image: neo4j:5-community
    ports:
      - "7474:7474"   # browser
      - "7687:7687"   # bolt
    environment:
      NEO4J_AUTH: neo4j/changeme
      NEO4J_PLUGINS: '["apoc"]'
    volumes:
      - neo4j_data:/data

  cobol-graph-api:
    build: .
    ports:
      - "8080:8080"
    env_file: .env
    depends_on:
      - neo4j

volumes:
  neo4j_data:
```

---

## 8. Overall Execution Plan

### Phase 1 — Foundation (Week 1-2)

1. Initialize Go module, install all deps, set up project structure
2. Build the filesystem scanner (`internal/scanner`) — walk dirs, classify `.cbl`/`.cpy`/`.jcl`, hash files
3. Build the SQLite cache layer (`internal/cache`) — store file hashes, skip unchanged on re-run
4. Build the Neo4j connection + schema migrations (`internal/neo4j`, `migrations/`)
5. Build the config system (`internal/config`) — Viper + `.env` loading
6. Docker Compose for Neo4j + basic Makefile

### Phase 2 — Pass 1 Pipeline (Week 2-3)

1. Build the Claude API client wrapper (`internal/claude`) — structured JSON output, retry + backoff, rate limiting
2. Write the Pass 1 prompt template (`prompts/pass1_structural.tmpl`) — skeleton extraction
3. Build the worker pool (`internal/pool`) — bounded goroutines, channel-based backpressure
4. Build the JSON response parser (`internal/parser`) — validate, map to domain model
5. Build the Neo4j batch writer (`internal/neo4j`) — UNWIND inserts, nodes first then relationships
6. Wire it all together in `cmd/ingest` — CLI that runs Pass 1 end-to-end
7. Test on sample COBOL programs

### Phase 3 — Pass 2 Deep Analysis (Week 3-5)

1. Build the copybook inliner (`internal/chunker`) — resolve COPY statements, inline recursively
2. Build the division splitter + token estimator — split PROCEDURE DIVISION at paragraph boundaries
3. Build the context preamble builder — query Neo4j for caller/callee/copybook context
4. Write the Pass 2 prompt template — full semantic extraction with graph context
5. Build the relationship deduplication logic — merge overlapping results from chunked analysis
6. Add Pass 2 to the CLI pipeline
7. Iterate on prompt quality — test against real COBOL, tune for accuracy

### Phase 4 — REST API (Week 5-6)

1. Set up Gin server with Swagger annotations (`api/`)
2. Implement all endpoints — programs, copybooks, call chains, impact analysis, search
3. Add async ingestion job tracking (POST ingest → job ID → poll status)
4. Generate Swagger docs (`swag init`)
5. Add CORS, logging middleware, basic auth

### Phase 5 — Pass 3 Cross-Cutting + Polish (Week 6-8)

1. Write graph queries that identify interesting patterns (clusters, hubs, orphans)
2. Write the Pass 3 prompt template — business domain identification, inconsistency detection
3. Add graph algorithms via APOC (PageRank, community detection, betweenness centrality)
4. Build the impact analysis endpoint — use graph traversal to find blast radius of changes
5. Add stats/dashboard endpoint
6. Performance tuning — profile Neo4j queries, optimize batch sizes, tune worker counts
7. Documentation — README, API docs, deployment guide

### Phase 6 — Production Hardening (Week 8+)

1. GitHub Actions CI — lint (golangci-lint), test, build, Docker push
2. Structured logging throughout (zap)
3. Prometheus metrics endpoint (optional) — tokens consumed, files processed, errors
4. Error recovery — resume interrupted ingestion from cache checkpoint
5. Multi-dialect support — handle IBM, MicroFocus, GnuCOBOL variations in prompts
6. Integration with Sentinovo platform (longer term)

---

## 9. Cost Estimates (500-Program Monolith)

| Pass | Model | Approx Input | Approx Output | Est. Cost |
|------|-------|-------------|---------------|-----------|
| Pass 1 | Sonnet 4.5 | 2.5M tokens | 1M tokens | ~$15-25 |
| Pass 2 | Opus 4.6 | 50M tokens | 5M tokens | ~$600-900 |
| Pass 3 | Opus 4.6 | 5M tokens | 500K tokens | ~$75-125 |
| **Total** | | | | **~$700-1050** |

Incremental re-runs (only changed files): **5-10% of initial cost.**

---

## 10. Dev Workflow

Build the tool itself using **Claude Code** (terminal agent) or **GitHub Copilot Enterprise** (IDE agent). The workflow:

1. Use Claude Code to scaffold each `internal/` package from this spec
2. Iterate on each package — write code, run `go build`, fix issues in the same session
3. Test against sample COBOL files (grab open-source samples from GitHub)
4. Use the `prompts/*.tmpl` files as living documents — refine after seeing real Claude output
5. Use `make run` / `make test` / `make swagger` as the tight inner dev loop

```makefile
.PHONY: build run test swagger docker

build:
	go build -o bin/cobol-graph ./cmd/ingest
	go build -o bin/cobol-graph-api ./cmd/server

run-ingest:
	./bin/cobol-graph ingest --dir=$(DIR)

run-api:
	./bin/cobol-graph-api

test:
	go test ./... -v -race

swagger:
	swag init -g cmd/server/main.go -o api/docs

docker:
	docker compose up -d

lint:
	golangci-lint run ./...
```
