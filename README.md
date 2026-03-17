# cobol-graph

A Go toolchain that parses COBOL codebases, maps their structure into a Neo4j graph database, and provides an AI-powered chat interface for exploring and modernizing the code. It uses Claude (Opus 4.6 and Sonnet 4.5) as the semantic parser to extract relationships between programs, copybooks, paragraphs, data items, and more — then persists the whole thing as a queryable knowledge graph.

Built for enterprise-scale COBOL monoliths: hundreds of programs, millions of lines of code.

## How It Works

The pipeline runs in five passes over your COBOL source:

**Pass 1 — Structural (Sonnet 4.5):** A fast, cheap sweep that pulls out program IDs, COPY references, CALL targets, paragraph names, file descriptors, and JCL analysis. Runs with 10-20 concurrent workers.

**Pass 2 — Deep Semantic (Opus 4.6):** Full relationship extraction with copybooks inlined and Neo4j context fed back in. The PROCEDURE DIVISION gets chunked at paragraph boundaries to stay within token limits. Runs with 3-5 workers.

**Pass 3 — Cross-Cutting (Opus 4.6):** Business domain classification, dead code detection, and risk analysis derived from graph pattern queries across the full codebase.

**Pass 4 — Cross-Program Data Flow (Opus 4.6):** Detects shared file and DB2 flows from the graph, then analyzes LINKAGE SECTION parameter mapping between caller and callee programs.

**Pass 5 — Validation & Repair:** Runs validation checks against the graph (missing relationships, dangling calls, unannotated paragraphs), then uses LLM-assisted repair to fill gaps. Merges duplicate business domains and re-runs Pass 3 where needed.

## Chat & Modernization Interface

Beyond ingestion, the project includes a web-based chat interface for exploring and modernizing COBOL codebases:

- **Single-turn chat** with tool use — ask questions about programs, call chains, data flows, and the AI queries the graph via MCP tools to answer
- **Multi-agent swarm mode** — four specialist agents (Structure, Data Flow, Dependencies, Business Logic) investigate in parallel, then a coordinator synthesizes findings
- **Session persistence** — conversations are saved to SQLite and can be resumed
- **Dual LLM provider support** — works with Anthropic API directly or via GitHub Copilot (with browser-based OAuth device flow)

## MCP Server

The Neo4j graph is exposed as a Model Context Protocol (MCP) server, making the COBOL knowledge graph available as tools to any MCP-compatible client. Includes 30+ tools for querying programs, call chains, impact analysis, copybook usage, data flows, business domains, and more.

## Graph Model

The resulting Neo4j graph contains nodes like `Program`, `Paragraph`, `Section`, `Copybook`, `DataItem`, `File`, `SQLStatement`, `CICSTransaction`, `JCLJob`, `JCLStep`, and `BusinessDomain`.

Relationships include `CALLS`, `INCLUDES`, `READS`, `WRITES`, `PERFORMS`, `BELONGS_TO`, `CHILD_OF`, `REDEFINES`, `MOVES_TO`, `EXECUTES_SQL`, `EXECUTES_CICS`, and others that reflect actual COBOL program structure.

## Getting Started

### Prerequisites

- Go 1.25+
- Docker and Docker Compose (for Neo4j)
- An Anthropic API key or a GitHub Copilot subscription

### Setup

1. Clone the repo and copy the example environment file:

```bash
git clone git@github.com:MuiGoku123432/cobol-ingestor.git
cd cobol-ingestor
cp .env.example .env
```

2. Fill in your `.env` — at minimum, set your LLM provider and credentials (see Configuration below).

3. Start Neo4j:

```bash
docker compose up -d
```

4. Build the binaries:

```bash
go build -o bin/cobol-graph ./cmd/ingest
go build -o bin/cobol-graph-api ./cmd/server
go build -o bin/cobol-graph-mcp ./cmd/mcp
go build -o bin/cobol-graph-modernize ./cmd/modernize
```

### Usage

Run the ingestion pipeline against a directory of COBOL source:

```bash
./bin/cobol-graph ingest --dir=/path/to/cobol/source
```

Start the API server to query the graph over HTTP:

```bash
./bin/cobol-graph-api
```

Start the MCP server (exposes graph as tools):

```bash
./bin/cobol-graph-mcp
```

Start the modernization chat interface:

```bash
./bin/cobol-graph-modernize
```

The API runs on port 8080 by default. The modernize chat UI runs on port 8081. A health check is available at `GET /health`.

## Configuration

All configuration is done through environment variables or a `.env` file. See `.env.example` for the full list. Key settings:

| Variable | Default | Description |
|---|---|---|
| `LLM_PROVIDER` | `anthropic` | LLM backend: `anthropic` or `copilot` |
| `ANTHROPIC_API_KEY` | — | Your Claude API key (for `anthropic` provider) |
| `COPILOT_GITHUB_TOKEN` | — | GitHub token (optional, can use device flow) |
| `COPILOT_ACCOUNT_TYPE` | `individual` | Copilot account type |
| `NEO4J_URI` | `bolt://localhost:7687` | Neo4j connection URI |
| `NEO4J_USER` | `neo4j` | Neo4j username |
| `NEO4J_PASSWORD` | `changeme` | Neo4j password |
| `CLAUDE_MAX_WORKERS` | `5` | Concurrent Claude API workers |
| `INGEST_TOKEN_LIMIT` | `150000` | Max tokens per Claude request |
| `INGEST_BATCH_SIZE` | `500` | Neo4j batch write size |
| `MODERNIZE_PORT` | `8081` | Port for the modernize chat UI |
| `MODERNIZE_CHAT_MODEL` | `claude-opus-4-6` | Model for chat completions |
| `MODERNIZE_CHAT_MAX_TOKENS` | `16384` | Max tokens for chat responses |
| `MCP_TRANSPORT` | `command` | MCP transport: `command` or `http` |
| `MCP_SERVER_BIN` | `./bin/cobol-graph-mcp` | Path to MCP server binary |

## Project Structure

```
cmd/
  ingest/       CLI entrypoint — runs the scan/analyze/write pipeline
  server/       Gin HTTP API for querying the graph
  mcp/          MCP server exposing Neo4j graph as tools
  modernize/    Chat/modernization web app
internal/
  scanner/      Filesystem walker, file classifier, hashing
  chunker/      Division splitter, copybook inliner, token estimator
  claude/       Claude API client, prompt templates, retry logic
  parser/       JSON response to internal graph model (passes 1-5)
  graph/        Domain model: nodes, relationships, deduplication
  neo4j/        Batch writer, query helpers, schema management, validation
  cache/        SQLite chunk hash cache for incremental runs
  pool/         Worker pool with backpressure
  config/       Viper + .env loading
  pipeline/     Five-pass pipeline orchestrator
  llm/          Multi-provider LLM abstraction (Anthropic + Copilot)
  auth/         GitHub OAuth device flow, token persistence
  mcp/          MCP server implementation and tool registration
  modernize/    Chat handler, swarm agents, session management
web/
  modernize/    Embedded static assets for the chat UI
prompts/        Embedded prompt templates
api/
  handlers/     HTTP handlers (programs, copybooks, search, analysis, dashboard, jobs)
  middleware/   Logging, error handling
```

## Running Tests

```bash
go test ./... -v -race
```
