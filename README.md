# cobol-ingestor

A Go CLI tool that parses COBOL codebases and maps their structure into a Neo4j graph database. It uses Claude (Opus 4.6 and Sonnet 4.5) as the semantic parser to extract relationships between programs, copybooks, paragraphs, data items, and more — then persists the whole thing as a queryable knowledge graph.

Built for enterprise-scale COBOL monoliths: hundreds of programs, millions of lines of code.

## How It Works

The pipeline runs in three passes over your COBOL source:

**Pass 1 — Structural (Sonnet 4.5):** A fast, cheap sweep that pulls out program IDs, COPY references, CALL targets, paragraph names, and file descriptors. Runs with 10-20 concurrent workers.

**Pass 2 — Deep Semantic (Opus 4.6):** Full relationship extraction with copybooks inlined and Neo4j context fed back in. The PROCEDURE DIVISION gets chunked at paragraph boundaries to stay within token limits. Runs with 3-5 workers.

**Pass 3 — Cross-Cutting (Opus 4.6):** Business domain classification, dead code detection, and risk analysis derived from graph pattern queries across the full codebase.

## Graph Model

The resulting Neo4j graph contains nodes like `Program`, `Paragraph`, `Section`, `Copybook`, `DataItem`, `File`, `SQLStatement`, `CICSTransaction`, `JCLJob`, `JCLStep`, and `BusinessDomain`.

Relationships include `CALLS`, `INCLUDES`, `READS`, `WRITES`, `PERFORMS`, `BELONGS_TO`, `EXECUTES_SQL`, `EXECUTES_CICS`, and others that reflect actual COBOL program structure.

## Getting Started

### Prerequisites

- Go 1.25+
- Docker and Docker Compose (for Neo4j)
- An Anthropic API key

### Setup

1. Clone the repo and copy the example environment file:

```bash
git clone git@github.com:MuiGoku123432/cobol-ingestor.git
cd cobol-ingestor
cp .env.example .env
```

2. Fill in your `.env` — at minimum, set your `ANTHROPIC_API_KEY` and adjust the Neo4j password if needed.

3. Start Neo4j:

```bash
docker compose up -d
```

4. Build the binaries:

```bash
go build -o bin/cobol-ingestor ./cmd/ingest
go build -o bin/cobol-ingestor-api ./cmd/server
```

### Usage

Run the ingestion pipeline against a directory of COBOL source:

```bash
./bin/cobol-ingestor ingest --dir=/path/to/cobol/source
```

Start the API server to query the graph over HTTP:

```bash
./bin/cobol-ingestor-api
```

The API runs on port 8080 by default. A health check is available at `GET /health`.

## Configuration

All configuration is done through environment variables or a `.env` file. See `.env.example` for the full list. Key settings:

| Variable | Default | Description |
|---|---|---|
| `ANTHROPIC_API_KEY` | — | Your Claude API key |
| `NEO4J_URI` | `bolt://localhost:7687` | Neo4j connection URI |
| `NEO4J_USER` | `neo4j` | Neo4j username |
| `NEO4J_PASSWORD` | `changeme` | Neo4j password |
| `CLAUDE_MAX_WORKERS` | `5` | Concurrent Claude API workers |
| `INGEST_TOKEN_LIMIT` | `150000` | Max tokens per Claude request |
| `INGEST_BATCH_SIZE` | `500` | Neo4j batch write size |

## Project Structure

```
cmd/
  ingest/       CLI entrypoint — runs the scan/analyze/write pipeline
  server/       Gin HTTP API for querying the graph
internal/
  scanner/      Filesystem walker, file classifier, hashing
  chunker/      Division splitter, copybook inliner, token estimator
  claude/       Claude API client, prompt templates, retry logic
  parser/       JSON response to internal graph model
  graph/        Domain model: nodes, relationships, deduplication
  neo4j/        Batch writer, query helpers, schema management
  cache/        SQLite chunk hash cache for incremental runs
  pool/         Worker pool with backpressure
  config/       Viper + .env loading
```

## Running Tests

```bash
go test ./... -v -race
```

## License

This project is licensed under the Business Source License 1.1. See [LICENSE](license.md) for details.

After March 5, 2032 the code converts to the Apache License 2.0.
