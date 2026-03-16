---
description: Event-driven decomposition with 4 specialist agents
mode: agent
tools: ["cobol-graph"]
---

# Event-Driven Decomposition Team

Decompose the COBOL monolith into event-driven microservices by identifying domain events from CICS transactions, file operations, and database writes, then building producers, consumers, and sagas that replicate the original system's behavior asynchronously.

## Strategy

COBOL systems process transactions through tightly coupled CALL chains and shared files. Event-driven decomposition replaces these synchronous couplings with domain events, enabling independent scaling, deployment, and evolution of each service. CICS transactions, SQL writes, and JCL job triggers are natural event sources.

## Agent Roles

### 1. Event Modeler

Identify domain events from the COBOL system's integration points and transaction patterns.

**MCP Tools**: `get_program_cics`, `get_program_external_interfaces`, `get_program_sql`, `list_jcl_jobs`, `get_jcl_job`, `get_dataset_usage`, `get_file_accessors`, `list_business_domains`

**Tasks**:
- Catalog CICS transactions as candidate command/event pairs
- Identify SQL INSERT/UPDATE operations that represent state changes (domain events)
- Map JCL job triggers and dataset handoffs as batch event sources
- Analyze file accessors to find programs that produce and consume shared datasets
- Group events by business domain to define event namespaces
- Identify external interfaces that become event boundaries
- Produce an event catalog with schemas, producers, and consumers for each event

### 2. Producer Builder

Build event publishers that emit domain events matching COBOL write operations.

**MCP Tools**: `get_program`, `get_program_source`, `get_paragraph_flow`, `get_data_items`, `get_type_mappings`, `get_program_sql`, `get_program_cics`

**Tasks**:
- Retrieve program source and trace paragraph flow to locate write operations
- Convert SQL INSERT/UPDATE points into event publication calls
- Transform CICS SEND/WRITEQ operations into event emissions
- Map COBOL data items to event payload schemas using type mappings
- Ensure event ordering matches the original paragraph execution sequence
- Handle transactional boundaries: events publish only on successful commit

### 3. Consumer Builder

Build event handlers that process domain events and replicate COBOL read/react logic.

**MCP Tools**: `get_program`, `get_program_source`, `get_paragraph_flow`, `get_program_conditional_logic`, `get_data_items`, `get_type_mappings`, `get_program_error_handlers`

**Tasks**:
- Identify programs that read shared files or respond to CICS triggers as consumer candidates
- Translate COBOL conditional logic into event handler routing rules
- Map incoming event payloads to local data models using type mappings
- Implement error handlers with retry and dead-letter patterns matching COBOL error paths
- Ensure idempotent processing for at-least-once delivery semantics
- Handle paragraph flow that spans multiple event types with stateful consumers

### 4. Saga Coordinator

Design distributed transaction patterns for operations that span multiple services.

**MCP Tools**: `get_call_chain`, `get_cross_program_data_flow`, `get_shared_data_channels`, `get_impact_analysis`, `get_program_table_access`, `get_table_usage`

**Tasks**:
- Trace call chains that span multiple programs to identify saga boundaries
- Map cross-program data flows to saga step sequences
- Identify shared data channels (tables, files) that require coordinated access
- Use impact analysis to determine compensation actions for rollback scenarios
- Design saga orchestration or choreography patterns for each multi-step transaction
- Handle table access patterns that require distributed consistency guarantees
- Produce saga definitions with steps, compensations, and timeout policies

## Workflow

1. **Event Modeler** catalogs all domain events from CICS, SQL, JCL, and file operations, producing an event catalog with schemas
2. **Producer Builder** and **Consumer Builder** work in parallel, building publishers and handlers for each event in the catalog
3. **Saga Coordinator** designs distributed transaction patterns for multi-program call chains
4. Integration testing with event replay: verify event sequences produce identical state to COBOL batch runs
5. Deploy with event sourcing, maintaining an event log for auditability and replay capability

## Handoff Protocol

- Event Modeler outputs the event catalog with schemas consumed by Producer Builder and Consumer Builder
- Producer Builder and Consumer Builder coordinate on event contracts to ensure schema compatibility
- Saga Coordinator consumes call chain analysis and produces saga definitions consumed by both builders
- Integration test failures route back to the specific builder (producer or consumer) with event trace details
- Each event stream is independently deployable once producer-consumer pairs pass integration tests
